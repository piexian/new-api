package minimax

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type MiniMaxTTSRequest struct {
	Model             string             `json:"model"`
	Text              string             `json:"text"`
	Stream            bool               `json:"stream,omitempty"`
	StreamOptions     *StreamOptions     `json:"stream_options,omitempty"`
	VoiceSetting      VoiceSetting       `json:"voice_setting"`
	PronunciationDict *PronunciationDict `json:"pronunciation_dict,omitempty"`
	AudioSetting      *AudioSetting      `json:"audio_setting,omitempty"`
	TimbreWeights     []TimbreWeight     `json:"timbre_weights,omitempty"`
	LanguageBoost     string             `json:"language_boost,omitempty"`
	VoiceModify       *VoiceModify       `json:"voice_modify,omitempty"`
	SubtitleEnable    bool               `json:"subtitle_enable,omitempty"`
	OutputFormat      string             `json:"output_format,omitempty"`
	AigcWatermark     bool               `json:"aigc_watermark,omitempty"`
}

type StreamOptions struct {
	ExcludeAggregatedAudio bool `json:"exclude_aggregated_audio,omitempty"`
}

type VoiceSetting struct {
	VoiceID           string  `json:"voice_id"`
	Speed             float64 `json:"speed,omitempty"`
	Vol               float64 `json:"vol,omitempty"`
	Pitch             int     `json:"pitch,omitempty"`
	Emotion           string  `json:"emotion,omitempty"`
	TextNormalization bool    `json:"text_normalization,omitempty"`
	LatexRead         bool    `json:"latex_read,omitempty"`
}

type PronunciationDict struct {
	Tone []string `json:"tone,omitempty"`
}

type AudioSetting struct {
	SampleRate int    `json:"sample_rate,omitempty"`
	Bitrate    int    `json:"bitrate,omitempty"`
	Format     string `json:"format,omitempty"`
	Channel    int    `json:"channel,omitempty"`
	ForceCbr   bool   `json:"force_cbr,omitempty"`
}

type TimbreWeight struct {
	VoiceID string `json:"voice_id"`
	Weight  int    `json:"weight"`
}

type VoiceModify struct {
	Pitch        int    `json:"pitch,omitempty"`
	Intensity    int    `json:"intensity,omitempty"`
	Timbre       int    `json:"timbre,omitempty"`
	SoundEffects string `json:"sound_effects,omitempty"`
}

type MiniMaxTTSResponse struct {
	Data      MiniMaxTTSData   `json:"data"`
	ExtraInfo MiniMaxExtraInfo `json:"extra_info"`
	TraceID   string           `json:"trace_id"`
	BaseResp  MiniMaxBaseResp  `json:"base_resp"`
}

type MiniMaxTTSData struct {
	Audio  string `json:"audio"`
	Status int    `json:"status"`
}

type MiniMaxExtraInfo struct {
	UsageCharacters int64 `json:"usage_characters"`
}

