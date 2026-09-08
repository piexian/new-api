package geminichat

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/relayconvert/internal/jsonutil"
	relaymeta "github.com/QuantumNous/new-api/service/relayconvert/internal/meta"
	sharedopenai "github.com/QuantumNous/new-api/service/relayconvert/internal/shared/openai"
)

func GeminiGenerateContentRequestToOpenAIChat(geminiRequest *dto.GeminiChatRequest, info *relaycommon.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
	modelName := ""
	isStream := false
	if info != nil {
		isStream = info.IsStream
	}
	modelName = relaymeta.RelayInfoUpstreamModelName(info)
	openaiRequest := &dto.GeneralOpenAIRequest{
		Model:  modelName,
		Stream: common.GetPointer(isStream),
	}

	var messages []dto.Message
	callIDs := make(map[string][]string)
	nextCallID := 0
	var pendingToolMedia []dto.MediaContent
	flushToolMedia := func() {
		if len(pendingToolMedia) > 0 {
			messages = append(messages, dto.Message{Role: "user", Content: pendingToolMedia})
			pendingToolMedia = nil
		}
	}
	for _, content := range geminiRequest.Contents {
		if content.Role != "user" && content.Role != "" {
			flushToolMedia()
		}
		hasToolResult := false
		message := dto.Message{
			Role: convertGeminiRoleToOpenAI(content.Role),
		}

		var mediaContents []dto.MediaContent
		var toolCalls []dto.ToolCallRequest
		for _, part := range content.Parts {
			if part.Text != "" {
				mediaContent := dto.MediaContent{
					Type: "text",
					Text: part.Text,
				}
				mediaContents = append(mediaContents, mediaContent)
			} else if part.InlineData != nil || part.FileData != nil {
				media, err := geminiMediaToOpenAI(part)
				if err != nil {
					return nil, err
				}
				mediaContents = append(mediaContents, media)
			} else if part.FunctionCall != nil {
				callID := part.FunctionCall.ID
				if callID == "" {
					nextCallID++
					callID = fmt.Sprintf("call_%d", nextCallID)
				}
				callIDs[part.FunctionCall.FunctionName] = append(callIDs[part.FunctionCall.FunctionName], callID)
				toolCall := dto.ToolCallRequest{
					ID:   callID,
					Type: "function",
					Function: dto.FunctionRequest{
						Name:      part.FunctionCall.FunctionName,
						Arguments: jsonutil.ToJSONString(part.FunctionCall.Arguments),
					},
				}
				toolCalls = append(toolCalls, toolCall)
			} else if part.FunctionResponse != nil {
				hasToolResult = true
				response := part.FunctionResponse
				var callID string
				if len(response.ID) > 0 {
					if err := common.Unmarshal(response.ID, &callID); err != nil {
						return nil, err
					}
				}
				if pending := callIDs[response.Name]; len(pending) > 0 {
					if callID == "" {
						callID = pending[0]
					}
					for i, id := range pending {
						if id == callID {
							callIDs[response.Name] = append(pending[:i], pending[i+1:]...)
							break
						}
					}
				}
				if callID == "" {
					return nil, fmt.Errorf("Gemini function response %q has no matching function call", response.Name)
				}
				toolMessage := dto.Message{Role: "tool", ToolCallId: callID, Content: jsonutil.ToJSONString(response.Response)}
				if len(response.Parts) > 0 {
					var parts []dto.GeminiPart
					if err := common.Unmarshal(response.Parts, &parts); err != nil {
						return nil, err
					}
					var converted []dto.MediaContent
					for _, part := range parts {
						media, err := geminiMediaToOpenAI(part)
						if err != nil {
							return nil, err
						}
						converted = append(converted, media)
					}
					_, attachments := sharedopenai.SplitToolContent(callID, converted)
					mediaContents = append(mediaContents, attachments...)
				}
				messages = append(messages, toolMessage)
			}
		}

		if len(toolCalls) > 0 {
			message.SetToolCalls(toolCalls)
		}
		if hasToolResult || (message.Role == "user" && len(pendingToolMedia) > 0) {
			pendingToolMedia = append(pendingToolMedia, mediaContents...)
		} else if len(mediaContents) == 1 && mediaContents[0].Type == "text" {
			message.Content = mediaContents[0].Text
		} else if len(mediaContents) > 0 {
			message.SetMediaContent(mediaContents)
		}

		if len(message.ParseContent()) > 0 || len(message.ToolCalls) > 0 {
			messages = append(messages, message)
		}
	}

	flushToolMedia()
	openaiRequest.Messages = messages

	if geminiRequest.GenerationConfig.Temperature != nil {
		openaiRequest.Temperature = geminiRequest.GenerationConfig.Temperature
	}
	if geminiRequest.GenerationConfig.TopP != nil {
		openaiRequest.TopP = common.GetPointer(*geminiRequest.GenerationConfig.TopP)
	}
	if geminiRequest.GenerationConfig.TopK != nil {
		openaiRequest.TopK = common.GetPointer(int(*geminiRequest.GenerationConfig.TopK))
	}
	if geminiRequest.GenerationConfig.MaxOutputTokens != nil && *geminiRequest.GenerationConfig.MaxOutputTokens > 0 {
		openaiRequest.MaxTokens = common.GetPointer(*geminiRequest.GenerationConfig.MaxOutputTokens)
	}
	if len(geminiRequest.GenerationConfig.StopSequences) > 0 {
		if len(geminiRequest.GenerationConfig.StopSequences) > 4 {
			return nil, fmt.Errorf("OpenAI Chat conversion supports at most 4 stop sequences")
		}
		openaiRequest.Stop = geminiRequest.GenerationConfig.StopSequences
	}
	if geminiRequest.GenerationConfig.CandidateCount != nil && *geminiRequest.GenerationConfig.CandidateCount > 0 {
		openaiRequest.N = common.GetPointer(*geminiRequest.GenerationConfig.CandidateCount)
	}

	if len(geminiRequest.GetTools()) > 0 {
		var tools []dto.ToolCallRequest
		for _, tool := range geminiRequest.GetTools() {
			if tool.FunctionDeclarations == nil {
				continue
			}
			functionDeclarations, err := common.Any2Type[[]dto.FunctionRequest](tool.FunctionDeclarations)
			if err != nil {
				common.SysError(fmt.Sprintf("failed to parse gemini function declarations: %v (type=%T)", err, tool.FunctionDeclarations))
				continue
			}
			for _, function := range functionDeclarations {
				openAITool := dto.ToolCallRequest{
					Type: "function",
					Function: dto.FunctionRequest{
						Name:        function.Name,
						Description: function.Description,
						Parameters:  function.Parameters,
					},
				}
				tools = append(tools, openAITool)
			}
		}
		if len(tools) > 0 {
			openaiRequest.Tools = tools
		}
	}

	if geminiRequest.SystemInstructions != nil {
		systemMessage := dto.Message{
			Role:    "system",
			Content: extractTextFromGeminiParts(geminiRequest.SystemInstructions.Parts),
		}
		openaiRequest.Messages = append([]dto.Message{systemMessage}, openaiRequest.Messages...)
	}

	if err := applyGeminiControls(geminiRequest, openaiRequest); err != nil {
		return nil, err
	}
	return openaiRequest, nil
}

func convertGeminiRoleToOpenAI(geminiRole string) string {
	switch geminiRole {
	case "user":
		return "user"
	case "model":
		return "assistant"
	case "function":
		return "function"
	default:
		return "user"
	}
}

func extractTextFromGeminiParts(parts []dto.GeminiPart) string {
	texts := make([]string, 0)
	for _, part := range parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n")
}
