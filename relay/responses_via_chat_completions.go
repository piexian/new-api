package relay

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/responsescompat"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// responsesViaChatCompletions 渠道不支持原生 /v1/responses 协议时的自动降级:
// 请求转 Chat Completions(复用渠道自身的 chat 转换/清洗), 响应再转回 Responses 协议。
// 约定: 新渠道不支持原生 Responses 时应在自身 ConvertOpenAIResponsesRequest 内自行降级
// (参考 mistral), 不要依赖本全局兜底; 它仅为兼容存量配置保留, 受渠道级开关控制。
func responsesViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, request *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	chatRequest, err := responsescompat.ConvertToOpenAIChatRequest(*request)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
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
		newAPIError := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return nil, newAPIError
	}

	if info.GetFinalRequestRelayFormat() != types.RelayFormatOpenAI {
		return nil, types.NewError(
			fmt.Errorf("channel converted the request to %q, which cannot be mapped back to the Responses protocol", info.GetFinalRequestRelayFormat()),
			types.ErrorCodeConvertRequestFailed,
			types.ErrOptionWithSkipRetry(),
		)
	}

	if info.IsStream {
		usage, newAPIError := openaichannel.ChatCompletionResponsesStreamHandler(c, info, httpResp)
		if newAPIError != nil {
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return nil, newAPIError
		}
		return usage, nil
	}
	usage, newAPIError := openaichannel.ChatCompletionResponsesHandler(c, info, httpResp)
	if newAPIError != nil {
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return nil, newAPIError
	}
	return usage, nil
}

// responsesViaChatFallbackDisabled 渠道级开关: false=关闭降级。
// ChannelSetting 是 ChannelMeta 的提升字段, InitChannelMeta 前恒为 nil, 先判空再读。
func responsesViaChatFallbackDisabled(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil {
		return false
	}
	if ps := info.ChannelSetting.ProtocolSupport; ps != nil && ps.Responses != nil {
		return *ps.Responses
	}
	if info.ChannelSetting.ResponsesViaChatFallbackEnabled == nil {
		return false
	}
	return !*info.ChannelSetting.ResponsesViaChatFallbackEnabled
}
