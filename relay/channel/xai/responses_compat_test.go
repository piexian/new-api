package xai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestPassesThroughOfficialResponsesAPI(t *testing.T) {
	t.Parallel()

	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeResponses,
		RelayFormat:    types.RelayFormatOpenAIResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.x.ai",
			UpstreamModelName: "grok-4",
		},
	}
	request := dto.OpenAIResponsesRequest{
		Input: []byte(`"hello"`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)

	require.NoError(t, err)
	got, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want dto.OpenAIResponsesRequest", converted)
	require.Equal(t, "grok-4", got.Model)
	require.Equal(t, request.Input, got.Input)
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.FinalRequestRelayFormat)

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.x.ai/v1/responses", url)
}

func TestConvertOpenAIResponsesRequestPassesThroughCompactionAPI(t *testing.T) {
	t.Parallel()

	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(`{
		"model": "grok-4.3",
		"input": [{"role":"user","content":"hello"}],
		"reasoning": {"effort": "medium"},
		"xai_native": {"preserve": true}
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeResponsesCompact,
		RelayFormat:    types.RelayFormatOpenAIResponsesCompaction,
		RequestURLPath: "/v1/responses/compact",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.x.ai",
			UpstreamModelName: "grok-4.3",
		},
	}
	request := dto.OpenAIResponsesRequest{
		Input: []byte(`[{"role":"user","content":"hello"}]`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)

	require.NoError(t, err)
	got, ok := converted.(map[string]any)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want map[string]any", converted)
	require.Equal(t, "grok-4.3", got["model"])
	require.Contains(t, got, "input")
	native, ok := got["xai_native"].(map[string]any)
	require.True(t, ok, "xai_native = %T", got["xai_native"])
	require.Equal(t, true, native["preserve"])
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponsesCompaction), info.FinalRequestRelayFormat)

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.x.ai/v1/responses/compact", url)
}

func TestDoResponsePassesThroughXAICompactionResponse(t *testing.T) {
	t.Parallel()

	body := `{
		"id": "cmp_123",
		"object": "response.compaction",
		"created_at": 1748895600,
		"model": "grok-4.3",
		"output": [
			{"type": "compaction", "id": "cmp_123", "encrypted_content": "opaque"}
		],
		"usage": {
			"input_tokens": 120,
			"input_tokens_details": {"cached_tokens": 10},
			"output_tokens": 8,
			"total_tokens": 128,
			"dropped_message_count": 4
		}
	}`
	recorder := httptest.NewRecorder()
	c := gin.CreateTestContextOnly(recorder, gin.New())
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponsesCompact,
	}

	usageAny, err := (&Adaptor{}).DoResponse(c, resp, info)

	require.Nil(t, err)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok, "usage = %T", usageAny)
	require.Equal(t, 120, usage.PromptTokens)
	require.Equal(t, 8, usage.CompletionTokens)
	require.Equal(t, 128, usage.TotalTokens)
	require.Equal(t, 10, usage.PromptTokensDetails.CachedTokens)
	require.JSONEq(t, body, recorder.Body.String())
}

func TestConvertOpenAIRequestPreservesXAINativeJSONFields(t *testing.T) {
	t.Parallel()

	body := `{
		"model": "grok-4.3",
		"messages": [{"role": "user", "content": "126/3=?"}],
		"deferred": true,
		"xai_native": {"foo": "bar"}
	}`

	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	var request dto.GeneralOpenAIRequest
	require.NoError(t, common.Unmarshal([]byte(body), &request))
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-4.3",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, &request)

	require.NoError(t, err)
	payload, ok := converted.(map[string]any)
	require.True(t, ok, "ConvertOpenAIRequest returned %T", converted)
	require.Equal(t, "grok-4.3", payload["model"])
	require.Equal(t, true, payload["deferred"])
	native, ok := payload["xai_native"].(map[string]any)
	require.True(t, ok, "xai_native = %T", payload["xai_native"])
	require.Equal(t, "bar", native["foo"])
}
