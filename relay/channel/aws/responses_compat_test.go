package aws

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestUsesDirectClaudeCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	maxOutputTokens := uint(64)
	temperature := 0.2
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-5-sonnet-20241022",
		},
	}
	adaptor := &Adaptor{}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:           "claude-3-5-sonnet-20241022",
		Instructions:    []byte(`"be concise"`),
		Input:           []byte(`"hello"`),
		MaxOutputTokens: &maxOutputTokens,
		Temperature:     &temperature,
	})

	require.NoError(t, err)
	require.False(t, adaptor.IsNova)
	claudeReq, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *dto.ClaudeRequest", converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.FinalRequestRelayFormat)
	require.NotNil(t, claudeReq.MaxTokens)
	require.Equal(t, maxOutputTokens, *claudeReq.MaxTokens)
	require.Equal(t, &temperature, claudeReq.Temperature)
	require.Len(t, claudeReq.Messages, 1)
	require.Equal(t, "user", claudeReq.Messages[0].Role)
	require.Equal(t, "hello", claudeReq.Messages[0].Content)
}

func TestConvertOpenAIResponsesRequestUsesDirectNovaCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	maxOutputTokens := uint(64)
	temperature := 0.2
	topP := 0.8
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "nova-lite-v1:0",
		},
	}
	adaptor := &Adaptor{}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:           "nova-lite-v1:0",
		Instructions:    []byte(`"be concise"`),
		Input:           []byte(`"hello"`),
		MaxOutputTokens: &maxOutputTokens,
		Temperature:     &temperature,
		TopP:            &topP,
	})

	require.NoError(t, err)
	require.True(t, adaptor.IsNova)
	novaReq, ok := converted.(*NovaRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *NovaRequest", converted)
	require.Empty(t, info.FinalRequestRelayFormat)
	require.Equal(t, "messages-v1", novaReq.SchemaVersion)
	require.Len(t, novaReq.Messages, 2)
	require.Equal(t, "system", novaReq.Messages[0].Role)
	require.Equal(t, "be concise", novaReq.Messages[0].Content[0].Text)
	require.Equal(t, "user", novaReq.Messages[1].Role)
	require.Equal(t, "hello", novaReq.Messages[1].Content[0].Text)
	require.NotNil(t, novaReq.InferenceConfig)
	require.Equal(t, int(maxOutputTokens), novaReq.InferenceConfig.MaxTokens)
	require.Equal(t, temperature, novaReq.InferenceConfig.Temperature)
	require.Equal(t, topP, novaReq.InferenceConfig.TopP)
}

func TestConvertOpenAIResponsesRequestRejectsStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	stream := true

	_, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{}, dto.OpenAIResponsesRequest{
		Model:  "claude-3-5-sonnet-20241022",
		Input:  []byte(`"hello"`),
		Stream: &stream,
	})
	require.Error(t, err)
}
