package minimax

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type MiniMaxImageRequest struct {
	Model            string                         `json:"model"`
	Prompt           string                         `json:"prompt"`
	AspectRatio      string                         `json:"aspect_ratio,omitempty"`
	Width            *int                           `json:"width,omitempty"`
	Height           *int                           `json:"height,omitempty"`
	ResponseFormat   string                         `json:"response_format,omitempty"`
	Seed             *int64                         `json:"seed,omitempty"`
	N                int                            `json:"n,omitempty"`
	PromptOptimizer  *bool                          `json:"prompt_optimizer,omitempty"`
	AigcWatermark    *bool                          `json:"aigc_watermark,omitempty"`
	Style            json.RawMessage                `json:"style,omitempty"`
	SubjectReference []MiniMaxImageSubjectReference `json:"subject_reference,omitempty"`
}

type MiniMaxImageSubjectReference struct {
	Type      string `json:"type,omitempty"`
	ImageFile string `json:"image_file,omitempty"`
}

type MiniMaxImageResponse struct {
	ID   string `json:"id"`
	Data struct {
		ImageURLs   []string `json:"image_urls"`
		ImageBase64 []string `json:"image_base64"`
	} `json:"data"`
	Metadata map[string]any `json:"metadata"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

func isMiniMaxNativeImageEndpoint(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	return strings.HasPrefix(c.Request.URL.Path, "/v1/image_generation")
}

func oaiImage2MiniMaxImageRequest(request dto.ImageRequest) MiniMaxImageRequest {
	responseFormat := normalizeMiniMaxResponseFormat(request.ResponseFormat)
	minimaxRequest := MiniMaxImageRequest{
		Model:          request.Model,
		Prompt:         request.Prompt,
		ResponseFormat: responseFormat,
		N:              1,
		AigcWatermark:  request.Watermark,
	}

	if request.Model == "" {
		minimaxRequest.Model = "image-01"
	}
	if request.N != nil && *request.N > 0 {
		minimaxRequest.N = int(*request.N)
	}
	if aspectRatio := aspectRatioFromImageRequest(request); aspectRatio != "" {
		minimaxRequest.AspectRatio = aspectRatio
	}
	if len(request.Style) > 0 {
		minimaxRequest.Style = request.Style
	}
	applyMiniMaxImageRawFields(request.Extra, &minimaxRequest)
	applyMiniMaxImageExtraFields(request.ExtraFields, &minimaxRequest)

	return minimaxRequest
}

func applyMiniMaxImageExtraFields(extraFields json.RawMessage, minimaxRequest *MiniMaxImageRequest) {
	if len(extraFields) == 0 {
		return
	}
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(extraFields, &fields); err != nil {
		return
	}
	applyMiniMaxImageRawFields(fields, minimaxRequest)
}

func applyMiniMaxImageRawFields(fields map[string]json.RawMessage, minimaxRequest *MiniMaxImageRequest) {
	if len(fields) == 0 {
		return
	}
	for key, raw := range fields {
		switch key {
		case "aspect_ratio":
			var value string
			if err := common.Unmarshal(raw, &value); err == nil && value != "" {
				minimaxRequest.AspectRatio = value
			}
		case "width":
			minimaxRequest.Width = unmarshalMiniMaxInt(raw)
		case "height":
			minimaxRequest.Height = unmarshalMiniMaxInt(raw)
		case "seed":
			minimaxRequest.Seed = unmarshalMiniMaxInt64(raw)
		case "prompt_optimizer":
			minimaxRequest.PromptOptimizer = unmarshalMiniMaxBool(raw)
		case "aigc_watermark":
			minimaxRequest.AigcWatermark = unmarshalMiniMaxBool(raw)
		case "style":
			minimaxRequest.Style = raw
		case "subject_reference":
			var subjectReference []MiniMaxImageSubjectReference
			if err := common.Unmarshal(raw, &subjectReference); err == nil {
				minimaxRequest.SubjectReference = subjectReference
			}
		}
	}
}

func unmarshalMiniMaxInt(raw json.RawMessage) *int {
	var value int
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return &value
}

func unmarshalMiniMaxInt64(raw json.RawMessage) *int64 {
	var value int64
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return &value
}

func unmarshalMiniMaxBool(raw json.RawMessage) *bool {
	var value bool
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return &value
}

func aspectRatioFromImageRequest(request dto.ImageRequest) string {
	if raw, ok := request.Extra["aspect_ratio"]; ok {
		var aspectRatio string
		if err := common.Unmarshal(raw, &aspectRatio); err == nil && aspectRatio != "" {
			return aspectRatio
		}
	}

	switch request.Size {
	case "1024x1024":
		return "1:1"
	case "1792x1024":
		return "16:9"
	case "1024x1792":
		return "9:16"
	case "1536x1024", "1248x832":
		return "3:2"
	case "1024x1536", "832x1248":
		return "2:3"
	case "1152x864":
		return "4:3"
	case "864x1152":
		return "3:4"
	case "1344x576":
		return "21:9"
	}

	width, height, ok := parseImageSize(request.Size)
	if !ok {
		return ""
	}
	ratio := reduceAspectRatio(width, height)
	switch ratio {
	case "1:1", "16:9", "4:3", "3:2", "2:3", "3:4", "9:16", "21:9":
		return ratio
	default:
		return ""
	}
}

func parseImageSize(size string) (int, int, bool) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func reduceAspectRatio(width, height int) string {
	divisor := gcd(width, height)
	return fmt.Sprintf("%d:%d", width/divisor, height/divisor)
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

func normalizeMiniMaxResponseFormat(responseFormat string) string {
	switch strings.ToLower(responseFormat) {
	case "", "url":
		return "url"
	case "b64_json", "base64":
		return "base64"
	default:
		return responseFormat
	}
}

func responseMiniMax2OpenAIImage(response *MiniMaxImageResponse, info *relaycommon.RelayInfo) (*dto.ImageResponse, error) {
	imageResponse := &dto.ImageResponse{
		Created: info.StartTime.Unix(),
	}

	for _, imageURL := range response.Data.ImageURLs {
		imageResponse.Data = append(imageResponse.Data, dto.ImageData{Url: imageURL})
	}
	for _, imageBase64 := range response.Data.ImageBase64 {
		imageResponse.Data = append(imageResponse.Data, dto.ImageData{B64Json: imageBase64})
	}
	if len(response.Metadata) > 0 {
		metadata, err := common.Marshal(response.Metadata)
		if err != nil {
			return nil, err
		}
		imageResponse.Metadata = metadata
	}

	return imageResponse, nil
}

func miniMaxImageHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	var minimaxResponse MiniMaxImageResponse
	if err := common.Unmarshal(responseBody, &minimaxResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if minimaxResponse.BaseResp.StatusCode != 0 {
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: minimaxResponse.BaseResp.StatusMsg,
			Type:    "minimax_image_error",
			Code:    fmt.Sprintf("%d", minimaxResponse.BaseResp.StatusCode),
		}, resp.StatusCode)
	}

	openAIResponse, err := responseMiniMax2OpenAIImage(&minimaxResponse, info)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	jsonResponse, err := common.Marshal(openAIResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := c.Writer.Write(jsonResponse); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	return &dto.Usage{}, nil
}

func miniMaxNativeImageHandler(c *gin.Context, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)

	return &dto.Usage{}, nil
}
