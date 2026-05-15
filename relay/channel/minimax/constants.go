package minimax

import "strings"

// https://platform.minimaxi.com/docs/api-reference/api-overview

var ModelList = []string{
	"MiniMax-M2.7",
	"MiniMax-M2.7-highspeed",
	"MiniMax-M2.5",
	"MiniMax-M2.5-highspeed",
	"MiniMax-M2.1",
	"MiniMax-M2.1-highspeed",
	"MiniMax-M2",
	"speech-2.8-hd",
	"speech-2.8-turbo",
	"speech-2.6-hd",
	"speech-2.6-turbo",
	"speech-02-hd",
	"speech-02-turbo",
	"speech-01-hd",
	"speech-01-turbo",
	"image-01",
	"image-01-live",
	"music-2.6",
	"music-cover",
	"music-2.6-free",
	"music-cover-free",
	MusicCoverPreprocessModel,
	LyricsGenerationModel,
	"MiniMax-Hailuo-2.3",
	"MiniMax-Hailuo-2.3-Fast",
	"MiniMax-Hailuo-02",
	"T2V-01-Director",
	"T2V-01",
	"I2V-01-Director",
	"I2V-01-live",
	"I2V-01",
	"S2V-01",
}

var ChannelName = "minimax"

const (
	MusicCoverPreprocessModel = "music_cover_preprocess"
	LyricsGenerationModel     = "lyrics_generation"

	MusicGenerationEndpoint      = "/v1/music_generation"
	MusicCoverPreprocessEndpoint = "/v1/music_cover_preprocess"
	LyricsGenerationEndpoint     = "/v1/lyrics_generation"
	ChatCompletionsEndpoint      = "/v1/chat/completions"
	AnthropicMessagesEndpoint    = "/v1/messages"
	SpeechEndpoint               = "/v1/audio/speech"
	ImageGenerationEndpoint      = "/v1/image_generation"
	MusicGenerationDocURL        = "https://platform.minimaxi.com/docs/api-reference/music-generation"
	MusicCoverPreprocessDocURL   = "https://platform.minimaxi.com/docs/api-reference/music-cover-preprocess"
	LyricsGenerationDocURL       = "https://platform.minimaxi.com/docs/api-reference/lyrics-generation"
	OpenAIChatCompletionsDocURL  = "https://platform.minimaxi.com/docs/api-reference/text-chat-openai"
	AnthropicMessagesDocURL      = "https://platform.minimaxi.com/docs/api-reference/text-chat-anthropic"
	SpeechDocURL                 = "https://platform.minimaxi.com/docs/api-reference/speech-t2a-http"
	ImageGenerationDocURL        = "https://platform.minimaxi.com/docs/api-reference/image-generation-t2i"
)

var NativeEndpointModelList = []string{
	MusicCoverPreprocessModel,
	LyricsGenerationModel,
}

func isMiniMaxMusicModel(model string) bool {
	return strings.HasPrefix(model, "music-")
}

func isMiniMaxSpeechModel(model string) bool {
	return strings.HasPrefix(model, "speech-")
}

func isMiniMaxImageModel(model string) bool {
	return model == "image-01" || model == "image-01-live"
}

func isMiniMaxTextModel(model string) bool {
	return strings.HasPrefix(model, "MiniMax-") || strings.HasPrefix(model, "abab")
}
