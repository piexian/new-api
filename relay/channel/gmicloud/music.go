package gmicloud

import (
	"bytes"
	"encoding/hex"
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

const maxMiniMaxMusicDownloadBytes = 32 << 20

// buildMiniMaxMusicRequestBody converts the MiniMax native request to GMI requestqueue.
func buildMiniMaxMusicRequestBody(info *relaycommon.RelayInfo, body io.Reader) (io.Reader, error) {
	if !IsSupportedMusicModel(gmiModelName(info)) {
		return nil, fmt.Errorf("gmicloud: model %q is not a supported music model", gmiModelName(info))
	}

	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("gmicloud: read MiniMax music request: %w", err)
	}
	var request map[string]any
	if err := common.Unmarshal(raw, &request); err != nil {
		return nil, fmt.Errorf("gmicloud: decode MiniMax music request: %w", err)
	}
	if stream, _ := request["stream"].(bool); stream {
		return nil, fmt.Errorf("gmicloud: MiniMax music streaming is not supported by the GMI upstream")
	}
	for _, field := range []string{"audio_url", "audio_base64", "cover_feature_id"} {
		if value, exists := request[field]; exists && strings.TrimSpace(fmt.Sprint(value)) != "" {
			return nil, fmt.Errorf("gmicloud: MiniMax music %s is not supported", field)
		}
	}
	if outputFormat, _ := request["output_format"].(string); outputFormat != "" && outputFormat != "url" && outputFormat != "hex" {
		return nil, fmt.Errorf("gmicloud: unsupported MiniMax music output_format %q", outputFormat)
	}

	lyrics, _ := request["lyrics"].(string)
	if strings.TrimSpace(lyrics) == "" {
		return nil, fmt.Errorf("gmicloud: MiniMax music request requires lyrics")
	}

	payload := map[string]any{"lyrics": lyrics}
	if prompt, _ := request["prompt"].(string); strings.TrimSpace(prompt) != "" {
		payload["prompt"] = prompt
	}
	if audioSetting, ok := request["audio_setting"].(map[string]any); ok {
		for _, field := range []string{"sample_rate", "bitrate", "format"} {
			if value, exists := audioSetting[field]; exists {
				payload[field] = value
			}
		}
	}

	encoded, err := common.Marshal(gmiSubmitRequest{
		Model:   gmiModelName(info),
		Payload: payload,
	})
	if err != nil {
		return nil, fmt.Errorf("gmicloud: marshal music request: %w", err)
	}
	return bytes.NewReader(encoded), nil
}

// ValidateMiniMaxMusicRequest rejects MiniMax features GMI Music 3.0 cannot emulate.
func ValidateMiniMaxMusicRequest(request *dto.MiniMaxMusicGenerationRequest) error {
	if request == nil {
		return fmt.Errorf("gmicloud: missing MiniMax music request")
	}
	if request.Stream {
		return fmt.Errorf("gmicloud: MiniMax music streaming is not supported by the GMI upstream")
	}
	if strings.TrimSpace(request.Lyrics) == "" {
		return fmt.Errorf("gmicloud: MiniMax music request requires lyrics")
	}
	if request.AudioURL != "" || request.AudioBase64 != "" || request.CoverFeatureID != "" {
		return fmt.Errorf("gmicloud: MiniMax music cover generation is not supported")
	}
	outputFormat := strings.TrimSpace(request.OutputFormat)
	if outputFormat != "" && outputFormat != "url" && outputFormat != "hex" {
		return fmt.Errorf("gmicloud: unsupported MiniMax music output_format %q", outputFormat)
	}
	return nil
}

type miniMaxMusicData struct {
	Audio  string `json:"audio"`
	Status int    `json:"status"`
}

type miniMaxMusicExtraInfo struct {
	MusicDuration   int64 `json:"music_duration,omitempty"`
	MusicSampleRate int64 `json:"music_sample_rate,omitempty"`
	MusicChannel    int   `json:"music_channel,omitempty"`
	Bitrate         int64 `json:"bitrate,omitempty"`
	MusicSize       int64 `json:"music_size,omitempty"`
}

