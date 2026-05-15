package vertex

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestUsesDirectGeminiCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	adaptor := &Adaptor{RequestMode: RequestModeGemini}
	maxOutputTokens := uint(64)
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeVertexAi,
			UpstreamModelName: "gemini-2.5-flash",
		},
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:           "gemini-2.5-flash",
		Instructions:    []byte(`"be concise"`),
		Input:           []byte(`"hello"`),
		MaxOutputTokens: &maxOutputTokens,
	})

	require.NoError(t, err)
	geminiReq, ok := converted.(*dto.GeminiChatRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *dto.GeminiChatRequest", converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.FinalRequestRelayFormat)
	require.NotNil(t, geminiReq.SystemInstructions)
	require.Equal(t, "be concise", geminiReq.SystemInstructions.Parts[0].Text)
	require.Len(t, geminiReq.Contents, 1)
	require.Equal(t, "hello", geminiReq.Contents[0].Parts[0].Text)
	require.NotNil(t, geminiReq.GenerationConfig.MaxOutputTokens)
	require.Equal(t, maxOutputTokens, *geminiReq.GenerationConfig.MaxOutputTokens)
}

func TestConvertOpenAIResponsesRequestUsesDirectOpenSourceCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	adaptor := &Adaptor{RequestMode: RequestModeOpenSource}
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeVertexAi,
			UpstreamModelName: "llama-3.1-maas",
		},
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "llama-3.1-maas",
		Input: []byte(`"hello"`),
	})

	require.NoError(t, err)
	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAI), info.FinalRequestRelayFormat)
	require.Len(t, chatReq.Messages, 1)
	require.Equal(t, "hello", chatReq.Messages[0].Content)
}
