package mistral

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// mistralSpeechRequest 对应上游 POST /v1/audio/speech 的 JSON 结构。
type mistralSpeechRequest struct {
	Model          string `json:"model,omitempty"`
	Input          string `json:"input"`
	VoiceID        string `json:"voice_id,omitempty"`
	RefAudio       string `json:"ref_audio,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	Stream         bool   `json:"stream"`
}

// convertSpeechRequest 把 OpenAI 风格的 TTS 请求转换为 Mistral /v1/audio/speech 请求。
// 客户端的 stream_format 只影响下游协议，上游始终用非流式（一次性返回完整音频 base64）。
func convertSpeechRequest(request *dto.AudioRequest) (io.Reader, error) {
	// 上游只支持这几种格式，其余（如 aac）直接省略，走上游默认 mp3
	format := ""
	switch strings.ToLower(request.ResponseFormat) {
	case "pcm", "wav", "mp3", "flac", "opus":
		format = strings.ToLower(request.ResponseFormat)
	}
	req := mistralSpeechRequest{
		Model:          request.Model,
		Input:          request.Input,
		VoiceID:        request.Voice,
		RefAudio:       common.JsonRawMessageToString(request.RefAudio),
		ResponseFormat: format,
		Stream:         false,
	}
	data, err := common.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshalling mistral speech request: %w", err)
	}
	return bytes.NewReader(data), nil
}

// convertTranscriptionRequest 原样透传 multipart 表单（file/file_url/file_id、diarize、
// context_bias、timestamp_granularities 等字段原样带过），仅重写 model 为映射后的模型名。
func convertTranscriptionRequest(c *gin.Context, request *dto.AudioRequest) (io.Reader, error) {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	_ = writer.WriteField("model", request.Model)

	formData, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return nil, fmt.Errorf("error parsing multipart form: %w", err)
	}

	for key, values := range formData.Value {
		if key == "model" {
			continue
		}
		for _, value := range values {
			_ = writer.WriteField(key, value)
		}
	}

	fileHeaders := formData.File["file"]
	if len(fileHeaders) == 0 {
		return nil, errors.New("file is required")
	}
	fileHeader := fileHeaders[0]
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening audio file: %w", err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", fileHeader.Filename)
	if err != nil {
		return nil, errors.New("create form file failed")
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, errors.New("copy file failed")
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return &requestBody, nil
}

// mistralSpeechContentType 按响应格式推导回写给客户端的 Content-Type。
func mistralSpeechContentType(format string) string {
	switch format {
	case "wav":
		return "audio/wav"
	case "flac":
		return "audio/flac"
	case "opus":
		return "audio/ogg"
	case "pcm":
		return "audio/L16"
	default:
		return "audio/mpeg"
	}
}

// MistralTTSHandler 处理 /v1/audio/speech 非流式响应：上游返回
// {"audio_data": "<base64>"}，需解码为二进制音频回写，用量按音频时长估算，
// 口径与 OpenAI TTS 一致（每分钟 1000 tokens）。
func MistralTTSHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	usage := &dto.Usage{}
	usage.PromptTokens = info.GetEstimatePromptTokens()
	usage.TotalTokens = info.GetEstimatePromptTokens()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	var speechResp struct {
		AudioData string `json:"audio_data"`
	}
	if err := common.Unmarshal(body, &speechResp); err != nil || speechResp.AudioData == "" {
		return nil, types.NewOpenAIError(
			fmt.Errorf("invalid mistral speech response: %w", err),
			types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	audioBytes, err := base64.StdEncoding.DecodeString(speechResp.AudioData)
	if err != nil {
		return nil, types.NewOpenAIError(fmt.Errorf("decode mistral audio_data failed: %w", err),
			types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	audioFormat := "mp3"
	if audioReq, ok := info.Request.(*dto.AudioRequest); ok && audioReq.ResponseFormat != "" {
		audioFormat = strings.ToLower(audioReq.ResponseFormat)
	}

	c.Writer.Header().Set("Content-Type", mistralSpeechContentType(audioFormat))
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := c.Writer.Write(audioBytes); err != nil {
		logger.LogError(c, fmt.Sprintf("failed to write mistral TTS response: %v", err))
	}

	common.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)

	var duration float64
	var durationErr error
	if audioFormat == "pcm" {
		// PCM 无文件头，按 24kHz/16bit/单声道估算时长
		duration = float64(len(audioBytes)) / float64(24000*2*1)
	} else {
		duration, durationErr = common.GetAudioDuration(c.Request.Context(), bytes.NewReader(audioBytes), "."+audioFormat)
	}

	usage.PromptTokensDetails.TextTokens = usage.PromptTokens
	if durationErr != nil {
		logger.LogWarn(c, fmt.Sprintf("failed to get audio duration: %v", durationErr))
		// 无法解析时长时按 body 大小保底估算
		estimatedTokens := int(math.Ceil(float64(len(audioBytes)) / 1000.0))
		usage.CompletionTokens = estimatedTokens
		usage.CompletionTokenDetails.AudioTokens = estimatedTokens
	} else if duration > 0 {
		completionTokens := common.QuotaRound(math.Ceil(duration) / 60.0 * 1000)
		usage.CompletionTokens = completionTokens
		usage.CompletionTokenDetails.AudioTokens = completionTokens
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage, nil
}
