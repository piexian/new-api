package oairesponses

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	geminiResponsesInputTypeCustomToolCall       = "custom_tool_call"
	geminiResponsesInputTypeCustomToolCallOutput = "custom_tool_call_output"
	geminiResponsesInputTypeFunctionCallOutput   = "function_call_output"
)

const (
	ResponsesInputTypeCustomToolCallOutput = geminiResponsesInputTypeCustomToolCallOutput
)

func PrepareOpenAIResponsesRequest(request dto.OpenAIResponsesRequest) (dto.OpenAIResponsesRequest, error) {
	tools, err := filterGeminiResponsesTools(request.Tools)
	if err != nil {
		return request, err
	}
	request.Tools = tools

	input, err := filterGeminiResponsesInput(request.Input)
	if err != nil {
		return request, err
	}
	request.Input = input

	return request, nil
}

func filterGeminiResponsesTools(raw []byte) ([]byte, error) {
	if !geminiRawJSONPresent(raw) || common.GetJsonType(raw) != "array" {
		return raw, nil
	}

	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil, err
	}

	filtered := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(common.Interface2String(tool["type"])) != "function" {
			return nil, fmt.Errorf("Gemini conversion cannot preserve Responses tool type %q", common.Interface2String(tool["type"]))
		}
		filtered = append(filtered, tool)
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	return common.Marshal(filtered)
}

func filterGeminiResponsesInput(raw []byte) ([]byte, error) {
	if !geminiRawJSONPresent(raw) || common.GetJsonType(raw) != "array" {
		return raw, nil
	}

	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		itemType := strings.TrimSpace(common.Interface2String(item["type"]))
		switch itemType {
		case geminiResponsesInputTypeCustomToolCall, geminiResponsesInputTypeCustomToolCallOutput:
			return nil, fmt.Errorf("Gemini conversion cannot preserve Responses input item %q", itemType)
		}
		filtered = append(filtered, item)
	}

	return common.Marshal(filtered)
}

func geminiRawJSONPresent(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	return common.GetJsonType(raw) != "null"
}
