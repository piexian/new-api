package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeRequestRemoveAnthropicBillingHeaderSystemBlockFromMedia(t *testing.T) {
	t.Parallel()

	billingHeader := "x-anthropic-billing-header: cc_version=2.1.177; cc_entrypoint=sdk-cli; cch=nonce;"
	systemPrompt := "You are a helpful assistant."
	cacheControl := json.RawMessage(`{"type":"ephemeral"}`)
	req := ClaudeRequest{
		System: []ClaudeMediaMessage{
			{
				Type: ContentTypeText,
				Text: &billingHeader,
			},
			{
				Type:         ContentTypeText,
				Text:         &systemPrompt,
				CacheControl: cacheControl,
			},
		},
	}

	require.True(t, req.RemoveAnthropicBillingHeaderSystemBlock())

	system := req.ParseSystem()
	require.Len(t, system, 1)
	require.Equal(t, systemPrompt, system[0].GetText())
	require.JSONEq(t, string(cacheControl), string(system[0].CacheControl))
}

func TestClaudeRequestRemoveAnthropicBillingHeaderLineFromStringSystem(t *testing.T) {
	t.Parallel()

	req := ClaudeRequest{
		System: " X-Anthropic-Billing-Header: cc_version=2.1.177; cch=nonce;\nKeep this system prompt.",
	}

	require.True(t, req.RemoveAnthropicBillingHeaderSystemBlock())
	require.Equal(t, "Keep this system prompt.", req.GetStringSystem())
}

func TestClaudeRequestRemoveAnthropicBillingHeaderClearsOnlyHeaderSystem(t *testing.T) {
	t.Parallel()

	req := ClaudeRequest{
		System: "x-anthropic-billing-header: cc_version=2.1.177; cch=nonce;",
	}

	require.True(t, req.RemoveAnthropicBillingHeaderSystemBlock())
	require.Nil(t, req.System)
}

func TestThinkingKeepRoundTrip(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Thinking{Type: "enabled", Keep: "all"})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"enabled","keep":"all"}`, string(data))

	var thinking Thinking
	require.NoError(t, json.Unmarshal(data, &thinking))
	require.Equal(t, "enabled", thinking.Type)
	require.Equal(t, "all", thinking.Keep)
}
