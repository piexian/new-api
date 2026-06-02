package minimax

import (
	"strings"
	"testing"

	appconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
)

func TestExpectedEndpointForModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		model        string
		relayFormat  types.RelayFormat
		wantEndpoint string
		wantDoc      string
		wantLabel    string
		wantOK       bool
	}{
		{name: "music", model: "music-2.6", wantEndpoint: MusicGenerationEndpoint, wantDoc: MusicGenerationDocURL, wantLabel: "music model", wantOK: true},
		{name: "music cover", model: "music-cover", wantEndpoint: MusicGenerationEndpoint, wantDoc: MusicGenerationDocURL, wantLabel: "music model", wantOK: true},
		{name: "cover preprocess", model: MusicCoverPreprocessModel, wantEndpoint: MusicCoverPreprocessEndpoint, wantDoc: MusicCoverPreprocessDocURL, wantLabel: "music cover preprocess model", wantOK: true},
		{name: "lyrics", model: LyricsGenerationModel, wantEndpoint: LyricsGenerationEndpoint, wantDoc: LyricsGenerationDocURL, wantLabel: "lyrics generation model", wantOK: true},
		{name: "speech", model: "speech-2.8-hd", wantEndpoint: SpeechEndpoint, wantDoc: SpeechDocURL, wantLabel: "speech model", wantOK: true},
		{name: "image", model: "image-01", wantEndpoint: ImageGenerationEndpoint, wantDoc: ImageGenerationDocURL, wantLabel: "image model", wantOK: true},
		{name: "m3 responses text", model: "MiniMax-M3", relayFormat: types.RelayFormatOpenAIResponses, wantEndpoint: ResponsesEndpoint, wantDoc: ResponsesDocURL, wantLabel: "text model", wantOK: true},
		{name: "openai text", model: "MiniMax-M2.7", wantEndpoint: ChatCompletionsEndpoint, wantDoc: OpenAIChatCompletionsDocURL, wantLabel: "text model", wantOK: true},
		{name: "anthropic text", model: "MiniMax-M2.7", relayFormat: types.RelayFormatClaude, wantEndpoint: AnthropicMessagesEndpoint, wantDoc: AnthropicMessagesDocURL, wantLabel: "text model", wantOK: true},
		{name: "unknown", model: "custom-model", wantOK: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := expectedEndpointForModel(tt.model, tt.relayFormat)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Endpoint != tt.wantEndpoint {
				t.Fatalf("Endpoint = %q, want %q", got.Endpoint, tt.wantEndpoint)
			}
			if got.DocURL != tt.wantDoc {
				t.Fatalf("DocURL = %q, want %q", got.DocURL, tt.wantDoc)
			}
			if got.Label != tt.wantLabel {
				t.Fatalf("Label = %q, want %q", got.Label, tt.wantLabel)
			}
		})
	}
}

func TestValidateEndpointForModelAllowsCorrectMiniMaxEndpoint(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatMiniMax,
		RelayMode:       relayconstant.RelayModeMiniMaxMusicGeneration,
		OriginModelName: "music-2.6",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       appconstant.ChannelTypeMiniMax,
			UpstreamModelName: "music-2.6",
		},
	}
	if err := ValidateEndpointForModel(info); err != nil {
		t.Fatalf("ValidateEndpointForModel returned error: %v", err)
	}
}

func TestValidateEndpointForModelAllowsOfficialCoverModelOnPreprocessEndpoint(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatMiniMax,
		RelayMode:       relayconstant.RelayModeMiniMaxMusicCoverPreprocess,
		OriginModelName: MusicCoverPreprocessModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       appconstant.ChannelTypeMiniMax,
			UpstreamModelName: "music-cover",
		},
	}
	if err := ValidateEndpointForModel(info); err != nil {
		t.Fatalf("ValidateEndpointForModel returned error: %v", err)
	}
}

