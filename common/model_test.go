package common

import (
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestIsOpenAIResponseCompactModel(t *testing.T) {
	t.Parallel()

	if !IsOpenAIResponseCompactModel("gpt-5-openai-compact") {
		t.Fatal("expected compact suffix model to be detected")
	}
	if IsOpenAIResponseCompactModel("gpt-5") {
		t.Fatal("did not expect regular model to be detected as compact")
	}
}

func TestGetEndpointTypesByChannelTypeForCompactModel(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "gpt-5-openai-compact")
	if len(endpoints) != 1 || endpoints[0] != constant.EndpointTypeOpenAIResponseCompact {
		t.Fatalf("expected compact model to expose only response compact endpoint, got %#v", endpoints)
	}
}

func TestOpenAIVideoDefaultEndpointInfo(t *testing.T) {
	t.Parallel()

	info, ok := GetDefaultEndpointInfo(constant.EndpointTypeOpenAIVideo)
	if !ok {
		t.Fatal("expected openai-video default endpoint info to exist")
	}
	if info.Path != "/v1/videos" || info.Method != "POST" {
		t.Fatalf("unexpected openai-video endpoint info: %#v", info)
	}
}

func TestGetEndpointTypesByChannelTypeForKilo(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeKilo, "gpt-5")
	if len(endpoints) != 1 || endpoints[0] != constant.EndpointTypeOpenAI {
		t.Fatalf("expected kilo channel to expose only chat completions endpoint, got %#v", endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForGMICloud(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model string
		want  []constant.EndpointType
	}{
		{
			name:  "text",
			model: "MiniMaxAI/MiniMax-M2.7",
			want: []constant.EndpointType{
				constant.EndpointTypeOpenAI,
				constant.EndpointTypeOpenAIResponse,
				constant.EndpointTypeAnthropic,
			},
		},
		{
			name:  "speech",
			model: "minimax-tts-speech-2.8-turbo",
			want:  []constant.EndpointType{constant.EndpointTypeAudioSpeech},
		},
		{
			name:  "voice clone",
			model: "minimax-audio-voice-clone-speech-2.8-hd",
			want:  []constant.EndpointType{constant.EndpointTypeAudioSpeech},
		},
		{
			name:  "music",
			model: "minimax-music-3.0",
			want:  []constant.EndpointType{constant.EndpointTypeOpenAI},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeGMICloud, tt.model)
			if !reflect.DeepEqual(endpoints, tt.want) {
				t.Fatalf("expected GMI Cloud %s endpoints %#v, got %#v", tt.name, tt.want, endpoints)
			}
		})
	}
}

func TestGetEndpointTypesByChannelTypeForPoeClaudeModel(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypePoe, "Claude-Sonnet-4.6")
	want := []constant.EndpointType{
		constant.EndpointTypeAnthropic,
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Poe Claude model endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForPoeNonClaudeModel(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypePoe, "GPT-5.4")
	want := []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Poe non-Claude model endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForZhipuClaudeModel(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeZhipu, "claude-3-7-sonnet")
	want := []constant.EndpointType{
		constant.EndpointTypeAnthropic,
		constant.EndpointTypeOpenAI,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Zhipu Claude model endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForZhipuNonClaudeModel(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeZhipu, "glm-4-plus")
	want := []constant.EndpointType{
		constant.EndpointTypeOpenAI,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Zhipu non-Claude model endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForZhipuV4(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeZhipu_v4, "glm-4.6")
	want := []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Zhipu V4 endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForDeepSeek(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeDeepSeek, "deepseek-chat")
	want := []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeAnthropic,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected DeepSeek endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForVolcEngine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model string
		want  []constant.EndpointType
	}{
		{
			name:  "chat model",
			model: "doubao-seed-2-1-pro-260628",
			want: []constant.EndpointType{
				constant.EndpointTypeOpenAI,
				constant.EndpointTypeOpenAIResponse,
			},
		},
		{
			name:  "text embedding model",
			model: "doubao-embedding-text-240715",
			want:  []constant.EndpointType{constant.EndpointTypeEmbeddings},
		},
		{
			name:  "image generation model",
			model: "doubao-seedream-5-0-260128",
			want: []constant.EndpointType{
				constant.EndpointTypeImageGeneration,
				constant.EndpointTypeOpenAI,
				constant.EndpointTypeOpenAIResponse,
			},
		},
		{
			name:  "video generation model",
			model: "doubao-seedance-2-0-fast-260128",
			want:  []constant.EndpointType{constant.EndpointTypeOpenAIVideo},
		},
		{
			name:  "wan video generation model",
			model: "wan2-1-14b-i2v-250225",
			want:  []constant.EndpointType{constant.EndpointTypeOpenAIVideo},
		},
		{
			name:  "3d generation model",
			model: "doubao-seed3d-2-0-260328",
			want:  []constant.EndpointType{constant.EndpointTypeOpenAIVideo},
		},
		{
			name:  "hyper 3d generation model",
			model: "hyper3d-gen2-260112",
			want:  []constant.EndpointType{constant.EndpointTypeOpenAIVideo},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeVolcEngine, tt.model)
			if !reflect.DeepEqual(endpoints, tt.want) {
				t.Fatalf("expected VolcEngine endpoints %#v, got %#v", tt.want, endpoints)
			}
		})
	}
}

