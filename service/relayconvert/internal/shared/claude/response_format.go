package claude

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func ApplyOpenAIResponseFormat(format *dto.ResponseFormat, request *dto.ClaudeRequest) error {
	if format == nil || format.Type == "" || format.Type == "text" {
		return nil
	}
	if format.Type != "json_schema" {
		return fmt.Errorf("Claude conversion requires an explicit JSON schema for response format %q", format.Type)
	}
	var schema dto.FormatJsonSchema
	if err := common.Unmarshal(format.JsonSchema, &schema); err != nil {
		return err
	}
	var config map[string]any
	if len(request.OutputConfig) > 0 {
		if err := common.Unmarshal(request.OutputConfig, &config); err != nil {
			return err
		}
	}
	if config == nil {
		config = make(map[string]any)
	}
	config["format"] = map[string]any{"type": "json_schema", "schema": schema.Schema}
	encoded, err := common.Marshal(config)
	request.OutputConfig = encoded
	return err
}
