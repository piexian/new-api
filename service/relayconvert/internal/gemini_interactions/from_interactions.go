package gemini_interactions

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// InteractionToGeminiChatResponse 非流式响应转换:steps 时间线 -> 单 candidate parts。
// model_output 文本/媒体 -> parts;thought -> thought parts;function_call -> functionCall parts;
// 其余步骤(user_input/function_result/grounding)跳过。
func InteractionToGeminiChatResponse(interaction *dto.GeminiInteraction, fallbackPromptTokens int) *dto.GeminiChatResponse {
	resp := &dto.GeminiChatResponse{}
	if interaction == nil {
		return resp
	}
	candidate := dto.GeminiChatCandidate{
		Content: dto.GeminiChatContent{Role: "model"},
		Index:   0,
	}
	for i := range interaction.Steps {
		step := &interaction.Steps[i]
		switch step.Type {
		case dto.GeminiInteractionStepModelOutput:
			for _, block := range step.Content {
				if part := contentBlockToGeminiPart(&block); part != nil {
					candidate.Content.Parts = append(candidate.Content.Parts, *part)
				}
			}
		case dto.GeminiInteractionStepThought:
			for _, block := range step.Content {
				if block.Text != "" {
					candidate.Content.Parts = append(candidate.Content.Parts, dto.GeminiPart{Text: block.Text, Thought: true})
				}
			}
		case dto.GeminiInteractionStepFunctionCall:
			candidate.Content.Parts = append(candidate.Content.Parts, dto.GeminiPart{
				FunctionCall: &dto.FunctionCall{
					FunctionName: step.Name,
					Arguments:    rawJSONAny(step.Arguments),
					ID:           step.ID,
				},
			})
		}
	}
	finishReason := "STOP"
	switch interaction.Status {
	case "incomplete", "budget_exceeded":
		finishReason = "MAX_TOKENS"
	case "cancelled", "failed":
		finishReason = "OTHER"
	}
	candidate.FinishReason = &finishReason
	resp.Candidates = []dto.GeminiChatCandidate{candidate}
	if interaction.Usage != nil {
		resp.UsageMetadata = usageToGeminiMetadata(interaction.Usage)
		resp.HasUsageMetadata = true
	}
	return resp
}

// contentBlockToGeminiPart interactions content 块 -> gemini part
func contentBlockToGeminiPart(block *dto.GeminiInteractionContent) *dto.GeminiPart {
	if block == nil {
		return nil
	}
	switch block.Type {
	case dto.GeminiInteractionContentText:
		if block.Text == "" {
			return nil
		}
		return &dto.GeminiPart{Text: block.Text}
	case dto.GeminiInteractionContentImage, dto.GeminiInteractionContentAudio, dto.GeminiInteractionContentVideo, dto.GeminiInteractionContentDocument:
		if block.Data != "" {
			return &dto.GeminiPart{
				InlineData: &dto.GeminiInlineData{MimeType: block.MimeType, Data: block.Data},
			}
		}
		if block.URI != "" {
			return &dto.GeminiPart{
				FileData: &dto.GeminiFileData{MimeType: block.MimeType, FileUri: block.URI},
			}
		}
	}
	return nil
}

func rawJSONAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var v any
	if err := common.Unmarshal(raw, &v); err != nil {
		return map[string]any{}
	}
	return v
}

// usageToGeminiMetadata usage 反向映射(与 UsageFromGeminiMetadata 的正向口径互逆)
func usageToGeminiMetadata(u *dto.GeminiInteractionUsage) dto.GeminiUsageMetadata {
	meta := dto.GeminiUsageMetadata{
		PromptTokenCount:        u.TotalInputTokens,
		ToolUsePromptTokenCount: u.TotalToolUseTokens,
		CandidatesTokenCount:    u.TotalOutputTokens,
		ThoughtsTokenCount:      u.TotalThoughtTokens,
		CachedContentTokenCount: u.TotalCachedTokens,
		TotalTokenCount:         u.TotalTokens,
	}
	for _, d := range u.InputTokensByModality {
		meta.PromptTokensDetails = append(meta.PromptTokensDetails, dto.GeminiPromptTokensDetails{
			Modality:   strings.ToUpper(d.Modality),
			TokenCount: d.Tokens,
		})
	}
	for _, d := range u.ToolUseTokensByModality {
		meta.ToolUsePromptTokensDetails = append(meta.ToolUsePromptTokensDetails, dto.GeminiPromptTokensDetails{
			Modality:   strings.ToUpper(d.Modality),
			TokenCount: d.Tokens,
		})
	}
	for _, d := range u.OutputTokensByModality {
		meta.CandidatesTokensDetails = append(meta.CandidatesTokensDetails, dto.GeminiPromptTokensDetails{
			Modality:   strings.ToUpper(d.Modality),
			TokenCount: d.Tokens,
		})
	}
	return meta
}

