package opencode

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenCodeReasoningEffort(t *testing.T) {
	goBase := constant.OpenCodeGoBaseURLAlias
	zenBase := constant.OpenCodeZenBaseURLAlias
	for _, tc := range []struct {
		baseURL string
		model   string
		effort  string
		want    string
		keep    bool
	}{
		{goBase, "hy4-flash-free", "high", "high", true},
		{goBase, "hy4-flash-free", "none", "none", true},
		{goBase, "hy4-flash-free", "max", "", false},
		{goBase, "hy4-flash-free", "auto", "", false},
		{goBase, "hy4-flash-free", " High ", "high", true},
		{goBase, "kimi-k3", "max", "max", true},
		{goBase, "kimi-k3", "low", "", false},
		{zenBase, "kimi-k3", "low", "low", true},
		{goBase, "mimo-v2.6-flash", "medium", "medium", true},
		{goBase, "kimi-k2.7-code", "none", "none", true},
		{goBase, "hy4", "ultra", "", false},
		{zenBase, "deepseek-v4-pro", "ultra", "max", true},
		{zenBase, "hy4-flash-free", "xhigh", "", false},
		{goBase, "models/hy4", "high", "high", true},
	} {
		got, keep := normalizeOpenCodeReasoningEffort(tc.baseURL, tc.model, tc.effort)
		require.Equal(t, tc.keep, keep, tc.model+" / "+tc.effort)
		require.Equal(t, tc.want, got, tc.model+" / "+tc.effort)
	}
}

func TestNormalizeOutboundReasoningEffort(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl:    constant.OpenCodeGoBaseURLAlias,
		UpstreamModelName: "hy4-flash-free",
	}}
	a := &Adaptor{}

	chat := &dto.GeneralOpenAIRequest{ReasoningEffort: "auto"}
	a.normalizeOutboundReasoningEffort(nil, info, chat)
	require.Empty(t, chat.ReasoningEffort, "invalid effort should be stripped")

	chat = &dto.GeneralOpenAIRequest{ReasoningEffort: "High"}
	a.normalizeOutboundReasoningEffort(nil, info, chat)
	require.Equal(t, "high", chat.ReasoningEffort)

	responses := &dto.OpenAIResponsesRequest{Reasoning: &dto.Reasoning{Effort: "auto"}}
	a.normalizeOutboundReasoningEffort(nil, info, responses)
	require.Empty(t, responses.Reasoning.Effort)

	responses = &dto.OpenAIResponsesRequest{Reasoning: &dto.Reasoning{Effort: "none"}}
	a.normalizeOutboundReasoningEffort(nil, info, responses)
	require.Equal(t, "none", responses.Reasoning.Effort)

	// nil/空字段不触碰,避免空请求被误改。
	empty := &dto.OpenAIResponsesRequest{}
	a.normalizeOutboundReasoningEffort(nil, info, empty)
	require.Nil(t, empty.Reasoning)
}

func TestFreeCompatibilitySkipsGoLane(t *testing.T) {
	for _, tc := range []struct {
		baseURL string
		model   string
		want    bool
	}{
		{constant.OpenCodeGoBaseURLAlias, "hy4-flash-free", false},
		{constant.OpenCodeGoBaseURLAlias, "beam-free", false},
		{constant.OpenCodeZenBaseURLAlias, "hy4-flash-free", true},
		{constant.OpenCodeZenBaseURLAlias, "tidepool-alpha-free", true},
		{constant.OpenCodeZenBaseURLAlias, "big-pickle", true},
	} {
		info := &relaycommon.RelayInfo{
			RelayFormat: types.RelayFormatOpenAI,
			RelayMode:   relayconstant.RelayModeUnknown,
			ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: tc.baseURL, UpstreamModelName: tc.model},
		}
		a := &Adaptor{}
		a.Init(info)
		require.Equal(t, tc.want, a.needsFreeCompatibility(info), tc.baseURL+" / "+tc.model)
	}
}
