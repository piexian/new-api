package opencode

import (
	_ "embed"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// opencodeToolsJSON holds the genuine OpenCode tool definitions (description
// and JSON schema) captured from the official CLI v2.0.32, including the 2.x
// "patch" tool and the Linux/bash shell description. Since 2026-10-14
// the free tier compares the description and parameter schema of every tool
// name it recognises byte-for-byte, so synthesised stubs are rejected with
// 403 FreeTierError.
//
//go:embed opencode_tools.json
var opencodeToolsJSON []byte

type opencodeToolDefinition struct {
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

var (
	opencodeToolsOnce sync.Once
	opencodeTools     map[string]opencodeToolDefinition
)

// knownOpenCodeTool returns the genuine definition for an OpenCode tool name.
func knownOpenCodeTool(name string) (opencodeToolDefinition, bool) {
	opencodeToolsOnce.Do(func() {
		definitions := map[string]opencodeToolDefinition{}
		if err := common.Unmarshal(opencodeToolsJSON, &definitions); err != nil {
			definitions = map[string]opencodeToolDefinition{}
		}
		opencodeTools = definitions
	})
	definition, ok := opencodeTools[name]
	return definition, ok
}

// openCodeToolWire renders one tool in the shape the given request mode
// expects, using the genuine OpenCode definition. Falls back to a minimal
// stub only for names the official client does not ship (never sent for the
// core tools, which are all known).
func openCodeToolWire(mode int, name string) map[string]any {
	description := "Agent tool " + name
	parameters := map[string]any{"type": "object", "properties": map[string]any{}}
	if definition, ok := knownOpenCodeTool(name); ok {
		description = definition.Description
		if definition.Parameters != nil {
			parameters = definition.Parameters
		}
	}
	if mode == requestModeResponses {
		return map[string]any{
			"type":        "function",
			"name":        name,
			"description": description,
			"parameters":  parameters,
		}
	}
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        name,
			"description": description,
			"parameters":  parameters,
		},
	}
}
