package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelTestEndpointRelayFormat(t *testing.T) {
	tests := []struct {
		endpoint constant.EndpointType
		want     types.RelayFormat
	}{
		{constant.EndpointTypeOpenAI, types.RelayFormatOpenAI},
		{constant.EndpointTypeCohereChat, types.RelayFormatOpenAI},
		{constant.EndpointTypeTypeSafe, types.RelayFormatTypeSafe},
		{constant.EndpointTypeOpenAIResponse, types.RelayFormatOpenAIResponses},
		{constant.EndpointTypeOpenAIResponseCompact, types.RelayFormatOpenAIResponsesCompaction},
		{constant.EndpointTypeAnthropic, types.RelayFormatClaude},
		{constant.EndpointTypeGemini, types.RelayFormatGemini},
		{constant.EndpointTypeGeminiInteractions, types.RelayFormatGemini},
		{constant.EndpointTypeJinaRerank, types.RelayFormatRerank},
		{constant.EndpointTypeCohereRerank, types.RelayFormatRerank},
		{constant.EndpointTypeImageGeneration, types.RelayFormatOpenAIImage},
		{constant.EndpointTypeImageEdit, types.RelayFormatOpenAIImage},
		{constant.EndpointTypeEmbeddings, types.RelayFormatEmbedding},
		{constant.EndpointTypeCohereEmbeddings, types.RelayFormatEmbedding},
		{constant.EndpointTypeGeminiEmbeddings, types.RelayFormatEmbedding},
		{constant.EndpointTypeOpenAIVideo, types.RelayFormatTask},
		{constant.EndpointTypeVideoEdit, types.RelayFormatTask},
		{constant.EndpointTypeVideoExtension, types.RelayFormatTask},
		{constant.EndpointTypeAudioSpeech, types.RelayFormatOpenAI},
		{constant.EndpointTypeAudioTranscription, types.RelayFormatOpenAI},
		{constant.EndpointTypeAudioTranslation, types.RelayFormatOpenAI},
		{constant.EndpointTypeModerations, types.RelayFormatOpenAI},
		{"unknown", types.RelayFormatOpenAI},
	}
	for _, tt := range tests {
		t.Run(string(tt.endpoint), func(t *testing.T) {
			require.Equal(t, tt.want, channelTestEndpointRelayFormat(tt.endpoint))
		})
	}
}

func TestChannelTestNormalizedEndpointBuildsRelayInfo(t *testing.T) {
	tests := []struct {
		name        string
		channelType int
		model       string
		endpoint    string
		want        types.RelayFormat
	}{
		{"opencode explicit mimo-v2.5", constant.ChannelTypeOpenCode, "mimo-v2.5", "openai", types.RelayFormatOpenAI},
		{"opencode explicit mimo-v2.6-flash", constant.ChannelTypeOpenCode, "mimo-v2.6-flash", "openai", types.RelayFormatOpenAI},
		{"opencode automatic mimo-v2.5", constant.ChannelTypeOpenCode, "mimo-v2.5", "", types.RelayFormatOpenAI},
		{"opencode automatic mimo-v2.6-flash", constant.ChannelTypeOpenCode, "mimo-v2.6-flash", "", types.RelayFormatOpenAI},
		{"cohere automatic", constant.ChannelTypeCohere, "command-r", "", types.RelayFormatOpenAI},
		{"typesafe explicit", constant.ChannelTypeTypeSafe, "jev-latest", "typesafe", types.RelayFormatTypeSafe},
		{"typesafe automatic", constant.ChannelTypeTypeSafe, "jev-latest", "", types.RelayFormatTypeSafe},
		{"opencode JEV free automatic", constant.ChannelTypeOpenCode, "jev-1.13-free", "", types.RelayFormatTypeSafe},
		{"opencode JEV explicit", constant.ChannelTypeOpenCode, "jev-1.13", "typesafe", types.RelayFormatTypeSafe},
		{"opencode Responses free automatic", constant.ChannelTypeOpenCode, "muse-spark-1.3-contributor-free", "", types.RelayFormatOpenAIResponses},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &model.Channel{Type: tt.channelType}
			endpoint := normalizeChannelTestEndpoint(channel, tt.model, tt.endpoint)
			require.NotEmpty(t, endpoint)
			endpointInfo, ok := common.GetDefaultEndpointInfo(constant.EndpointType(endpoint))
			require.True(t, ok)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, endpointInfo.Path, nil)
			request := buildTestRequest(tt.model, endpoint, channel, false)
			format := channelTestEndpointRelayFormat(constant.EndpointType(endpoint))
			info, err := relaycommon.GenRelayInfo(c, format, request, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, info.RelayFormat)
			require.Equal(t, []types.RelayFormat{tt.want}, info.RequestConversionChain)
			if tt.channelType == constant.ChannelTypeTypeSafe {
				require.Equal(t, relayconstant.RelayModeTypeSafeNative, info.RelayMode)
			}
		})
	}
}
