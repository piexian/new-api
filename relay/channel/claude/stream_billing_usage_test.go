package claude

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeStreamBillingWhenFinalUsageMissing(t *testing.T) {
	originalTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = originalTimeout })

	for _, tc := range []struct {
		name       string
		blocks     []string
		finalUsage string
		format     types.RelayFormat
		wantOutput int
		wantText   string
		estimated  bool
	}{
		{
			name: "text without message_delta",
			blocks: []string{
				`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"BLUE42"}}`,
			},
			wantText: "BLUE42", estimated: true,
		},
		{
			name:   "OpenAI ingress without message_delta",
			format: types.RelayFormatOpenAI,
			blocks: []string{
				`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"BLUE42"}}`,
			},
			wantText: "BLUE42", estimated: true,
		},
		{
			name: "tool-only without message_delta",
			blocks: []string{
				`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tool_1","name":"Read","input":{}}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"file_path\":\"fixture.txt\"}"}}`,
			},
			wantText: `"tool_use"`, estimated: true,
		},
		{
			name: "thinking without message_delta",
			blocks: []string{
				`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Consider options"}}`,
			},
			wantText: "Consider options", estimated: true,
		},
		{
			name: "zero message_delta despite text",
			blocks: []string{
				`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"BLUE42"}}`,
			},
			finalUsage: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":0}}`,
			wantText:   "BLUE42", estimated: true,
		},
		{
			name: "actual final usage wins",
			blocks: []string{
				`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"BLUE42"}}`,
			},
			finalUsage: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`,
			wantOutput: 7, wantText: "BLUE42",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := `{"type":"message_start","message":{"id":"msg_probe","type":"message","role":"assistant","model":"agnes-2.5-flash","content":[],"usage":{"input_tokens":80,"output_tokens":0,"cache_read_input_tokens":30,"cache_creation_input_tokens":5,"cache_creation":{"ephemeral_5m_input_tokens":2,"ephemeral_1h_input_tokens":3}}}}`
			events := append([]string{start}, tc.blocks...)
			if tc.finalUsage != "" {
				events = append(events, tc.finalUsage)
			}
			events = append(events, `{"type":"message_stop"}`)
			var sse strings.Builder
			for _, event := range events {
				sse.WriteString(fmt.Sprintf("data: %s\n\n", event))
			}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			format := tc.format
			if format == "" {
				format = types.RelayFormatClaude
			}
			info := &relaycommon.RelayInfo{RelayFormat: format, IsStream: true, ShouldIncludeUsage: true, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "agnes-2.5-flash"}}
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(sse.String()))}
			usage, apiErr := ClaudeStreamHandler(c, resp, info)
			require.Nil(t, apiErr)
			require.Contains(t, recorder.Body.String(), tc.wantText)
			require.Equal(t, 80, usage.PromptTokens)
			require.Equal(t, 30, usage.PromptTokensDetails.CachedTokens)
			require.Equal(t, 5, usage.PromptTokensDetails.CachedCreationTokens)
			require.NotNil(t, usage.BillingUsage)
			require.NotNil(t, usage.BillingUsage.ClaudeUsage)
			if tc.wantOutput > 0 {
				require.Equal(t, tc.wantOutput, usage.CompletionTokens)
			} else {
				require.Positive(t, usage.CompletionTokens)
			}
			require.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens)
			require.Equal(t, usage.CompletionTokens, usage.BillingUsage.ClaudeUsage.OutputTokens)
			require.Equal(t, usage.PromptTokens, usage.BillingUsage.ClaudeUsage.InputTokens)
			require.Equal(t, 30, usage.BillingUsage.ClaudeUsage.CacheReadInputTokens)
			require.Equal(t, 5, usage.BillingUsage.ClaudeUsage.CacheCreationInputTokens)
			require.Equal(t, 2, usage.BillingUsage.ClaudeUsage.GetCacheCreation5mTokens())
			require.Equal(t, 3, usage.BillingUsage.ClaudeUsage.GetCacheCreation1hTokens())
			require.Equal(t, tc.estimated, usage.BillingUsage.Estimated)
		})
	}
}

func TestFormatClaudeResponseInfoToolFallbackIncludesArguments(t *testing.T) {
	info := &ClaudeResponseInfo{Usage: &dto.Usage{}}
	start := &dto.ClaudeResponse{Type: "content_block_start", ContentBlock: &dto.ClaudeMediaMessage{Type: "tool_use", Name: "Read"}}
	require.True(t, FormatClaudeResponseInfo(start, nil, info))
	require.Contains(t, info.ResponseText.String(), "Read")
	for _, partialJSON := range []string{`{"file_path":"`, `fixture.txt"}`} {
		delta := &dto.ClaudeResponse{Type: "content_block_delta", Delta: &dto.ClaudeMediaMessage{Type: "input_json_delta", PartialJson: &partialJSON}}
		require.True(t, FormatClaudeResponseInfo(delta, nil, info))
	}
	require.Contains(t, info.ResponseText.String(), `{"file_path":"fixture.txt"}`)
}
