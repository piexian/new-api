package opencode

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// freeCoreTools are the tool names the free tier expects on an agent-shaped
// request; at minimum "shell" and "read" must be present with genuine
// definitions. "shell" is the name OpenCode 2.x ships; the pre-2.x "bash"
// spelling is deliberately not injected because the gate validates the
// definition of every tool it recognises byte-for-byte.
var freeCoreTools = []string{"shell", "edit", "glob", "grep", "read"}

func (a *Adaptor) needsFreeCompatibility(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil || (relaycommon.IsRequestPassThroughEnabled(info) && !a.RouteByModel) {
		return false
	}
	if info.RelayMode != relayconstant.RelayModeUnknown && info.RelayMode != relayconstant.RelayModeChatCompletions && info.RelayMode != relayconstant.RelayModeResponses {
		return false
	}
	if a.RequestMode != requestModeOpenAI && a.RequestMode != requestModeResponses {
		return false
	}
	model := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(info.UpstreamModelName, "models/")))
	// Space Bunny has a Chat endpoint, but its Free stream/tool contract is unverified.
	if model == "space-bunny-free" {
		return false
	}
	return model == "big-pickle" || strings.HasSuffix(model, "-free") &&
		(stringListContains(constant.OpenCodeZenChatModels, model) || stringListContains(constant.OpenCodeZenResponsesModels, model))
}

// shapeFreeRequest preserves caller tools; compatibility-only tools are never executable downstream.
func (a *Adaptor) shapeFreeRequest(body []byte) ([]byte, map[string]bool, error) {
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return nil, nil, errors.New("invalid request body")
	}
	var tools []json.RawMessage
	rawTools := gjson.GetBytes(body, "tools")
	if rawTools.Exists() && rawTools.Type != gjson.Null {
		if !rawTools.IsArray() || common.Unmarshal([]byte(rawTools.Raw), &tools) != nil {
			return nil, nil, errors.New("invalid tools")
		}
	}
	originalCount := len(tools)
	namePath := "name"
	if a.RequestMode == requestModeOpenAI {
		namePath = "function.name"
	}
	existing := map[string]bool{}
	for index, tool := range tools {
		if gjson.GetBytes(tool, "type").String() != "function" {
			continue
		}
		name := gjson.GetBytes(tool, namePath).String()
		existing[name] = true
		// A caller-declared definition for a recognised tool name is replaced
		// with the genuine one: the free tier rejects any other schema, and a
		// caller that happens to reuse the name would otherwise fail the request.
		if _, known := knownOpenCodeTool(name); !known {
			continue
		}
		encoded, err := common.Marshal(openCodeToolWire(a.RequestMode, name))
		if err != nil {
			return nil, nil, err
		}
		tools[index] = encoded
	}
	injected := map[string]bool{}
	for _, name := range freeCoreTools {
		if existing[name] {
			continue
		}
		encoded, err := common.Marshal(openCodeToolWire(a.RequestMode, name))
		if err != nil {
			return nil, nil, err
		}
		tools = append(tools, encoded)
		injected[name] = true
	}
	var err error
	for path, value := range map[string]any{"stream": true, "tools": tools} {
		body, err = sjson.SetBytes(body, path, value)
		if err != nil {
			return nil, nil, err
		}
	}
	if a.RequestMode == requestModeOpenAI {
		body, err = sjson.SetBytes(body, "stream_options.include_usage", true)
		if err != nil {
			return nil, nil, err
		}
		// The Responses free provider only accepts auto; its stream guard remains mandatory.
		if originalCount == 0 && !gjson.GetBytes(body, "tool_choice").Exists() {
			body, err = sjson.SetBytes(body, "tool_choice", "none")
		}
	}
	return body, injected, err
}
