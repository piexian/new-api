package relay

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel/deepseek"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// DeepSeekToolsHelper 处理 POST /v1/tools/{search,fetch} 入站:请求体透传,
// 按次计费(deepseek-search / deepseek-fetch 走 per-call 固定价),响应原样回传。
func DeepSeekToolsHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	toolsReq, ok := info.Request.(*kitdto.DeepSeekToolsRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *kitdto.DeepSeekToolsRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	if info.ChannelType != constant.ChannelTypeDeepSeek {
		return types.NewErrorWithStatusCode(errors.New("the /v1/tools endpoints only support deepseek channels"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	if err := helper.ModelMappedHelper(c, info, toolsReq); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	// 路径与模型必须一致:search 配 deepseek-search,fetch 配 deepseek-fetch
	if _, err := deepseek.DeepSeekToolsUpstreamPath(info.RequestURLPath, info.UpstreamModelName); err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	requestBody, _, err := relaycommon.PassThroughRequestBody(c, info)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		logger.LogError(c, "Do deepseek tools request failed: "+err.Error())
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

	responseBody, readErr := io.ReadAll(httpResp.Body)
	if readErr != nil {
		return types.NewError(fmt.Errorf("failed to read tools response: %w", readErr), types.ErrorCodeReadResponseBodyFailed)
	}
	service.CloseResponseBodyGracefully(httpResp)
	service.IOCopyBytesGracefully(c, httpResp, responseBody)

	// per-call 固定价经 PriceData.UsePrice 结算,与 token 用量无关
	service.PostTextConsumeQuota(c, info, &kitdto.Usage{}, nil)
	return nil
}
