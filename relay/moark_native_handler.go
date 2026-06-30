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

func MoarkNativeHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.ChannelType != constant.ChannelTypeMoark {
		return types.NewErrorWithStatusCode(fmt.Errorf("Moark native endpoint requires Moark channel, got channel type %d", info.ChannelType), types.ErrorCodeInvalidApiType, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	requestBody, closer, err := moarkNativeRequestBody(c, info)
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

	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(httpResp)
	service.IOCopyBytesGracefully(c, httpResp, responseBody)
	service.PostTextConsumeQuota(c, info, moarkNativeUsage(info), nil)
	return nil
}

func moarkNativeRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, io.Closer, error) {
	if c == nil || c.Request == nil || !moarkNativeMethodMayHaveBody(c.Request.Method, c.Request.ContentLength) {
		return bytes.NewReader(nil), nil, nil
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, nil, err
	}

	if moarkNativeIsJSONContentType(c.Request.Header.Get("Content-Type")) && len(info.ParamOverride) > 0 {
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

func moarkNativeMethodMayHaveBody(method string, contentLength int64) bool {
	if contentLength > 0 {
		return true
	}
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func moarkNativeIsJSONContentType(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "application/json")
}

func moarkNativeUsage(info *relaycommon.RelayInfo) *dto.Usage {
	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens <= 0 {
		promptTokens = 1
	}
	return &dto.Usage{
		PromptTokens: promptTokens,
		TotalTokens:  promptTokens,
	}
}
