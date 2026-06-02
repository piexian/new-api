package xai

import (
	"net/http"
	"testing"

	appconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestValidateEndpointForModelAllowsMatchingXAIRoutes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		info  *relaycommon.RelayInfo
		model string
	}{
		{
			name:  "image model on image generation",
			info:  newXAIEndpointInfo(relayconstant.RelayModeImagesGenerations, types.RelayFormatOpenAIImage, "/v1/images/generations"),
			model: "grok-imagine-image-quality",
		},
		{
			name:  "video model on video generation",
			info:  newXAIEndpointInfo(relayconstant.RelayModeVideoSubmit, types.RelayFormatTask, "/v1/videos/generations"),
			model: "grok-imagine-video-1.5-preview",
		},
		{
			name:  "voice model on audio speech",
			info:  newXAIEndpointInfo(relayconstant.RelayModeAudioSpeech, types.RelayFormatOpenAIAudio, "/v1/audio/speech"),
			model: "grok-voice-latest",
		},
		{
			name:  "voice model on native tts",
			info:  newXAIEndpointInfo(relayconstant.RelayModeXAINative, types.RelayFormatXAI, "/v1/tts"),
			model: "grok-voice-fast-1.0",
		},
		{
			name:  "text model on native responses",
			info:  newXAIEndpointInfo(relayconstant.RelayModeXAINative, types.RelayFormatXAI, "/v1/responses/resp_123"),
			model: "grok-4.3",
		},
		{
			name:  "custom router alias",
			info:  newXAIEndpointInfo(relayconstant.RelayModeXAINative, types.RelayFormatXAI, "/v1/tts/voices"),
			model: "custom-voice-router",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.info.UpstreamModelName = test.model

			err := ValidateEndpointForModel(test.info)

			require.Nil(t, err)
		})
	}
}

func TestValidateEndpointForModelRejectsDedicatedModelsOnWrongXAIRoutes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		info       *relaycommon.RelayInfo
		model      string
		wantStatus int
		want       string
	}{
		{
			name:       "image model on chat",
			info:       newXAIEndpointInfo(relayconstant.RelayModeChatCompletions, types.RelayFormatOpenAI, "/v1/chat/completions"),
			model:      "grok-imagine-image-pro",
			wantStatus: http.StatusBadRequest,
			want:       "must be used with /v1/chat/completions or /v1/responses endpoint",
		},
		{
			name:       "video model on images",
			info:       newXAIEndpointInfo(relayconstant.RelayModeImagesGenerations, types.RelayFormatOpenAIImage, "/v1/images/generations"),
			model:      "grok-imagine-video",
			wantStatus: http.StatusBadRequest,
			want:       "must be used with /v1/images/generations or /v1/images/edits endpoint",
		},
		{
			name:       "voice model on responses",
			info:       newXAIEndpointInfo(relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses, "/v1/responses"),
			model:      "grok-voice-latest",
			wantStatus: http.StatusBadRequest,
			want:       "must be used with /v1/chat/completions or /v1/responses endpoint",
		},
		{
			name:       "image model on native tts",
			info:       newXAIEndpointInfo(relayconstant.RelayModeXAINative, types.RelayFormatXAI, "/v1/tts"),
			model:      "grok-2-image-1212",
			wantStatus: http.StatusBadRequest,
			want:       "must be used with /v1/tts, /v1/stt, or /v1/realtime endpoint",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.info.UpstreamModelName = test.model

			err := ValidateEndpointForModel(test.info)

			require.NotNil(t, err)
			require.Equal(t, test.wantStatus, err.StatusCode)
			require.Equal(t, types.ErrorCodeInvalidRequest, err.GetErrorCode())
			require.Contains(t, err.Error(), test.want)
		})
	}
}

func newXAIEndpointInfo(relayMode int, relayFormat types.RelayFormat, path string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayMode:      relayMode,
		RelayFormat:    relayFormat,
		RequestURLPath: path,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    appconstant.ChannelTypeXai,
			ChannelBaseUrl: "https://api.x.ai",
		},
	}
}
