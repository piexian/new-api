package responsescompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestConvertToOpenAIChatRequestStringInput(t *testing.T) {
	instructions, _ := common.Marshal("be concise")
	input, _ := common.Marshal("hello")
	stream := true
	maxOutputTokens := uint(128)
	temperature := 0.2

	req, err := ConvertToOpenAIChatRequest(dto.OpenAIResponsesRequest{
		Model:           "deepseek-chat",
		Instructions:    instructions,
		Input:           input,
		Stream:          &stream,
		MaxOutputTokens: &maxOutputTokens,
		Temperature:     &temperature,
		Reasoning:       &dto.Reasoning{Effort: "high"},
	})

	require.NoError(t, err)
	require.Equal(t, "deepseek-chat", req.Model)
	require.Len(t, req.Messages, 2)
	require.Equal(t, "system", req.Messages[0].Role)
	require.Equal(t, "be concise", req.Messages[0].Content)
	require.Equal(t, "user", req.Messages[1].Role)
	require.Equal(t, "hello", req.Messages[1].Content)
	require.Equal(t, &stream, req.Stream)
	require.Equal(t, &maxOutputTokens, req.MaxCompletionTokens)
	require.Equal(t, &temperature, req.Temperature)
	require.Equal(t, "high", req.ReasoningEffort)
}

func TestConvertToOpenAIChatRequestFunctionItemsAndTools(t *testing.T) {
	input, _ := common.Marshal([]map[string]any{
		{
			"type": "message",
			"role": "user",
			"content": []map[string]any{
				{"type": "input_text", "text": "look"},
				{"type": "input_image", "image_url": "https://example.com/a.png", "detail": "low"},
			},
		},
		{
			"type":      "function_call",
			"call_id":   "call_1",
			"name":      "search",
			"arguments": map[string]any{"q": "new-api"},
		},
		{
			"type":    "function_call_output",
			"call_id": "call_1",
			"output":  map[string]any{"ok": true},
		},
	})
	tools, _ := common.Marshal([]map[string]any{
		{
			"type":        "function",
			"name":        "search",
			"description": "search docs",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"q": map[string]any{"type": "string"},
				},
			},
		},
	})
	toolChoice, _ := common.Marshal(map[string]any{"type": "function", "name": "search"})
	text, _ := common.Marshal(map[string]any{"format": map[string]any{"type": "json_object"}})

	req, err := ConvertToOpenAIChatRequest(dto.OpenAIResponsesRequest{
		Model:      "moonshot-v1-8k",
		Input:      input,
		Tools:      tools,
		ToolChoice: toolChoice,
		Text:       text,
	})

	require.NoError(t, err)
	require.Len(t, req.Messages, 3)
	require.Equal(t, "user", req.Messages[0].Role)
	require.IsType(t, []any{}, req.Messages[0].Content)
	content := req.Messages[0].ParseContent()
	require.Len(t, content, 2)
	require.Equal(t, dto.ContentTypeText, content[0].Type)
	require.Equal(t, "look", content[0].Text)
	require.Equal(t, dto.ContentTypeImageURL, content[1].Type)
	require.Equal(t, "https://example.com/a.png", content[1].GetImageMedia().Url)
	require.Equal(t, "low", content[1].GetImageMedia().Detail)
	require.Equal(t, "assistant", req.Messages[1].Role)
	toolCalls := req.Messages[1].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	require.Equal(t, "call_1", toolCalls[0].ID)
	require.Equal(t, "search", toolCalls[0].Function.Name)
	require.JSONEq(t, `{"q":"new-api"}`, toolCalls[0].Function.Arguments)
	require.Equal(t, "tool", req.Messages[2].Role)
	require.Equal(t, "call_1", req.Messages[2].ToolCallId)
	require.JSONEq(t, `{"ok":true}`, req.Messages[2].Content.(string))
	require.Len(t, req.Tools, 1)
	require.Equal(t, "search", req.Tools[0].Function.Name)
	require.Equal(t, map[string]any{"type": "function", "function": map[string]any{"name": "search"}}, req.ToolChoice)
	require.NotNil(t, req.ResponseFormat)
	require.Equal(t, "json_object", req.ResponseFormat.Type)
}

func TestConvertToNonStreamOpenAIChatRequestRejectsStream(t *testing.T) {
	stream := true

	_, err := ConvertToNonStreamOpenAIChatRequest(dto.OpenAIResponsesRequest{
		Model:  "deepseek-chat",
		Stream: &stream,
	})

	require.ErrorContains(t, err, "does not support stream yet")
}

func TestConvertToOpenAIChatRequestPreservesExtraBody(t *testing.T) {
	extraBody := []byte(`{"google":{"cached_content":"cachedContents/cache-123"}}`)

	req, err := ConvertToOpenAIChatRequest(dto.OpenAIResponsesRequest{
		Model:     "gemini-2.5-flash",
		Input:     []byte(`"hello"`),
		ExtraBody: extraBody,
	})

	require.NoError(t, err)
	require.JSONEq(t, string(extraBody), string(req.ExtraBody))
}
