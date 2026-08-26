package gmicloud

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// gmiSubmitRequest is the outer envelope for GMI requestqueue.
type gmiSubmitRequest struct {
	Model   string         `json:"model"`
	Payload map[string]any `json:"payload"`
}

// gmiSubmitResponse is returned by POST /apikey/requests.
type gmiSubmitResponse struct {
	RequestID string `json:"request_id"`
	Model     string `json:"model"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	QueuedAt  int64  `json:"queued_at"`
	Message   string `json:"message,omitempty"`
}

// gmiOutcome holds the generated media URLs.
// gmiMedia identifies one generated file in a requestqueue outcome.
type gmiMedia struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// gmiOutcome holds the generated media URLs.
type gmiOutcome struct {
	AudioURL   string     `json:"audio_url,omitempty"`
	Status     string     `json:"status,omitempty"`
	DurationMS int64      `json:"duration_ms,omitempty"`
	SampleRate int64      `json:"sample_rate,omitempty"`
	Channels   int        `json:"channels,omitempty"`
	Bitrate    int64      `json:"bitrate,omitempty"`
	MediaURLs  []gmiMedia `json:"media_urls,omitempty"`
	Medias     []gmiMedia `json:"medias,omitempty"`
}

// gmiStatusResponse is returned by GET /apikey/requests/{id}.
type gmiStatusResponse struct {
	RequestID string         `json:"request_id"`
	Model     string         `json:"model"`
	Status    string         `json:"status"`
	Payload   map[string]any `json:"payload,omitempty"`
	Outcome   *gmiOutcome    `json:"outcome,omitempty"`
	Message   string         `json:"message,omitempty"`
	Error     string         `json:"error,omitempty"`
}

func buildAudioRequestBody(_ *gin.Context, info *relaycommon.RelayInfo, request *dto.AudioRequest) (io.Reader, error) {
	model := gmiModelName(info)
	payload := map[string]any{}

	// Merge metadata first so explicit fields below take precedence.
	if len(request.Metadata) > 0 {
		if err := common.Unmarshal(request.Metadata, &payload); err != nil {
			return nil, fmt.Errorf("invalid gmicloud metadata: %w", err)
		}
	}

	switch {
	case isGMIMusicModel(model):
		return nil, fmt.Errorf("gmicloud music models require /v1/music_generation")
	case isGMIVoiceCloneModel(model) && IsSupportedSpeechModel(model):
		payload["text"] = request.Input
		if request.Voice != "" {
			payload["voice_id"] = request.Voice
		}
		sourceAudio, ok := payload["source_audio"].(string)
		if !ok || !isHTTPURL(sourceAudio) {
			return nil, fmt.Errorf("gmicloud voice clone requires metadata.source_audio as an HTTP(S) URL")
		}

	case isGMITTSModel(model) && IsSupportedSpeechModel(model):
		payload["text"] = request.Input
		if request.Voice != "" {
			payload["voice_id"] = request.Voice
		}
		if request.Speed != nil {
			payload["speed"] = fmt.Sprintf("%.1f", *request.Speed)
		}
		if request.ResponseFormat != "" {
			payload["format"] = normalizeAudioFormat(request.ResponseFormat)
		}

	default:
		return nil, fmt.Errorf("gmicloud model %q does not support /v1/audio/speech", model)
	}
	body := gmiSubmitRequest{
		Model:   model,
		Payload: payload,
	}
	jsonData, err := common.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal gmicloud request: %w", err)
	}
	return bytes.NewReader(jsonData), nil
}

func handleAudioResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: empty audio submit response"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: read submit response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusBadGateway,
		)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, gmiUpstreamHTTPError("audio submit", resp.StatusCode, body)
	}

	var submit gmiSubmitResponse
	if err := common.Unmarshal(body, &submit); err != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: decode submit response: %w", err),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}

	if submit.Status == "failed" || submit.Status == "cancelled" {
		message := strings.TrimSpace(submit.Message)
		if message == "" {
			message = "gmicloud: task " + submit.Status
		}
		return nil, gmiTaskCreatedError(types.NewErrorWithStatusCode(
			errors.New(message),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		))
	}

	// Poll for the result if not immediately ready.
	result, pollErr := pollGMIResult(c, info, submit.RequestID, submit.Status)
	if pollErr != nil {
		return nil, gmiTaskCreatedError(pollErr)
	}

	audioURL := extractAudioURL(result.Outcome)
	if audioURL == "" {
		return nil, gmiTaskCreatedError(types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: no audio URL in outcome for request %s", submit.RequestID),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		))
	}

	// Download the generated audio and stream it to the client.
	usage, downloadErr := downloadAndStreamAudio(c, info, audioURL)
	if downloadErr != nil {
		return nil, gmiTaskCreatedError(downloadErr)
	}
	return usage, nil
}

func pollGMIResult(c *gin.Context, info *relaycommon.RelayInfo, requestID, initialStatus string) (*gmiStatusResponse, *types.NewAPIError) {
	if requestID == "" {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: empty request_id in submit response"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	status := initialStatus
	maxWait := 90 * time.Second
	if isGMIMusicModel(gmiModelName(info)) {
		maxWait = 180 * time.Second
	}
	deadline := time.Now().Add(maxWait)
	interval := 2 * time.Second

	for time.Now().Before(deadline) {
		if status == "success" {
			break
		}
		if status == "failed" || status == "cancelled" {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("gmicloud: task %s", status),
				types.ErrorCodeBadResponse,
				http.StatusBadGateway,
			)
		}
		select {
		case <-c.Request.Context().Done():
			return nil, types.NewErrorWithStatusCode(
				c.Request.Context().Err(),
				types.ErrorCodeDoRequestFailed,
				http.StatusGatewayTimeout,
			)
		case <-time.After(interval):
		}

		result, err := fetchGMIStatus(c, info, requestID)
		if err != nil {
			return nil, err
		}
		status = result.Status
		if status == "success" && result.Outcome != nil {
			return result, nil
		}
	}

	// Final fetch even if initial status was success (outcome may not be in submit).
	result, err := fetchGMIStatus(c, info, requestID)
	if err != nil {
		return nil, err
	}
	if result.Status != "success" {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: task timed out with status %s after %s", result.Status, maxWait),
			types.ErrorCodeDoRequestFailed,
			http.StatusGatewayTimeout,
		)
	}
	return result, nil
}

func fetchGMIStatus(c *gin.Context, info *relaycommon.RelayInfo, requestID string) (*gmiStatusResponse, *types.NewAPIError) {
	url := audioBaseURL(info) + requestStatusPath + requestID
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)

	client := relayHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: fetch status: %w", err),
			types.ErrorCodeDoRequestFailed,
			http.StatusBadGateway,
		)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: read status response: %w", err),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusBadGateway,
		)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: status HTTP %d: %s", resp.StatusCode, truncate(body, 500)),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	var result gmiStatusResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: decode status response: %w", err),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}
	return &result, nil
}

func gmiUpstreamHTTPError(stage string, statusCode int, body []byte) *types.NewAPIError {
	message := fmt.Errorf("gmicloud: %s HTTP %d: %s", stage, statusCode, truncate(body, 500))
	if statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError && statusCode != http.StatusTooManyRequests {
		return types.NewErrorWithStatusCode(message, types.ErrorCodeBadResponse, statusCode, types.ErrOptionWithSkipRetry())
	}
	return types.NewErrorWithStatusCode(message, types.ErrorCodeBadResponse, http.StatusBadGateway)
}

func extractAudioURL(outcome *gmiOutcome) string {
	if outcome == nil {
		return ""
	}
	if outcome.AudioURL != "" {
		return outcome.AudioURL
	}
	if len(outcome.MediaURLs) > 0 && outcome.MediaURLs[0].URL != "" {
		return outcome.MediaURLs[0].URL
	}
	if len(outcome.Medias) > 0 && outcome.Medias[0].URL != "" {
		return outcome.Medias[0].URL
	}
	return ""
}

func downloadAndStreamAudio(c *gin.Context, info *relaycommon.RelayInfo, audioURL string) (*dto.Usage, *types.NewAPIError) {
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, audioURL, nil)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	client := relayHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: download audio: %w", err),
			types.ErrorCodeDoRequestFailed,
			http.StatusBadGateway,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: audio download HTTP %d", resp.StatusCode),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg"
	}

	c.DataFromReader(http.StatusOK, resp.ContentLength, contentType, resp.Body, nil)

	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens <= 0 {
		promptTokens = 1
	}
	return &dto.Usage{
		PromptTokens: promptTokens,
		TotalTokens:  promptTokens,
	}, nil
}

func normalizeAudioFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "wav", "flac", "pcm":
		return format
	default:
		return "mp3"
	}
}

func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func truncate(b []byte, max int) string {
	s := string(b)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// gmiTaskCreatedError prevents retrying a requestqueue task after it may have been accepted upstream.
func gmiTaskCreatedError(err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		return nil
	}
	return types.NewError(err, err.GetErrorCode(), types.ErrOptionWithSkipRetry())
}

// relayHTTPClient falls back to http.DefaultClient so tests without
// service.InitHttpClient still work.
func relayHTTPClient() *http.Client {
	if client := service.GetHttpClient(); client != nil {
		return client
	}
	return http.DefaultClient
}
