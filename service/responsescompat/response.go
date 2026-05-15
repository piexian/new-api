package responsescompat

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ChatCompletionToResponse(c *gin.Context, info *relaycommon.RelayInfo, chatResp *dto.OpenAITextResponse) (*dto.OpenAIResponsesResponse, *dto.Usage) {
	if chatResp == nil {
		return nil, nil
	}
	usage := chatResp.Usage
	if usage.InputTokens == 0 {
		usage.InputTokens = usage.PromptTokens
	}
	if usage.OutputTokens == 0 {
		usage.OutputTokens = usage.CompletionTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	responseID := "resp_chatcmpl"
	if c != nil {
		responseID = fmt.Sprintf("resp_%s", helper.GetResponseID(c))
	}
	createdAt := common.GetTimestamp()
	if created, ok := chatResp.Created.(float64); ok && created > 0 {
		createdAt = int64(created)
	} else if created, ok := chatResp.Created.(int64); ok && created > 0 {
		createdAt = created
	} else if created, ok := chatResp.Created.(int); ok && created > 0 {
		createdAt = int64(created)
	}
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}

	model := chatResp.Model
	if model == "" && info != nil {
		model = info.UpstreamModelName
	}

	response := &dto.OpenAIResponsesResponse{
		ID:        responseID,
		Object:    "response",
		CreatedAt: int(createdAt),
		Status:    []byte(`"completed"`),
		Model:     model,
		Output:    chatChoicesToResponsesOutput(chatResp.Choices),
		Usage:     &usage,
	}
	return response, &usage
}

func chatChoicesToResponsesOutput(choices []dto.OpenAITextResponseChoice) []dto.ResponsesOutput {
	outputs := make([]dto.ResponsesOutput, 0, len(choices))
	for idx, choice := range choices {
		messageID := fmt.Sprintf("msg_%d", idx)
		if len(choice.Message.ParseToolCalls()) > 0 {
			for _, toolCall := range choice.Message.ParseToolCalls() {
				arguments := toolCall.Function.Arguments
				if arguments == "" {
					arguments = "{}"
				}
				outputs = append(outputs, dto.ResponsesOutput{
					Type:      "function_call",
					ID:        toolCall.ID,
					Status:    "completed",
					CallId:    toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: []byte(arguments),
				})
			}
			continue
		}
		text := choice.Message.StringContent()
		if text == "" && choice.Message.Content != nil {
			if data, err := common.Marshal(choice.Message.Content); err == nil {
				text = string(data)
			}
		}
		outputs = append(outputs, dto.ResponsesOutput{
			Type:   "message",
			ID:     messageID,
			Status: "completed",
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{
				{
					Type: "output_text",
					Text: text,
				},
			},
		})
	}
	return outputs
}

type StreamEmitter struct {
	c                 *gin.Context
	info              *relaycommon.RelayInfo
	responseID        string
	messageID         string
	createdAt         int
	model             string
	usage             *dto.Usage
	outputText        strings.Builder
	usageText         strings.Builder
	sentCreated       bool
	sentOutputItem    bool
	sentContentPart   bool
	sentCompleted     bool
	err               *types.NewAPIError
	estimateUsageFunc func(*gin.Context, string, string, int) *dto.Usage
}

func NewStreamEmitter(c *gin.Context, info *relaycommon.RelayInfo) *StreamEmitter {
	responseID := "resp_chatcmpl"
	if c != nil {
		responseID = fmt.Sprintf("resp_%s", helper.GetResponseID(c))
	}
	model := ""
	if info != nil {
		model = info.UpstreamModelName
	}
	return &StreamEmitter{
		c:                 c,
		info:              info,
		responseID:        responseID,
		messageID:         "msg_0",
		createdAt:         int(time.Now().Unix()),
		model:             model,
		usage:             &dto.Usage{},
		estimateUsageFunc: service.ResponseText2Usage,
	}
}

func (e *StreamEmitter) SetResponseID(responseID string) {
	if responseID != "" {
		e.responseID = responseID
	}
}

func (e *StreamEmitter) SetModel(model string) {
	if model != "" {
		e.model = model
	}
}

func (e *StreamEmitter) SetCreatedAt(createdAt int64) {
	if createdAt > 0 {
		e.createdAt = int(createdAt)
	}
}

func (e *StreamEmitter) SetUsage(usage *dto.Usage) {
	if usage != nil {
		normalizedUsage := NormalizeUsage(usage)
		if normalizedUsage.InputTokens != 0 || normalizedUsage.OutputTokens != 0 || normalizedUsage.TotalTokens != 0 {
			e.usage = normalizedUsage
		}
	}
}

func (e *StreamEmitter) Usage() *dto.Usage {
	if e.usage == nil || e.usage.TotalTokens == 0 {
		e.usage = e.estimateUsage()
	}
	return e.usage
}

func (e *StreamEmitter) Err() *types.NewAPIError {
	return e.err
}

func (e *StreamEmitter) SendTextDelta(delta string) bool {
	if delta == "" {
		return true
	}
	if !e.sendContentPartIfNeeded() {
		return false
	}
	e.outputText.WriteString(delta)
	e.usageText.WriteString(delta)
	return e.sendEvent(dto.ResponsesStreamResponse{
		Type:         "response.output_text.delta",
		ItemID:       e.messageID,
		OutputIndex:  common.GetPointer(0),
		ContentIndex: common.GetPointer(0),
		Delta:        delta,
	})
}

