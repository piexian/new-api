package volcengine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	openSpeechTTSV3BaseAlias  = "openspeech-tts-v3"
	openSpeechTTSV3DefaultURL = "https://openspeech.bytedance.com/api/v3/tts/unidirectional"
	defaultTTSV3ResourceID    = "seed-tts-2.0"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type volcengineAudioSpeechMetadata struct {
	ResourceID string `json:"resource_id,omitempty"`
}

type volcengineTTSV3Request struct {
	User      volcengineTTSV3User      `json:"user"`
	ReqParams volcengineTTSV3ReqParams `json:"req_params"`
}

type volcengineTTSV3User struct {
	UID string `json:"uid"`
}

type volcengineTTSV3ReqParams struct {
	Text        string                     `json:"text"`
	Speaker     string                     `json:"speaker"`
	SpeedRatio  *float64                   `json:"speed_ratio,omitempty"`
	AudioParams volcengineTTSV3AudioParams `json:"audio_params"`
}

type volcengineTTSV3AudioParams struct {
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
}

func isOpenSpeechTTSV3Base(baseURL string) bool {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false
	}
	if strings.EqualFold(baseURL, openSpeechTTSV3BaseAlias) {
		return true
	}
	lower := strings.ToLower(baseURL)
	if lower == "https://openspeech.bytedance.com" || lower == "http://openspeech.bytedance.com" {
		return true
	}
	return strings.HasPrefix(lower, "http") && strings.Contains(lower, "/api/v3/tts/unidirectional")
}

func openSpeechTTSV3RequestURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasPrefix(strings.ToLower(baseURL), "http") &&
		strings.Contains(strings.ToLower(baseURL), "/api/v3/tts/unidirectional") {
		return baseURL
	}
	return openSpeechTTSV3DefaultURL
}

func mapEncodingToTTSV3Format(encoding string) string {
	switch encoding {
	case "ogg_opus", "pcm", "wav":
		return encoding
	default:
		return "mp3"
	}
}

func parseTTSV3ResourceID(metadata []byte) string {
	if len(metadata) == 0 {
		return defaultTTSV3ResourceID
	}
	var meta volcengineAudioSpeechMetadata
	if err := common.Unmarshal(metadata, &meta); err != nil || strings.TrimSpace(meta.ResourceID) == "" {
		return defaultTTSV3ResourceID
	}
	return strings.TrimSpace(meta.ResourceID)
}

func buildTTSV3RequestBody(text, speaker, encoding string, speed *float64) ([]byte, error) {
	req := volcengineTTSV3Request{
		User: volcengineTTSV3User{
			UID: "openai_relay_user",
		},
		ReqParams: volcengineTTSV3ReqParams{
			Text:       text,
			Speaker:    speaker,
			SpeedRatio: speed,
			AudioParams: volcengineTTSV3AudioParams{
				Format:     mapEncodingToTTSV3Format(encoding),
				SampleRate: 24000,
			},
		},
	}
	return common.Marshal(req)
}

func setupOpenSpeechTTSV3Header(req *http.Header, apiKey string, resourceID string) error {
	req.Set("Content-Type", "application/json")
	req.Set("X-Api-Resource-Id", resourceID)
	req.Set("X-Api-Request-Id", generateRequestID())

	appID, token, err := parseVolcengineAuth(apiKey)
	if err == nil {
		req.Set("Authorization", "Bearer;"+token)
		req.Set("X-Api-App-Id", appID)
		req.Set("X-Api-Access-Key", token)
		return nil
	}
	if strings.TrimSpace(apiKey) == "" {
		return err
	}
	req.Set("X-Api-Key", strings.TrimSpace(apiKey))
	return nil
}

func isOpenSpeechTTSV3Context(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, ok := c.Get(contextKeyTTSOpenSpeechV3)
	if !ok {
		return false
	}
	useV3, ok := value.(bool)
	return ok && useV3
}

func normalizeNdjsonLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "event:") ||
		strings.HasPrefix(lower, "id:") ||
		strings.HasPrefix(lower, "retry:") {
		return ""
	}
	if strings.HasPrefix(lower, "data:") {
		line = strings.TrimSpace(line[5:])
		if strings.HasPrefix(line, "[DONE]") {
			return ""
		}
	}
	return line
}

func parseNdjsonLineCodeAndData(line string) (code int, hasCode bool, dataStr string, err error) {
	var m map[string]json.RawMessage
	if err = common.UnmarshalJsonStr(line, &m); err != nil {
		return 0, false, "", err
	}
	rawCode, ok := m["code"]
	if !ok || len(rawCode) == 0 {
		return 0, false, "", nil
	}
	var f float64
	if err = common.Unmarshal(rawCode, &f); err == nil {
		return int(f), true, extractDataString(m["data"]), nil
	}
	var s string
	if err = common.Unmarshal(rawCode, &s); err != nil {
		return 0, false, "", fmt.Errorf("openspeech v3 line code: %w", err)
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(s))
	if convErr != nil {
		return 0, false, "", fmt.Errorf("openspeech v3 line code string: %w", convErr)
	}
	return n, true, extractDataString(m["data"]), nil
}

func extractDataString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := common.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

func decodeBase64AudioPayload(s string) ([]byte, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", ""))
	if s == "" {
		return nil, errors.New("empty payload")
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawURLEncoding.DecodeString(s)
}

func looksLikeJSONObject(b []byte) bool {
	b = bytes.TrimSpace(b)
	return len(b) >= 2 && b[0] == '{' && b[len(b)-1] == '}'
}

func handleTTSV3NdjsonResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, encoding string) (usage any, err *types.NewAPIError) {
	if resp == nil {
		return nil, types.NewErrorWithStatusCode(
			errors.New("empty upstream response"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to read openspeech v3 response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("openspeech v3 HTTP %d: %s", resp.StatusCode, truncateForErr(string(body), 800)),
			types.ErrorCodeBadResponseStatusCode,
			http.StatusBadGateway,
		)
	}

	body = bytes.TrimPrefix(body, utf8BOM)

	var chunks [][]byte
	for _, line := range strings.Split(string(body), "\n") {
		line = normalizeNdjsonLine(line)
		if line == "" {
			continue
		}
		code, hasCode, dataStr, lineErr := parseNdjsonLineCodeAndData(line)
		if lineErr != nil {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("openspeech v3 non-JSON line: %w", lineErr),
				types.ErrorCodeBadResponseBody,
				http.StatusBadGateway,
			)
		}
		if !hasCode {
			continue
		}
		switch code {
		case 0:
			if dataStr == "" {
				continue
			}
			audioChunk, decodeErr := decodeBase64AudioPayload(dataStr)
			if decodeErr != nil {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("openspeech v3 invalid base64 audio: %w", decodeErr),
					types.ErrorCodeBadResponseBody,
					http.StatusBadGateway,
				)
			}
			if looksLikeJSONObject(audioChunk) {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("openspeech v3 audio chunk looks like JSON: %s", truncateForErr(string(audioChunk), 500)),
					types.ErrorCodeBadResponse,
					http.StatusBadGateway,
				)
			}
			chunks = append(chunks, audioChunk)
		case 20000000:
			goto done
		default:
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("openspeech v3 error code=%d line=%s", code, truncateForErr(line, 400)),
				types.ErrorCodeBadResponse,
				http.StatusBadGateway,
			)
		}
	}
done:
	if len(chunks) == 0 {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("openspeech v3: no audio chunks in response: %s", truncateForErr(string(body), 500)),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}

	out := bytes.Join(chunks, nil)
	contentType := getContentTypeByEncoding(mapEncodingToTTSV3Format(encoding))
	c.Header("Content-Type", contentType)
	c.Data(http.StatusOK, contentType, out)

	usage = &dto.Usage{
		PromptTokens:     info.GetEstimatePromptTokens(),
		CompletionTokens: 0,
		TotalTokens:      info.GetEstimatePromptTokens(),
	}
	return usage, nil
}

func truncateForErr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
