package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestToolCallRequestMarshalJSON(t *testing.T) {
	t.Run("function tool keeps function field", func(t *testing.T) {
		data, err := common.Marshal(ToolCallRequest{
			ID:   "call_1",
			Type: "function",
			Function: FunctionRequest{
				Name:      "lookup",
				Arguments: `{"q":"x"}`,
			},
		})
		require.NoError(t, err)
		require.JSONEq(t, `{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}}`, string(data))
	})

	t.Run("builtin web_search omits empty function", func(t *testing.T) {
		data, err := common.Marshal(ToolCallRequest{Type: "web_search"})
		require.NoError(t, err)
		require.JSONEq(t, `{"type":"web_search"}`, string(data))
	})

	t.Run("builtin code_interpreter omits empty function", func(t *testing.T) {
		data, err := common.Marshal(ToolCallRequest{Type: "code_interpreter"})
		require.NoError(t, err)
		require.JSONEq(t, `{"type":"code_interpreter"}`, string(data))
	})

	t.Run("custom type keeps custom payload", func(t *testing.T) {
		data, err := common.Marshal(ToolCallRequest{
			Type:   "custom",
			Custom: []byte(`{"name":"my_tool"}`),
		})
		require.NoError(t, err)
		require.JSONEq(t, `{"type":"custom","custom":{"name":"my_tool"}}`, string(data))
	})

	t.Run("custom tool call keeps non-empty function", func(t *testing.T) {
		// responses 的 custom_tool_call 转成 chat 后把 name/input 放在 function 里，不能丢
		data, err := common.Marshal(ToolCallRequest{
			ID:   "call_custom",
			Type: CustomType,
			Function: FunctionRequest{
				Name:      "apply_patch",
				Arguments: "patch body",
			},
			Custom: []byte(`{"type":"custom_tool_call"}`),
		})
		require.NoError(t, err)
		require.JSONEq(t, `{"id":"call_custom","type":"custom","function":{"name":"apply_patch","arguments":"patch body"},"custom":{"type":"custom_tool_call"}}`, string(data))
	})
}

func TestToolCallRequestRoundTripBuiltinTool(t *testing.T) {
	// 客户端原样下发内置工具定义，经解析→再序列化后不得携带幽灵 function 字段
	var req GeneralOpenAIRequest
	err := common.Unmarshal([]byte(`{
		"model":"mistral-large-latest",
		"messages":[{"role":"user","content":"hi"}],
		"tools":[{"type":"web_search"}]
	}`), &req)
	require.NoError(t, err)
	require.Len(t, req.Tools, 1)

	data, err := common.Marshal(req)
	require.NoError(t, err)
	require.Contains(t, string(data), `"tools":[{"type":"web_search"}]`)
	require.NotContains(t, string(data), `"function"`)
}