func TestValidateEndpointForModelAllowsResponsesConversionForTextModel(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeResponses,
		OriginModelName: "MiniMax-M2.7",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       appconstant.ChannelTypeMiniMax,
			UpstreamModelName: "MiniMax-M2.7",
		},
	}
	if err := ValidateEndpointForModel(info); err != nil {
		t.Fatalf("ValidateEndpointForModel returned error: %v", err)
	}
}

func TestValidateEndpointForModelAllowsResponsesInputTokensForTextModel(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeResponsesInputTokens,
		OriginModelName: "MiniMax-M3",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       appconstant.ChannelTypeMiniMax,
			UpstreamModelName: "MiniMax-M3",
		},
	}
	if err := ValidateEndpointForModel(info); err != nil {
		t.Fatalf("ValidateEndpointForModel returned error: %v", err)
	}
}

func TestValidateEndpointForModelAllowsAnthropicCountTokensForTextModel(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatClaude,
		RelayMode:       relayconstant.RelayModeClaudeCountTokens,
		RequestURLPath:  AnthropicCountTokensEndpoint,
		OriginModelName: "MiniMax-M3",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       appconstant.ChannelTypeMiniMax,
			UpstreamModelName: "MiniMax-M3",
		},
	}
	if err := ValidateEndpointForModel(info); err != nil {
		t.Fatalf("ValidateEndpointForModel returned error: %v", err)
	}
}

func TestValidateEndpointForModelRejectsResponsesCompactForTextModel(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAIResponsesCompaction,
		RelayMode:       relayconstant.RelayModeResponsesCompact,
		OriginModelName: "MiniMax-M2.7-openai-compact",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       appconstant.ChannelTypeMiniMax,
			UpstreamModelName: "MiniMax-M2.7-openai-compact",
		},
	}
	err := ValidateEndpointForModel(info)
	if err == nil {
		t.Fatal("ValidateEndpointForModel returned nil")
	}
	message := err.ToOpenAIError().Message
	for _, want := range []string{"MiniMax text model", ChatCompletionsEndpoint, OpenAIChatCompletionsDocURL} {
		if !strings.Contains(message, want) {
			t.Fatalf("OpenAI error message %q does not contain %q", message, want)
		}
	}
}

func TestValidateEndpointForModelRejectsWrongMiniMaxEndpointAndKeepsDocURL(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "music-2.6",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       appconstant.ChannelTypeMiniMax,
			UpstreamModelName: "music-2.6",
		},
	}

	err := ValidateEndpointForModel(info)
	if err == nil {
		t.Fatal("ValidateEndpointForModel returned nil")
	}
	if err.StatusCode != 400 {
		t.Fatalf("StatusCode = %d, want 400", err.StatusCode)
	}
	message := err.ToOpenAIError().Message
	for _, want := range []string{"MiniMax music model", MusicGenerationEndpoint, MusicGenerationDocURL} {
		if !strings.Contains(message, want) {
			t.Fatalf("OpenAI error message %q does not contain %q", message, want)
		}
	}
	if strings.Contains(message, "music-2.6") {
		t.Fatalf("OpenAI error message should not echo raw model name: %q", message)
	}
	logMessage := err.MaskSensitiveErrorWithStatusCode()
	if !strings.Contains(logMessage, MusicGenerationDocURL) {
		t.Fatalf("masked log message %q does not keep doc URL %q", logMessage, MusicGenerationDocURL)
	}
	if strings.Contains(logMessage, "platform.***") || strings.Contains(logMessage, "/***/") {
		t.Fatalf("doc URL was masked in log message: %q", logMessage)
	}
}

func TestValidateEndpointForModelIgnoresNonMiniMaxChannel(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "music-2.6",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       appconstant.ChannelTypeOpenAI,
			UpstreamModelName: "music-2.6",
		},
	}
	if err := ValidateEndpointForModel(info); err != nil {
		t.Fatalf("ValidateEndpointForModel returned error for non-MiniMax channel: %v", err)
	}
}
