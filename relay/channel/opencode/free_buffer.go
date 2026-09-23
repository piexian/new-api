package opencode

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type freeChatChoice struct {
	message map[string]json.RawMessage
	tools   map[int]map[string]json.RawMessage
	finish  json.RawMessage
}

func collapseFreeStream(stream *freeStream) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(stream, freeBufferedLimit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > freeBufferedLimit {
		return nil, errors.New("buffered upstream response exceeds size limit")
	}
	if stream.mode == requestModeResponses {
		return collapseFreeResponses(body)
	}
	root := map[string]json.RawMessage{}
	choices := map[int]*freeChatChoice{}
	for _, line := range bytes.Split(body, []byte{'\n'}) {
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(line[5:])
		if bytes.Equal(data, []byte("[DONE]")) {
			continue
		}
		var chunk map[string]json.RawMessage
		if err := common.Unmarshal(data, &chunk); err != nil {
			return nil, err
		}
		for key, value := range chunk {
			if key != "choices" && key != "object" && string(value) != "null" {
				root[key] = value
			}
		}
		for _, choice := range gjson.GetBytes(data, "choices").Array() {
			index := int(choice.Get("index").Int())
			current := choices[index]
			if current == nil {
				current = &freeChatChoice{message: map[string]json.RawMessage{"role": json.RawMessage(`"assistant"`)}, tools: map[int]map[string]json.RawMessage{}}
				choices[index] = current
			}
			if finish := choice.Get("finish_reason"); finish.Exists() && finish.Type != gjson.Null {
				current.finish = json.RawMessage(finish.Raw)
			}
			var delta map[string]json.RawMessage
			if raw := choice.Get("delta"); raw.IsObject() {
				if err := common.Unmarshal([]byte(raw.Raw), &delta); err != nil {
					return nil, err
				}
			}
			for key, value := range delta {
				switch key {
				case "tool_calls":
					for _, tool := range gjson.ParseBytes(value).Array() {
						target := current.tools[int(tool.Get("index").Int())]
						if target == nil {
							target = map[string]json.RawMessage{}
							current.tools[int(tool.Get("index").Int())] = target
						}
						tool.ForEach(func(k, v gjson.Result) bool {
							if k.String() != "index" {
								if k.String() == "function" {
									target["function"] = mergeFreeFunction(target["function"], json.RawMessage(v.Raw))
								} else {
									target[k.String()] = json.RawMessage(v.Raw)
								}
							}
							return true
						})
					}
				case "content", "reasoning_content", "reasoning", "refusal", "annotations":
					current.message[key] = appendFreeDelta(current.message[key], value)
				case "function_call":
					current.message[key] = mergeFreeFunction(current.message[key], value)
				default:
					current.message[key] = value
				}
			}
		}
	}
	indices := make([]int, 0, len(choices))
	for index := range choices {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	out := make([]any, 0, len(indices))
	for _, index := range indices {
		choice := choices[index]
		if len(choice.tools) > 0 {
			toolIndices := make([]int, 0, len(choice.tools))
			for index := range choice.tools {
				toolIndices = append(toolIndices, index)
			}
			sort.Ints(toolIndices)
			tools := make([]any, 0, len(toolIndices))
			for _, index := range toolIndices {
				tools = append(tools, choice.tools[index])
			}
			choice.message["tool_calls"], err = common.Marshal(tools)
			if err != nil {
				return nil, err
			}
		}
		if _, ok := choice.message["content"]; !ok {
			choice.message["content"] = json.RawMessage("null")
		}
		out = append(out, map[string]any{"index": index, "message": choice.message, "finish_reason": choice.finish})
	}
	root["choices"], err = common.Marshal(out)
	if err != nil {
		return nil, err
	}
	root["object"] = json.RawMessage(`"chat.completion"`)
	return common.Marshal(root)
}

func appendFreeDelta(previous, next json.RawMessage) json.RawMessage {
	before, after := gjson.ParseBytes(previous), gjson.ParseBytes(next)
	if after.Type == gjson.Null {
		if len(previous) > 0 {
			return previous
		}
		return next
	}
	if after.Type == gjson.String {
		encoded, _ := common.Marshal(before.String() + after.String())
		return encoded
	}
	if after.IsArray() {
		var first, second []json.RawMessage
		_ = common.Unmarshal(previous, &first)
		_ = common.Unmarshal(next, &second)
		encoded, _ := common.Marshal(append(first, second...))
		return encoded
	}
	return next
}

func mergeFreeFunction(previous, next json.RawMessage) json.RawMessage {
	fields := map[string]json.RawMessage{}
	if len(previous) > 0 {
		_ = common.Unmarshal(previous, &fields)
	}
	gjson.ParseBytes(next).ForEach(func(k, v gjson.Result) bool {
		fields[k.String()] = appendFreeDelta(fields[k.String()], json.RawMessage(v.Raw))
		return true
	})
	encoded, _ := common.Marshal(fields)
	return encoded
}

func collapseFreeResponses(body []byte) ([]byte, error) {
	acc := relayconvert.NewResponsesBufferedAccumulator()
	var final []byte
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(line[5:])
		var event dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &event); err != nil {
			return nil, err
		}
		acc.ProcessEvent(&event)
		switch event.Type {
		case "response.completed", "response.done", "response.incomplete":
			final = []byte(gjson.Get(data, "response").Raw)
			if event.Response == nil {
				return nil, errors.New("upstream stream has no final response")
			}
			acc.SupplementResponseOutput(event.Response)
			if len(gjson.GetBytes(final, "output").Array()) == 0 && len(event.Response.Output) > 0 {
				var err error
				final, err = sjson.SetBytes(final, "output", event.Response.Output)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if len(final) == 0 {
		return nil, errors.New("upstream stream has no final response")
	}
	return final, nil
}
