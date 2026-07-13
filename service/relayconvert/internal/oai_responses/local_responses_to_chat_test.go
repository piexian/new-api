package oairesponses

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesResponseToChatCompletionsResponseUsesOutputTokensDetails(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		ID:        "resp_123",
		CreatedAt: 1765193461,
		Model:     "doubao-seed-2-1-pro-260628",
		Output: []dto.ResponsesOutput{
			{
				Type: "message",
				Role: "assistant",
				Content: []dto.ResponsesOutputContent{
					{
						Type: "output_text",
						Text: "hello",
					},
				},
			},
		},
		Usage: &dto.Usage{
			InputTokens:  35,
			OutputTokens: 118,
			TotalTokens:  153,
			InputTokensDetails: &dto.InputTokenDetails{
				CachedTokens: 7,
			},
			OutputTokensDetails: &dto.OutputTokenDetails{
				TextTokens:      36,
				ReasoningTokens: 82,
			},
		},
	}

	chatResp, usage, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_123")

	require.NoError(t, err)
	require.Equal(t, "hello", chatResp.Choices[0].Message.Content)
	require.Equal(t, 35, usage.PromptTokens)
	require.Equal(t, 118, usage.CompletionTokens)
	require.Equal(t, 7, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 36, usage.CompletionTokenDetails.TextTokens)
	require.Equal(t, 82, usage.CompletionTokenDetails.ReasoningTokens)
	require.Equal(t, usage.CompletionTokenDetails, chatResp.Usage.CompletionTokenDetails)
}
