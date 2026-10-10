package types

// EndpointType identifies a downstream API surface. Moved from constant so
// the conversion kit (dto/relayconvert) has no host imports; constant keeps
// aliases for host code.
type EndpointType string

const (
	EndpointTypeOpenAI                EndpointType = "openai"
	EndpointTypeOpenAIResponse        EndpointType = "openai-response"
	EndpointTypeOpenAIResponseCompact EndpointType = "openai-response-compact"
	EndpointTypeOpenAIAlphaSearch     EndpointType = "openai-alpha-search"
	EndpointTypeAnthropic             EndpointType = "anthropic"
	EndpointTypeGemini                EndpointType = "gemini"
	EndpointTypeJinaRerank            EndpointType = "jina-rerank"
	EndpointTypeImageGeneration       EndpointType = "image-generation"
	EndpointTypeImageEdit             EndpointType = "image-edit"
	EndpointTypeEmbeddings            EndpointType = "embeddings"
	EndpointTypeGeminiEmbeddings      EndpointType = "gemini-embeddings"
	EndpointTypeGeminiInteractions    EndpointType = "gemini-interactions"
	EndpointTypeOpenAIVideo           EndpointType = "openai-video"
	EndpointTypeVideoEdit             EndpointType = "video-edit"
	EndpointTypeVideoExtension        EndpointType = "video-extension"
	EndpointTypeAudioSpeech           EndpointType = "audio-speech"
	EndpointTypeAudioTranscription    EndpointType = "audio-transcription"
	EndpointTypeAudioTranslation      EndpointType = "audio-translation"
	EndpointTypeModerations           EndpointType = "moderations"
	EndpointTypeCohereChat            EndpointType = "cohere-chat"
	EndpointTypeCohereRerank          EndpointType = "cohere-rerank"
	EndpointTypeCohereEmbeddings      EndpointType = "cohere-embeddings"
	EndpointTypeTypeSafe              EndpointType = "typesafe"
)

// Finish reasons shared by the OpenAI-compatible response formats.
// Declared as vars (not consts) because converter code takes their address
// for *string finish-reason fields.
var (
	FinishReasonStop          = "stop"
	FinishReasonToolCalls     = "tool_calls"
	FinishReasonLength        = "length"
	FinishReasonFunctionCall  = "function_call"
	FinishReasonContentFilter = "content_filter"
)
