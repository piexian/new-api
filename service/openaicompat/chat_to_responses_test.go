package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsRequestToResponsesRequestPreservesDoubaoThinking(t *testing.T) {
	thinking := []byte(`{"type":"disabled"}`)

	out, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
		Model: "doubao-seed-2-1-pro-260628",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
		THINKING: thinking,
	})

	require.NoError(t, err)
	require.JSONEq(t, string(thinking), string(out.Thinking))

	var input []map[string]any
	require.NoError(t, common.Unmarshal(out.Input, &input))
	require.Len(t, input, 1)
	require.Equal(t, "user", input[0]["role"])
	require.Equal(t, "hello", input[0]["content"])
}
