package relayconvert

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestClaudeStreamPreservesFirstChunkParallelToolsAndText(t *testing.T) {
	info := &relaycommon.RelayInfo{SendResponseCount: 1}
	chunk := &dto.ChatCompletionsStreamResponse{Id: "chat_1", Model: "test", Usage: &dto.Usage{PromptTokens: 2, CompletionTokens: 3}, Choices: []dto.ChatCompletionsStreamResponseChoice{{
		Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
			ReasoningContent: common.GetPointer("consider"), Content: common.GetPointer("checking"),
			ToolCalls: []dto.ToolCallResponse{
				{Index: common.GetPointer(0), ID: "call_1", Type: "function", Function: dto.FunctionResponse{Name: "first", Arguments: `{"x":1}`}},
				{Index: common.GetPointer(1), ID: "call_2", Type: "function", Function: dto.FunctionResponse{Name: "second", Arguments: `{"x":2}`}},
			},
		}, FinishReason: common.GetPointer("tool_calls"),
	}}}
	events := StreamResponseOpenAI2Claude(chunk, info)
	starts, stops := map[int]string{}, map[int]bool{}
	args := map[int]string{}
	var text, thinking string
	for _, event := range events {
		switch event.Type {
		case "content_block_start":
			starts[event.GetIndex()] = event.ContentBlock.Type
		case "content_block_stop":
			stops[event.GetIndex()] = true
		case "content_block_delta":
			if event.Delta.PartialJson != nil {
				args[event.GetIndex()] += *event.Delta.PartialJson
			}
			text += event.Delta.GetText()
			if event.Delta.Thinking != nil {
				thinking += *event.Delta.Thinking
			}
		}
	}
	require.Equal(t, "checking", text)
	require.Equal(t, "consider", thinking)
	require.Equal(t, map[int]string{0: "thinking", 1: "text", 2: "tool_use", 3: "tool_use"}, starts)
	require.Equal(t, map[int]string{2: `{"x":1}`, 3: `{"x":2}`}, args)
	require.Len(t, stops, len(starts))
	require.Equal(t, "message_stop", events[len(events)-1].Type)
}

func TestClaudeResponsePreservesAllTextAndThinkingBlocks(t *testing.T) {
	var response dto.ClaudeResponse
	require.NoError(t, common.Unmarshal([]byte(`{"id":"msg_1","model":"test","stop_reason":"end_turn","content":[{"type":"thinking","thinking":"reason A"},{"type":"text","text":"text A"},{"type":"thinking","thinking":"reason B"},{"type":"text","text":"text B"}]}`), &response))
	chat := ResponseClaude2OpenAI(&response)
	require.Equal(t, "text Atext B", chat.Choices[0].Message.StringContent())
	require.Equal(t, "reason Areason B", chat.Choices[0].Message.GetReasoningContent())
}

func TestResponsesWirePreservesReasoningRefusalAndCustomInput(t *testing.T) {
	var response dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal([]byte(`{"model":"test","status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"reason A"},{"type":"summary_text","text":"reason B"}]},{"type":"custom_tool_call","id":"item_1","call_id":"call_1","name":"shell","input":"echo hello"},{"type":"message","role":"assistant","content":[{"type":"refusal","refusal":"Cannot comply"}]}]}`), &response))
	chat, _, err := ResponsesResponseToChatCompletionsResponse(&response, "chat_1")
	require.NoError(t, err)
	msg := chat.Choices[0].Message
	require.Equal(t, "reason Areason B", msg.GetReasoningContent())
	require.NotNil(t, msg.Refusal)
	require.Equal(t, "Cannot comply", *msg.Refusal)
	require.Empty(t, msg.StringContent())
	require.Equal(t, "custom", gjson.GetBytes(msg.ToolCalls, "0.type").String())
	require.Equal(t, "echo hello", gjson.GetBytes(msg.ToolCalls, "0.custom.input").String())
	require.False(t, gjson.GetBytes(msg.ToolCalls, "0.function").Exists())
}

