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

// GeminiDecisionsHelper 处理原生 POST /v1beta/decisions 入站:请求体透传,仅做渠道校验与 model 映射。
func GeminiDecisionsHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	decisionReq, ok := info.Request.(*kitdto.GeminiDecisionsRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *kitdto.GeminiDecisionsRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	// Decisions 属于 Gemini 原生端点,仅放行 Gemini 渠道
	if info.ChannelType != constant.ChannelTypeGemini {
		return types.NewErrorWithStatusCode(fmt.Errorf("gemini decisions API only supports gemini channels, got channel type %d", info.ChannelType), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(decisionReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeminiDecisionsRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
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
		logger.LogError(c, "Do gemini decisions request failed: "+err.Error())
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

// GeminiDecisionsViaSystemOneHelper 将 /v1/systemone(TypeSafe 协议)入站转换为
// Gemini Decisions 原生调用:state→input、questions 同形透传(Box 题型 Gemini 不支持),
// assets 图片按引用映射为带标签的内联图片块,响应回填 SystemOne 形状。
func GeminiDecisionsViaSystemOneHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	systemOneReq, ok := info.Request.(*dto.SystemOneRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *dto.SystemOneRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	if info.ChannelType != constant.ChannelTypeGemini {
		return types.NewErrorWithStatusCode(errors.New("the /v1/systemone endpoint on gemini channels only supports gemini decisions models"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	if err := helper.ModelMappedHelper(c, info, systemOneReq); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	if !kitdto.IsGeminiDecisionsModel(info.UpstreamModelName) {
		return types.NewErrorWithStatusCode(fmt.Errorf("gemini channels only support gemini-decision models via /v1/systemone, got %q", info.UpstreamModelName), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	if hasBoxQuestion(systemOneReq.Questions) {
		return types.NewErrorWithStatusCode(errors.New("the box question type is not supported by gemini decisions models"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if len(systemOneReq.Questions) == 0 {
		return types.NewErrorWithStatusCode(errors.New("questions is required"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if len(systemOneReq.State) == 0 {
		return types.NewErrorWithStatusCode(errors.New("state is required"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	decisionInput, err := buildDecisionsInputFromState(systemOneReq.State, systemOneReq.Assets)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeConvertRequestFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	decisionReq := &kitdto.GeminiDecisionsRequest{
		Model:     info.UpstreamModelName,
		Input:     decisionInput,
		Questions: systemOneReq.Questions,
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
		logger.LogError(c, "Do gemini decisions (systemone) request failed: "+err.Error())
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

// systemOneAsset 对齐 TypeSafe assets 表条目;转换路径仅支持 data_base64 形式
type systemOneAsset struct {
	Type       string `json:"type"`
	DataBase64 string `json:"data_base64"`
	Format     string `json:"format"`
	Part       string `json:"part"`
}

// hasBoxQuestion 检查 questions 中是否含 TypeSafe 独有的 box 题型。
func hasBoxQuestion(questions json.RawMessage) bool {
	if len(questions) == 0 {
		return false
	}
	var parsed map[string]struct {
		Type string `json:"type"`
	}
	if err := common.Unmarshal(questions, &parsed); err != nil {
		return false
	}
	for _, question := range parsed {
		if strings.EqualFold(strings.TrimSpace(question.Type), "box") {
			return true
		}
	}
	return false
}

// buildDecisionsInputFromState 把 TypeSafe state 转换为 Gemini Decisions input:
//   - 纯字符串 state 整值命中引用 → 单图片内容块
//   - 其余形态 → 文本块(state 原样) + 每个资产一张标签文本块与图片块,
//     使 instructions 中的引用文本可按标签关联到对应图片
func buildDecisionsInputFromState(state json.RawMessage, assets json.RawMessage) (json.RawMessage, error) {
	parsedAssets, err := parseSystemOneAssets(assets)
	if err != nil {
		return nil, err
	}

	// 整值引用:state 即引用串,模型直接收到该图片
	var stateString string
	if err := common.Unmarshal(state, &stateString); err == nil {
		if asset, found := parsedAssets[stateString]; found {
			imageBlock, imgErr := systemOneAssetImageBlock(asset, stateString)
			if imgErr != nil {
				return nil, imgErr
			}
			return common.Marshal([]any{imageBlock})
		}
		return state, nil
	}

	blocks := []any{
		map[string]any{"type": "text", "text": string(state)},
	}
	for _, ref := range sortedAssetRefs(parsedAssets) {
		imageBlock, imgErr := systemOneAssetImageBlock(parsedAssets[ref], ref)
		if imgErr != nil {
			return nil, imgErr
		}
		blocks = append(blocks,
			map[string]any{"type": "text", "text": ref + ":"},
			imageBlock,
		)
	}
	return common.Marshal(blocks)
}

// parseSystemOneAssets 解析 assets 表;multipart part 形式转换路径无法内联,明确报错。
func parseSystemOneAssets(assets json.RawMessage) (map[string]systemOneAsset, error) {
	if len(assets) == 0 {
		return nil, nil
	}
	var parsed map[string]systemOneAsset
	if err := common.Unmarshal(assets, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse assets: %w", err)
	}
	for ref, asset := range parsed {
		if asset.DataBase64 == "" && asset.Part != "" {
			return nil, fmt.Errorf("multipart asset parts are not supported by gemini decisions conversion, use data_base64 for asset %q", ref)
		}
	}
	return parsed, nil
}

// systemOneAssetImageBlock 把 TypeSafe 资产转换为 Gemini 内联图片内容块,前置标签块由调用方拼接。
func systemOneAssetImageBlock(asset systemOneAsset, ref string) (map[string]any, error) {
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
	return map[string]any{
		"type":      "image",
		"mime_type": mimeType,
		"data":      asset.DataBase64,
	}, nil
}

// sortedAssetRefs 返回稳定的引用顺序,保证转换结果可复现。
func sortedAssetRefs(assets map[string]systemOneAsset) []string {
	refs := make([]string, 0, len(assets))
	for ref := range assets {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}
