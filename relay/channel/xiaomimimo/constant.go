package xiaomimimo

import "strings"

// https://platform.xiaomimimo.com/docs/zh-CN/api/chat/openai-api
// https://platform.xiaomimimo.com/docs/zh-CN/api/chat/anthropic-api
// https://platform.xiaomimimo.com/docs/zh-CN/usage-guide/speech-synthesis-v2.5
// https://platform.xiaomimimo.com/docs/zh-CN/usage-guide/speech-synthesis

var ModelList = []string{
	// Text models (OpenAI + Anthropic)
	"mimo-v2.5-pro",
	"mimo-v2.5",
	"mimo-v2-pro",
	"mimo-v2-omni",
	"mimo-v2-flash",
	// TTS V2
	"mimo-v2-tts",
	// TTS V2.5
	"mimo-v2.5-tts",
	"mimo-v2.5-tts-voicedesign",
	"mimo-v2.5-tts-voiceclone",
}

var ChannelName = "xiaomimimo"

func isMimoTTSModel(model string) bool {
	return strings.HasPrefix(model, "mimo-") && strings.Contains(model, "tts")
}

func NormalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	for _, suffix := range []string{"/v1", "/anthropic"} {
		baseURL = strings.TrimSuffix(baseURL, suffix)
	}
	if baseURL == "" {
		baseURL = "https://api.xiaomimimo.com"
	}
	return baseURL
}
