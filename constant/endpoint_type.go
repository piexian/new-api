package constant

import kit_types "github.com/QuantumNous/new-api/relaykit/types"

// EndpointType aliases relaykit/types.EndpointType so host code keeps its
// references while the conversion kit stays free of host imports.
type EndpointType = kit_types.EndpointType

const (
	EndpointTypeOpenAI                = kit_types.EndpointTypeOpenAI
	EndpointTypeOpenAIResponse        = kit_types.EndpointTypeOpenAIResponse
	EndpointTypeOpenAIResponseCompact = kit_types.EndpointTypeOpenAIResponseCompact
	EndpointTypeOpenAIAlphaSearch     = kit_types.EndpointTypeOpenAIAlphaSearch
	EndpointTypeAnthropic             = kit_types.EndpointTypeAnthropic
	EndpointTypeGemini                = kit_types.EndpointTypeGemini
	EndpointTypeJinaRerank            = kit_types.EndpointTypeJinaRerank
	EndpointTypeImageGeneration       = kit_types.EndpointTypeImageGeneration
	EndpointTypeImageEdit             = kit_types.EndpointTypeImageEdit
	EndpointTypeEmbeddings            = kit_types.EndpointTypeEmbeddings
	EndpointTypeGeminiEmbeddings      = kit_types.EndpointTypeGeminiEmbeddings
	EndpointTypeGeminiInteractions    = kit_types.EndpointTypeGeminiInteractions
	EndpointTypeOpenAIVideo           = kit_types.EndpointTypeOpenAIVideo
	EndpointTypeVideoEdit             = kit_types.EndpointTypeVideoEdit
	EndpointTypeVideoExtension        = kit_types.EndpointTypeVideoExtension
	EndpointTypeAudioSpeech           = kit_types.EndpointTypeAudioSpeech
	EndpointTypeAudioTranscription    = kit_types.EndpointTypeAudioTranscription
	EndpointTypeAudioTranslation      = kit_types.EndpointTypeAudioTranslation
	EndpointTypeModerations           = kit_types.EndpointTypeModerations
	EndpointTypeCohereChat            = kit_types.EndpointTypeCohereChat
	EndpointTypeCohereRerank          = kit_types.EndpointTypeCohereRerank
	EndpointTypeCohereEmbeddings      = kit_types.EndpointTypeCohereEmbeddings
	EndpointTypeTypeSafe              = kit_types.EndpointTypeTypeSafe
)