type MiniMaxBaseResp struct {
	StatusCode int64  `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

func getContentTypeByFormat(format string) string {
	contentTypeMap := map[string]string{
		"mp3":  "audio/mpeg",
		"wav":  "audio/wav",
		"flac": "audio/flac",
		"aac":  "audio/aac",
		"pcm":  "audio/pcm",
	}
	if ct, ok := contentTypeMap[format]; ok {
		return ct
	}
	return "audio/mpeg" // default to mp3
}

func handleTTSResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to read minimax response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}
	defer resp.Body.Close()

	// Parse response
	var minimaxResp MiniMaxTTSResponse
	if unmarshalErr := json.Unmarshal(body, &minimaxResp); unmarshalErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to unmarshal minimax TTS response: %w", unmarshalErr),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	// Check base_resp status code
	if minimaxResp.BaseResp.StatusCode != 0 {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("minimax TTS error: %d - %s", minimaxResp.BaseResp.StatusCode, minimaxResp.BaseResp.StatusMsg),
			types.ErrorCodeBadResponse,
			http.StatusBadRequest,
		)
	}

	// Check if we have audio data
	if minimaxResp.Data.Audio == "" {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("no audio data in minimax TTS response"),
			types.ErrorCodeBadResponse,
			http.StatusBadRequest,
		)
	}

	if strings.HasPrefix(minimaxResp.Data.Audio, "http") {
		c.Redirect(http.StatusFound, minimaxResp.Data.Audio)
	} else {
		// Handle hex-encoded audio data
		audioData, decodeErr := hex.DecodeString(minimaxResp.Data.Audio)
		if decodeErr != nil {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("failed to decode hex audio data: %w", decodeErr),
				types.ErrorCodeBadResponse,
				http.StatusInternalServerError,
			)
		}

		// Determine content type - default to mp3
		contentType := "audio/mpeg"

		c.Data(http.StatusOK, contentType, audioData)
	}

	usage = &dto.Usage{
		PromptTokens:     info.GetEstimatePromptTokens(),
		CompletionTokens: 0,
		TotalTokens:      int(minimaxResp.ExtraInfo.UsageCharacters),
	}

	return usage, nil
}

func handleChatCompletionResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("invalid minimax response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	if strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		adaptor := openai.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("failed to read minimax response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}
	service.CloseResponseBodyGracefully(resp)
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, types.NewOpenAIError(
			errors.New("minimax returned empty response body"),
			types.ErrorCodeEmptyResponse,
			http.StatusInternalServerError,
		)
	}

	var minimaxResp struct {
		BaseResp MiniMaxBaseResp `json:"base_resp"`
	}
	if unmarshalErr := common.Unmarshal(body, &minimaxResp); unmarshalErr == nil && minimaxResp.BaseResp.StatusCode != 0 {
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: miniMaxStatusMessage(minimaxResp.BaseResp.StatusCode, minimaxResp.BaseResp.StatusMsg),
			Type:    "minimax_error",
			Code:    fmt.Sprintf("%d", minimaxResp.BaseResp.StatusCode),
		}, miniMaxHTTPStatusCode(minimaxResp.BaseResp.StatusCode, resp.StatusCode))
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))
	usage, err = openai.OpenaiHandler(c, info, resp)
	if err != nil && err.StatusCode == http.StatusOK {
		if statusCode := miniMaxHTTPStatusFromAny(err.ToOpenAIError().Code, resp.StatusCode); statusCode != http.StatusOK {
			err.StatusCode = statusCode
		}
	}
	return usage, err
}

func miniMaxStatusMessage(statusCode int64, statusMsg string) string {
	if statusMsg != "" {
		return statusMsg
	}
	return fmt.Sprintf("minimax error: %d", statusCode)
}

func miniMaxHTTPStatusCode(statusCode int64, fallback int) int {
	if statusCode >= 100 && statusCode <= 599 {
		return int(statusCode)
	}
	switch statusCode {
	case 1000, 1024:
		return http.StatusInternalServerError
	case 1001:
		return http.StatusGatewayTimeout
	case 1002, 1041, 2045, 2056:
		return http.StatusTooManyRequests
	case 1004, 2049:
		return http.StatusUnauthorized
	case 1008:
		return http.StatusPaymentRequired
	case 1026, 1027, 1039, 1042, 1043, 1044, 2013, 20132, 2037, 2039, 2048:
		return http.StatusBadRequest
	case 1033:
		return http.StatusBadGateway
	case 2038, 2042:
		return http.StatusForbidden
	}
	if fallback >= 100 && fallback <= 599 && fallback != http.StatusOK {
		return fallback
	}
	return http.StatusBadRequest
}

func miniMaxHTTPStatusFromAny(code any, fallback int) int {
	switch v := code.(type) {
	case int:
		return miniMaxHTTPStatusCode(int64(v), fallback)
	case int32:
		return miniMaxHTTPStatusCode(int64(v), fallback)
	case int64:
		return miniMaxHTTPStatusCode(v, fallback)
	case float64:
		return miniMaxHTTPStatusCode(int64(v), fallback)
	case string:
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return miniMaxHTTPStatusCode(parsed, fallback)
		}
	}
	return fallback
}
