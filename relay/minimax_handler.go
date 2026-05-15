package relay

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/minimax"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func MiniMaxNativeHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	billingModelName := info.OriginModelName
	restoreBillingModel := func() {
		info.OriginModelName = billingModelName
		if info.ChannelMeta != nil {
			info.ChannelMeta.UpstreamModelName = billingModelName
		}
	}
	if upstreamModel := common.GetContextKeyString(c, constant.ContextKeyMiniMaxUpstreamModel); upstreamModel != "" {
		info.OriginModelName = upstreamModel
	}
	defer restoreBillingModel()

	info.InitChannelMeta(c)
	if info.ChannelMeta != nil {
		info.ChannelMeta.UpstreamModelName = info.OriginModelName
	}
	if info.ChannelType != constant.ChannelTypeMiniMax {
		return types.NewErrorWithStatusCode(fmt.Errorf("minimax native endpoint requires minimax channel, got channel type %d", info.ChannelType), types.ErrorCodeInvalidApiType, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if newAPIError = minimax.ValidateEndpointForModel(info); newAPIError != nil {
		return newAPIError
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	requestBody, err := storage.Bytes()
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if len(info.ParamOverride) > 0 {
		requestBody, err = relaycommon.ApplyParamOverrideWithRelayInfo(requestBody, info)
		if err != nil {
			return newAPIErrorFromParamOverride(err)
		}
	}

	resp, err := adaptor.DoRequest(c, info, bytes.NewReader(requestBody))
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	var httpResp *http.Response
	if resp != nil {
		var ok bool
		httpResp, ok = resp.(*http.Response)
		if !ok {
			return types.NewErrorWithStatusCode(fmt.Errorf("invalid response type: %T", resp), types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}
	restoreBillingModel()

	usageDto, ok := usage.(*dto.Usage)
	if !ok || usageDto == nil {
		usageDto = &dto.Usage{PromptTokens: 1, TotalTokens: 1}
	}
	if usageDto.PromptTokens <= 0 {
		usageDto.PromptTokens = 1
	}
	if usageDto.TotalTokens <= 0 {
		usageDto.TotalTokens = usageDto.PromptTokens
	}
	var logContent []string
	switch request := info.Request.(type) {
	case interface{ GetLogContent() []string }:
		logContent = request.GetLogContent()
	}
	service.PostTextConsumeQuota(c, info, usageDto, logContent)
	return nil
}
