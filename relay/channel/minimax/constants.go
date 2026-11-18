package minimax

import "strings"

// https://platform.minimaxi.com/docs/api-reference/api-overview

var ModelList = []string{
	// 语言模型
	"MiniMax-M3.1-Flash",
	"MiniMax-M3",
	// 语音模型
	"speech-2.8-hd",
	"speech-2.8-turbo",
	"speech-2.6-hd",
	"speech-2.6-turbo",
	"speech-02-hd",
	"speech-02-turbo",
	// 图片模型
	"MiniMax-H3-Image",
	"MiniMax-H3-Image-Highspeed",
	"image-01",
	"image-01-live",
	// 视频模型(视频任务接口)
	"MiniMax-H3",
	"MiniMax-H3-Highspeed",
	"MiniMax-H3-Lite",
	"MiniMax-H3-Max",
	"MiniMax-Hailuo-2.3",
	"MiniMax-Hailuo-2.3-Fast",
	"MiniMax-Hailuo-02",
	// 音乐模型:官方 2026-08-20 停止新用户提供,历史付费用户仍可调用
	"music-2.6",
	"music-cover",
	"music-2.6-free",
	"music-cover-free",
	MusicCoverPreprocessModel,
	LyricsGenerationModel,
}

var ChannelName = "minimax"

const (
	MusicCoverPreprocessModel = "music_cover_preprocess"
	LyricsGenerationModel     = "lyrics_generation"

	MusicGenerationEndpoint      = "/v1/music_generation"
	MusicCoverPreprocessEndpoint = "/v1/music_cover_preprocess"
	LyricsGenerationEndpoint     = "/v1/lyrics_generation"
	ChatCompletionsEndpoint      = "/v1/chat/completions"
	ResponsesEndpoint            = "/v1/responses"
	ResponsesInputTokensEndpoint = "/v1/responses/input_tokens"
	AnthropicMessagesEndpoint    = "/v1/messages"
	AnthropicCountTokensEndpoint = "/v1/messages/count_tokens"
	SpeechEndpoint               = "/v1/audio/speech"
	ImageGenerationEndpoint      = "/v1/image_generation"
	MusicGenerationDocURL        = "https://platform.minimaxi.com/docs/api-reference/music-generation"
	MusicCoverPreprocessDocURL   = "https://platform.minimaxi.com/docs/api-reference/music-cover-preprocess"
	LyricsGenerationDocURL       = "https://platform.minimaxi.com/docs/api-reference/lyrics-generation"
	OpenAIChatCompletionsDocURL  = "https://platform.minimaxi.com/docs/api-reference/text-chat-openai"
	ResponsesDocURL              = "https://platform.minimaxi.com/docs/api-reference/responses-create"
	ResponsesInputTokensDocURL   = "https://platform.minimaxi.com/docs/api-reference/responses-input-tokens"
	AnthropicMessagesDocURL      = "https://platform.minimaxi.com/docs/api-reference/text-chat-anthropic"
	SpeechDocURL                 = "https://platform.minimaxi.com/docs/api-reference/speech-t2a-http"
	ImageGenerationDocURL        = "https://platform.minimaxi.com/docs/api-reference/image-generation-t2i"
)

var NativeEndpointModelList = []string{
	MusicCoverPreprocessModel,
	LyricsGenerationModel,
}

// MiniMax 上游模型名大小写不敏感，分类统一按小写匹配，避免同名模型因写法不同绕过或触发校验
func isMiniMaxMusicModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(model), "music-")
}

func isMiniMaxSpeechModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(model), "speech-")
}

func isMiniMaxImageModel(model string) bool {
	switch strings.ToLower(model) {
	case "image-01", "image-01-live":
		return true
	default:
		return false
	}
}

func isMiniMaxTextModel(model string) bool {
	model = strings.ToLower(model)
	return strings.HasPrefix(model, "minimax-") || strings.HasPrefix(model, "abab")
}
