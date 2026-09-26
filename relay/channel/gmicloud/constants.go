package gmicloud

import "strings"

const ChannelName = "gmicloud"

// GMI 的 LLM 与 requestqueue（音频、图像）服务在不同主机上。
const (
	defaultLLMBaseURL          = "https://api.gmi-serving.com"
	defaultRequestQueueBaseURL = "https://console.gmicloud.ai"
)

// Requestqueue paths on console.gmicloud.ai.
const (
	submitRequestPath = "/api/v1/ie/requestqueue/apikey/requests"
	requestStatusPath = "/api/v1/ie/requestqueue/apikey/requests/"
)

// maxMediaDownloadBytes 上游返回的媒体 URL 由服务端代理下载，限制单次下载体积。
const maxMediaDownloadBytes = 32 << 20

var ModelList = []string{
	"MiniMaxAI/MiniMax-M2.7",
	"minimax-tts-speech-2.8-turbo",
	"minimax-tts-speech-2.8-hd",
	"minimax-audio-voice-clone-speech-2.8-hd",
	"minimax-audio-voice-clone-speech-2.6-hd",
	"minimax-music-3.0",
	"hy-image-v3.5-preview",
}

func isGMIImageModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "hy-image-")
}

// IsSupportedImageModel reports whether the upstream model is implemented by the image adaptor.
func IsSupportedImageModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "hy-image-v3.5-preview")
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
