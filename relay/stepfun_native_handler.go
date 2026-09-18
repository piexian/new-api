package relay

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/stepfun"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// StepFunNativeHelper 透传 StepFun 原生音频/音乐/音色/文件端点：
// 入站路径与上游 1:1 对应，仅剥离用于渠道路由的 model 字段，响应（JSON 或 SSE）原样回传。
// 提交类端点按模型价格计费，查询/列表类不重复计费（轮询不应二次扣费）。
func StepFunNativeHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.ChannelType != constant.ChannelTypeStepFun {
		// 与 RelayNotImplemented 保持等价语义：非 StepFun 渠道仍视为未实现
		return types.NewErrorWithStatusCode(
			errors.New("StepFun native endpoints require a StepFun channel"),
			types.ErrorCodeInvalidApiType,
			http.StatusNotImplemented,
			types.ErrOptionWithSkipRetry(),
		)
	}

	endpoint, ok := stepfun.LookupNativeEndpoint(c.Request.URL.Path, c.Request.Method)
	if !ok {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("unsupported StepFun native endpoint %s %s", c.Request.Method, c.Request.URL.Path),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	if !endpoint.AvailableOnBase(info.ChannelBaseUrl) {
		// Phase 0 实测：这些端点仅开放平台提供，Step Plan 通道上游直接 404
		return types.NewErrorWithStatusCode(
			fmt.Errorf("endpoint %s is only available on the StepFun open platform, not on the Step Plan channel", endpoint.Path),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	requestBody, closer, err := stepFunNativeRequestBody(c, info, endpoint)
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

	if isStepFunStreamResponse(httpResp) {
		if err := copyStepFunStreamResponse(c, httpResp); err != nil {
			service.CloseResponseBodyGracefully(httpResp)
			return types.NewError(err, types.ErrorCodeReadResponseBodyFailed, types.ErrOptionWithSkipSensitiveMask())
		}
		service.CloseResponseBodyGracefully(httpResp)
	} else {
		responseBody, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			service.CloseResponseBodyGracefully(httpResp)
			return types.NewOpenAIError(readErr, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
		}
		service.CloseResponseBodyGracefully(httpResp)
		service.IOCopyBytesGracefully(c, httpResp, responseBody)
	}

	if endpoint.Billable {
		service.PostTextConsumeQuota(c, info, stepFunNativeUsage(info), nil)
	}
	return nil
}

// stepFunNativeRequestBody 读取请求体，应用参数覆盖并剥离路由用 model 字段。
func stepFunNativeRequestBody(c *gin.Context, info *relaycommon.RelayInfo, endpoint stepfun.NativeEndpoint) (io.Reader, io.Closer, error) {
	if c == nil || c.Request == nil || !stepFunNativeMethodMayHaveBody(c.Request.Method, c.Request.ContentLength) {
		return bytes.NewReader(nil), nil, nil
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, nil, err
	}

	needsRewrite := endpoint.StripBodyModel || len(info.ParamOverride) > 0
	if !needsRewrite {
		if _, err := storage.Seek(0, io.SeekStart); err != nil {
			return nil, nil, err
		}
		c.Request.Body = io.NopCloser(storage)
		info.UpstreamRequestBodySize = storage.Size()
		return common.ReaderOnly(storage), storage, nil
	}

	requestBody, err := storage.Bytes()
	if err != nil {
		return nil, nil, err
	}
	if prepared, prepareErr := stepfun.PrepareNativeRequestBody(requestBody, endpoint); prepareErr == nil {
		requestBody = prepared
	}
	if len(info.ParamOverride) > 0 && stepFunNativeIsJSONContentType(c.Request.Header.Get("Content-Type")) {
		requestBody, err = relaycommon.ApplyParamOverrideWithRelayInfo(requestBody, info)
		if err != nil {
			return nil, nil, err
		}
	}
	info.UpstreamRequestBodySize = int64(len(requestBody))
	return bytes.NewReader(requestBody), nil, nil
}

func stepFunNativeMethodMayHaveBody(method string, contentLength int64) bool {
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

func stepFunNativeIsJSONContentType(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "application/json")
}

func stepFunNativeUsage(info *relaycommon.RelayInfo) *dto.Usage {
	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens <= 0 {
		promptTokens = 1
	}
	return &dto.Usage{
		PromptTokens: promptTokens,
		TotalTokens:  promptTokens,
	}
}

func isStepFunStreamResponse(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	return strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream")
}

// copyStepFunStreamResponse 原样转发 SSE（ASR 流式返回文本等）：逐块写出并立即 flush，
// 不做任何事件改写（StepFun 的事件格式与 OpenAI 不同）。
func copyStepFunStreamResponse(c *gin.Context, resp *http.Response) error {
	for key, values := range resp.Header {
		if !service.ShouldCopyUpstreamHeader(c, key, values) {
			continue
		}
		c.Writer.Header().Set(key, values[0])
	}
	c.Writer.WriteHeader(resp.StatusCode)

	flusher, _ := c.Writer.(http.Flusher)
	buffer := make([]byte, 4096)
	for {
		read, readErr := resp.Body.Read(buffer)
		if read > 0 {
			if _, writeErr := c.Writer.Write(buffer[:read]); writeErr != nil {
				return writeErr
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}
