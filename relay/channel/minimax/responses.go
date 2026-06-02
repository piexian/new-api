package minimax

import (
	"errors"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type miniMaxResponsesInputTokensResponse struct {
	Object      string `json:"object,omitempty"`
	InputTokens int    `json:"input_tokens"`
}

type miniMaxClaudeCountTokensResponse struct {
	Usage struct {
		InputTokens              int `json:"input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func miniMaxResponsesInputTokensHandler(c *gin.Context, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var tokenResponse miniMaxResponsesInputTokensResponse
	if err := common.Unmarshal(responseBody, &tokenResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	return &dto.Usage{
		PromptTokens:  tokenResponse.InputTokens,
		InputTokens:   tokenResponse.InputTokens,
		TotalTokens:   tokenResponse.InputTokens,
		UsageSemantic: "openai",
		UsageSource:   "minimax_responses_input_tokens",
	}, nil
}

func miniMaxClaudeCountTokensHandler(c *gin.Context, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var tokenResponse miniMaxClaudeCountTokensResponse
	if err := common.Unmarshal(responseBody, &tokenResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	usage := &dto.Usage{
		PromptTokens:  tokenResponse.Usage.InputTokens,
		InputTokens:   tokenResponse.Usage.InputTokens,
		TotalTokens:   tokenResponse.Usage.InputTokens,
		UsageSemantic: "anthropic",
		UsageSource:   "minimax_anthropic_count_tokens",
	}
	usage.PromptTokensDetails.CachedTokens = tokenResponse.Usage.CacheReadInputTokens
	usage.PromptTokensDetails.CachedCreationTokens = tokenResponse.Usage.CacheCreationInputTokens

	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}
