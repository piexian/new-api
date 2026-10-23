package relay

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	claudechannel "github.com/QuantumNous/new-api/relay/channel/claude"
	geminichannel "github.com/QuantumNous/new-api/relay/channel/gemini"
	ollamachannel "github.com/QuantumNous/new-api/relay/channel/ollama"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	"github.com/QuantumNous/new-api/relay/channel/zhipu_4v"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/responsescompat"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func protocolNotSupportedMessage(c *gin.Context) string {
	if c != nil {
		if translated := i18n.T(c, i18n.MsgDistributorProtocolNotSupported); translated != "" {
			return translated
		}
	}
	return "The current request protocol is not enabled or supported; please call it with a supported protocol"
}

func protocolNotSupportedError(c *gin.Context) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		errors.New(protocolNotSupportedMessage(c)),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

// targetFormatUnsupportedError marks a dispatch target whose converted request
// cannot be relayed (its response could not be rendered back to the client).
// It is always raised before any upstream call, so the next target may be tried.
type targetFormatUnsupportedError struct{ message string }

func (e *targetFormatUnsupportedError) Error() string { return e.message }

func newTargetFormatUnsupportedError(c *gin.Context) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		&targetFormatUnsupportedError{message: protocolNotSupportedMessage(c)},
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

func isTargetFormatUnsupported(apiErr *types.NewAPIError) bool {
	var target *targetFormatUnsupportedError
	return apiErr != nil && errors.As(apiErr.Err, &target)
}

func shouldCheckProtocolRouting(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil {
		return false
	}
	if isKnownOpenCodeRoute(info) && info.ChannelSetting.ProtocolSupport == nil {
		return false
	}
	return true
}

func checkInboundProtocolInterception(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	if !shouldCheckProtocolRouting(info) {
		return nil
	}
	if relaycommon.ShouldInterceptInboundProtocol(
		info.ChannelType,
		info.ChannelBaseUrl,
		info.ChannelSetting,
		shouldPassThroughModelRequest(info),
		info.RelayFormat,
		info.RelayMode,
	) {
		return protocolNotSupportedError(c)
	}
	return nil
}

// usesInternalResponsesChatConversion reports whether a channel adaptor converts
// Responses requests into its own provider-specific Chat format inside
// ConvertOpenAIResponsesRequest + DoResponse.
func usesInternalResponsesChatConversion(channelType int) bool {
	switch channelType {
	case constant.ChannelTypeBaidu,
		constant.ChannelTypeCoze,
		constant.ChannelTypeDify,
		constant.ChannelTypeTencent,
		constant.ChannelTypeXunfei,
		constant.ChannelTypeCohere,
		constant.ChannelTypeSubmodel,
		constant.ChannelTypeZhipu,
		constant.ChannelTypeSiliconFlow,
		constant.ChannelTypeMistral,
		constant.ChannelTypeCerebras,
		constant.ChannelTypeGMICloud:
		return true
	default:
		return false
	}
}

// chatTargetAdaptor picks the adaptor that relays a converted OpenAI Chat request
// whose response is rendered by the OpenAI handlers; nil means the channel
// cannot serve Chat that way.
func chatTargetAdaptor(info *relaycommon.RelayInfo, adaptor channel.Adaptor) channel.Adaptor {
	if requiresGenericAdaptor(info, relaycommon.ProtocolChat) {
		return newGenericProtocolAdaptor(relaycommon.ProtocolChat)
	}
	if info == nil || info.ChannelMeta == nil {
		return adaptor
	}
	switch {
	case info.ChannelType == constant.ChannelTypeOllama:
		// Ollama 原生 /api/chat 为私有格式，改走其 OpenAI 兼容端点
		return &ollamachannel.OpenAICompatAdaptor{}
	case relaycommon.UsesPrivateChatProtocol(info.ChannelType):
		return nil
	case isGeminiFamilyChannel(info.ChannelType):
		// Gemini 渠道仅在开启 OpenAI 兼容时把 Chat 原样发往上游兼容端点
		if relaycommon.ResolveChannelProtocolSupportFromInfo(info).Chat {
			return adaptor
		}
		return nil
	case relaycommon.NativeChannelProtocol(info.ChannelType, info.ChannelBaseUrl, info.ChannelSetting) != "":
		return nil
	default:
		return adaptor
	}
}

