package deepseek

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// 工具 API 桥(2026-11-13 官方全家桶):/v1/tools/{tts,asr,search,fetch}。
// TTS/ASR 经 OpenAI 语音入站转发;search/fetch 由 relay.DeepSeekToolsHelper 直通。

const (
	deepSeekTTSDefaultVoice = "stella"
)

type deepSeekTTSRequest struct {
	Text           string `json:"text"`
	VoiceID        string `json:"voice_id,omitempty"`
	Format         string `json:"format,omitempty"`
	FilterMarkdown *bool  `json:"filter_markdown,omitempty"`
}

type deepSeekASRResponse struct {
	Text       string `json:"text"`
	DurationMs int64  `json:"duration_ms"`
}

// isDeepSeekTTSModel / isDeepSeekASRModel 判定语音工具模型
func isDeepSeekTTSModel(model string) bool {
	return strings.HasPrefix(model, "deepseek-tts")
}

func isDeepSeekASRModel(model string) bool {
	return strings.HasPrefix(model, "deepseek-asr")
}

// convertDeepSeekAudioRequest 把 OpenAI 语音请求转换为工具 API 请求
func convertDeepSeekAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	switch info.RelayMode {
	case relayconstant.RelayModeAudioSpeech:
		if !isDeepSeekTTSModel(info.UpstreamModelName) {
			return nil, fmt.Errorf("deepseek channel only supports deepseek-tts for speech generation")
		}
		// 官方仅支持 wav/pcm;其余格式一律回落 wav
		format := "wav"
		switch strings.ToLower(request.ResponseFormat) {
		case "wav", "pcm":
			format = strings.ToLower(request.ResponseFormat)
		}
		ttsRequest := &deepSeekTTSRequest{
			Text:   request.Input,
			Format: format,
		}
		if voice := strings.TrimSpace(request.Voice); voice != "" {
			ttsRequest.VoiceID = voice
		} else {
			ttsRequest.VoiceID = deepSeekTTSDefaultVoice
		}
		jsonData, err := common.Marshal(ttsRequest)
		if err != nil {
			return nil, err
		}
		c.Set("deepseek_tts_format", format)
		return bytes.NewReader(jsonData), nil
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		if !isDeepSeekASRModel(info.UpstreamModelName) {
			return nil, fmt.Errorf("deepseek channel only supports deepseek-asr for transcription")
		}
		// 上游只要 file 字段,重建 multipart 转发
		formData, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, fmt.Errorf("error parsing multipart form: %w", err)
		}
		fileHeaders := formData.File["file"]
		if len(fileHeaders) == 0 {
			return nil, errors.New("file is required")
		}
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)
		file, err := fileHeaders[0].Open()
		if err != nil {
			return nil, fmt.Errorf("error opening uploaded file: %w", err)
		}
		part, err := writer.CreateFormFile("file", fileHeaders[0].Filename)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("error creating form file: %w", err)
		}
		if _, err = io.Copy(part, file); err != nil {
			file.Close()
			return nil, fmt.Errorf("error copying file: %w", err)
		}
		file.Close()
		if err = writer.Close(); err != nil {
			return nil, err
		}
		// 供上游请求头设置正确的 multipart boundary
		c.Set("deepseek_asr_content_type", writer.FormDataContentType())
		return bytes.NewReader(requestBody.Bytes()), nil
	default:
		return nil, errors.New("unsupported audio relay mode")
	}
}

// DeepSeekToolsUpstreamPath 校验 /v1/tools/{search,fetch} 路径与模型匹配并返回上游路径
func DeepSeekToolsUpstreamPath(path string, model string) (string, error) {
	switch {
	case strings.HasSuffix(path, "/tools/search"):
		if model != "deepseek-search" {
			return "", errors.New("the /v1/tools/search endpoint requires model \"deepseek-search\"")
		}
		return "/v1/tools/search", nil
	case strings.HasSuffix(path, "/tools/fetch"):
		if model != "deepseek-fetch" {
			return "", errors.New("the /v1/tools/fetch endpoint requires model \"deepseek-fetch\"")
		}
		return "/v1/tools/fetch", nil
	default:
		return "", errors.New("unsupported deepseek tools endpoint")
	}
}

// doDeepSeekTTSResponse 流式回传音频(wav/pcm),计费按字符数
func doDeepSeekTTSResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	contentType := "audio/wav"
	if format := c.GetString("deepseek_tts_format"); format == "pcm" {
		contentType = "application/octet-stream"
	}
	c.Writer.Header().Set("Content-Type", contentType)
	c.Writer.WriteHeader(http.StatusOK)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponse)
	}

	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens <= 0 {
		promptTokens = 1
	}
	return &dto.Usage{PromptTokens: promptTokens, TotalTokens: promptTokens}, nil
}

// doDeepSeekASRResponse 透传上游 {text,duration_ms},按音频秒数计费(1 秒=1 计费 token)
func doDeepSeekASRResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	var asrResp deepSeekASRResponse
	if unmarshalErr := common.Unmarshal(responseBody, &asrResp); unmarshalErr != nil {
		// 解析失败仍透传原文,计费走字符估算兜底
		service.IOCopyBytesGracefully(c, resp, responseBody)
		promptTokens := info.GetEstimatePromptTokens()
		if promptTokens <= 0 {
			promptTokens = 1
		}
		return &dto.Usage{PromptTokens: promptTokens, TotalTokens: promptTokens}, nil
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)

	seconds := (asrResp.DurationMs + 999) / 1000
	if seconds <= 0 {
		seconds = 1
	}
	return &dto.Usage{PromptTokens: int(seconds), TotalTokens: int(seconds)}, nil
}
