package xiaomimimo

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// MiMoChatRequest is the request format for MiMo's /v1/chat/completions endpoint.
type MiMoChatRequest struct {
	Model    string        `json:"model"`
	Messages []MiMoMessage `json:"messages"`
	Audio    *MiMoAudio    `json:"audio,omitempty"`
	Stream   bool          `json:"stream,omitempty"`
}

type MiMoMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type MiMoAudio struct {
	Format              string `json:"format,omitempty"`
	Voice               string `json:"voice,omitempty"`
	OptimizeTextPreview *bool  `json:"optimize_text_preview,omitempty"`
}

// MiMoChatResponse is the non-streaming response from MiMo's /v1/chat/completions.
type MiMoChatResponse struct {
	Choices []MiMoChoice `json:"choices"`
}

type MiMoChoice struct {
	Message MiMoRespMessage `json:"message"`
}

type MiMoRespMessage struct {
	Audio json.RawMessage `json:"audio,omitempty"`
}

type MiMoAudioData struct {
	Data string `json:"data"`
}

// MiMoStreamResponse is a single SSE chunk from MiMo's streaming response.
type MiMoStreamResponse struct {
	Choices []MiMoStreamChoice `json:"choices"`
}

type MiMoStreamChoice struct {
	Delta MiMoStreamDelta `json:"delta"`
}

type MiMoStreamDelta struct {
	Audio json.RawMessage `json:"audio,omitempty"`
}

// ConvertOpenAITTSToMiMo converts an OpenAI-format AudioRequest to MiMo's chat completions format.
func ConvertOpenAITTSToMiMo(request dto.AudioRequest, model string, stream bool) MiMoChatRequest {
	mimoReq := MiMoChatRequest{
		Model: model,
		Messages: []MiMoMessage{
			{
				Role:    "assistant",
				Content: request.Input,
			},
		},
		Audio: &MiMoAudio{
			Format: request.ResponseFormat,
			Voice:  request.Voice,
		},
		Stream: stream,
	}
	// Normalize format: ensure it's set
	if mimoReq.Audio.Format == "" {
		mimoReq.Audio.Format = "wav"
	}
	// Normalize voice: ensure it's set
	if mimoReq.Audio.Voice == "" {
		mimoReq.Audio.Voice = "mimo_default"
	}
	return mimoReq
}

func getContentTypeByFormat(format string) string {
	switch strings.ToLower(format) {
	case "wav":
		return "audio/wav"
	case "pcm", "pcm16":
		return "audio/pcm"
	case "mp3":
		return "audio/mpeg"
	default:
		return "audio/wav"
	}
}

// handleTTSResponse decodes MiMo's chat completions response and returns raw audio bytes.
func handleTTSResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("failed to read xiaomi mimo response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}
	defer resp.Body.Close()

	// Parse MiMo chat response
	var mimoResp MiMoChatResponse
	if unmarshalErr := common.Unmarshal(body, &mimoResp); unmarshalErr != nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("failed to unmarshal xiaomi mimo response: %w", unmarshalErr),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	// Extract audio data
	if len(mimoResp.Choices) == 0 {
		return nil, types.NewOpenAIError(
			fmt.Errorf("no choices in xiaomi mimo response"),
			types.ErrorCodeBadResponse,
			http.StatusBadRequest,
		)
	}

	if len(mimoResp.Choices[0].Message.Audio) == 0 {
		return nil, types.NewOpenAIError(
			fmt.Errorf("no audio data in xiaomi mimo response"),
			types.ErrorCodeBadResponse,
			http.StatusBadRequest,
		)
	}

	var audioData MiMoAudioData
	if unmarshalErr := common.Unmarshal(mimoResp.Choices[0].Message.Audio, &audioData); unmarshalErr != nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("failed to unmarshal audio data: %w", unmarshalErr),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	if audioData.Data == "" {
		return nil, types.NewOpenAIError(
			fmt.Errorf("empty audio data in xiaomi mimo response"),
			types.ErrorCodeBadResponse,
			http.StatusBadRequest,
		)
	}

	// Decode base64 audio
	audioBytes, decodeErr := base64.StdEncoding.DecodeString(audioData.Data)
	if decodeErr != nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("failed to decode base64 audio: %w", decodeErr),
			types.ErrorCodeBadResponse,
			http.StatusInternalServerError,
		)
	}

	// Determine content type from the format that was set during ConvertAudioRequest
	responseFormat := c.GetString("response_format")
	if responseFormat == "" {
		responseFormat = "wav"
	}
	contentType := getContentTypeByFormat(responseFormat)

	c.Data(http.StatusOK, contentType, audioBytes)

	promptTokens := info.GetEstimatePromptTokens()
	usage = &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: 0,
		TotalTokens:      promptTokens,
	}

	return usage, nil
}

// handleStreamTTSResponse handles SSE streaming with base64 PCM16 audio chunks.
func handleStreamTTSResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	c.Writer.Header().Set("Content-Type", "audio/pcm")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, types.NewOpenAIError(
			fmt.Errorf("streaming not supported"),
			types.ErrorCodeBadResponse,
			http.StatusInternalServerError,
		)
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("failed to read stream response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}

	// MiMo V2/V2.5 streaming is in "compatibility mode" — it returns a single SSE event
	// with all the audio data at once. Parse SSE lines to extract audio.
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var streamResp MiMoStreamResponse
		if unmarshalErr := common.Unmarshal([]byte(data), &streamResp); unmarshalErr != nil {
			continue
		}

		for _, choice := range streamResp.Choices {
			if len(choice.Delta.Audio) == 0 {
				continue
			}

			var audioData MiMoAudioData
			if unmarshalErr := common.Unmarshal(choice.Delta.Audio, &audioData); unmarshalErr != nil {
				continue
			}

			if audioData.Data != "" {
				// Decode base64 PCM16 chunk
				pcmBytes, decodeErr := base64.StdEncoding.DecodeString(audioData.Data)
				if decodeErr != nil {
					continue
				}
				if _, writeErr := c.Writer.Write(pcmBytes); writeErr != nil {
					break
				}
				flusher.Flush()
			}
		}
	}

	promptTokens := info.GetEstimatePromptTokens()
	usage = &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: 0,
		TotalTokens:      promptTokens,
	}

	return usage, nil
}
