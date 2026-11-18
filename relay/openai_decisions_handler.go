package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	dto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// OpenAIDecisionsHelper 处理原生 POST /v1/decisions 入站:请求体透传,仅做渠道校验与 model 映射。
func OpenAIDecisionsHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	decisionReq, ok := info.Request.(*kitdto.OpenAIDecisionsRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *kitdto.OpenAIDecisionsRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	// Decisions 属于 OpenAI 原生端点,仅放行 OpenAI 渠道
	if info.ChannelType != constant.ChannelTypeOpenAI {
		return types.NewErrorWithStatusCode(fmt.Errorf("openai decisions API only supports openai channels, got channel type %d", info.ChannelType), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(decisionReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to OpenAIDecisionsRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	if err = helper.ModelMappedHelper(c, info, request); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	var requestBody io.Reader
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		body, _, err := relaycommon.PassThroughRequestBody(c, info)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		requestBody = body
	} else {
		jsonData, err := common.Marshal(request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}

		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		info.UpstreamRequestBodySize = size
		requestBody = body
	}

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		logger.LogError(c, "Do openai decisions request failed: "+err.Error())
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	httpResp, ok := resp.(*http.Response)
	if !ok || httpResp == nil {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid response type: %T", resp), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	if httpResp.StatusCode != http.StatusOK {
		newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	usage, openaiErr := adaptor.DoResponse(c, httpResp, info)
	if openaiErr != nil {
		service.ResetStatusCode(openaiErr, statusCodeMappingStr)
		return openaiErr
	}

	billingUsage, _ := usage.(*kitdto.Usage)
	service.PostTextConsumeQuota(c, info, billingUsage, nil)
	return nil
}

// OpenAIDecisionsViaSystemOneHelper 将 /v1/systemone(TypeSafe 协议)入站转换为
// OpenAI Decisions 原生调用:questions 映射表转带 name 的数组(noul→predicate)、
// criteria/levels 字段映射、assets 引用图转带标签的 input_image 内容,
// 响应 answers 数组按 name 回填 SystemOne 映射表形状。
func OpenAIDecisionsViaSystemOneHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	systemOneReq, ok := info.Request.(*dto.SystemOneRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *dto.SystemOneRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	if info.ChannelType != constant.ChannelTypeOpenAI {
		return types.NewErrorWithStatusCode(errors.New("the /v1/systemone endpoint on openai channels only serves decisions requests"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	if err := helper.ModelMappedHelper(c, info, systemOneReq); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	if len(systemOneReq.Questions) == 0 {
		return types.NewErrorWithStatusCode(errors.New("questions is required"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if len(systemOneReq.State) == 0 {
		return types.NewErrorWithStatusCode(errors.New("state is required"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	decisionQuestions, err := systemOneToOpenAIDecisionsQuestions(systemOneReq.Questions)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeConvertRequestFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	decisionInput, err := buildOpenAIDecisionsInputFromState(systemOneReq.State, systemOneReq.Assets)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeConvertRequestFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	decisionReq := &kitdto.OpenAIDecisionsRequest{
		Model:     info.UpstreamModelName,
		Input:     decisionInput,
		Questions: decisionQuestions,
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	jsonData, err := common.Marshal(decisionReq)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return newAPIErrorFromParamOverride(err)
		}
	}

	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	resp, err := adaptor.DoRequest(c, info, body)
	if err != nil {
		logger.LogError(c, "Do openai decisions (systemone) request failed: "+err.Error())
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	httpResp, ok := resp.(*http.Response)
	if !ok || httpResp == nil {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid response type: %T", resp), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	if httpResp.StatusCode != http.StatusOK {
		newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	usage, respErr := adaptor.DoResponse(c, httpResp, info)
	if respErr != nil {
		service.ResetStatusCode(respErr, statusCodeMappingStr)
		return respErr
	}

	billingUsage, _ := usage.(*kitdto.Usage)
	service.PostTextConsumeQuota(c, info, billingUsage, nil)
	return nil
}

// systemOneQuestion 对齐 TypeSafe 题面字段;instructions/criteria 允许字符串或任意结构
type systemOneQuestion struct {
	Type         string          `json:"type"`
	Instructions json.RawMessage `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria"`
}

// systemOneToOpenAIDecisionsQuestions 把 TypeSafe questions 映射表转换为 OpenAI 带 name 的数组:
// noul→predicate、choice→choice(criteria→choices)、score→score(criteria→levels)、box 直接拒绝。
// 键名稳定排序保证 answers 顺序可复现;非字符串 instructions/criteria 序列化为紧凑 JSON 文本。
func systemOneToOpenAIDecisionsQuestions(questions json.RawMessage) (json.RawMessage, error) {
	var parsed map[string]systemOneQuestion
	if err := common.Unmarshal(questions, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse questions: %w", err)
	}
	if len(parsed) == 0 {
		return nil, errors.New("questions must not be empty")
	}

	names := make([]string, 0, len(parsed))
	for name := range parsed {
		names = append(names, name)
	}
	sort.Strings(names)

	converted := make([]any, 0, len(parsed))
	for _, name := range names {
		question := parsed[name]
		switch strings.ToLower(strings.TrimSpace(question.Type)) {
		case "noul":
			converted = append(converted, map[string]any{
				"type":         "predicate",
				"name":         name,
				"instructions": systemOneInstructionText(question.Instructions, question.Criteria),
			})
		case "choice":
			choices, err := systemOneChoiceCriteria(question.Criteria)
			if err != nil {
				return nil, fmt.Errorf("question %q: %w", name, err)
			}
			converted = append(converted, map[string]any{
				"type":         "choice",
				"name":         name,
				"instructions": systemOneInstructionText(question.Instructions, nil),
				"choices":      choices,
			})
		case "score":
			levels, err := systemOneScoreCriteria(question.Criteria)
			if err != nil {
				return nil, fmt.Errorf("question %q: %w", name, err)
			}
			converted = append(converted, map[string]any{
				"type":         "score",
				"name":         name,
				"instructions": systemOneInstructionText(question.Instructions, nil),
				"levels":       levels,
			})
		case "box":
			return nil, fmt.Errorf("question %q: the box question type is not supported by openai decisions", name)
		default:
			return nil, fmt.Errorf("question %q: unsupported question type %q", name, question.Type)
		}
	}
	return common.Marshal(converted)
}

// systemOneInstructionText 把 instructions 展开为文本;noul 的 true/false criteria
// 一并并入指令文本(predicate 无 criteria 字段)。
func systemOneInstructionText(instructions json.RawMessage, extra json.RawMessage) string {
	parts := make([]string, 0, 2)
	if text := systemOneValueText(instructions); text != "" {
		parts = append(parts, text)
	}
	if len(extra) > 0 && string(extra) != "null" {
		if text := systemOneValueText(extra); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

// systemOneValueText 把字符串值原样返回,结构化值序列化为紧凑 JSON 文本。
func systemOneValueText(value json.RawMessage) string {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	if strings.HasPrefix(trimmed, `"`) {
		var text string
		if err := common.Unmarshal(value, &text); err == nil {
			return text
		}
	}
	return trimmed
}

// systemOneChoiceCriteria 把 criteria 映射表转换为 OpenAI choices(键→value,值→description)。
func systemOneChoiceCriteria(criteria json.RawMessage) ([]any, error) {
	var parsed map[string]json.RawMessage
	if err := common.Unmarshal(criteria, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse choice criteria: %w", err)
	}
	if len(parsed) < 2 {
		return nil, errors.New("choice criteria must contain at least 2 options")
	}
	if len(parsed) > 255 {
		return nil, errors.New("choice criteria must contain at most 255 options")
	}

	names := make([]string, 0, len(parsed))
	for name := range parsed {
		names = append(names, name)
	}
	sort.Strings(names)

	choices := make([]any, 0, len(parsed))
	for _, name := range names {
		choice := map[string]any{"value": name}
		if description := systemOneValueText(parsed[name]); description != "" {
			choice["description"] = description
		}
		choices = append(choices, choice)
	}
	return choices, nil
}

// systemOneScoreCriteria 把 criteria 数组转换为 OpenAI levels(下标即 0-based 层级)。
func systemOneScoreCriteria(criteria json.RawMessage) ([]any, error) {
	var parsed []json.RawMessage
	if err := common.Unmarshal(criteria, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse score criteria: %w", err)
	}
	if len(parsed) < 2 {
		return nil, errors.New("score criteria must contain at least 2 levels")
	}
	if len(parsed) > 10 {
		return nil, errors.New("score criteria must contain at most 10 levels")
	}

	levels := make([]any, 0, len(parsed))
	for index, item := range parsed {
		level := map[string]any{"label": fmt.Sprintf("%d", index)}
		if description := systemOneValueText(item); description != "" {
			level["description"] = description
		}
		levels = append(levels, level)
	}
	return levels, nil
}

// buildOpenAIDecisionsInputFromState 把 TypeSafe state 转换为 OpenAI Decisions input:
//   - 纯字符串 state 无资产:原样作为 input 字符串
//   - 其余形态:单条 user 消息内容数组(结构化 state 走 input_json,
//     assets 引用图转带标签的 input_text+input_image 对,multipart part 形式明确报错)
func buildOpenAIDecisionsInputFromState(state json.RawMessage, assets json.RawMessage) (json.RawMessage, error) {
	parsedAssets, err := parseSystemOneAssets(assets)
	if err != nil {
		return nil, err
	}

	var stateString string
	if err := common.Unmarshal(state, &stateString); err == nil {
		if len(parsedAssets) == 0 {
			return state, nil
		}
		if _, found := parsedAssets[stateString]; found {
			// 整值引用:state 即引用串,模型直接收到该图片
			content, imgErr := systemOneAssetImageContent(parsedAssets[stateString], stateString)
			if imgErr != nil {
				return nil, imgErr
			}
			return common.Marshal([]any{map[string]any{
				"role":    "user",
				"type":    "message",
				"content": content,
			}})
		}
	}

	content := make([]any, 0, 1+2*len(parsedAssets))
	if len(stateString) > 0 {
		content = append(content, map[string]any{"type": "input_text", "text": stateString})
	} else {
		content = append(content, map[string]any{"type": "input_json", "data": json.RawMessage(state)})
	}
	for _, ref := range sortedAssetRefs(parsedAssets) {
		labeled, imgErr := systemOneAssetImageContent(parsedAssets[ref], ref)
		if imgErr != nil {
			return nil, imgErr
		}
		content = append(content, labeled...)
	}
	return common.Marshal([]any{map[string]any{
		"role":    "user",
		"type":    "message",
		"content": content,
	}})
}

// systemOneAssetImageContent 生成 [引用标签文本块, 图片块];OpenAI 图片走 data URL 内联。
func systemOneAssetImageContent(asset systemOneAsset, ref string) ([]any, error) {
	if asset.DataBase64 == "" {
		return nil, fmt.Errorf("asset %q has no data_base64", ref)
	}
	var mimeType string
	switch strings.ToLower(strings.TrimSpace(asset.Format)) {
	case "png":
		mimeType = "image/png"
	case "jpg", "jpeg":
		mimeType = "image/jpeg"
	default:
		return nil, fmt.Errorf("asset %q has unsupported image format %q (want png/jpg/jpeg)", ref, asset.Format)
	}
	return []any{
		map[string]any{"type": "input_text", "text": ref + ":"},
		map[string]any{"type": "input_image", "image_url": "data:" + mimeType + ";base64," + asset.DataBase64},
	}, nil
}