// ---------------------------------------------------------------------------
// SSE 翻译: interactions 事件流 -> generateContent 风格 data 行
// ---------------------------------------------------------------------------

// InteractionsSSEToGeminiSSE 返回一个翻译 reader,把上游 interactions SSE 逐事件翻译为
// gemini 原生 SSE data 行,供既有 GeminiChatStreamHandler/GeminiTextGenerationStreamHandler 消费。
type InteractionsSSEToGeminiSSE struct {
	scanner  *bufio.Scanner
	pending  []byte // 待输出的完整行(含 \n)
	curStep  dto.GeminiInteractionStep
	argsBuf  []byte
	sentRole bool
	err      error
	// onPending: requires_action/completed 时回调(interaction id + 待续链的 function_call id),
	// 供调用方保存有状态桥接;可为 nil
	onPending         func(interactionID string, callIDs []string)
	lastInteractionID string
	pendingCallIDs    []string
}

func NewInteractionsSSEToGeminiSSE(reader io.Reader) *InteractionsSSEToGeminiSSE {
	return NewInteractionsSSEToGeminiSSEWithCallback(reader, nil)
}

func NewInteractionsSSEToGeminiSSEWithCallback(reader io.Reader, onPending func(interactionID string, callIDs []string)) *InteractionsSSEToGeminiSSE {
	s := &InteractionsSSEToGeminiSSE{scanner: bufio.NewScanner(reader), onPending: onPending}
	s.scanner.Buffer(make([]byte, 64<<10), 128<<20)
	return s
}

func (t *InteractionsSSEToGeminiSSE) Read(p []byte) (int, error) {
	for len(t.pending) == 0 {
		if t.err != nil {
			return 0, t.err
		}
		if !t.scanner.Scan() {
			t.err = io.EOF
			return 0, io.EOF
		}
		line := t.scanner.Text()
		t.handleLine(line)
	}
	n := copy(p, t.pending)
	t.pending = t.pending[n:]
	return n, nil
}

// handleLine 处理一行:只关心 "data: {...}";event 行信息已内嵌在 payload 的 event_type/type 字段
func (t *InteractionsSSEToGeminiSSE) handleLine(line string) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "data:") {
		return
	}
	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if payload == "" || payload == "[DONE]" {
		return
	}
	var event dto.GeminiInteractionSseEvent
	if err := common.UnmarshalJsonStr(payload, &event); err != nil {
		return
	}
	if event.Interaction != nil && event.Interaction.ID != "" {
		t.lastInteractionID = event.Interaction.ID
	}
	switch event.EventName() {
	case "step.start":
		t.handleStepStart(payload)
	case "step.delta":
		t.handleStepDelta(payload)
	case "step.stop":
		t.handleStepStop(payload)
	case "interaction.completed", "interaction.requires_action":
		t.handleInteractionEnd(&event)
	}
}

