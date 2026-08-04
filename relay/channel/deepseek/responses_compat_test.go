package deepseek

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	globalconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestConvertOpenAIResponsesRequestUsesNativeResponsesPassthrough(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-v4-flash",
		},
	}
	stream := true
	maxOutputTokens := uint(1024)
	request := dto.OpenAIResponsesRequest{
		Model:           "deepseek-v4-flash",
		Input:           []byte(`[{"role":"user","content":"hello"}]`),
		Instructions:    []byte(`"answer briefly"`),
		MaxOutputTokens: &maxOutputTokens,
		Reasoning:       &dto.Reasoning{Effort: "high"},
		Stream:          &stream,
		Text:            []byte(`{"format":{"type":"text"}}`),
		Tools:           []byte(`[{"type":"function","name":"lookup","parameters":{"type":"object"}}]`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	responsesReq, ok := converted.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want dto.OpenAIResponsesRequest", converted)
	}
	if info.FinalRequestRelayFormat != types.RelayFormatOpenAIResponses {
		t.Fatalf("FinalRequestRelayFormat = %q, want %q", info.FinalRequestRelayFormat, types.RelayFormatOpenAIResponses)
	}
	if responsesReq.Model != request.Model || string(responsesReq.Input) != string(request.Input) {
		t.Fatalf("model/input = %q/%s, want %q/%s", responsesReq.Model, responsesReq.Input, request.Model, request.Input)
	}
	if string(responsesReq.Instructions) != string(request.Instructions) || string(responsesReq.Tools) != string(request.Tools) || string(responsesReq.Text) != string(request.Text) {
		t.Fatalf("Responses fields were not preserved: %#v", responsesReq)
	}
	if responsesReq.MaxOutputTokens == nil || *responsesReq.MaxOutputTokens != maxOutputTokens || responsesReq.Stream == nil || !*responsesReq.Stream {
		t.Fatalf("max_output_tokens/stream = %#v/%#v, want %d/true", responsesReq.MaxOutputTokens, responsesReq.Stream, maxOutputTokens)
	}
	if responsesReq.Reasoning == nil || responsesReq.Reasoning.Effort != "high" {
		t.Fatalf("reasoning = %#v, want effort high", responsesReq.Reasoning)
	}
}

func TestConvertOpenAIResponsesRequestAppliesDeepSeekV4ThinkingSuffix(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-v4-chat-max",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "deepseek-v4-chat-max",
		Input: []byte(`"hello"`),
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	responsesReq, ok := converted.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want dto.OpenAIResponsesRequest", converted)
	}
	if responsesReq.Model != "deepseek-v4-chat" {
		t.Fatalf("model = %q, want deepseek-v4-chat", responsesReq.Model)
	}
	if responsesReq.Reasoning == nil || responsesReq.Reasoning.Effort != "max" || info.ReasoningEffort != "max" {
		t.Fatalf("reasoning effort request/info = %#v/%q, want max/max", responsesReq.Reasoning, info.ReasoningEffort)
	}
	if info.UpstreamModelName != "deepseek-v4-chat" {
		t.Fatalf("UpstreamModelName = %q, want deepseek-v4-chat", info.UpstreamModelName)
	}
}

func TestConvertOpenAIResponsesRequestAllowsStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	stream := true

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{}, dto.OpenAIResponsesRequest{
		Model:  "deepseek-chat",
		Input:  []byte(`"hello"`),
		Stream: &stream,
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}
	responsesReq, ok := converted.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want dto.OpenAIResponsesRequest", converted)
	}
	if responsesReq.Stream == nil || !*responsesReq.Stream {
		t.Fatalf("Stream = %#v, want true", responsesReq.Stream)
	}
}

func TestDoResponsePassesThroughNativeResponsesStreamWithoutDoneSentinel(t *testing.T) {
	oldStreamingTimeout := globalconstant.StreamingTimeout
	globalconstant.StreamingTimeout = 30
	t.Cleanup(func() { globalconstant.StreamingTimeout = oldStreamingTimeout })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-v4-flash",
		},
	}
	_, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "deepseek-v4-flash",
		Input: []byte(`"hello"`),
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	completed := `{"type":"response.completed","sequence_number":2,"response":{"id":"resp_1","status":"completed","usage":{"input_tokens":3,"output_tokens":5,"total_tokens":8}}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"event: response.created\n" +
				`data: {"type":"response.created","sequence_number":1,"response":{"id":"resp_1","status":"in_progress"}}` + "\n\n" +
				"event: response.completed\n" +
				"data: " + completed + "\n\n",
		)),
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	if apiErr != nil {
		t.Fatalf("DoResponse returned error: %v", apiErr)
	}
	usageDto, ok := usage.(*dto.Usage)
	if !ok || usageDto.PromptTokens != 3 || usageDto.CompletionTokens != 5 || usageDto.TotalTokens != 8 {
		t.Fatalf("usage = %#v, want 3/5/8", usage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: response.completed") || !strings.Contains(body, completed) {
		t.Fatalf("response stream did not preserve terminal Responses event: %s", body)
	}
	if strings.Contains(body, "[DONE]") {
		t.Fatalf("response stream unexpectedly contains [DONE]: %s", body)
	}
}