func isGeminiFamilyChannel(channelType int) bool {
	return channelType == constant.ChannelTypeGemini || channelType == constant.ChannelTypeGeminiInteractions
}

// adaptorConvertsNatively reports whether the channel adaptor's own Convert*
// path already turns the inbound protocol into target, so no dispatch is needed.
func adaptorConvertsNatively(info *relaycommon.RelayInfo, inbound, target relaycommon.ChannelProtocol) bool {
	switch target {
	case relaycommon.ProtocolChat:
		return inbound == relaycommon.ProtocolResponses && usesInternalResponsesChatConversion(info.ChannelType)
	case relaycommon.ProtocolMessages:
		if info.ChannelType == constant.ChannelTypeAnthropic || info.ChannelType == constant.ChannelTypeAws {
			return true
		}
		return inbound == relaycommon.ProtocolResponses &&
			zhipu_4v.IsZCodeModeChannel(info.ChannelType, info.ChannelBaseUrl, info.ChannelSetting)
	case relaycommon.ProtocolGemini:
		return isGeminiFamilyChannel(info.ChannelType) || info.ChannelType == constant.ChannelTypeVertexAi
	default:
		return false
	}
}

// requiresGenericAdaptor reports whether protocol is enabled on the channel only
// through an explicit override, so requests must go through the generic adaptor
// of that protocol instead of the channel adaptor. Channels with a non-Chat
// native protocol (Anthropic, AWS, Gemini, Vertex, Codex, ZCode) carry their own
// auth and URL rules, so they always keep the channel adaptor.
func requiresGenericAdaptor(info *relaycommon.RelayInfo, protocol relaycommon.ChannelProtocol) bool {
	if info == nil || info.ChannelMeta == nil {
		return false
	}
	if relaycommon.NativeChannelProtocol(info.ChannelType, info.ChannelBaseUrl, info.ChannelSetting) != "" {
		return false
	}
	return relaycommon.ProtocolExplicitlyEnabled(info.ChannelSetting, protocol) &&
		!relaycommon.DefaultChannelProtocolSupport(info.ChannelType, info.ChannelBaseUrl, info.ChannelSetting).IsEnabled(protocol)
}

func newGenericProtocolAdaptor(protocol relaycommon.ChannelProtocol) channel.Adaptor {
	switch protocol {
	case relaycommon.ProtocolMessages:
		return &claudechannel.Adaptor{}
	case relaycommon.ProtocolGemini:
		return &geminichannel.Adaptor{}
	default:
		return &openaichannel.Adaptor{}
	}
}

// inboundRoute describes how an inbound protocol is served on the selected channel.
type inboundRoute struct {
	// dispatch: convert the request to one of targets (in order) via dispatchToTargetProtocol.
	dispatch bool
	targets  []relaycommon.ChannelProtocol
	// adaptor: non-nil when the inbound protocol is passed straight through by the
	// generic adaptor of that protocol (explicitly enabled on the channel).
	adaptor channel.Adaptor
}

