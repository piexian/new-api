package gmicloud

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
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// Hunyuan Image 3.5 Preview 与 TTS/音乐共用 requestqueue 信封，差异只在 payload 字段和 outcome.media_urls。
// 上游 size 只接受精确枚举；本地额外接受 "4k+16:9"、"16:9"、"4k" 这类简写，解析后一律回落到枚举再发上游，
// 非法取值在本地 400，不浪费上游额度。

const (
	// gmiImageOver2KPriceRatio 是 4K 档（> 4194304 像素）相对 2K 档的价格倍数：0.032 / 0.024。
	gmiImageOver2KPriceRatio = 4.0 / 3.0
	// gmiImageMaxReferenceImages 是上游单次请求允许的参考图数量。
	gmiImageMaxReferenceImages = 5
	// gmiImageAutoSize 交由上游按 prompt 和像素预算自选尺寸。
	gmiImageAutoSize = ""
)

// gmiImageSize 是上游接受的一个精确尺寸，over2K 表示像素超过 4194304，按 4K 档计费。
type gmiImageSize struct {
	size   string
	tier   string
	ratio  string
	over2K bool
}

// gmiImageSizeAuto 表示不传 size。
var gmiImageSizeAuto = gmiImageSize{size: gmiImageAutoSize, tier: "auto"}

// gmiImageSizes 覆盖上游文档给出的全部精确枚举，tier/ratio 用于本地简写解析。
var gmiImageSizes = []gmiImageSize{
	{size: "1024x1024", tier: "1k", ratio: "1:1"},
	{size: "1536x1536", tier: "1.5k", ratio: "1:1"},
	{size: "2048x2048", tier: "2k", ratio: "1:1"},
	{size: "4096x4096", tier: "4k", ratio: "1:1", over2K: true},
	{size: "1920x1080", tier: "2k", ratio: "16:9"},
	{size: "2560x1440", tier: "qhd", ratio: "16:9"},
	{size: "3840x2160", tier: "4k", ratio: "16:9", over2K: true},
	{size: "1080x1920", tier: "2k", ratio: "9:16"},
	{size: "1440x2560", tier: "qhd", ratio: "9:16"},
	{size: "2160x3840", tier: "4k", ratio: "9:16", over2K: true},
	{size: "1536x1152", tier: "2k", ratio: "4:3"},
	{size: "1152x1536", tier: "2k", ratio: "3:4"},
}

// gmiImageSizeTiers / gmiImageSizeRatios 列出可用的简写档位与比例。
var gmiImageSizeTiers = []string{"1k", "1.5k", "2k", "qhd", "4k"}

var gmiImageSizeRatios = []string{"1:1", "16:9", "9:16", "4:3", "3:4"}

// gmiImagePixelBudgets 是 generate_max_pixels 的合法取值（仅在 size 为空时生效，上游封顶 2K）。
var gmiImagePixelBudgets = []int64{1048576, 2359296, 4194304}

// resolveGMISize 把客户端传入的 size 解析为上游精确枚举。
// 接受：精确枚举（1024x1024）、档位（4k）、比例（16:9）、组合简写（4k+16:9）、空值（Auto）。
func resolveGMISize(raw string) (gmiImageSize, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" || value == "auto" {
		return gmiImageSizeAuto, nil
	}

	for _, candidate := range gmiImageSizes {
		if value == candidate.size {
			return candidate, nil
		}
	}

	tier, ratio, ok := splitGMISizeShorthand(value)
	if !ok {
		return gmiImageSize{}, invalidGMISizeError(raw)
	}
	// 缺比例按 1:1 解析，缺档位时取该比例最小的可用档。
	if ratio == "" {
		ratio = "1:1"
	}
	if tier == "" {
		tier = smallestTierForRatio(ratio)
	}
	for _, candidate := range gmiImageSizes {
		if candidate.tier == tier && candidate.ratio == ratio {
			return candidate, nil
		}
	}
	return gmiImageSize{}, invalidGMISizeError(raw)
}

// splitGMISizeShorthand 拆出简写里的档位和比例，任一段不是合法档位/比例即判失败。
func splitGMISizeShorthand(value string) (tier string, ratio string, ok bool) {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '+' || r == '-' || r == '_' || r == ' ' || r == '\t'
	})
	switch len(parts) {
	case 1:
		if isGMISizeTier(parts[0]) {
			return parts[0], "", true
		}
		if isGMISizeRatio(parts[0]) {
			return "", parts[0], true
		}
	case 2:
		if isGMISizeTier(parts[0]) && isGMISizeRatio(parts[1]) {
			return parts[0], parts[1], true
		}
	}
	return "", "", false
}

// smallestTierForRatio 返回该比例下像素最少的一档，用作只给比例时的缺省档位。
func smallestTierForRatio(ratio string) string {
	for _, candidate := range gmiImageSizes {
		if candidate.ratio == ratio {
			return candidate.tier
		}
	}
	return ""
}

func isGMISizeTier(value string) bool {
	for _, tier := range gmiImageSizeTiers {
		if value == tier {
			return true
		}
	}
	return false
}

