package opencode

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFreeGuardPreservesSplitDeclaredNameWithCorePrefix(t *testing.T) {
	body := `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read"}}]},"finish_reason":null}]}` + "\n\n" +
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"_file","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}` + "\n\n" + "data: [DONE]\n\n"
	for _, legitimate := range []bool{true, false} {
		fixture := body
		if !legitimate {
			fixture = strings.ReplaceAll(body, `"name":"_file",`, "")
		}
		stream := newFreeStream(context.Background(), io.NopCloser(strings.NewReader(fixture)), requestModeOpenAI, map[string]bool{"read": true}, time.Second)
		stream.declared = map[string]bool{"read_file": true}
		result, err := collapseFreeStream(stream)
		require.NoError(t, stream.Close())
		if legitimate {
			require.NoError(t, err)
			require.Equal(t, "read_file", gjson.GetBytes(result, "choices.0.message.tool_calls.0.function.name").String())
		} else {
			require.ErrorContains(t, err, "compatibility-only")
			require.Empty(t, result)
		}
	}
}

func TestFreeStreamFailureDoesNotEmitSuccessTerminator(t *testing.T) {
	old := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = old })
	for _, fixture := range []string{
		strings.ReplaceAll(freeChatFixture, "data: [DONE]\n\n", ""),
		strings.Split(freeChatFixture, "data: [DONE]")[0] + `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"bash","arguments":"{}"}}]}}]}` + "\n\n",
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeUnknown, IsStream: true, DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenCode, UpstreamModelName: "mimo-v2.5-free"}}
		stream := newFreeStream(context.Background(), io.NopCloser(strings.NewReader(fixture)), requestModeOpenAI, map[string]bool{"bash": true}, time.Second)
		adaptor := &Adaptor{RequestMode: requestModeOpenAI, freeResponseStream: stream}
		_, apiErr := adaptor.DoResponse(c, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: stream}, info)
		require.NotNil(t, apiErr)
		require.Equal(t, 502, apiErr.StatusCode)
		require.NotContains(t, recorder.Body.String(), "[DONE]")
		require.NotContains(t, recorder.Body.String(), `"name":"bash"`)
		if strings.Contains(recorder.Body.String(), "event: error") {
			before := recorder.Body.String()
			c.JSON(502, gin.H{"unexpected": "controller JSON"})
			require.Equal(t, before, recorder.Body.String())
		}
	}
}

func TestFreeResponsesPreservesFinalFieldsAndIncompleteStatus(t *testing.T) {
	fixture := strings.ReplaceAll(freeResponsesFixture, "response.completed", "response.incomplete")
	fixture = strings.ReplaceAll(fixture, `"status":"completed","created_at"`, `"status":"incomplete","created_at"`)
	stream := newFreeStream(context.Background(), io.NopCloser(strings.NewReader(fixture)), requestModeResponses, nil, time.Second)
	defer stream.Close()
	body, err := collapseFreeStream(stream)
	require.NoError(t, err)
	require.Equal(t, "incomplete", gjson.GetBytes(body, "status").String())
	require.Equal(t, "false", gjson.GetBytes(body, "extra").Raw)
	require.Equal(t, int64(1), gjson.GetBytes(body, "usage.input_tokens_details.cached_tokens").Int())
}

func TestFreeStreamSizeLimits(t *testing.T) {
	for _, body := range []string{"data: " + strings.Repeat("x", freeEventLimit), strings.Repeat(": comment\n", freeEventLimit/10+1)} {
		stream := newFreeStream(context.Background(), io.NopCloser(strings.NewReader(body)), requestModeOpenAI, nil, time.Second)
		_, err := io.ReadAll(stream)
		require.Error(t, err)
		require.NoError(t, stream.Close())
	}
}

func TestFreePassThroughOnlyForUnknownModels(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  bool
	}{
		{"mimo-v2.5-free", true},
		{"space-bunny-free", false},
		{"unlisted-free", false},
	} {
		info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeUnknown, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: constant.OpenCodeZenBaseURLAlias, UpstreamModelName: tc.model}}
		info.ChannelSetting.PassThroughBodyEnabled = true
		adaptor := &Adaptor{}
		adaptor.Init(info)
		require.Equal(t, tc.want, adaptor.needsFreeCompatibility(info), tc.model)
	}
}

func TestFreeRequestCancellationBeforeUpstreamHeaders(t *testing.T) {
	service.InitHttpClient()
	entered := make(chan struct{})
	exited := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
		close(exited)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeUnknown, ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenCode, ChannelBaseUrl: server.URL, UpstreamModelName: "mimo-v2.5-free", ApiKey: "test"}}
	a := &Adaptor{}
	a.Init(info)
	result := make(chan error, 1)
	go func() {
		_, err := a.DoRequest(c, info, strings.NewReader(`{"model":"mimo-v2.5-free","messages":[]}`))
		result <- err
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream was not reached")
	}
	cancel()
	select {
	case err := <-result:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("request was not cancelled")
	}
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream context was not cancelled")
	}
}