func TestGeminiStreamReassemblesToolArgumentsAndPreservesUsage(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	wire := []string{
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":""}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"reasoning_content":"consider","content":"checking","tool_calls":[{"index":0,"function":{"arguments":"{\"q\":"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"x\"}"}}]},"finish_reason":"tool_calls"}]}`,
		`{"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`,
	}
	var calls []*dto.FunctionCall
	var text, thinking string
	for i, raw := range wire {
		var chunk dto.ChatCompletionsStreamResponse
		require.NoError(t, common.Unmarshal([]byte(raw), &chunk))
		response, err := StreamResponseOpenAI2Gemini(&chunk, info)
		require.NoError(t, err)
		if response == nil {
			require.Equal(t, 0, i)
			continue
		}
		for _, candidate := range response.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					calls = append(calls, part.FunctionCall)
				}
				if part.Thought {
					thinking += part.Text
				} else {
					text += part.Text
				}
			}
		}
		if i == 3 {
			require.True(t, response.HasUsageMetadata)
			require.Equal(t, 12, response.UsageMetadata.TotalTokenCount)
		}
	}
	require.Equal(t, "checking", text)
	require.Equal(t, "consider", thinking)
	require.Len(t, calls, 1)
	require.Equal(t, "call_1", calls[0].ID)
	require.Equal(t, "lookup", calls[0].FunctionName)
	require.Equal(t, map[string]any{"q": "x"}, calls[0].Arguments)
}

func TestCompletedInteractionDoesNotReplayExecutedTools(t *testing.T) {
	var interaction dto.GeminiInteraction
	require.NoError(t, common.Unmarshal([]byte(`{"status":"completed","steps":[{"type":"function_call","id":"call_done","name":"lookup","arguments":{}},{"type":"function_result","call_id":"call_done","result":[{"type":"text","text":"done"}]},{"type":"model_output","content":[{"type":"text","text":"Final answer"}]}]}`), &interaction))
	response := InteractionToGeminiChatResponse(&interaction, 0)
	require.Len(t, response.Candidates, 1)
	require.Len(t, response.Candidates[0].Content.Parts, 1)
	require.Equal(t, "Final answer", response.Candidates[0].Content.Parts[0].Text)
	require.Equal(t, "STOP", *response.Candidates[0].FinishReason)
}

func TestClaudeStreamKeepsToolBlockOpenAcrossInterleavedText(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	frames := []string{
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"content":"body","reasoning_content":"reason","tool_calls":[{"index":0,"function":{"arguments":"\"x\""}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
	}
	active := map[int]string{}
	var args, text, thinking string
	toolStarts := 0
	for _, raw := range frames {
		var chunk dto.ChatCompletionsStreamResponse
		require.NoError(t, common.Unmarshal([]byte(raw), &chunk))
		info.SendResponseCount++
		for _, event := range StreamResponseOpenAI2Claude(&chunk, info) {
			index := event.GetIndex()
			switch event.Type {
			case "content_block_start":
				require.Empty(t, active[index])
				active[index] = event.ContentBlock.Type
				if event.ContentBlock.Type == "tool_use" {
					toolStarts++
				}
			case "content_block_delta":
				require.NotEmpty(t, active[index], "delta for unopened block %d", index)
				if event.Delta.PartialJson != nil {
					require.Equal(t, "tool_use", active[index])
					args += *event.Delta.PartialJson
				}
				text += event.Delta.GetText()
				if event.Delta.Thinking != nil {
					thinking += *event.Delta.Thinking
				}
			case "content_block_stop":
				require.NotEmpty(t, active[index])
				delete(active, index)
			}
		}
	}
	require.Empty(t, active)
	require.Equal(t, 1, toolStarts)
	require.JSONEq(t, `{"q":"x"}`, args)
	require.Equal(t, "body", text)
	require.Equal(t, "reason", thinking)
}

func TestChatToResponsesPreservesRefusalAndCustomCalls(t *testing.T) {
	var chat dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"refusal":"Cannot comply","tool_calls":[{"type":"custom","id":"call_1","custom":{"name":"shell","input":"echo hello"}}]},"finish_reason":"tool_calls"}]}`), &chat))
	response, _, err := ChatCompletionsResponseToResponsesResponse(&chat, "resp_1")
	require.NoError(t, err)
	require.Len(t, response.Output, 2)
	require.Equal(t, "Cannot comply", response.Output[0].Content[0].Refusal)
	require.Equal(t, "custom_tool_call", response.Output[1].Type)
	require.Equal(t, "shell", response.Output[1].Name)
	require.Equal(t, "echo hello", *response.Output[1].Input)
	state := NewChatToResponsesStreamState("resp_2", "test")
	var events []ChatToResponsesStreamEvent
	for _, raw := range []string{
		`{"choices":[{"delta":{"refusal":"Cannot ","tool_calls":[{"index":0,"type":"custom","id":"call_1","custom":{"name":"shell","input":"echo "}}]}}]}`,
		`{"choices":[{"delta":{"refusal":"comply","tool_calls":[{"index":0,"custom":{"input":"hello"}}]},"finish_reason":"tool_calls"}]}`,
	} {
		var chunk dto.ChatCompletionsStreamResponse
		require.NoError(t, common.Unmarshal([]byte(raw), &chunk))
		next, err := ChatCompletionsStreamChunkToResponsesEvents(&chunk, state)
		require.NoError(t, err)
		events = append(events, next...)
	}
	events = append(events, FinalizeChatCompletionsStreamToResponses(state)...)
	response = events[len(events)-1].Payload.Response
	require.Equal(t, "Cannot comply", response.Output[0].Content[0].Refusal)
	require.Equal(t, "custom_tool_call", response.Output[1].Type)
	require.Equal(t, "echo hello", *response.Output[1].Input)
}

func TestResponsesGeneratedImagesAndCitationsReachChat(t *testing.T) {
	var source dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal([]byte(`{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Source","annotations":[{"type":"url_citation","start_index":0,"end_index":6,"title":"Reference","url":"https://example.test/source"}]}]},{"type":"image_generation_call","id":"image_1","result":"aGVsbG8=","output_format":"webp"}]}`), &source))
	chat, _, err := ResponsesResponseToChatCompletionsResponse(&source, "chat_1")
	require.NoError(t, err)
	require.Contains(t, chat.Choices[0].Message.StringContent(), "data:image/webp;base64,aGVsbG8=")
	wire, err := common.Marshal(chat)
	require.NoError(t, err)
	require.Equal(t, "https://example.test/source", gjson.GetBytes(wire, "choices.0.message.annotations.0.url_citation.url").String())
	state := NewResponsesToChatStreamState("test", false)
	_, err = ResponsesStreamEventToChatChunks(&dto.ResponsesStreamResponse{Type: "response.output_text.delta", Delta: "Source"}, state)
	require.NoError(t, err)
	chunks, err := ResponsesStreamEventToChatChunks(&dto.ResponsesStreamResponse{Type: "response.completed", Response: &source}, state)
	require.NoError(t, err)
	var text string
	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			text += choice.Delta.GetContentString()
		}
	}
	require.Equal(t, "![image](data:image/webp;base64,aGVsbG8=)", text)
}