func (t *InteractionsSSEToGeminiSSE) handleStepStart(payload string) {
	t.curStep = dto.GeminiInteractionStep{}
	var envelope struct {
		Step *struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"step"`
	}
	if err := common.UnmarshalJsonStr(payload, &envelope); err != nil || envelope.Step == nil {
		return
	}
	t.curStep.Type = envelope.Step.Type
	t.curStep.ID = envelope.Step.ID
	t.curStep.Name = envelope.Step.Name
	if t.curStep.Type == dto.GeminiInteractionStepModelOutput && !t.sentRole {
		t.sentRole = true
		t.emit(dto.GeminiChatResponse{
			Candidates: []dto.GeminiChatCandidate{{Content: dto.GeminiChatContent{Role: "model"}, Index: 0}},
		})
	}
}

func (t *InteractionsSSEToGeminiSSE) handleStepDelta(payload string) {
	var envelope struct {
		Delta *struct {
			Type             string `json:"type"`
			Text             string `json:"text"`
			PartialArguments string `json:"partial_arguments"`
			Data             string `json:"data"`
			MimeType         string `json:"mime_type"`
		} `json:"delta"`
	}
	if err := common.UnmarshalJsonStr(payload, &envelope); err != nil || envelope.Delta == nil {
		return
	}
	delta := envelope.Delta
	switch delta.Type {
	case "text":
		if delta.Text != "" {
			t.emitPart(dto.GeminiPart{Text: delta.Text})
		}
	case "thought", "thought_summary":
		if delta.Text != "" {
			t.emitPart(dto.GeminiPart{Text: delta.Text, Thought: true})
		}
	case "arguments":
		// function_call 参数分片(JSON 字符串):累积到 step 结束一次性吐出
		t.argsBuf = append(t.argsBuf, delta.PartialArguments...)
	case "image", "audio", "video":
		// 媒体增量按整块到达时透传
		if delta.Data != "" {
			t.emitPart(dto.GeminiPart{
				InlineData: &dto.GeminiInlineData{MimeType: delta.MimeType, Data: delta.Data},
			})
		}
	}
}

func (t *InteractionsSSEToGeminiSSE) handleStepStop(payload string) {
	step := t.curStep
	t.curStep = dto.GeminiInteractionStep{}
	switch step.Type {
	case dto.GeminiInteractionStepFunctionCall:
		args := t.argsBuf
		if len(args) == 0 {
			args = json.RawMessage("{}")
		}
		t.argsBuf = nil
		if step.ID != "" {
			t.pendingCallIDs = append(t.pendingCallIDs, step.ID)
		}
		t.emitPart(dto.GeminiPart{
			FunctionCall: &dto.FunctionCall{
				FunctionName: step.Name,
				Arguments:    rawJSONAny(args),
				ID:           step.ID,
			},
		})
	case dto.GeminiInteractionStepModelOutput:
		// step.stop 可能携带完整 step(含媒体 content),补发未在 delta 中出现的块
		var envelope struct {
			Step *dto.GeminiInteractionStep `json:"step"`
		}
		if err := common.UnmarshalJsonStr(payload, &envelope); err == nil && envelope.Step != nil {
			for i := range envelope.Step.Content {
				if part := contentBlockToGeminiPart(&envelope.Step.Content[i]); part != nil && part.InlineData != nil {
					t.emitPart(*part)
				}
			}
		}
	}
}

func (t *InteractionsSSEToGeminiSSE) handleInteractionEnd(event *dto.GeminiInteractionSseEvent) {
	if t.onPending != nil && t.lastInteractionID != "" && len(t.pendingCallIDs) > 0 {
		t.onPending(t.lastInteractionID, t.pendingCallIDs)
		t.pendingCallIDs = nil
	}
	finishReason := "STOP"
	resp := dto.GeminiChatResponse{}
	candidate := dto.GeminiChatCandidate{Index: 0}
	if event.Interaction != nil {
		switch event.Interaction.Status {
		case "incomplete", "budget_exceeded":
			finishReason = "MAX_TOKENS"
		case "cancelled", "failed":
			finishReason = "OTHER"
		}
		if event.Interaction.Usage != nil {
			resp.UsageMetadata = usageToGeminiMetadata(event.Interaction.Usage)
			resp.HasUsageMetadata = true
		}
	}
	candidate.FinishReason = &finishReason
	resp.Candidates = []dto.GeminiChatCandidate{candidate}
	t.emit(resp)
}

func (t *InteractionsSSEToGeminiSSE) emitPart(part dto.GeminiPart) {
	t.emit(dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Content: dto.GeminiChatContent{Role: "model", Parts: []dto.GeminiPart{part}},
			Index:   0,
		}},
	})
}

func (t *InteractionsSSEToGeminiSSE) emit(resp dto.GeminiChatResponse) {
	data, err := common.Marshal(resp)
	if err != nil {
		return
	}
	t.pending = append(t.pending, append(append([]byte("data: "), data...), '\n', '\n')...)
}

// Ensure interface compliance
var _ io.Reader = (*InteractionsSSEToGeminiSSE)(nil)