func TestGetEndpointTypesByChannelTypeForOpenCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model string
		want  []constant.EndpointType
	}{
		{
			name:  "zen responses model",
			model: "gpt-5.5",
			want:  []constant.EndpointType{constant.EndpointTypeOpenAIResponse},
		},
		{
			name:  "anthropic model",
			model: "claude-sonnet-4-6",
			want:  []constant.EndpointType{constant.EndpointTypeAnthropic},
		},
		{
			name:  "gemini model",
			model: "gemini-3-flash",
			want:  []constant.EndpointType{constant.EndpointTypeGemini},
		},
		{
			name:  "chat model",
			model: "glm-5.1",
			want:  []constant.EndpointType{constant.EndpointTypeOpenAI},
		},
		{
			name:  "zen chat and go anthropic model",
			model: "minimax-m2.7",
			want: []constant.EndpointType{
				constant.EndpointTypeAnthropic,
				constant.EndpointTypeOpenAI,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeOpenCode, tt.model)

			if !reflect.DeepEqual(endpoints, tt.want) {
				t.Fatalf("expected OpenCode endpoints %#v, got %#v", tt.want, endpoints)
			}
		})
	}
}

func TestGetEndpointTypesByChannelTypeForQwenTokenPlan(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeQwenTokenPlan, "qwen3.7-plus")
	want := []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
		constant.EndpointTypeAnthropic,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Qwen Token Plan endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForMoark(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model string
		want  []constant.EndpointType
	}{
		{
			name:  "chat model",
			model: "DeepSeek-V3",
			want: []constant.EndpointType{
				constant.EndpointTypeOpenAI,
				constant.EndpointTypeOpenAIResponse,
				constant.EndpointTypeAnthropic,
			},
		},
		{
			name:  "embedding model",
			model: "Qwen3-Embedding-8B",
			want:  []constant.EndpointType{constant.EndpointTypeEmbeddings},
		},
		{
			name:  "bge embedding model",
			model: "bge-m3",
			want:  []constant.EndpointType{constant.EndpointTypeEmbeddings},
		},
		{
			name:  "reranker model",
			model: "Qwen3-Reranker-4B",
			want:  []constant.EndpointType{constant.EndpointTypeJinaRerank},
		},
		{
			name:  "image model",
			model: "Qwen-Image",
			want: []constant.EndpointType{
				constant.EndpointTypeImageGeneration,
				constant.EndpointTypeOpenAI,
				constant.EndpointTypeOpenAIResponse,
				constant.EndpointTypeAnthropic,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeMoark, tt.model)
			if !reflect.DeepEqual(endpoints, tt.want) {
				t.Fatalf("expected Moark endpoints %#v, got %#v", tt.want, endpoints)
			}
		})
	}
}

func TestGetEndpointTypesByChannelTypeForXunfeiMaaSImage(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeXunfeiMaaSImage, "idxskolorss2b6")
	want := []constant.EndpointType{
		constant.EndpointTypeImageGeneration,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Xunfei MaaS image channel endpoints %#v, got %#v", want, endpoints)
	}
}

func TestDefaultEndpointInfoCoversGeneralTemplates(t *testing.T) {
	t.Parallel()
	tests := map[constant.EndpointType]string{
		constant.EndpointTypeGemini:             "/v1beta/models/{model}:generateContent",
		constant.EndpointTypeGeminiEmbeddings:   "/v1beta/models/{model}:embedContent",
		constant.EndpointTypeImageEdit:          "/v1/images/edits",
		constant.EndpointTypeVideoEdit:          "/v1/videos/edits",
		constant.EndpointTypeVideoExtension:     "/v1/videos/extensions",
		constant.EndpointTypeAudioSpeech:        "/v1/audio/speech",
		constant.EndpointTypeAudioTranscription: "/v1/audio/transcriptions",
		constant.EndpointTypeAudioTranslation:   "/v1/audio/translations",
		constant.EndpointTypeModerations:        "/v1/moderations",
	}
	for endpointType, wantPath := range tests {
		info, ok := GetDefaultEndpointInfo(endpointType)
		if !ok || info.Path != wantPath || info.Method != "POST" {
			t.Fatalf("unexpected default endpoint for %s: %#v (ok=%v)", endpointType, info, ok)
		}
	}
}