type miniMaxMusicResponse struct {
	Data      miniMaxMusicData      `json:"data"`
	TraceID   string                `json:"trace_id"`
	ExtraInfo miniMaxMusicExtraInfo `json:"extra_info"`
	BaseResp  miniMaxBaseResp       `json:"base_resp"`
}

type miniMaxBaseResp struct {
	StatusCode int64  `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// handleMiniMaxMusicResponse translates a GMI task result to the MiniMax native response.
func handleMiniMaxMusicResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: empty music submit response"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	body, readErr := io.ReadAll(resp.Body)
	service.CloseResponseBodyGracefully(resp)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: read music submit response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusBadGateway,
		)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, gmiUpstreamHTTPError("music submit", resp.StatusCode, body)
	}

	var submit gmiSubmitResponse
	if err := common.Unmarshal(body, &submit); err != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: decode music submit response: %w", err),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}
	if submit.Status == "failed" || submit.Status == "cancelled" {
		message := strings.TrimSpace(submit.Message)
		if message == "" {
			message = "gmicloud: music task " + submit.Status
		}
		return nil, gmiTaskCreatedError(types.NewErrorWithStatusCode(
			fmt.Errorf("%s", message),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		))
	}

	result, pollErr := pollGMIResult(c, info, submit.RequestID, submit.Status)
	if pollErr != nil {
		return nil, gmiTaskCreatedError(pollErr)
	}
	audioURL := extractAudioURL(result.Outcome)
	if audioURL == "" {
		return nil, gmiTaskCreatedError(types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: no music URL in outcome for request %s", submit.RequestID),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		))
	}

	audio, size, outputErr := miniMaxMusicOutput(c, info, audioURL)
	if outputErr != nil {
		return nil, gmiTaskCreatedError(outputErr)
	}

	outcome := result.Outcome
	response := miniMaxMusicResponse{
		Data:    miniMaxMusicData{Audio: audio, Status: 2},
		TraceID: result.RequestID,
		ExtraInfo: miniMaxMusicExtraInfo{
			MusicDuration:   outcome.DurationMS,
			MusicSampleRate: outcome.SampleRate,
			MusicChannel:    outcome.Channels,
			Bitrate:         outcome.Bitrate,
			MusicSize:       size,
		},
		BaseResp: miniMaxBaseResp{StatusCode: 0, StatusMsg: "success"},
	}
	encoded, err := common.Marshal(response)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	c.Data(http.StatusOK, "application/json", encoded)

	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens <= 0 {
		promptTokens = 1
	}
	return &dto.Usage{PromptTokens: promptTokens, TotalTokens: promptTokens}, nil
}

func miniMaxMusicOutput(c *gin.Context, info *relaycommon.RelayInfo, audioURL string) (string, int64, *types.NewAPIError) {
	format := "hex"
	if request, ok := info.Request.(*dto.MiniMaxMusicGenerationRequest); ok && request != nil && request.OutputFormat != "" {
		format = strings.ToLower(strings.TrimSpace(request.OutputFormat))
	}
	switch format {
	case "url":
		return audioURL, 0, nil
	case "", "hex":
		// MiniMax's default response embeds hexadecimal audio instead of a URL.
	default:
		return "", 0, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: unsupported MiniMax music output_format %q", format),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, audioURL, nil)
	if err != nil {
		return "", 0, types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	resp, err := relayHTTPClient().Do(req)
	if err != nil {
		return "", 0, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: download music: %w", err),
			types.ErrorCodeDoRequestFailed,
			http.StatusBadGateway,
		)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: music download HTTP %d", resp.StatusCode),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMiniMaxMusicDownloadBytes+1))
	if len(data) > maxMiniMaxMusicDownloadBytes {
		return "", 0, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: music download exceeds %d byte limit", maxMiniMaxMusicDownloadBytes),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}
	if err != nil {
		return "", 0, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: read music download: %w", err),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusBadGateway,
		)
	}
	return hex.EncodeToString(data), int64(len(data)), nil
}
