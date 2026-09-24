package opencode

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenCodeNativeIngressResponseConversion(t *testing.T) {
	for _, tc := range []struct {
		name, baseURL, model, path, response, wantedPath string
		format                                           types.RelayFormat
		mode                                             int
	}{
		{"Gemini to Go Claude", constant.OpenCodeGoBaseURLAlias, "qwen3.8-flash", "/v1beta/models/qwen3.8-flash:generateContent", `{"id":"msg_test","type":"message","role":"assistant","model":"qwen3.8-flash","content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}`, "candidates.0.content.parts.0.text", types.RelayFormatGemini, relayconstant.RelayModeGemini},
		{"Claude to Zen Responses", constant.OpenCodeZenBaseURLAlias, "gpt-6-sol", "/v1/messages", `{"id":"resp_test","object":"response","model":"gpt-6-sol","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`, "content.0.text", types.RelayFormatClaude, relayconstant.RelayModeUnknown},
		{"Claude to Zen Gemini", constant.OpenCodeZenBaseURLAlias, "gemini-3.8-flash", "/v1/messages", `{"candidates":[{"content":{"role":"model","parts":[{"text":"OK"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2,"totalTokenCount":5}}`, "content.0.text", types.RelayFormatClaude, relayconstant.RelayModeUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)
			info := &relaycommon.RelayInfo{RelayFormat: tc.format, RelayMode: tc.mode, RequestURLPath: tc.path, ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenCode, ChannelBaseUrl: tc.baseURL, UpstreamModelName: tc.model}}
			a := &Adaptor{}
			a.Init(info)
			response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(tc.response))}
			usage, apiErr := a.DoResponse(c, response, info)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			require.Equal(t, 5, usage.(*dto.Usage).TotalTokens)
			require.Equal(t, "OK", gjson.GetBytes(recorder.Body.Bytes(), tc.wantedPath).String())
		})
	}
}

func TestOpenCodeNativeIngressClaudeStreamConversion(t *testing.T) {
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	const events = "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"qwen3.8-flash\",\"content\":[],\"usage\":{\"input_tokens\":3,\"output_tokens\":0}}}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"OK\"}}\n\n" +
		"event: content_block_stop\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"
	for _, tc := range []struct {
		name, path, expected string
		format               types.RelayFormat
		mode                 int
	}{
		{"Gemini", "/v1beta/models/qwen3.8-flash:streamGenerateContent", `"candidates"`, types.RelayFormatGemini, relayconstant.RelayModeGemini},
		{"Responses", "/v1/responses", "response.completed", types.RelayFormatOpenAIResponses, relayconstant.RelayModeResponses},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)
			info := &relaycommon.RelayInfo{RelayFormat: tc.format, RelayMode: tc.mode, IsStream: true, RequestURLPath: tc.path, ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenCode, ChannelBaseUrl: constant.OpenCodeGoBaseURLAlias, UpstreamModelName: "qwen3.8-flash"}}
			a := &Adaptor{}
			a.Init(info)
			response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(events))}
			usage, apiErr := a.DoResponse(c, response, info)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			require.Equal(t, 5, usage.(*dto.Usage).TotalTokens)
			require.Contains(t, recorder.Body.String(), tc.expected)
			require.Contains(t, recorder.Body.String(), "OK")
		})
	}
}
