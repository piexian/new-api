package stepfun

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
)

func stepfunTestInfo(baseURL string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeUnknown,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: baseURL,
			ChannelType:    constant.ChannelTypeStepFun,
		},
	}
}

func TestGetRequestURLTripleEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		base        string
		relayMode   int
		relayFormat types.RelayFormat
		finalFormat types.RelayFormat
		want        string
	}{
		{
			name:        "claude inbound passthrough",
			base:        "stepfun",
			relayFormat: types.RelayFormatClaude,
			finalFormat: types.RelayFormatClaude,
			want:        "https://api.stepfun.com/v1/messages",
		},
		{
			name:        "responses inbound passthrough",
			base:        "https://api.stepfun.ai",
			relayMode:   relayconstant.RelayModeResponses,
			relayFormat: types.RelayFormatOpenAIResponses,
			finalFormat: types.RelayFormatOpenAIResponses,
			want:        "https://api.stepfun.ai/v1/responses",
		},
		{
			name:        "chat inbound",
			base:        "https://api.stepfun.com/v1",
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://api.stepfun.com/v1/chat/completions",
		},
		{
			name:        "step plan prefix for chat",
			base:        "stepfun-step-plan",
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://api.stepfun.com/step_plan/v1/chat/completions",
		},
		{
			name:        "step plan prefix for claude",
			base:        "https://api.stepfun.com/step_plan/v1",
			relayFormat: types.RelayFormatClaude,
			finalFormat: types.RelayFormatClaude,
			want:        "https://api.stepfun.com/step_plan/v1/messages",
		},
		{
			name:        "step plan prefix for responses",
			base:        "stepfun-step-plan",
			relayMode:   relayconstant.RelayModeResponses,
			relayFormat: types.RelayFormatOpenAIResponses,
			finalFormat: types.RelayFormatOpenAIResponses,
			want:        "https://api.stepfun.com/step_plan/v1/responses",
		},
		{
			name:        "claude converted to chat falls back to chat path",
			base:        "stepfun",
			relayFormat: types.RelayFormatClaude,
			finalFormat: types.RelayFormatOpenAI,
			want:        "https://api.stepfun.com/v1/chat/completions",
		},
		{
			name:        "tts keeps native path",
			base:        "stepfun",
			relayMode:   relayconstant.RelayModeAudioSpeech,
			relayFormat: types.RelayFormatOpenAIAudio,
			want:        "https://api.stepfun.com/v1/audio/speech",
		},
		{
			name:        "transcription keeps native path on step plan",
			base:        "stepfun-step-plan",
			relayMode:   relayconstant.RelayModeAudioTranscription,
			relayFormat: types.RelayFormatOpenAIAudio,
			want:        "https://api.stepfun.com/step_plan/v1/audio/transcriptions",
		},
		{
			name:        "non whitelisted base falls back to generic conversion path",
			base:        "https://my-proxy.example.com",
			relayFormat: types.RelayFormatClaude,
			want:        "https://my-proxy.example.com/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := stepfunTestInfo(tt.base)
			info.RelayMode = tt.relayMode
			info.RelayFormat = tt.relayFormat
			info.FinalRequestRelayFormat = tt.finalFormat
			info.RequestURLPath = "/v1/messages"
			if tt.relayFormat == types.RelayFormatOpenAI {
				info.RequestURLPath = "/v1/chat/completions"
			}
			if tt.relayMode == relayconstant.RelayModeAudioSpeech {
				info.RequestURLPath = "/v1/audio/speech"
			}
			if tt.relayMode == relayconstant.RelayModeAudioTranscription {
				info.RequestURLPath = "/v1/audio/transcriptions"
			}
			got, err := (&Adaptor{}).GetRequestURL(info)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("GetRequestURL(%s) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}

func TestValidateEndpointForModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		base       string
		channel    int
		model      string
		relayMode  int
		relayFmt   types.RelayFormat
		wantDenied bool
	}{
		{"responses unsupported model", "stepfun", constant.ChannelTypeStepFun, "step-3.5-flash", relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses, true},
		{"responses unsupported model 2603", "stepfun", constant.ChannelTypeStepFun, "step-3.5-flash-2603", relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses, true},
		{"responses supported model", "stepfun", constant.ChannelTypeStepFun, "step-3.7-flash", relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses, false},
		{"responses unknown model allowed", "stepfun", constant.ChannelTypeStepFun, "step-overture-preview", relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses, false},
		{"messages unsupported model", "stepfun", constant.ChannelTypeStepFun, "step-1o-turbo-vision", relayconstant.RelayModeUnknown, types.RelayFormatClaude, true},
		{"messages supported model", "stepfun", constant.ChannelTypeStepFun, "step-3.5-flash", relayconstant.RelayModeUnknown, types.RelayFormatClaude, false},
		{"non whitelisted base skipped", "https://my-proxy.example.com", constant.ChannelTypeStepFun, "step-3.5-flash", relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses, false},
		{"other channel type skipped", "stepfun", constant.ChannelTypeOpenAI, "step-3.5-flash", relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses, false},
		{"compact rejected", "stepfun", constant.ChannelTypeStepFun, "step-3.7-flash", relayconstant.RelayModeResponsesCompact, types.RelayFormatOpenAIResponses, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := stepfunTestInfo(tt.base)
			info.ChannelType = tt.channel
			info.ChannelMeta.ChannelType = tt.channel
			info.UpstreamModelName = tt.model
			info.OriginModelName = tt.model
			info.RelayMode = tt.relayMode
			info.RelayFormat = tt.relayFmt
			err := ValidateEndpointForModel(info)
			if tt.wantDenied && err == nil {
				t.Fatalf("expected validation error for %s on %s", tt.model, tt.relayFmt)
			}
			if !tt.wantDenied && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestGetRequestURLRejectsUnsupportedModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		relayMode int
	}{
		{"realtime", relayconstant.RelayModeRealtime},
		{"embeddings", relayconstant.RelayModeEmbeddings},
		{"rerank", relayconstant.RelayModeRerank},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := stepfunTestInfo("stepfun")
			info.RelayMode = tt.relayMode
			if _, err := (&Adaptor{}).GetRequestURL(info); err == nil {
				t.Fatalf("expected error for relay mode %d", tt.relayMode)
			}
		})
	}
}

