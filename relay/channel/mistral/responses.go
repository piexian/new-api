package mistral

import (
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// mistralResponsesInputTokensHandler 处理 /v1/responses/input_tokens 的响应：
// body 原样透传，input_tokens 计入 usage。
func mistralResponsesInputTokensHandler(c *gin.Context, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)
	var tokenResp struct {
		InputTokens int `json:"input_tokens"`
	}
	if err = common.Unmarshal(body, &tokenResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, body)
	return &dto.Usage{
		PromptTokens: tokenResp.InputTokens,
		InputTokens:  tokenResp.InputTokens,
		TotalTokens:  tokenResp.InputTokens,
	}, nil
}
