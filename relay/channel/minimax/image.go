package minimax

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
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
	if len(minimaxRequest.SubjectReference) == 0 {
		minimaxRequest.SubjectReference = subjectReferenceFromImageInput(request.Image, request.Images)
	}

	return minimaxRequest
}

// OpenAI edits 的 image/images 入参近似映射为 MiniMax subject_reference 角色参考图
func subjectReferenceFromImageInput(inputs ...json.RawMessage) []MiniMaxImageSubjectReference {
	var urls []string
	for _, raw := range inputs {
		if len(raw) == 0 {
			continue
		}
		var list []string
		if err := common.Unmarshal(raw, &list); err == nil {
			urls = append(urls, list...)
			continue
		}
		var single string
		if err := common.Unmarshal(raw, &single); err == nil && single != "" {
			urls = append(urls, single)
		}
	}
	refs := make([]MiniMaxImageSubjectReference, 0, len(urls))
	for _, u := range urls {
		if u = strings.TrimSpace(u); u != "" {
			refs = append(refs, MiniMaxImageSubjectReference{Type: "character", ImageFile: u})
		}
	}
	return refs
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

// ── MiniMax H3-Image(OpenAI images 兼容接口) ─────────────────────

// isMiniMaxH3ImageModel 判断是否走 H3-Image 新接口
func isMiniMaxH3ImageModel(model string) bool {
	return model == "MiniMax-H3-Image" || model == "MiniMax-H3-Image-Highspeed"
}

// H3ImageRequest 对齐 /v1/images/generations|edits 原生请求体
type H3ImageRequest struct {
	Model                 string          `json:"model"`
	Prompt                string          `json:"prompt"`
	N                     *uint           `json:"n,omitempty"`
	Size                  string          `json:"size,omitempty"`
	AspectRatio           string          `json:"aspect_ratio,omitempty"`
	Area                  string          `json:"area,omitempty"`
	OutputFormat          json.RawMessage `json:"output_format,omitempty"`
	OutputCompression     json.RawMessage `json:"output_compression,omitempty"`
	ResponseFormat        string          `json:"response_format,omitempty"`
	Stream                *bool           `json:"stream,omitempty"`
	DisablePromptEnhanced *bool           `json:"disable_prompt_enhanced,omitempty"`
	AigcWatermark         *bool           `json:"aigc_watermark,omitempty"`
	Images                json.RawMessage `json:"images,omitempty"`
}

// MiniMaxH3ImageUsage 只统计成功图片的 token 用量,按此计费
type MiniMaxH3ImageUsage struct {
	GeneratedImages    int `json:"generated_images,omitempty"`
	TotalTokens        int `json:"total_tokens,omitempty"`
	InputTokens        int `json:"input_tokens,omitempty"`
	OutputTokens       int `json:"output_tokens,omitempty"`
	InputTokensDetails struct {
		TextTokens  int `json:"text_tokens,omitempty"`
		ImageTokens int `json:"image_tokens,omitempty"`
	} `json:"input_tokens_details,omitempty"`
}

// oaiImage2H3ImageRequest 把网关图片请求转换为 H3-Image 原生请求
func oaiImage2H3ImageRequest(request dto.ImageRequest) *H3ImageRequest {
	h3Request := &H3ImageRequest{
		Model:             request.Model,
		Prompt:            request.Prompt,
		N:                 request.N,
		Size:              request.Size,
		OutputFormat:      request.OutputFormat,
		OutputCompression: request.OutputCompression,
		ResponseFormat:    normalizeMiniMaxResponseFormat(request.ResponseFormat),
		Stream:            request.Stream,
		AigcWatermark:     request.Watermark,
		Images:            h3ImagesFromImageInput(request.Images, request.Image),
	}
	if aspectRatio, ok := request.Extra["aspect_ratio"]; ok {
		var value string
		if common.Unmarshal(aspectRatio, &value) == nil && value != "" {
			h3Request.AspectRatio = value
		}
	}
	if area, ok := request.Extra["area"]; ok {
		var value string
		if common.Unmarshal(area, &value) == nil && value != "" {
			h3Request.Area = value
		}
	}
	if disable, ok := request.Extra["disable_prompt_enhanced"]; ok {
		var value bool
		if common.Unmarshal(disable, &value) == nil {
			h3Request.DisablePromptEnhanced = &value
		}
	}
	return h3Request
}

// h3ImagesFromImageInput 把 OpenAI edits 的 images 入参归一为 H3 的 images[].image_url 形状
func h3ImagesFromImageInput(inputs ...json.RawMessage) json.RawMessage {
	for _, raw := range inputs {
		if len(raw) == 0 {
			continue
		}
		// 已是 [{image_url:{url}}] 形状则原样透传
		var objectItems []map[string]any
		if common.Unmarshal(raw, &objectItems) == nil && len(objectItems) > 0 {
			if _, ok := objectItems[0]["image_url"]; ok {
				return raw
			}
		}
		var urls []string
		if common.Unmarshal(raw, &urls) == nil && len(urls) > 0 {
			items := make([]map[string]any, 0, len(urls))
			for _, u := range urls {
				if u = strings.TrimSpace(u); u != "" {
					items = append(items, map[string]any{"image_url": map[string]string{"url": u}})
				}
			}
			if len(items) > 0 {
				data, _ := common.Marshal(items)
				return data
			}
		}
		var single string
		if common.Unmarshal(raw, &single) == nil && strings.TrimSpace(single) != "" {
			data, _ := common.Marshal([]map[string]any{{"image_url": map[string]string{"url": single}}})
			return data
		}
	}
	return nil
}

// h3ImageUsageToUsage 把 H3-Image 用量映射为站内计费 Usage:文本入走基础倍率,
// 图片入走 ImageRatio,图片出走 completion 倍率
func h3ImageUsageToUsage(u *MiniMaxH3ImageUsage) *dto.Usage {
	if u == nil {
		return &dto.Usage{}
	}
	usage := &dto.Usage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.TotalTokens,
	}
	usage.PromptTokensDetails.TextTokens = u.InputTokensDetails.TextTokens
	usage.PromptTokensDetails.ImageTokens = u.InputTokensDetails.ImageTokens
	return usage
}

