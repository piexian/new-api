package types

// relaykit 迁移：RelayFormat 统一到 relaykit/types，此处保留别名与 fork 专属格式常量。
import (
	kit_types "github.com/QuantumNous/new-api/relaykit/types"
)

type RelayFormat = kit_types.RelayFormat

const (
	RelayFormatOpenAI                       = kit_types.RelayFormatOpenAI
	RelayFormatClaude                       = kit_types.RelayFormatClaude
	RelayFormatGemini                       = kit_types.RelayFormatGemini
	RelayFormatOpenAIResponses              = kit_types.RelayFormatOpenAIResponses
	RelayFormatOpenAIResponsesCompaction    = kit_types.RelayFormatOpenAIResponsesCompaction
	RelayFormatOpenAIAudio                  = kit_types.RelayFormatOpenAIAudio
	RelayFormatOpenAIImage                  = kit_types.RelayFormatOpenAIImage
	RelayFormatOpenAIRealtime               = kit_types.RelayFormatOpenAIRealtime
	RelayFormatRerank                       = kit_types.RelayFormatRerank
	RelayFormatEmbedding                    = kit_types.RelayFormatEmbedding
	RelayFormatTask                         = kit_types.RelayFormatTask
	RelayFormatMjProxy                      = kit_types.RelayFormatMjProxy
	RelayFormatOpenAIAlphaSearch            = kit_types.RelayFormatOpenAIAlphaSearch

	// 以下为 fork 专属协议格式（kit 内无对应转换路由）。
	RelayFormatGeminiInteractions RelayFormat = "gemini_interactions"
	RelayFormatMiniMax            RelayFormat = "minimax"
	RelayFormatXAI                RelayFormat = "xai"
	RelayFormatXAIRealtime        RelayFormat = "xai_realtime"
	RelayFormatMoarkNative        RelayFormat = "moark_native"
	RelayFormatStepFunNative      RelayFormat = "stepfun_native"
	RelayFormatCohereNative       RelayFormat = "cohere_native"
	RelayFormatStepFunWss         RelayFormat = "stepfun_wss"
	RelayFormatOCR                RelayFormat = "ocr"
	RelayFormatTypeSafe           RelayFormat = "typesafe"
)
