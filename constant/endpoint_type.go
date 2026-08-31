package constant

type EndpointType string

const (
	EndpointTypeOpenAI                EndpointType = "openai"
	EndpointTypeOpenAIResponse        EndpointType = "openai-response"
	EndpointTypeOpenAIResponseCompact EndpointType = "openai-response-compact"
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
	//EndpointTypeMidjourney     EndpointType = "midjourney-proxy"
	//EndpointTypeSuno           EndpointType = "suno-proxy"
	//EndpointTypeKling          EndpointType = "kling"
	//EndpointTypeJimeng         EndpointType = "jimeng"
)
