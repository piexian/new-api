package ollama

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ollama Cloud serves Anthropic clients from its native /v1/messages route, so
// the adaptor must hand the upstream body back without an OpenAI re-encoding.
func TestDoResponseClaudePassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("non stream", func(t *testing.T) {
		body := `{"id":"msg_1","type":"message","role":"assistant","model":"gemma4:31b","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}

		usage, apiErr := (&Adaptor{}).DoResponse(c, resp, claudeRelayInfo(false))
		require.Nil(t, apiErr)
		require.NotNil(t, usage)
		assert.Contains(t, w.Body.String(), `"type":"message"`)
		assert.NotContains(t, w.Body.String(), `"object":"chat.completion"`)
	})

	t.Run("stream", func(t *testing.T) {
		body := strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"gemma4:31b","content":[],"stop_reason":null,"usage":{"input_tokens":5,"output_tokens":0}}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
			``,
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}, "\n")

		previousTimeout := constant.StreamingTimeout
		constant.StreamingTimeout = 30
		defer func() { constant.StreamingTimeout = previousTimeout }()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
		resp.Request = c.Request

		usage, apiErr := (&Adaptor{}).DoResponse(c, resp, claudeRelayInfo(true))
		require.Nil(t, apiErr)
		require.NotNil(t, usage)
		assert.Contains(t, w.Body.String(), "message_start")
		assert.Contains(t, w.Body.String(), "content_block_delta")
		assert.NotContains(t, w.Body.String(), "chat.completion.chunk")
	})
}

func claudeRelayInfo(stream bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		IsStream:    stream,
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://ollama.com",
			UpstreamModelName: "gemma4:31b",
		},
	}
}
