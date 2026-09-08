package relayconvert_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const imageData = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC"

func TestRequestConversionImageMatrix(t *testing.T) {
	fixtures := []struct {
		format     types.RelayFormat
		body       string
		newRequest func() any
	}{
		{types.RelayFormatOpenAI, `{"model":"test","messages":[{"role":"user","content":[{"type":"text","text":"inspect"},{"type":"image_url","image_url":{"url":"data:image/png;base64,IMAGE"}}]}]}`, func() any { return &dto.GeneralOpenAIRequest{} }},
		{types.RelayFormatClaude, `{"model":"test","max_tokens":100,"messages":[{"role":"user","content":[{"type":"text","text":"inspect"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"IMAGE"}}]}]}`, func() any { return &dto.ClaudeRequest{} }},
		{types.RelayFormatOpenAIResponses, `{"model":"test","input":[{"role":"user","content":[{"type":"input_text","text":"inspect"},{"type":"input_image","image_url":"data:image/png;base64,IMAGE"}]}]}`, func() any { return &dto.OpenAIResponsesRequest{} }},
		{types.RelayFormatGemini, `{"contents":[{"role":"user","parts":[{"text":"inspect"},{"inlineData":{"mimeType":"image/png","data":"IMAGE"}}]}]}`, func() any { return &dto.GeminiChatRequest{} }},
	}
	for _, fixture := range fixtures {
		for _, target := range []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude, types.RelayFormatOpenAIResponses, types.RelayFormatGemini, types.RelayFormatGeminiInteractions} {
			if target == fixture.format {
				continue
			}
			t.Run(string(fixture.format)+"/"+string(target), func(t *testing.T) {
				req := fixture.newRequest()
				require.NoError(t, common.Unmarshal([]byte(strings.ReplaceAll(fixture.body, "IMAGE", imageData)), req))
				converted := convertRequest(t, req, fixture.format, target)
				wire, err := common.Marshal(converted)
				require.NoError(t, err)
				var value any
				require.NoError(t, common.Unmarshal(wire, &value))
				require.Equal(t, []string{imageData}, collectImages(value), string(wire))
				require.Contains(t, string(wire), "inspect")
			})
		}
	}
}

func TestRequestConversionToolMediaMatrix(t *testing.T) {
	fixtures := []struct {
		format     types.RelayFormat
		body       string
		newRequest func() any
	}{
		{types.RelayFormatOpenAI, `{"model":"test","messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"screenshot","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_1","content":[{"type":"text","text":"tool result text"},{"type":"image_url","image_url":{"url":"data:image/png;base64,IMAGE"}}]}]}`, func() any { return &dto.GeneralOpenAIRequest{} }},
		{types.RelayFormatClaude, `{"model":"test","max_tokens":100,"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"call_1","name":"screenshot","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":[{"type":"text","text":"tool result text"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"IMAGE"}}]}]}]}`, func() any { return &dto.ClaudeRequest{} }},
		{types.RelayFormatOpenAIResponses, `{"model":"test","input":[{"type":"function_call","call_id":"call_1","name":"screenshot","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":[{"type":"input_text","text":"tool result text"},{"type":"input_image","image_url":"data:image/png;base64,IMAGE"}]}]}`, func() any { return &dto.OpenAIResponsesRequest{} }},
		{types.RelayFormatGemini, `{"contents":[{"role":"model","parts":[{"functionCall":{"id":"call_1","name":"screenshot","args":{}}}]},{"role":"user","parts":[{"functionResponse":{"id":"call_1","name":"screenshot","response":{"text":"tool result text"},"parts":[{"inlineData":{"mimeType":"image/png","data":"IMAGE"}}]}}]}]}`, func() any { return &dto.GeminiChatRequest{} }},
	}
	for _, fixture := range fixtures {
		for _, target := range []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude, types.RelayFormatOpenAIResponses, types.RelayFormatGemini, types.RelayFormatGeminiInteractions} {
			if target == fixture.format {
				continue
			}
			t.Run(string(fixture.format)+"/"+string(target), func(t *testing.T) {
				req := fixture.newRequest()
				require.NoError(t, common.Unmarshal([]byte(strings.ReplaceAll(fixture.body, "IMAGE", imageData)), req))
				wire, err := common.Marshal(convertRequest(t, req, fixture.format, target))
				require.NoError(t, err)
				var value any
				require.NoError(t, common.Unmarshal(wire, &value))
				require.Equal(t, []string{imageData}, collectImages(value), string(wire))
				require.Contains(t, string(wire), "tool result text")
			})
		}
	}
}

func convertRequest(t *testing.T, req any, from, to types.RelayFormat) any {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)
	t.Cleanup(func() { service.CleanupFileSources(c) })
	info := &relaycommon.RelayInfo{RelayFormat: from, OriginModelName: "test", ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "test"}}
	result, err := relayconvert.ConvertRequest(c, info, to, req)
	require.NoError(t, err)
	return result.Value
}

// Count actual media blocks in the serialized request, never strings containing
// JSON or a textual image placeholder.
func collectImages(value any) []string {
	var found []string
	switch v := value.(type) {
	case []any:
		for _, child := range v {
			found = append(found, collectImages(child)...)
		}
	case map[string]any:
		var source string
		switch v["type"] {
		case "image_url", "input_image":
			switch img := v["image_url"].(type) {
			case string:
				source = img
			case map[string]any:
				source, _ = img["url"].(string)
			}
		case "image":
			if img, ok := v["source"].(map[string]any); ok {
				source, _ = img["data"].(string)
				if source == "" {
					source, _ = img["url"].(string)
				}
			} else {
				source, _ = v["data"].(string)
				if source == "" {
					source, _ = v["uri"].(string)
				}
			}
		}
		if img, ok := v["inlineData"].(map[string]any); ok {
			if mime, _ := img["mimeType"].(string); strings.HasPrefix(mime, "image/") {
				source, _ = img["data"].(string)
			}
		}
		if source != "" {
			found = append(found, strings.TrimPrefix(source, "data:image/png;base64,"))
		}
		for _, child := range v {
			found = append(found, collectImages(child)...)
		}
	}
	return found
}
