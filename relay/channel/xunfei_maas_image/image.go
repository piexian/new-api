package xunfei_maas_image

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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

type imageAuth struct {
	AppID     string
	APIKey    string
	APISecret string
}

type imageMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type negativePrompts struct {
	Text string `json:"text"`
}

type imageRequest struct {
	Header struct {
		AppID   string   `json:"app_id"`
		UID     string   `json:"uid,omitempty"`
		PatchID []string `json:"patch_id,omitempty"`
	} `json:"header"`
	Parameter struct {
		Chat struct {
			Domain            string  `json:"domain"`
			Width             int     `json:"width"`
			Height            int     `json:"height"`
			Seed              int     `json:"seed"`
			NumInferenceSteps int     `json:"num_inference_steps"`
			GuidanceScale     float64 `json:"guidance_scale"`
			Scheduler         string  `json:"scheduler"`
		} `json:"chat"`
	} `json:"parameter"`
	Payload struct {
		Message struct {
			Text []imageMessage `json:"text"`
		} `json:"message"`
		NegativePrompts *negativePrompts `json:"negative_prompts,omitempty"`
	} `json:"payload"`
}

type imageResponseTextItem struct {
	Content string `json:"content"`
	Role    string `json:"role"`
	Index   int    `json:"index"`
}

type imageResponse struct {
	Header struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Sid     string `json:"sid"`
		Status  int    `json:"status"`
	} `json:"header"`
	Payload struct {
		Choices struct {
			Status int                     `json:"status"`
			Seq    int                     `json:"seq"`
			Text   []imageResponseTextItem `json:"text"`
		} `json:"choices"`
	} `json:"payload"`
}

func parseAuth(raw string) (*imageAuth, error) {
	parts := strings.Split(raw, "|")
	if len(parts) != 3 {
		return nil, errors.New("xunfei maas image generation requires channel key format app_id|api_key|api_secret")
	}
	auth := &imageAuth{
		AppID:     strings.TrimSpace(parts[0]),
		APIKey:    strings.TrimSpace(parts[1]),
		APISecret: strings.TrimSpace(parts[2]),
	}
	if auth.AppID == "" || auth.APIKey == "" || auth.APISecret == "" {
		return nil, errors.New("xunfei maas image generation requires non-empty app_id, api_key and api_secret")
	}
	return auth, nil
}

func buildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("relay info is nil")
	}
	auth, err := parseAuth(info.ApiKey)
	if err != nil {
		return "", err
	}
	endpoint, err := normalizeEndpoint(info.ChannelBaseUrl)
	if err != nil {
		return "", err
	}
	return signURL(endpoint, auth.APIKey, auth.APISecret)
}

func normalizeEndpoint(baseURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", fmt.Errorf("invalid xunfei maas image base url: %q", baseURL)
	}

	path := strings.TrimRight(parsedURL.Path, "/")
	switch {
	case path == "":
		parsedURL.Path = "/v2.1/tti"
	case strings.HasSuffix(path, "/v2.1/tti"):
		parsedURL.Path = path
	case strings.HasSuffix(path, "/v2.1"):
		parsedURL.Path = path + "/tti"
	default:
		segments := strings.Split(strings.Trim(path, "/"), "/")
		last := segments[len(segments)-1]
		if looksLikeVersionSegment(last) {
			segments[len(segments)-1] = "v2.1"
			segments = append(segments, "tti")
			parsedURL.Path = "/" + strings.Join(segments, "/")
		} else {
			parsedURL.Path = path + "/tti"
		}
	}
	parsedURL.RawQuery = ""
	return parsedURL.String(), nil
}

