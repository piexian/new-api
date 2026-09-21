package xiaomimimo

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// MiMo ASR(mimo-v2.5-asr) 复用 /v1/chat/completions, 音频经 input_audio 内容块传入, 与官方文档对齐
type MiMoASRRequest struct {
	Model      string           `json:"model"`
	Messages   []MiMoASRMessage `json:"messages"`
	ASROptions *MiMoASROptions  `json:"asr_options,omitempty"`
	Stream     bool             `json:"stream,omitempty"`
}

type MiMoASRMessage struct {
	Role    string           `json:"role"`
	Content []MiMoASRContent `json:"content"`
}

type MiMoASRContent struct {
	Type       string            `json:"type"`
	InputAudio MiMoASRInputAudio `json:"input_audio"`
}

type MiMoASRInputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format,omitempty"`
}

type MiMoASROptions struct {
	Language string `json:"language,omitempty"`
}

// MiMoASRChatResponse 是 ASR 调用的 chat.completion 响应(仅取转录文本与用量)
type MiMoASRChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		Seconds          int `json:"seconds"`
	} `json:"usage"`
}

var mimoASRMimeFormats = map[string]string{
	"audio/mpeg":  "mp3",
	"audio/mp3":   "mp3",
	"audio/wav":   "wav",
	"audio/x-wav": "wav",
	"audio/wave":  "wav",
}

// mimoASRAudioFormat 依次从 Content-Type 与文件扩展名推断上游要求的 mp3/wav 格式
func mimoASRAudioFormat(fileHeader *multipart.FileHeader) (format, mimeType string) {
	mimeType = strings.ToLower(strings.TrimSpace(fileHeader.Header.Get("Content-Type")))
	if f, ok := mimoASRMimeFormats[mimeType]; ok {
		return f, mimeType
	}
	switch strings.ToLower(filepath.Ext(fileHeader.Filename)) {
	case ".mp3":
		return "mp3", "audio/mpeg"
	case ".wav":
		return "wav", "audio/wav"
	}
	return "", mimeType
}

// convertOpenAISTTToMiMo 将 OpenAI /v1/audio/transcriptions 请求(multipart)转为 MiMo ASR chat 请求
func convertOpenAISTTToMiMo(c *gin.Context, request dto.AudioRequest, model string) (io.Reader, error) {
	formData, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return nil, fmt.Errorf("error parsing multipart form: %w", err)
	}
	fileHeaders := formData.File["file"]
	if len(fileHeaders) == 0 {
		return nil, errors.New("file is required")
	}
	fileHeader := fileHeaders[0]

	format, mimeType := mimoASRAudioFormat(fileHeader)
	if format == "" {
		return nil, fmt.Errorf("unsupported audio format %q: mimo asr only accepts mp3/wav", mimeType)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening audio file: %w", err)
	}
	defer file.Close()
	audioData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading audio file: %w", err)
	}

	language := "auto"
	if vals := formData.Value["language"]; len(vals) > 0 && (vals[0] == "zh" || vals[0] == "en") {
		language = vals[0]
	}
	if model == "" {
		model = request.Model
	}

	mimoReq := MiMoASRRequest{
		Model: model,
		Messages: []MiMoASRMessage{{
			Role: "user",
			Content: []MiMoASRContent{{
				Type: "input_audio",
				InputAudio: MiMoASRInputAudio{
					Data:   "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(audioData),
					Format: format,
				},
			}},
		}},
		ASROptions: &MiMoASROptions{Language: language},
	}

	jsonData, err := common.Marshal(mimoReq)
	if err != nil {
		return nil, fmt.Errorf("error marshalling mimo asr request: %w", err)
	}
	// 请求从 multipart 换成 JSON, 必须同步覆盖 Content-Type
	c.Request.Header.Set("Content-Type", "application/json")
	return bytes.NewReader(jsonData), nil
}

// handleASRResponse 将 MiMo ASR chat 响应转为 OpenAI transcription JSON({text}), 并提取 token 用量计费
func handleASRResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, newAPIError *types.NewAPIError) {
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("failed to read xiaomi mimo asr response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}
	defer resp.Body.Close()

	var mimoResp MiMoASRChatResponse
	if unmarshalErr := common.Unmarshal(body, &mimoResp); unmarshalErr != nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("failed to unmarshal xiaomi mimo asr response: %w", unmarshalErr),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}
	if len(mimoResp.Choices) == 0 {
		return nil, types.NewOpenAIError(
			errors.New("xiaomi mimo asr response has no choices"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	c.JSON(http.StatusOK, gin.H{"text": mimoResp.Choices[0].Message.Content})
	return &dto.Usage{
		PromptTokens:     mimoResp.Usage.PromptTokens,
		CompletionTokens: mimoResp.Usage.CompletionTokens,
		TotalTokens:      mimoResp.Usage.TotalTokens,
	}, nil
}