func isGMISizeRatio(value string) bool {
	for _, ratio := range gmiImageSizeRatios {
		if value == ratio {
			return true
		}
	}
	return false
}

func invalidGMISizeError(raw string) error {
	exact := make([]string, 0, len(gmiImageSizes))
	for _, candidate := range gmiImageSizes {
		exact = append(exact, candidate.size)
	}
	return types.NewErrorWithStatusCode(
		fmt.Errorf("gmicloud: unsupported size %q, supported sizes: %s, or shorthand %s + %s (e.g. 4k+16:9)",
			strings.TrimSpace(raw), strings.Join(exact, ", "),
			strings.Join(gmiImageSizeTiers, "/"), strings.Join(gmiImageSizeRatios, "/")),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

// collectReferenceImages 取出参考图 URL：image 或 images 字段，单个字符串、字符串数组、
// 以及数组内逗号分隔的写法都接受。
func collectReferenceImages(request dto.ImageRequest) ([]string, error) {
	raw := firstNonEmptyRaw(request.Image, request.Images)
	if len(raw) == 0 {
		return nil, nil
	}

	var urls []string
	var single string
	if err := common.Unmarshal(raw, &single); err == nil {
		urls = append(urls, single)
	} else {
		if err := common.Unmarshal(raw, &urls); err != nil {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("gmicloud: image must be a URL string or an array of URL strings"),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	normalized := make([]string, 0, len(urls))
	for _, value := range urls {
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if !isHTTPURL(item) {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("gmicloud: reference image must be a public HTTP(S) URL: %s", item),
					types.ErrorCodeInvalidRequest,
					http.StatusBadRequest,
					types.ErrOptionWithSkipRetry(),
				)
			}
			normalized = append(normalized, item)
		}
	}
	if len(normalized) > gmiImageMaxReferenceImages {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: at most %d reference images are supported, got %d", gmiImageMaxReferenceImages, len(normalized)),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	return normalized, nil
}

func firstNonEmptyRaw(values ...json.RawMessage) json.RawMessage {
	for _, value := range values {
		if len(value) > 0 && string(value) != "null" {
			return value
		}
	}
	return nil
}

// imageExtraInt 读取请求体里的整数扩展参数（如 seed、generate_max_pixels）。
func imageExtraInt(request dto.ImageRequest, key string) (int64, bool, error) {
	raw, ok := request.Extra[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return 0, false, nil
	}
	var number json.Number
	if err := common.Unmarshal(raw, &number); err != nil {
		return 0, false, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: %s must be an integer", key),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	parsed, err := number.Int64()
	if err != nil {
		return 0, false, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: %s must be an integer", key),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	return parsed, true, nil
}

// buildImageRequest 校验 OpenAI 图片入参并转成 GMI requestqueue 信封。
// 文生图与图生图共用同一个上游端点，区别只在 payload.image 是否带参考图。
func buildImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (gmiSubmitRequest, error) {
	model := gmiModelName(info)
	if !IsSupportedImageModel(model) {
		return gmiSubmitRequest{}, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud model %q does not support /v1/images/generations", model),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	if info.RelayMode != relayconstant.RelayModeImagesGenerations &&
		info.RelayMode != relayconstant.RelayModeImagesEdits {
		return gmiSubmitRequest{}, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud only serves /v1/images/generations and /v1/images/edits for image models"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	prompt := strings.TrimSpace(request.Prompt)
	if prompt == "" {
		return gmiSubmitRequest{}, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: prompt is required"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	// 上游一次 submit 只回一张图，放行 n>1 会按 n 倍扣费却只出一张。
	if request.N != nil && *request.N > 1 {
		return gmiSubmitRequest{}, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: n must be 1, the upstream returns exactly one image per request"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	// 上游只回 URL，b64_json 无法满足，静默返回 URL 会让客户端解析失败。
	if format := strings.ToLower(strings.TrimSpace(request.ResponseFormat)); format == "b64_json" {
		return gmiSubmitRequest{}, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: response_format b64_json is not supported, the upstream only returns image URLs"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	// 参考图由上游自己去拉，文件上传没有可公开访问的地址，直接拒绝避免静默退化成文生图。
	if err := rejectUploadedImages(c); err != nil {
		return gmiSubmitRequest{}, err
	}

	references, err := collectReferenceImages(request)
	if err != nil {
		return gmiSubmitRequest{}, err
	}
	if info.RelayMode == relayconstant.RelayModeImagesEdits && len(references) == 0 {
		return gmiSubmitRequest{}, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: /v1/images/edits requires at least one reference image URL"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	size, err := resolveGMISize(request.Size)
	if err != nil {
		return gmiSubmitRequest{}, err
	}

	seed, hasSeed, err := imageExtraInt(request, "seed")
	if err != nil {
		return gmiSubmitRequest{}, err
	}
	if hasSeed && seed < 0 {
		return gmiSubmitRequest{}, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: seed must not be negative"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	maxPixels, hasMaxPixels, err := imageExtraInt(request, "generate_max_pixels")
	if err != nil {
		return gmiSubmitRequest{}, err
	}
	if hasMaxPixels && !isGMIPixelBudget(maxPixels) {
		return gmiSubmitRequest{}, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: generate_max_pixels must be one of %s", formatInt64List(gmiImagePixelBudgets)),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	payload := map[string]any{"prompt": prompt}
	if size.size != gmiImageAutoSize {
		payload["size"] = size.size
	} else if hasMaxPixels {
		// 像素预算只在不指定 size 时生效，且封顶 2K。
		payload["generate_max_pixels"] = maxPixels
	}
	if hasSeed {
		payload["seed"] = seed
	}
	if len(references) > 0 {
		payload["image"] = references
	}

	// 4K 档按 0.032/张 计费，2K 及以下 0.024/张，倍率在结算时会乘进最终额度。
	if size.over2K && info != nil {
		info.PriceData.AddOtherRatio("size", gmiImageOver2KPriceRatio)
	}
	// 消费日志记实际计费尺寸：简写入参无法从日志反推落在哪一档。
	recordImageLogSize(c, size)

	return gmiSubmitRequest{Model: model, Payload: payload}, nil
}

func rejectUploadedImages(c *gin.Context) error {
	if c == nil || c.Request == nil || c.Request.MultipartForm == nil || len(c.Request.MultipartForm.File) == 0 {
		return nil
	}
	return types.NewErrorWithStatusCode(
		fmt.Errorf("gmicloud: uploaded image files are not supported, pass public HTTP(S) URLs in the image field instead"),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

// recordImageLogSize 把解析后的精确尺寸写进图片日志明细，ImageHelper 会优先沿用。
func recordImageLogSize(c *gin.Context, size gmiImageSize) {
	if c == nil {
		return
	}
	value := size.size
	if value == gmiImageAutoSize {
		value = "auto"
	}
	c.Set("image_request_detail", map[string]interface{}{"size": value})
}

func isGMIPixelBudget(value int64) bool {
	for _, budget := range gmiImagePixelBudgets {
		if value == budget {
			return true
		}
	}
	return false
}

func formatInt64List(values []int64) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatInt(value, 10))
	}
	return strings.Join(parts, ", ")
}

// handleImageResponse 把 GMI requestqueue 结果转成 OpenAI 图片响应。
func handleImageResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	result, apiErr := submitGMIImage(c, info, resp)
	if apiErr != nil {
		return nil, apiErr
	}

	imageResponse := dto.ImageResponse{Created: gmiImageCreatedAt(result, info)}
	for _, url := range extractImageURLs(result.Outcome) {
		imageResponse.Data = append(imageResponse.Data, dto.ImageData{Url: url})
	}
	encoded, err := common.Marshal(imageResponse)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	c.Data(http.StatusOK, "application/json", encoded)

	// 按张计费，token 数只用于让消费日志有非零用量。
	return &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil
}

// submitGMIImage 提交 requestqueue 并等到终态，返回带结果的响应。
// 提交是同步的（10-60 秒直接返回终态），只有拿到非终态时才继续轮询。
func submitGMIImage(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*gmiStatusResponse, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: empty image submit response"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: read image submit response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusBadGateway,
		)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, gmiUpstreamHTTPError("image submit", resp.StatusCode, body)
	}

	// 提交响应与状态响应同形，直接按状态响应解析，成功时省掉一次轮询。
	var result gmiStatusResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: decode image submit response: %w", err),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}

	if result.Status != "success" || result.Outcome == nil {
		polled, pollErr := pollGMIResult(c, info, result.RequestID, result.Status)
		if pollErr != nil {
			return nil, gmiTaskCreatedError(pollErr)
		}
		result = *polled
	}

	if len(extractImageURLs(result.Outcome)) == 0 {
		return nil, gmiTaskCreatedError(types.NewErrorWithStatusCode(
			fmt.Errorf("gmicloud: no image URL in outcome for request %s", result.RequestID),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		))
	}
	return &result, nil
}

func gmiImageCreatedAt(result *gmiStatusResponse, info *relaycommon.RelayInfo) int64 {
	if result != nil {
		if result.CreatedAt > 0 {
			return result.CreatedAt
		}
		if result.UpdatedAt > 0 {
			return result.UpdatedAt
		}
	}
	if info != nil && !info.StartTime.IsZero() {
		return info.StartTime.Unix()
	}
	return common.GetTimestamp()
}

func extractImageURLs(outcome *gmiOutcome) []string {
	if outcome == nil {
		return nil
	}
	urls := make([]string, 0, len(outcome.MediaURLs)+1)
	seen := make(map[string]struct{}, len(outcome.MediaURLs)+1)
	appendURL := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		urls = append(urls, value)
	}
	for _, media := range outcome.MediaURLs {
		appendURL(media.URL)
	}
	for _, media := range outcome.Medias {
		appendURL(media.URL)
	}
	if len(urls) == 0 {
		appendURL(outcome.ThumbnailImageURL)
	}
	return urls
}