func looksLikeVersionSegment(segment string) bool {
	if !strings.HasPrefix(segment, "v") || len(segment) < 2 {
		return false
	}
	for _, r := range segment[1:] {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

func signURL(endpoint, apiKey, apiSecret string) (string, error) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	date := time.Now().UTC().Format(time.RFC1123)
	stringToSign := fmt.Sprintf("host: %s\ndate: %s\nPOST %s HTTP/1.1", parsedURL.Host, date, parsedURL.Path)
	mac := hmac.New(sha256.New, []byte(apiSecret))
	if _, err = mac.Write([]byte(stringToSign)); err != nil {
		return "", err
	}
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authorizationOrigin := fmt.Sprintf(
		"api_key=\"%s\", algorithm=\"hmac-sha256\", headers=\"host date request-line\", signature=\"%s\"",
		apiKey,
		signature,
	)
	query := parsedURL.Query()
	query.Set("host", parsedURL.Host)
	query.Set("date", date)
	query.Set("authorization", base64.StdEncoding.EncodeToString([]byte(authorizationOrigin)))
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func convertImageRequest(c *gin.Context, request dto.ImageRequest, info *relaycommon.RelayInfo) (*imageRequest, error) {
	auth, err := parseAuth(info.ApiKey)
	if err != nil {
		return nil, err
	}
	if request.N != nil && *request.N > 1 {
		return nil, errors.New("xunfei maas image generation does not support n > 1")
	}
	width, height, err := parseImageSize(request.Size)
	if err != nil {
		return nil, err
	}

	modelID := strings.TrimSpace(request.Model)
	if modelID == "" && info != nil {
		modelID = strings.TrimSpace(info.UpstreamModelName)
	}
	if modelID == "" {
		return nil, errors.New("model is required")
	}

	imageReq := &imageRequest{}
	imageReq.Header.AppID = auth.AppID
	imageReq.Header.UID = extractUID(request)
	imageReq.Header.PatchID = extractPatchID(c, request)
	imageReq.Parameter.Chat.Domain = modelID
	imageReq.Parameter.Chat.Width = width
	imageReq.Parameter.Chat.Height = height
	imageReq.Parameter.Chat.Seed = getIntExtra(request, "seed", 42)
	imageReq.Parameter.Chat.NumInferenceSteps = getIntExtra(request, "num_inference_steps", 20)
	imageReq.Parameter.Chat.GuidanceScale = getFloatExtra(request, "guidance_scale", 5.0)
	imageReq.Parameter.Chat.Scheduler = getStringExtra(request, "scheduler", "DPM++ 2M Karras")
	imageReq.Payload.Message.Text = []imageMessage{
		{
			Role:    "user",
			Content: request.Prompt,
		},
	}

	if negativePrompt := extractNegativePrompt(request); negativePrompt != "" {
		imageReq.Payload.NegativePrompts = &negativePrompts{Text: negativePrompt}
	}
	return imageReq, nil
}

func parseImageSize(size string) (int, int, error) {
	if strings.TrimSpace(size) == "" {
		return 512, 512, nil
	}
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid image size: %q", size)
	}
	var width, height int
	if err := common.UnmarshalJsonStr(parts[0], &width); err != nil {
		return 0, 0, fmt.Errorf("invalid image width: %q", size)
	}
	if err := common.UnmarshalJsonStr(parts[1], &height); err != nil {
		return 0, 0, fmt.Errorf("invalid image height: %q", size)
	}
	if width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("invalid image size: %q", size)
	}
	return width, height, nil
}

