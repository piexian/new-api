package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func geminiReqFromJSON(t *testing.T, body string) *GeminiChatRequest {
	t.Helper()
	req := &GeminiChatRequest{}
	require.NoError(t, json.Unmarshal([]byte(body), req))
	return req
}

func TestRemoveEmptyParts(t *testing.T) {
	t.Parallel()

	req := geminiReqFromJSON(t, `{
		"contents": [
			{"role": "user", "parts": [{}]},
			{"role": "user", "parts": [{"text": "hi"}]},
			{"role": "model", "parts": [
				{"thoughtSignature": "sig", "thought": true},
				{"functionCall": {"name": "get_weather", "args": {"city": "sf"}}}
			]},
			{"role": "user", "parts": [{"text": ""}, {"functionResponse": {"name": "get_weather", "response": {"result": "sunny"}}}]},
			{"role": "user", "parts": [{"inlineData": {"data": ""}}]},
			{"role": "user", "parts": [{"text": ""}]}
		],
		"systemInstruction": {"parts": [{"text": ""}, {"text": "be brief"}]}
	}`)

	req.RemoveEmptyParts()

	// 空 part 与纯 thoughtSignature/thought part 被移除
	require.Len(t, req.Contents, 4)
	require.Equal(t, "hi", req.Contents[0].Parts[0].Text)
	require.Len(t, req.Contents[1].Parts, 1)
	require.NotNil(t, req.Contents[1].Parts[0].FunctionCall)
	// functionResponse 与空文本 part 同 content:空文本被移除,functionResponse 保留
	require.Len(t, req.Contents[2].Parts, 1)
	require.NotNil(t, req.Contents[2].Parts[0].FunctionResponse)
	require.NotNil(t, req.Contents[3].Parts[0].InlineData)

	// 有内容的 systemInstruction 只摘掉空 part
	require.NotNil(t, req.SystemInstructions)
	require.Len(t, req.SystemInstructions.Parts, 1)
	require.Equal(t, "be brief", req.SystemInstructions.Parts[0].Text)
}

func TestRemoveEmptyPartsDropsEmptyContentsAndSystemInstruction(t *testing.T) {
	t.Parallel()

	req := geminiReqFromJSON(t, `{
		"contents": [
			{"role": "user", "parts": [{"text": ""}]},
			{"role": "user", "parts": [{"text": "real"}]}
		],
		"systemInstruction": {"parts": [{}]}
	}`)

	req.RemoveEmptyParts()

	require.Len(t, req.Contents, 1)
	require.Equal(t, "real", req.Contents[0].Parts[0].Text)
	require.Nil(t, req.SystemInstructions)
}

func TestRemoveEmptyPartsRecursesBatchRequests(t *testing.T) {
	t.Parallel()

	req := geminiReqFromJSON(t, `{
		"requests": [
			{"contents": [{"role": "user", "parts": [{}]}]},
			{"contents": [{"role": "user", "parts": [{"text": "ok"}]}]}
		]
	}`)

	req.RemoveEmptyParts()

	require.Len(t, req.Requests, 1)
	require.Len(t, req.Requests[0].Contents[0].Parts, 1)
	require.Equal(t, "ok", req.Requests[0].Contents[0].Parts[0].Text)
}

func TestRemoveEmptyPartsKeepsAllDataVariants(t *testing.T) {
	t.Parallel()

	req := geminiReqFromJSON(t, `{
		"contents": [{"role": "user", "parts": [
			{"text": "t"},
			{"inlineData": {"mimeType": "image/png", "data": "abc"}},
			{"fileData": {"fileUri": "gs://bucket/a.png", "mimeType": "image/png"}},
			{"executableCode": {"language": "PYTHON", "code": "print(1)"}},
			{"codeExecutionResult": {"outcome": "OK", "output": "1"}}
		]}]
	}`)

	req.RemoveEmptyParts()

	require.Len(t, req.Contents[0].Parts, 5)
}
