package gmicloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// chat 端点的图像生成：prompt 取最后一条 user 消息文本，参考图取消息里的 image_url，
// 尺寸走顶层 size（与图片端点同一套枚举校验），不传即 Auto，由上游按 prompt 和像素预算自选。

// IsChatEndpointRequest 判断是否为 OpenAI Chat 入站：/v1/chat/completions 与 /v1/messages 在
// Path2RelayMode 里都落到 RelayModeUnknown，chat 白名单必须同时包含这两个模式。
func IsChatEndpointRequest(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	return info.RelayMode == relayconstant.RelayModeUnknown || info.RelayMode == relayconstant.RelayModeChatCompletions
}

// buildChatImageRequest 把 chat 请求转成 GMI requestqueue 信封。
func buildChatImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, gmiImageError("request is nil")
	}
	model := gmiModelName(info)
	if !IsSupportedImageModel(model) {
		return nil, gmiImageError(fmt.Sprintf("gmicloud model %q does not support /v1/chat/completions", model))
	}

	// 提交是同步的一次性生成，没有可增量输出的中间态。
	if request.Stream != nil && *request.Stream {
		return nil, gmiImageError("gmicloud: streaming image generation on /v1/chat/completions is not supported")
	}
	// 上游一次 submit 只回一张图，放行 n>1 会按 n 倍扣费却只出一张。
	if request.N != nil && *request.N > 1 {
		return nil, gmiImageError("gmicloud: n must be 1, the upstream returns exactly one image per request")
	}

	prompt, references, err := collectChatImageInput(request.Messages)
	if err != nil {
		return nil, err
	}
	if prompt == "" {
		return nil, gmiImageError("gmicloud: the last user message must contain a text prompt")
	}

	size, err := resolveGMISize(request.Size)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{"prompt": prompt}
	if size.size != gmiImageAutoSize {
		payload["size"] = size.size
	} else if maxPixels, ok, budgetErr := chatImageMaxPixels(request.ExtraBody); budgetErr != nil {
		return nil, budgetErr
	} else if ok {
		payload["generate_max_pixels"] = maxPixels
	}
	if request.Seed != nil {
		seed := int64(*request.Seed)
		if float64(seed) != *request.Seed || seed < 0 {
			return nil, gmiImageError("gmicloud: seed must be a non-negative integer")
		}
		payload["seed"] = seed
	}
	if len(references) > 0 {
		payload["image"] = references
	}

	// 4K 档按 0.032/张 计费，2K 及以下 0.024/张，倍率在结算时会乘进最终额度。
	if size.over2K && info != nil {
		info.PriceData.AddOtherRatio("size", gmiImageOver2KPriceRatio)
	}
	recordImageLogSize(c, size)

	return gmiSubmitRequest{Model: model, Payload: payload}, nil
}

// collectChatImageInput 取最后一条 user 消息的文本作为 prompt，并收集全部消息里的参考图 URL。
func collectChatImageInput(messages []dto.Message) (string, []string, error) {
	var promptParts []string
	promptFound := false
	var references []string
	seen := make(map[string]struct{})

	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		var texts []string
		for _, part := range message.ParseContent() {
			switch part.Type {
			case dto.ContentTypeText:
				if text := strings.TrimSpace(part.Text); text != "" {
					texts = append(texts, text)
				}
			case dto.ContentTypeImageURL:
				media := part.GetImageMedia()
				if media == nil {
					continue
				}
				url := strings.TrimSpace(media.Url)
				if url == "" {
					continue
				}
				// 上游自己去拉参考图，data: URL 没有可公开访问的地址。
				if !isHTTPURL(url) {
					return "", nil, gmiImageError("gmicloud: reference image must be a public HTTP(S) URL, data URLs are not supported")
				}
				if _, ok := seen[url]; ok {
					continue
				}
				seen[url] = struct{}{}
				references = append(references, url)
			}
		}
		// prompt 取最后一条带文本的 user 消息，但不中断扫描：参考图要收集全部消息里的。
		if !promptFound && len(texts) > 0 && message.Role == "user" {
			promptParts = texts
			promptFound = true
		}
	}

	if len(references) > gmiImageMaxReferenceImages {
		return "", nil, gmiImageError(fmt.Sprintf("gmicloud: at most %d reference images are supported, got %d", gmiImageMaxReferenceImages, len(references)))
	}
	return strings.Join(promptParts, "\n"), references, nil
}

// chatImageMaxPixels 读取 extra_body.generate_max_pixels，仅在 size 为空时生效。
func chatImageMaxPixels(extraBody json.RawMessage) (int64, bool, error) {
	if len(extraBody) == 0 {
		return 0, false, nil
	}
	var body map[string]json.RawMessage
	if err := common.Unmarshal(extraBody, &body); err != nil {
		return 0, false, gmiImageError("gmicloud: extra_body must be a JSON object")
	}
	raw, ok := body["generate_max_pixels"]
	if !ok || len(raw) == 0 {
		return 0, false, nil
	}
	var number json.Number
	if err := common.Unmarshal(raw, &number); err != nil {
		return 0, false, gmiImageError("gmicloud: generate_max_pixels must be an integer")
	}
	value, err := number.Int64()
	if err != nil || !isGMIPixelBudget(value) {
		return 0, false, gmiImageError(fmt.Sprintf("gmicloud: generate_max_pixels must be one of %s", formatInt64List(gmiImagePixelBudgets)))
	}
	return value, true, nil
}

func gmiImageError(message string) error {
	return types.NewErrorWithStatusCode(
		fmt.Errorf("%s", message),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

// handleChatImageResponse 把 GMI requestqueue 结果转成 chat.completion，
// 图片以 markdown 链接放进 assistant content（与 Gemini 图片模型的回包形态一致）。
func handleChatImageResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	result, apiErr := submitGMIImage(c, info, resp)
	if apiErr != nil {
		return nil, apiErr
	}

	links := make([]string, 0, 2)
	for _, url := range extractImageURLs(result.Outcome) {
		links = append(links, "![image]("+url+")")
	}

	usage := &dto.Usage{PromptTokens: 1, TotalTokens: 1}
	response := dto.OpenAITextResponse{
		Id:      helper.GetResponseID(c),
		Object:  "chat.completion",
		Created: gmiImageCreatedAt(result, info),
		Model:   info.OriginModelName,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: strings.Join(links, "\n"),
				},
				FinishReason: "stop",
			},
		},
		Usage: *usage,
	}
	encoded, err := common.Marshal(response)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	c.Data(http.StatusOK, "application/json", encoded)

	// 按张计费，token 数只用于让消费日志有非零用量。
	return usage, nil
}
