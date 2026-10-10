package dto

// relaykit 迁移：Gemini Interactions 协议类型统一到 relaykit/dto,此处保留别名以维持宿主引用。

import (
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
)

type GeminiInteractionsRequest = kitdto.GeminiInteractionsRequest

type GeminiInteractionUsage = kitdto.GeminiInteractionUsage

type GeminiInteractionModalityTokens = kitdto.GeminiInteractionModalityTokens

type GeminiInteraction = kitdto.GeminiInteraction

type GeminiInteractionSseEvent = kitdto.GeminiInteractionSseEvent

type GeminiInteractionStep = kitdto.GeminiInteractionStep

type GeminiInteractionContent = kitdto.GeminiInteractionContent

// IsGeminiInteractionTerminal 判断 interaction 状态是否终态
func IsGeminiInteractionTerminal(status string) bool {
	return kitdto.IsGeminiInteractionTerminal(status)
}

// 常量
const (
	GeminiInteractionStepUserInput      = kitdto.GeminiInteractionStepUserInput
	GeminiInteractionStepModelOutput    = kitdto.GeminiInteractionStepModelOutput
	GeminiInteractionStepThought        = kitdto.GeminiInteractionStepThought
	GeminiInteractionStepFunctionCall   = kitdto.GeminiInteractionStepFunctionCall
	GeminiInteractionStepFunctionResult = kitdto.GeminiInteractionStepFunctionResult

	GeminiInteractionContentText     = kitdto.GeminiInteractionContentText
	GeminiInteractionContentImage    = kitdto.GeminiInteractionContentImage
	GeminiInteractionContentAudio    = kitdto.GeminiInteractionContentAudio
	GeminiInteractionContentVideo    = kitdto.GeminiInteractionContentVideo
	GeminiInteractionContentDocument = kitdto.GeminiInteractionContentDocument
)