func (e *StreamEmitter) Complete() bool {
	if e.sentCompleted {
		return true
	}
	if !e.sendCreatedIfNeeded() {
		return false
	}
	if e.usage == nil || e.usage.TotalTokens == 0 {
		e.usage = e.estimateUsage()
	}
	text := e.outputText.String()
	if e.sentContentPart {
		if !e.sendEvent(dto.ResponsesStreamResponse{
			Type:         "response.output_text.done",
			ItemID:       e.messageID,
			OutputIndex:  common.GetPointer(0),
			ContentIndex: common.GetPointer(0),
		}) {
			return false
		}
		if !e.sendEvent(dto.ResponsesStreamResponse{
			Type:         "response.content_part.done",
			ItemID:       e.messageID,
			OutputIndex:  common.GetPointer(0),
			ContentIndex: common.GetPointer(0),
			Part: &dto.ResponsesReasoningSummaryPart{
				Type: "output_text",
				Text: text,
			},
		}) {
			return false
		}
	}
	if e.sentOutputItem {
		if !e.sendEvent(dto.ResponsesStreamResponse{
			Type:        "response.output_item.done",
			OutputIndex: common.GetPointer(0),
			Item: &dto.ResponsesOutput{
				Type:   "message",
				ID:     e.messageID,
				Status: "completed",
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{
					{
						Type: "output_text",
						Text: text,
					},
				},
			},
		}) {
			return false
		}
	}
	completedResponse := e.responseSnapshot("completed", text)
	completedResponse.Usage = e.usage
	if !e.sendEvent(dto.ResponsesStreamResponse{
		Type:     "response.completed",
		Response: completedResponse,
	}) {
		return false
	}
	helper.Done(e.c)
	e.sentCompleted = true
	return true
}

func (e *StreamEmitter) sendEvent(event dto.ResponsesStreamResponse) bool {
	data, err := common.Marshal(event)
	if err != nil {
		e.err = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
		return false
	}
	helper.ResponseChunkData(e.c, event, string(data))
	return true
}

func (e *StreamEmitter) responseSnapshot(status string, contentText string) *dto.OpenAIResponsesResponse {
	if status == "" {
		status = "in_progress"
	}
	output := []dto.ResponsesOutput{}
	if e.sentOutputItem || contentText != "" {
		output = []dto.ResponsesOutput{
			{
				Type:   "message",
				ID:     e.messageID,
				Status: status,
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{
					{
						Type: "output_text",
						Text: contentText,
					},
				},
			},
		}
	}
	return &dto.OpenAIResponsesResponse{
		ID:        e.responseID,
		Object:    "response",
		CreatedAt: e.createdAt,
		Status:    []byte(fmt.Sprintf("%q", status)),
		Model:     e.model,
		Output:    output,
	}
}

func (e *StreamEmitter) sendCreatedIfNeeded() bool {
	if e.sentCreated {
		return true
	}
	if !e.sendEvent(dto.ResponsesStreamResponse{
		Type:     "response.created",
		Response: e.responseSnapshot("in_progress", ""),
	}) {
		return false
	}
	e.sentCreated = true
	return true
}

func (e *StreamEmitter) sendOutputItemIfNeeded() bool {
	if e.sentOutputItem {
		return true
	}
	if !e.sendCreatedIfNeeded() {
		return false
	}
	if !e.sendEvent(dto.ResponsesStreamResponse{
		Type:        "response.output_item.added",
		OutputIndex: common.GetPointer(0),
		Item: &dto.ResponsesOutput{
			Type:    "message",
			ID:      e.messageID,
			Status:  "in_progress",
			Role:    "assistant",
			Content: []dto.ResponsesOutputContent{},
		},
	}) {
		return false
	}
	e.sentOutputItem = true
	return true
}

func (e *StreamEmitter) sendContentPartIfNeeded() bool {
	if e.sentContentPart {
		return true
	}
	if !e.sendOutputItemIfNeeded() {
		return false
	}
	if !e.sendEvent(dto.ResponsesStreamResponse{
		Type:         "response.content_part.added",
		ItemID:       e.messageID,
		OutputIndex:  common.GetPointer(0),
		ContentIndex: common.GetPointer(0),
		Part: &dto.ResponsesReasoningSummaryPart{
			Type: "output_text",
			Text: "",
		},
	}) {
		return false
	}
	e.sentContentPart = true
	return true
}

func (e *StreamEmitter) estimateUsage() *dto.Usage {
	modelName := e.model
	estimatePromptTokens := 0
	if e.info != nil {
		modelName = e.info.UpstreamModelName
		estimatePromptTokens = e.info.GetEstimatePromptTokens()
	}
	usage := e.estimateUsageFunc(e.c, e.usageText.String(), modelName, estimatePromptTokens)
	usage.InputTokens = usage.PromptTokens
	usage.OutputTokens = usage.CompletionTokens
	return NormalizeUsage(usage)
}

func NormalizeUsage(usage *dto.Usage) *dto.Usage {
	if usage == nil {
		return &dto.Usage{}
	}
	out := *usage
	if out.InputTokens == 0 {
		out.InputTokens = out.PromptTokens
	}
	if out.OutputTokens == 0 {
		out.OutputTokens = out.CompletionTokens
	}
	if out.TotalTokens == 0 {
		out.TotalTokens = out.InputTokens + out.OutputTokens
	}
	if out.PromptTokens == 0 {
		out.PromptTokens = out.InputTokens
	}
	if out.CompletionTokens == 0 {
		out.CompletionTokens = out.OutputTokens
	}
	return &out
}