func TestGeminiStreamPreservesAudioFilesAndMixedThinking(t *testing.T) {
	var source dto.GeminiChatResponse
	require.NoError(t, common.Unmarshal([]byte(`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"reason A","thought":true},{"text":"body"},{"text":"reason B","thought":true},{"inlineData":{"mimeType":"audio/pcm","data":"AQID"}},{"fileData":{"mimeType":"application/pdf","fileUri":"https://example.test/report.pdf"}}]}}]}`), &source))
	chat, _ := StreamResponseGeminiChat2OpenAI(&source)
	require.Equal(t, "reason Areason B", chat.Choices[0].Delta.GetReasoningContent())
	text := chat.Choices[0].Delta.GetContentString()
	require.Contains(t, text, "body")
	require.Contains(t, text, "data:audio/pcm;base64,AQID")
	require.Contains(t, text, "https://example.test/report.pdf")
}

func TestResponsesStreamTerminalOutputDoesNotDuplicateTools(t *testing.T) {
	state := NewResponsesToChatStreamState("test", false)
	item := dto.ResponsesOutput{Type: "function_call", ID: "fc_1", CallId: "call_1", Name: "lookup"}
	completed := item
	completed.Arguments = []byte(`{"q":"x"}`)
	events := []*dto.ResponsesStreamResponse{
		{Type: "response.output_item.added", OutputIndex: common.GetPointer(0), Item: &item},
		{Type: "response.function_call_arguments.delta", OutputIndex: common.GetPointer(0), ItemID: "fc_1", Delta: `{"q":"x"}`},
		{Type: "response.output_item.done", OutputIndex: common.GetPointer(0), Item: &completed},
		{Type: "response.completed", Response: &dto.OpenAIResponsesResponse{Status: []byte(`"completed"`), Output: []dto.ResponsesOutput{completed}}},
	}
	var ids []string
	var arguments string
	for _, event := range events {
		chunks, err := ResponsesStreamEventToChatChunks(event, state)
		require.NoError(t, err)
		for _, chunk := range chunks {
			for _, choice := range chunk.Choices {
				for _, tool := range choice.Delta.ToolCalls {
					require.Equal(t, 0, *tool.Index)
					if tool.Function.Name != "" {
						ids = append(ids, tool.ID)
					}
					arguments += tool.Function.Arguments
				}
			}
		}
	}
	require.Equal(t, []string{"call_1"}, ids)
	require.JSONEq(t, `{"q":"x"}`, arguments)
}
