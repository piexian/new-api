package common

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"
)

type ChannelProtocol string

const (
	ProtocolChat      ChannelProtocol = "chat"
	ProtocolResponses ChannelProtocol = "responses"
	ProtocolMessages  ChannelProtocol = "messages"
	ProtocolGemini    ChannelProtocol = "gemini"
)

type ResolvedProtocolSupport struct {
	Chat      bool `json:"chat"`
	Responses bool `json:"responses"`
	Messages  bool `json:"messages"`
	Gemini    bool `json:"gemini"`
}

func (s ResolvedProtocolSupport) IsEnabled(protocol ChannelProtocol) bool {
	switch protocol {
	case ProtocolChat:
		return s.Chat
	case ProtocolResponses:
		return s.Responses
	case ProtocolMessages:
		return s.Messages
	case ProtocolGemini:
		return s.Gemini
	default:
		return true
	}
}

func (s ResolvedProtocolSupport) HasAnyEnabled() bool {
	return s.Chat || s.Responses || s.Messages || s.Gemini
}

// PreferredTargetProtocols lists, in order of preference, the upstream protocols
// usable when the inbound protocol is disabled on the channel: the channel's
// native protocol first (its adaptor converts with the least loss), then
// Chat > Responses > Messages > Gemini. Targets whose responses cannot be
// rendered back to the inbound protocol are skipped.
func (s ResolvedProtocolSupport) PreferredTargetProtocols(inbound, native ChannelProtocol, isStream bool) []ChannelProtocol {
	candidates := [...]ChannelProtocol{native, ProtocolChat, ProtocolResponses, ProtocolMessages, ProtocolGemini}
	targets := make([]ChannelProtocol, 0, len(candidates))
	for _, target := range candidates {
		if target == "" || target == inbound || !s.IsEnabled(target) || !canRenderTargetToInbound(inbound, target, isStream) {
			continue
		}
		if slices.Contains(targets, target) {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

// canRenderTargetToInbound 排除响应无法转回入站协议的组合：Claude 响应处理只输出
// OpenAI/Claude 格式，Responses 入站经 Messages 仅支持非流式。
func canRenderTargetToInbound(inbound, target ChannelProtocol, isStream bool) bool {
	if target != ProtocolMessages {
		return true
	}
	switch inbound {
	case ProtocolGemini:
		return false
	case ProtocolResponses:
		return !isStream
	default:
		return true
	}
}

// NativeChannelProtocol returns the upstream protocol a channel's own adaptor
// speaks when it is not OpenAI Chat; empty means the channel is Chat-native.
func NativeChannelProtocol(channelType int, baseURL string, setting dto.ChannelSettings) ChannelProtocol {
	switch channelType {
	case constant.ChannelTypeAnthropic, constant.ChannelTypeAws:
		return ProtocolMessages
	case constant.ChannelTypeGemini, constant.ChannelTypeGeminiInteractions, constant.ChannelTypeVertexAi:
		return ProtocolGemini
	case constant.ChannelTypeCodex:
		return ProtocolResponses
	case constant.ChannelTypeMiniMaxCode:
		return ProtocolMessages
	case constant.ChannelTypeZhipu_v4:
		if setting.ZcodeModeEnabled && isZhipuZCodeBase(baseURL) {
			return ProtocolMessages
		}
	}
	return ""
}

// ProtocolExplicitlyDisabled reports whether protocol is switched off by an
// explicit protocol_support override. Protocols that are only off by the
// channel-type default keep the adaptor's built-in conversion.
func ProtocolExplicitlyDisabled(setting dto.ChannelSettings, protocol ChannelProtocol) bool {
	value := explicitProtocolValue(setting, protocol)
	return value != nil && !*value
}

// ProtocolExplicitlyEnabled reports whether protocol is switched on by an
// explicit protocol_support override.
func ProtocolExplicitlyEnabled(setting dto.ChannelSettings, protocol ChannelProtocol) bool {
	value := explicitProtocolValue(setting, protocol)
	return value != nil && *value
}

func explicitProtocolValue(setting dto.ChannelSettings, protocol ChannelProtocol) *bool {
	ps := setting.ProtocolSupport
	if ps == nil {
		return nil
	}
	switch protocol {
	case ProtocolChat:
		return ps.Chat
	case ProtocolResponses:
		return ps.Responses
	case ProtocolMessages:
		return ps.Messages
	case ProtocolGemini:
		return ps.Gemini
	default:
		return nil
	}
}

func isZhipuZCodeBase(baseURL string) bool {
	switch strings.ToLower(strings.TrimSpace(baseURL)) {
	case "glm-coding-plan", "glm-coding-plan-international", "zcode-start-plan":
		return true
	default:
		return false
	}
}

// DefaultChannelProtocolSupport returns the built-in upstream protocol support
// matrix for a channel type before per-channel overrides are applied.
func DefaultChannelProtocolSupport(channelType int, baseURL string, setting dto.ChannelSettings) ResolvedProtocolSupport {
	switch channelType {
	case constant.ChannelTypeUnknown,
		constant.ChannelTypeNewAPI,
		constant.ChannelTypeSub2API,
		constant.ChannelTypeCLIProxyAPI,
		constant.ChannelTypeZenMux,
		constant.ChannelTypeOpenCode,
		constant.ChannelTypeAdvancedCustom:
		return ResolvedProtocolSupport{
			Chat:      true,
			Responses: true,
			Messages:  true,
			Gemini:    true,
		}
	case constant.ChannelTypeZhipu_v4:
		if setting.ZcodeModeEnabled && isZhipuZCodeBase(baseURL) {
			return ResolvedProtocolSupport{
				Chat:      false,
				Responses: false,
				Messages:  true,
				Gemini:    false,
			}
		}
		return ResolvedProtocolSupport{
			Chat:      true,
			Responses: true,
			Messages:  true,
			Gemini:    false,
		}
	case constant.ChannelTypeOllama,
		constant.ChannelTypeAgnesAI,
		constant.ChannelTypeQwenTokenPlan,
		constant.ChannelTypeMiniMax,
		constant.ChannelTypeDeepSeek,
		constant.ChannelTypeAli,
		constant.ChannelTypeMoonshot,
		constant.ChannelTypeStepFun,
		constant.ChannelTypePoe,
		constant.ChannelTypeXiaomiMiMo,
		constant.ChannelTypeMoark:
		return ResolvedProtocolSupport{
			Chat:      true,
			Responses: true,
			Messages:  true,
			Gemini:    false,
		}
	case constant.ChannelTypeOpenAI,
		constant.ChannelTypeAzure,
		constant.ChannelTypeVolcEngine,
		constant.ChannelTypeXai,
		constant.ChannelTypeBaiduV2,
		constant.ChannelTypePerplexity:
		return ResolvedProtocolSupport{
			Chat:      true,
			Responses: true,
			Messages:  false,
			Gemini:    false,
		}
	case constant.ChannelTypeGMICloud,
		constant.ChannelTypeZhipu:
		return ResolvedProtocolSupport{
			Chat:      true,
			Responses: false,
			Messages:  true,
			Gemini:    false,
		}
	case constant.ChannelTypeCodex:
		return ResolvedProtocolSupport{
			Chat:      false,
			Responses: true,
			Messages:  false,
			Gemini:    false,
		}
	case constant.ChannelTypeAnthropic,
		constant.ChannelTypeAws:
		return ResolvedProtocolSupport{
			Chat:      false,
			Responses: false,
			Messages:  true,
			Gemini:    false,
		}
	case constant.ChannelTypeMiniMaxCode:
		return ResolvedProtocolSupport{
			Chat:      false,
			Responses: false,
			Messages:  true,
			Gemini:    false,
		}
	case constant.ChannelTypeGemini,
		constant.ChannelTypeGeminiInteractions:
		return ResolvedProtocolSupport{
			Chat:      setting.UpstreamOpenAICompatEnabled,
			Responses: false,
			Messages:  false,
			Gemini:    true,
		}
	case constant.ChannelTypeVertexAi:
		return ResolvedProtocolSupport{
			Chat:      false,
			Responses: false,
			Messages:  false,
			Gemini:    true,
		}
	case constant.ChannelTypePaLM,
		constant.ChannelTypeBaidu,
		constant.ChannelTypeXunfei,
		constant.ChannelTypeTencent,
		constant.ChannelTypeCohere,
		constant.ChannelTypeDify,
		constant.ChannelTypeJina,
		constant.ChannelTypeSiliconFlow,
		constant.ChannelTypeMistral,
		constant.ChannelTypeMokaAI,
		constant.ChannelTypeCoze,
		constant.ChannelTypeJimeng,
		constant.ChannelTypeSubmodel,
		constant.ChannelTypeReplicate,
		constant.ChannelTypeXunfeiMaaSImage,
		constant.ChannelTypeCerebras,
		constant.ChannelTypeMistralConsole,
		constant.ChannelTypeTypeSafe:
		// 自有 adaptor 且上游无原生 /v1/responses（由 adaptor 自行降级或不支持）
		return ResolvedProtocolSupport{
			Chat:      true,
			Responses: false,
			Messages:  false,
			Gemini:    false,
		}
	default:
		// OpenAI 兼容上游（openai adaptor 及其派生）原样转发 /v1/responses
		return ResolvedProtocolSupport{
			Chat:      true,
			Responses: true,
			Messages:  false,
			Gemini:    false,
		}
	}
}

// UsesPrivateChatProtocol reports whether the channel adaptor relays Chat over a
// non-OpenAI wire protocol, so the OpenAI handlers cannot render its responses.
func UsesPrivateChatProtocol(channelType int) bool {
	switch channelType {
	case constant.ChannelTypePaLM,
		constant.ChannelTypeBaidu,
		constant.ChannelTypeXunfei,
		constant.ChannelTypeTencent,
		constant.ChannelTypeZhipu,
		constant.ChannelTypeCohere,
		constant.ChannelTypeDify,
		constant.ChannelTypeJina,
		constant.ChannelTypeMokaAI,
		constant.ChannelTypeCoze,
		constant.ChannelTypeJimeng,
		constant.ChannelTypeReplicate,
		constant.ChannelTypeXunfeiMaaSImage,
		constant.ChannelTypeMistralConsole,
		constant.ChannelTypeTypeSafe:
		return true
	default:
		return false
	}
}

// ProtocolSupportLocked reports whether the channel's upstream speaks one fixed
// protocol (or a private wire protocol). Other protocols only work through the
// adaptor's own conversion, so protocol_support overrides are ignored there.
func ProtocolSupportLocked(channelType int, baseURL string, setting dto.ChannelSettings) bool {
	switch channelType {
	case constant.ChannelTypeAnthropic,
		constant.ChannelTypeAws,
		constant.ChannelTypeVertexAi,
		constant.ChannelTypeCodex,
		constant.ChannelTypeMiniMaxCode:
		return true
	case constant.ChannelTypeZhipu_v4:
		return setting.ZcodeModeEnabled && isZhipuZCodeBase(baseURL)
	}
	return UsesPrivateChatProtocol(channelType)
}

// WithoutLockedProtocolOverrides drops protocol_support on locked channels, so
// overrides saved before the lock existed cannot steer routing.
func WithoutLockedProtocolOverrides(channelType int, baseURL string, setting dto.ChannelSettings) dto.ChannelSettings {
	if setting.ProtocolSupport != nil && ProtocolSupportLocked(channelType, baseURL, setting) {
		setting.ProtocolSupport = nil
	}
	return setting
}

// ChannelProtocolSupportInfo is the protocol view exposed to the admin UI, so
// the frontends do not keep their own copy of the defaults matrix.
type ChannelProtocolSupportInfo struct {
	Defaults ResolvedProtocolSupport `json:"defaults"`
	Resolved ResolvedProtocolSupport `json:"resolved"`
	Locked   bool                    `json:"locked"`
}

func BuildChannelProtocolSupportInfo(channelType int, baseURL string, setting dto.ChannelSettings) ChannelProtocolSupportInfo {
	return ChannelProtocolSupportInfo{
		Defaults: DefaultChannelProtocolSupport(channelType, baseURL, setting),
		Resolved: ResolveChannelProtocolSupport(channelType, baseURL, setting),
		Locked:   ProtocolSupportLocked(channelType, baseURL, setting),
	}
}

// NormalizeProtocolSupportOverrides turns the desired protocol state submitted
// by the UI into stored overrides: values equal to the channel-type default are
// dropped, Gemini Chat is carried by upstream_openai_compat_enabled, and locked
// channels keep no overrides at all.
func NormalizeProtocolSupportOverrides(channelType int, baseURL string, setting dto.ChannelSettings) dto.ChannelSettings {
	ps := setting.ProtocolSupport
	if ps == nil {
		return setting
	}
	if ProtocolSupportLocked(channelType, baseURL, setting) {
		setting.ProtocolSupport = nil
		return setting
	}
	next := *ps
	if channelType == constant.ChannelTypeGemini || channelType == constant.ChannelTypeGeminiInteractions {
		if next.Chat != nil {
			setting.UpstreamOpenAICompatEnabled = *next.Chat
			next.Chat = nil
		}
	}
	defaults := DefaultChannelProtocolSupport(channelType, baseURL, setting)
	dropDefault := func(value *bool, defaultValue bool) *bool {
		if value == nil || *value == defaultValue {
			return nil
		}
		return value
	}
	next.Chat = dropDefault(next.Chat, defaults.Chat)
	next.Responses = dropDefault(next.Responses, defaults.Responses)
	next.Messages = dropDefault(next.Messages, defaults.Messages)
	next.Gemini = dropDefault(next.Gemini, defaults.Gemini)
	if next.Chat == nil && next.Responses == nil && next.Messages == nil && next.Gemini == nil {
		setting.ProtocolSupport = nil
	} else {
		setting.ProtocolSupport = &next
	}
	return setting
}

// ResolveChannelProtocolSupport merges channel-type defaults with explicit
// protocol_support overrides; locked channels always use the defaults.
func ResolveChannelProtocolSupport(channelType int, baseURL string, setting dto.ChannelSettings) ResolvedProtocolSupport {
	setting = WithoutLockedProtocolOverrides(channelType, baseURL, setting)
	res := DefaultChannelProtocolSupport(channelType, baseURL, setting)
	if ps := setting.ProtocolSupport; ps != nil {
		if ps.Chat != nil {
			res.Chat = *ps.Chat
		}
		if ps.Responses != nil {
			res.Responses = *ps.Responses
		}
		if ps.Messages != nil {
			res.Messages = *ps.Messages
		}
		if ps.Gemini != nil {
			res.Gemini = *ps.Gemini
		}
	}
	return res
}

func ResolveChannelProtocolSupportFromInfo(info *RelayInfo) ResolvedProtocolSupport {
	if info == nil || info.ChannelMeta == nil {
		return ResolvedProtocolSupport{Chat: true, Responses: true, Messages: true, Gemini: true}
	}
	return ResolveChannelProtocolSupport(info.ChannelType, info.ChannelBaseUrl, info.ChannelSetting)
}

// InboundProtocolForRelay maps an inbound RelayFormat and RelayMode to one of
// the 4 conversational protocols governed by ChannelProtocolSupport.
func InboundProtocolForRelay(relayFormat types.RelayFormat, relayMode int) (ChannelProtocol, bool) {
	switch relayFormat {
	case types.RelayFormatOpenAI:
		if relayMode == relayconstant.RelayModeChatCompletions || relayMode == relayconstant.RelayModeUnknown {
			return ProtocolChat, true
		}
		return "", false
	case types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
		return ProtocolResponses, true
	case types.RelayFormatClaude:
		return ProtocolMessages, true
	case types.RelayFormatGemini:
		if relayMode == relayconstant.RelayModeGemini || relayMode == relayconstant.RelayModeUnknown {
			return ProtocolGemini, true
		}
		return "", false
	default:
		return "", false
	}
}

// ShouldInterceptInboundProtocol reports whether an inbound request must be
// rejected locally at the gateway: the inbound protocol is disabled on the
// channel and the request body would be passed through as-is, no protocol is
// enabled at all, or it targets a non-convertible auxiliary endpoint
// (/v1/responses/compact, /v1/responses/input_tokens, /v1/messages/count_tokens).
func ShouldInterceptInboundProtocol(
	channelType int,
	baseURL string,
	setting dto.ChannelSettings,
	passThrough bool,
	relayFormat types.RelayFormat,
	relayMode int,
) bool {
	protocol, ok := InboundProtocolForRelay(relayFormat, relayMode)
	if !ok {
		return false
	}
	support := ResolveChannelProtocolSupport(channelType, baseURL, setting)
	if support.IsEnabled(protocol) {
		return false
	}
	if !support.HasAnyEnabled() || passThrough {
		return true
	}
	switch relayMode {
	case relayconstant.RelayModeResponsesCompact, relayconstant.RelayModeResponsesInputTokens:
		return protocol == ProtocolResponses
	case relayconstant.RelayModeClaudeCountTokens:
		return protocol == ProtocolMessages
	default:
		return false
	}
}
