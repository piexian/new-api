package typesafe

import (
	"errors"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

var errNativeOnly = errors.New("this channel only supports the native POST /v1/systemone endpoint")

// typesafeResponseHandler 将上游响应原样写回客户端, 并从 usage 字段提取计费信息
func typesafeResponseHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (usage any, newAPIError *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)
	service.IOCopyBytesGracefully(c, resp, responseBody)
	usage = extractUsage(responseBody)
	if usage == nil {
		logger.LogWarn(c, "typesafe response has no parsable usage field")
	}
	return usage, nil
}