func getStringExtra(request dto.ImageRequest, key, defaultValue string) string {
	raw, ok := request.Extra[key]
	if !ok || len(raw) == 0 {
		return defaultValue
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func getIntExtra(request dto.ImageRequest, key string, defaultValue int) int {
	raw, ok := request.Extra[key]
	if !ok || len(raw) == 0 {
		return defaultValue
	}
	var value int
	if err := common.Unmarshal(raw, &value); err != nil {
		return defaultValue
	}
	return value
}

func getFloatExtra(request dto.ImageRequest, key string, defaultValue float64) float64 {
	raw, ok := request.Extra[key]
	if !ok || len(raw) == 0 {
		return defaultValue
	}
	var value float64
	if err := common.Unmarshal(raw, &value); err != nil {
		return defaultValue
	}
	return value
}

func extractUID(request dto.ImageRequest) string {
	if len(request.User) > 0 {
		var uid string
		if err := common.Unmarshal(request.User, &uid); err == nil {
			return strings.TrimSpace(uid)
		}
	}
	for _, key := range []string{"uid", "user_id"} {
		if value := getStringExtra(request, key, ""); value != "" {
			return value
		}
	}
	return ""
}

func extractPatchID(c *gin.Context, request dto.ImageRequest) []string {
	raw, ok := request.Extra["patch_id"]
	if !ok || len(raw) == 0 {
		return extractDefaultPatchID(c)
	}
	var patchIDs []string
	if err := common.Unmarshal(raw, &patchIDs); err == nil && len(patchIDs) > 0 {
		return patchIDs
	}
	var patchID string
	if err := common.Unmarshal(raw, &patchID); err == nil && strings.TrimSpace(patchID) != "" {
		return []string{strings.TrimSpace(patchID)}
	}
	return extractDefaultPatchID(c)
}

func extractDefaultPatchID(c *gin.Context) []string {
	if c == nil {
		return nil
	}
	patchID := strings.TrimSpace(c.GetString("patch_id"))
	if patchID == "" {
		return nil
	}
	return []string{patchID}
}

func extractNegativePrompt(request dto.ImageRequest) string {
	for _, key := range []string{"negative_prompt", "negative_prompts"} {
		raw, ok := request.Extra[key]
		if !ok || len(raw) == 0 {
			continue
		}
		var prompt string
		if err := common.Unmarshal(raw, &prompt); err == nil && strings.TrimSpace(prompt) != "" {
			return strings.TrimSpace(prompt)
		}
		var promptObject negativePrompts
		if err := common.Unmarshal(raw, &promptObject); err == nil && strings.TrimSpace(promptObject.Text) != "" {
			return strings.TrimSpace(promptObject.Text)
		}
	}
	return ""
}

func imageHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)
	if len(responseBody) == 0 {
		return nil, types.NewOpenAIError(errors.New("empty xunfei maas image response"), types.ErrorCodeEmptyResponse, http.StatusInternalServerError)
	}

	var response imageResponse
	if err = common.Unmarshal(responseBody, &response); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if response.Header.Code != 0 {
		metadata := buildErrorMetadata(response.Header.Sid)
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message:  response.Header.Message,
			Type:     "xunfei_maas_image_error",
			Code:     fmt.Sprintf("%d", response.Header.Code),
			Metadata: metadata,
		}, imageStatusCode(response.Header.Code, resp.StatusCode))
	}

	openAIResponse, err := responseToOpenAIImage(&response, info)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	jsonResponse, err := common.Marshal(openAIResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err = c.Writer.Write(jsonResponse); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	return &dto.Usage{}, nil
}

func buildErrorMetadata(sid string) []byte {
	if strings.TrimSpace(sid) == "" {
		return nil
	}
	metadata, err := common.Marshal(map[string]string{"sid": sid})
	if err != nil {
		return nil
	}
	return metadata
}

func imageStatusCode(code int, upstreamStatus int) int {
	switch code {
	case 10003, 10004, 10005, 10021, 10022:
		return http.StatusBadRequest
	case 10008:
		return http.StatusTooManyRequests
	default:
		if upstreamStatus >= http.StatusBadRequest {
			return upstreamStatus
		}
		return http.StatusBadRequest
	}
}

func responseToOpenAIImage(response *imageResponse, info *relaycommon.RelayInfo) (*dto.ImageResponse, error) {
	imageResp := &dto.ImageResponse{}
	if info != nil && !info.StartTime.IsZero() {
		imageResp.Created = info.StartTime.Unix()
	} else {
		imageResp.Created = common.GetTimestamp()
	}

	responseFormat := imageResponseFormat(info)
	for _, item := range response.Payload.Choices.Text {
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		data := dto.ImageData{}
		if strings.EqualFold(responseFormat, "b64_json") {
			data.B64Json = item.Content
		} else {
			data.Url = "data:image/png;base64," + item.Content
		}
		imageResp.Data = append(imageResp.Data, data)
	}
	if len(imageResp.Data) == 0 {
		return nil, errors.New("no images generated")
	}
	return imageResp, nil
}

func imageResponseFormat(info *relaycommon.RelayInfo) string {
	if info == nil || info.Request == nil {
		return ""
	}
	if request, ok := info.Request.(*dto.ImageRequest); ok && request != nil {
		return request.ResponseFormat
	}
	return ""
}