// miniMaxH3ImageHandler 处理 H3-Image 响应:200 时 body 原样透传(部分失败保留 data[].error),
// 从 usage 提取 token 计费;流式走 SSE 透传并在 completed 事件取 usage
func miniMaxH3ImageHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return miniMaxH3ImageStreamHandler(c, resp, info)
	}

	defer service.CloseResponseBodyGracefully(resp)
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var h3Resp struct {
		Usage *MiniMaxH3ImageUsage `json:"usage"`
	}
	if unmarshalErr := common.Unmarshal(responseBody, &h3Resp); unmarshalErr != nil {
		// 解析失败仍透传原文,计费走 usage 缺失路径
		service.IOCopyBytesGracefully(c, resp, responseBody)
		return &dto.Usage{}, nil
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	return h3ImageUsageToUsage(h3Resp.Usage), nil
}

// miniMaxH3ImageStreamHandler 逐行转发 H3-Image SSE 事件,从 completed/error 事件提取 usage
func miniMaxH3ImageStreamHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	var finalUsage *MiniMaxH3ImageUsage

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if payload, ok := strings.CutPrefix(line, "data:"); ok {
			payload = strings.TrimSpace(payload)
			if payload != "" {
				var event struct {
					Type  string               `json:"type"`
					Usage *MiniMaxH3ImageUsage `json:"usage"`
				}
				if common.Unmarshal([]byte(payload), &event) == nil && event.Usage != nil {
					if strings.HasSuffix(event.Type, ".completed") || event.Type == "error" {
						finalUsage = event.Usage
					}
				}
			}
		}
		if _, writeErr := fmt.Fprintf(c.Writer, "%s\n", line); writeErr != nil {
			break
		}
		c.Writer.Flush()
	}

	return h3ImageUsageToUsage(finalUsage), nil
}