// planInboundRoute decides how the inbound protocol is routed:
//   - enabled only by an explicit override → generic adaptor of that protocol;
//   - explicitly disabled → convert to PreferredTargetProtocol, unless the
//     channel adaptor already converts natively to that target;
//   - disabled only by the channel-type default → keep the adaptor's built-in handling.
func planInboundRoute(c *gin.Context, info *relaycommon.RelayInfo, inbound relaycommon.ChannelProtocol) (inboundRoute, *types.NewAPIError) {
	if !shouldCheckProtocolRouting(info) {
		return inboundRoute{}, nil
	}
	support := relaycommon.ResolveChannelProtocolSupportFromInfo(info)
	if support.IsEnabled(inbound) {
		if requiresGenericAdaptor(info, inbound) {
			return inboundRoute{adaptor: newGenericProtocolAdaptor(inbound)}, nil
		}
		return inboundRoute{}, nil
	}
	if !relaycommon.ProtocolExplicitlyDisabled(info.ChannelSetting, inbound) {
		return inboundRoute{}, nil
	}
	native := relaycommon.NativeChannelProtocol(info.ChannelType, info.ChannelBaseUrl, info.ChannelSetting)
	if native != "" && support.IsEnabled(native) && adaptorConvertsNatively(info, inbound, native) {
		return inboundRoute{}, nil
	}
	targets := support.PreferredTargetProtocols(inbound, native, info.IsStream)
	if len(targets) == 0 {
		return inboundRoute{}, protocolNotSupportedError(c)
	}
	if adaptorConvertsNatively(info, inbound, targets[0]) {
		return inboundRoute{}, nil
	}
	return inboundRoute{dispatch: true, targets: targets}, nil
}

func convertToOpenAIChatRequest(c *gin.Context, info *relaycommon.RelayInfo, sourceRequest any) (*dto.GeneralOpenAIRequest, *types.NewAPIError) {
	switch req := sourceRequest.(type) {
	case *dto.GeneralOpenAIRequest:
		return req, nil
	case *dto.OpenAIResponsesRequest:
		chatReq, err := responsescompat.ConvertToOpenAIChatRequest(*req)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		return chatReq, nil
	default:
		result, err := service.ConvertRequest(c, info, types.RelayFormatOpenAI, sourceRequest)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		chatReq, ok := result.Value.(*dto.GeneralOpenAIRequest)
		if !ok {
			return nil, types.NewError(
				fmt.Errorf("expected OpenAI chat completions request, got %T", result.Value),
				types.ErrorCodeConvertRequestFailed,
				types.ErrOptionWithSkipRetry(),
			)
		}
		if info != nil && info.SupportStreamOptions && info.IsStream {
			chatReq.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
		}
		return chatReq, nil
	}
}

func toUsagePtr(usageValue any) (*dto.Usage, *types.NewAPIError) {
	if usageValue == nil {
		return nil, nil
	}
	switch u := usageValue.(type) {
	case *dto.Usage:
		return u, nil
	case dto.Usage:
		return &u, nil
	default:
		return nil, types.NewOpenAIError(
			fmt.Errorf("expected *dto.Usage, got %T", usageValue),
			types.ErrorCodeBadResponse,
			http.StatusInternalServerError,
		)
	}
}

func relayViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, sourceRequest any) (*dto.Usage, *types.NewAPIError) {
	if respReq, ok := sourceRequest.(*dto.OpenAIResponsesRequest); ok {
		return responsesViaChatCompletions(c, info, adaptor, respReq)
	}

	chatRequest, newAPIErr := convertToOpenAIChatRequest(c, info, sourceRequest)
	if newAPIErr != nil {
		return nil, newAPIErr
	}

	targetAdaptor := chatTargetAdaptor(info, adaptor)
	if targetAdaptor == nil {
		return nil, newTargetFormatUnsupportedError(c)
	}

	savedRelayFormat := info.RelayFormat
	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayFormat = savedRelayFormat
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayFormat = types.RelayFormatOpenAI
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RequestURLPath = "/v1/chat/completions"
	info.FinalRequestRelayFormat = types.RelayFormatOpenAI
	targetAdaptor.Init(info)

	convertedRequest, err := targetAdaptor.ConvertOpenAIRequest(c, info, chatRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	logger.LogDebug(c, "requestBody: %s", jsonData)
	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	jsonData = nil
	info.UpstreamRequestBodySize = size
	var requestBody io.Reader = body

	resp, err := targetAdaptor.DoRequest(c, info, requestBody)
	info.RelayFormat = savedRelayFormat
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	if resp == nil {
		return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	httpResp := resp.(*http.Response)

	statusCodeMappingStr := c.GetString("status_code_mapping")
	if httpResp.StatusCode != http.StatusOK {
		apiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(apiErr, statusCodeMappingStr)
		return nil, apiErr
	}

	var usage any
	var apiErr *types.NewAPIError
	if info.IsStream {
		usage, apiErr = openaichannel.OaiStreamHandler(c, info, httpResp)
	} else {
		usage, apiErr = openaichannel.OpenaiHandler(c, info, httpResp)
	}
	if apiErr != nil {
		service.ResetStatusCode(apiErr, statusCodeMappingStr)
		return nil, apiErr
	}
	return toUsagePtr(usage)
}

func relayViaResponses(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, sourceRequest any) (*dto.Usage, *types.NewAPIError) {
	chatRequest, newAPIErr := convertToOpenAIChatRequest(c, info, sourceRequest)
	if newAPIErr != nil {
		return nil, newAPIErr
	}
	return chatCompletionsViaResponses(c, info, adaptor, chatRequest)
}

func relayViaClaudeMessages(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, sourceRequest any) (*dto.Usage, *types.NewAPIError) {
	var claudeRequest *dto.ClaudeRequest
	switch req := sourceRequest.(type) {
	case *dto.ClaudeRequest:
		claudeRequest = req
	default:
		result, err := service.ConvertRequest(c, info, types.RelayFormatClaude, sourceRequest)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		converted, ok := result.Value.(*dto.ClaudeRequest)
		if !ok {
			return nil, types.NewError(
				fmt.Errorf("expected Claude messages request, got %T", result.Value),
				types.ErrorCodeConvertRequestFailed,
				types.ErrOptionWithSkipRetry(),
			)
		}
		claudeRequest = converted
	}

	if claudeRequest.MaxTokens == nil || *claudeRequest.MaxTokens == 0 {
		defaultMaxTokens := uint(model_setting.GetClaudeSettings().GetDefaultMaxTokens(claudeRequest.Model))
		claudeRequest.MaxTokens = &defaultMaxTokens
	}

	targetAdaptor := adaptor
	if requiresGenericAdaptor(info, relaycommon.ProtocolMessages) {
		targetAdaptor = newGenericProtocolAdaptor(relaycommon.ProtocolMessages)
	}

	savedRelayFormat := info.RelayFormat
	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayFormat = savedRelayFormat
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayFormat = types.RelayFormatClaude
	info.RelayMode = relayconstant.RelayModeUnknown
	info.RequestURLPath = "/v1/messages"
	info.FinalRequestRelayFormat = types.RelayFormatClaude
	targetAdaptor.Init(info)

	convertedRequest, err := targetAdaptor.ConvertClaudeRequest(c, info, claudeRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	// 响应固定按 Claude 格式解析；adaptor 降级成其他格式（如 Ali 非 Anthropic 模型转 Chat）时不可用
	if _, isClaude := convertedRequest.(*dto.ClaudeRequest); !isClaude {
		return nil, newTargetFormatUnsupportedError(c)
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	jsonData = nil
	info.UpstreamRequestBodySize = size
	var requestBody io.Reader = body

	resp, err := targetAdaptor.DoRequest(c, info, requestBody)
	info.RelayFormat = savedRelayFormat
	info.RelayMode = savedRelayMode
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	// AWS SDK 模式 DoRequest 不返回 http.Response，响应由 DoResponse 从上下文读取
	if info.ChannelType == constant.ChannelTypeAws {
		usage, apiErr := targetAdaptor.DoResponse(c, nil, info)
		if apiErr != nil {
			service.ResetStatusCode(apiErr, statusCodeMappingStr)
			return nil, apiErr
		}
		return toUsagePtr(usage)
	}
	if resp == nil {
		return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	httpResp := resp.(*http.Response)
	if httpResp.StatusCode != http.StatusOK {
		apiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(apiErr, statusCodeMappingStr)
		return nil, apiErr
	}

	claudeAdaptor := &claudechannel.Adaptor{}
	usage, apiErr := claudeAdaptor.DoResponse(c, httpResp, info)
	if apiErr != nil {
		service.ResetStatusCode(apiErr, statusCodeMappingStr)
		return nil, apiErr
	}
	return toUsagePtr(usage)
}

func relayViaGemini(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, sourceRequest any) (*dto.Usage, *types.NewAPIError) {
	var geminiRequest *dto.GeminiChatRequest
	switch req := sourceRequest.(type) {
	case *dto.GeminiChatRequest:
		geminiRequest = req
	default:
		result, err := service.ConvertRequest(c, info, types.RelayFormatGemini, sourceRequest)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		converted, ok := result.Value.(*dto.GeminiChatRequest)
		if !ok {
			return nil, types.NewError(
				fmt.Errorf("expected Gemini request, got %T", result.Value),
				types.ErrorCodeConvertRequestFailed,
				types.ErrOptionWithSkipRetry(),
			)
		}
		geminiRequest = converted
	}

	targetAdaptor := adaptor
	if requiresGenericAdaptor(info, relaycommon.ProtocolGemini) {
		targetAdaptor = newGenericProtocolAdaptor(relaycommon.ProtocolGemini)
	}

	savedRelayFormat := info.RelayFormat
	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayFormat = savedRelayFormat
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	action := "generateContent"
	if info.IsStream {
		action = "streamGenerateContent"
	}
	info.RelayFormat = types.RelayFormatGemini
	info.RelayMode = relayconstant.RelayModeGemini
	info.RequestURLPath = fmt.Sprintf("/v1beta/models/%s:%s", info.UpstreamModelName, action)
	info.FinalRequestRelayFormat = types.RelayFormatGemini
	targetAdaptor.Init(info)

	convertedRequest, err := targetAdaptor.ConvertGeminiRequest(c, info, geminiRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	// 响应固定按 Gemini 格式解析；adaptor 转成其他格式（如 openai adaptor 转 Chat）时不可用
	if _, isGemini := convertedRequest.(*dto.GeminiChatRequest); !isGemini {
		return nil, newTargetFormatUnsupportedError(c)
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	jsonData = nil
	info.UpstreamRequestBodySize = size
	var requestBody io.Reader = body

	resp, err := targetAdaptor.DoRequest(c, info, requestBody)
	info.RelayFormat = savedRelayFormat
	info.RelayMode = savedRelayMode
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	if resp == nil {
		return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	httpResp := resp.(*http.Response)

	statusCodeMappingStr := c.GetString("status_code_mapping")
	if httpResp.StatusCode != http.StatusOK {
		apiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(apiErr, statusCodeMappingStr)
		return nil, apiErr
	}

	geminiAdaptor := &geminichannel.Adaptor{}
	usage, apiErr := geminiAdaptor.DoResponse(c, httpResp, info)
	if apiErr != nil {
		service.ResetStatusCode(apiErr, statusCodeMappingStr)
		return nil, apiErr
	}
	return toUsagePtr(usage)
}

// dispatchToTargetProtocol relays the request through the first target that
// can serve it; targets rejected before any upstream call fall through to the next.
func dispatchToTargetProtocol(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	adaptor channel.Adaptor,
	targets []relaycommon.ChannelProtocol,
	sourceRequest any,
) (*dto.Usage, *types.NewAPIError) {
	for _, target := range targets {
		var usage *dto.Usage
		var apiErr *types.NewAPIError
		switch target {
		case relaycommon.ProtocolChat:
			usage, apiErr = relayViaChatCompletions(c, info, adaptor, sourceRequest)
		case relaycommon.ProtocolResponses:
			usage, apiErr = relayViaResponses(c, info, adaptor, sourceRequest)
		case relaycommon.ProtocolMessages:
			usage, apiErr = relayViaClaudeMessages(c, info, adaptor, sourceRequest)
		case relaycommon.ProtocolGemini:
			usage, apiErr = relayViaGemini(c, info, adaptor, sourceRequest)
		default:
			continue
		}
		if isTargetFormatUnsupported(apiErr) {
			continue
		}
		return usage, apiErr
	}
	return nil, protocolNotSupportedError(c)
}
