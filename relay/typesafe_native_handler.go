package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// TypeSafeNativeHelper 将 POST /v1/systemone 原样透传到 TypeSafe 上游（仅 TypeSafe 渠道）,
// 响应原样写回客户端, usage 从响应体提取后按实际 token 计费
func TypeSafeNativeHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.ChannelType != constant.ChannelTypeTypeSafe {
		return types.NewErrorWithStatusCode(fmt.Errorf("TypeSafe native endpoint requires TypeSafe channel, got channel type %d", info.ChannelType), types.ErrorCodeInvalidApiType, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	requestBody, closer, err := typesafeNativeRequestBody(c, info)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if closer != nil {
		defer closer.Close()
	}

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	httpResp, ok := resp.(*http.Response)
	if !ok || httpResp == nil {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid response type: %T", resp), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	statusCodeMappingStr := c.GetString("status_code_mapping")
	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	usage, respErr := adaptor.DoResponse(c, httpResp, info)
	if respErr != nil {
		return respErr
	}
	billingUsage, _ := usage.(*dto.Usage)
	service.PostTextConsumeQuota(c, info, billingUsage, nil)
	return nil
}

// typesafeNativeRequestBody 与 xAI native 相同策略: JSON 体支持渠道参数覆盖, 其余字节级透传
func typesafeNativeRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, io.Closer, error) {
	if c == nil || c.Request == nil {
		return bytes.NewReader(nil), nil, nil
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, nil, err
	}

	if typesafeIsJSONContentType(c.Request.Header.Get("Content-Type")) && info != nil && info.ChannelMeta != nil && len(info.ParamOverride) > 0 {
		requestBody, err := storage.Bytes()
		if err != nil {
			return nil, nil, err
		}
		requestBody, err = relaycommon.ApplyParamOverrideWithRelayInfo(requestBody, info)
		if err != nil {
			return nil, nil, err
		}
		info.UpstreamRequestBodySize = int64(len(requestBody))
		return bytes.NewReader(requestBody), nil, nil
	}

	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	c.Request.Body = io.NopCloser(storage)
	info.UpstreamRequestBodySize = storage.Size()
	return common.ReaderOnly(storage), storage, nil
}

func typesafeIsJSONContentType(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "application/json")
}
