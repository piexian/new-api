package mistral

import (
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// MistralOCRHandler 原样透传上游 OCR 响应，并按 usage_info.pages_processed 计费。
// Mistral OCR 按页计费，这里约定 1 页 = 1000 tokens 写入 PromptTokens，
// 实际价格由运营在模型倍率中配置。
func MistralOCRHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)

	var ocrResp struct {
		UsageInfo *struct {
			PagesProcessed int   `json:"pages_processed"`
			DocSizeBytes   int64 `json:"doc_size_bytes"`
		} `json:"usage_info"`
	}
	usage := &dto.Usage{}
	if err := common.Unmarshal(responseBody, &ocrResp); err == nil &&
		ocrResp.UsageInfo != nil && ocrResp.UsageInfo.PagesProcessed > 0 {
		usage.PromptTokens = ocrResp.UsageInfo.PagesProcessed * 1000
	} else {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage, nil
}