func TestConvertClaudeRequestPassthroughAndSanitize(t *testing.T) {
	t.Parallel()

	text := "hi"
	cacheControl := []byte(`{"type":"ephemeral"}`)
	thinkingBudget := 40000
	req := &dto.ClaudeRequest{
		Model:     "step-3.5-flash",
		MaxTokens: boolPtrUint(1024),
		System: []dto.ClaudeMediaMessage{
			{Type: dto.ContentTypeText, Text: &text, CacheControl: cacheControl},
			{Type: "thinking", Thinking: &text},
		},
		Messages: []dto.ClaudeMessage{
			{Role: "assistant", Content: []dto.ClaudeMediaMessage{
				{Type: dto.ContentTypeText, Text: &text, CacheControl: cacheControl},
				{Type: "thinking", Thinking: &text},
				{Type: "tool_use", Id: "toolu_1", Name: "lookup", Input: map[string]any{"q": "x"}},
			}},
			{Role: "user", Content: []dto.ClaudeMediaMessage{
				{Type: "tool_result", ToolUseId: "toolu_1", Content: []any{
					map[string]any{"type": "text", "text": "ok", "cache_control": map[string]any{"type": "ephemeral"}},
				}},
			}},
		},
		Metadata:     []byte(`{"user_id":"abc"}`),
		ToolChoice:   map[string]any{"type": "auto"},
		Thinking:     &dto.Thinking{Type: "enabled", BudgetTokens: &thinkingBudget},
		CacheControl: cacheControl,
		McpServers:   []byte(`[]`),
		Tools: []any{
			map[string]any{"name": "lookup", "description": "d", "input_schema": map[string]any{"type": "object"}},
			map[string]any{"type": "web_search_20250305", "name": "web_search"},
		},
	}

	info := stepfunTestInfo("stepfun")
	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, req)
	if err != nil {
		t.Fatalf("ConvertClaudeRequest returned error: %v", err)
	}
	if converted != any(req) {
		t.Fatalf("claude request should be passed through as-is")
	}
	if info.FinalRequestRelayFormat != types.RelayFormatClaude {
		t.Fatalf("final relay format = %q, want claude", info.FinalRequestRelayFormat)
	}

	if req.Metadata != nil || req.ToolChoice != nil || req.Thinking != nil || req.CacheControl != nil || req.McpServers != nil {
		t.Fatalf("anthropic private top-level fields must be stripped: %+v", req)
	}
	if string(req.OutputConfig) != `{"effort":"high"}` {
		t.Fatalf("thinking budget should map to output_config.effort, got %s", req.OutputConfig)
	}

	systemBlocks, ok := req.System.([]dto.ClaudeMediaMessage)
	if !ok || len(systemBlocks) != 1 || systemBlocks[0].Type != dto.ContentTypeText {
		t.Fatalf("system should keep text blocks only, got %#v", req.System)
	}
	if systemBlocks[0].CacheControl != nil {
		t.Fatalf("system cache_control must be stripped")
	}

	if len(req.Messages) != 2 {
		t.Fatalf("messages length = %d", len(req.Messages))
	}
	assistantBlocks, ok := req.Messages[0].Content.([]dto.ClaudeMediaMessage)
	if !ok || len(assistantBlocks) != 2 {
		t.Fatalf("assistant thinking block should be dropped, got %#v", req.Messages[0].Content)
	}
	if assistantBlocks[0].CacheControl != nil {
		t.Fatalf("block cache_control must be stripped")
	}
	if assistantBlocks[1].Type != "tool_use" {
		t.Fatalf("tool_use block must be preserved")
	}

	tools, ok := req.Tools.([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("server tools should be dropped, got %#v", req.Tools)
	}
	if _, isMap := tools[0].(map[string]any); !isMap {
		t.Fatalf("tool entry should stay a map, got %T", tools[0])
	}
}

func TestConvertClaudeRequestFallsBackForNonWhitelistedBase(t *testing.T) {
	t.Parallel()

	req := &dto.ClaudeRequest{
		Model:     "gpt-4o",
		MaxTokens: boolPtrUint(16),
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	}
	info := stepfunTestInfo("https://my-proxy.example.com")
	info.RelayFormat = types.RelayFormatClaude
	if _, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, req); err != nil {
		t.Fatalf("fallback conversion failed: %v", err)
	}
	if info.FinalRequestRelayFormat == types.RelayFormatClaude {
		t.Fatalf("non whitelisted base must not claim native claude passthrough")
	}
}

func boolPtrUint(v uint) *uint { return &v }
