package gmicloud

import "strings"

const ChannelName = "gmicloud"

// GMI serves LLM and audio workloads from different hosts.
const (
	defaultLLMBaseURL   = "https://api.gmi-serving.com"
	defaultAudioBaseURL = "https://console.gmicloud.ai"
)

// Requestqueue paths on console.gmicloud.ai.
const (
	submitRequestPath = "/api/v1/ie/requestqueue/apikey/requests"
	requestStatusPath = "/api/v1/ie/requestqueue/apikey/requests/"
)

var ModelList = []string{
	"MiniMaxAI/MiniMax-M2.7",
	"minimax-tts-speech-2.8-turbo",
	"minimax-tts-speech-2.8-hd",
	"minimax-audio-voice-clone-speech-2.8-hd",
	"minimax-audio-voice-clone-speech-2.6-hd",
	"minimax-music-3.0",
}

func isGMIMusicModel(model string) bool {
	return strings.HasPrefix(model, "minimax-music-")
}

// IsSupportedMusicModel reports whether the upstream model is implemented by this adaptor.
func IsSupportedMusicModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "minimax-music-3.0")
}

// IsSupportedSpeechModel reports whether the upstream model is implemented by the speech adaptor.
func IsSupportedSpeechModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "minimax-tts-speech-2.8-turbo",
		"minimax-tts-speech-2.8-hd",
		"minimax-audio-voice-clone-speech-2.8-hd",
		"minimax-audio-voice-clone-speech-2.6-hd":
		return true
	default:
		return false
	}
}
func isGMIVoiceCloneModel(model string) bool {
	return strings.HasPrefix(model, "minimax-audio-voice-clone-")
}

func isGMITTSModel(model string) bool {
	return strings.HasPrefix(model, "minimax-tts-")
}

func isGMIAudioModel(model string) bool {
	return isGMITTSModel(model) || isGMIVoiceCloneModel(model) || isGMIMusicModel(model)
}
