package zhipu_4v

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// ZCode 请求体指纹补全与过滤。
//
// 背景：GLM Coding Plan 按 ZCode 客户端指纹发放权益，而下游客户端可能自行
// 做过协议转换（OpenAI→Claude 等），带进来非官方产物。凡 ZCode 模式渠道
// 发往上游的 /v1/messages 请求体，统一在此补齐官方字段并剥离非官方字段，
// 保证落在上游的一定是官方形态：
//
//  1. metadata 整体替换为官方 user_id（{"device_id":...,"account_uuid":"",
//     "session_id":...} 的 JSON 字符串），丢弃客户端自带 metadata；
//  2. cache_control 断点补齐：末尾 system 块 + 末尾消息块（ephemeral），
//     字符串形态先转块数组；
//  3. 剥离官方不发送的顶层字段（speed/inference_geo/service_tier/
//     context_management/container/output_format/output_config/prompt/
//     max_tokens_to_sample）。

var zcodeEphemeralCacheControl = json.RawMessage(`{"type":"ephemeral"}`)

// applyZCodeBodyFingerprint 对 ZCode 模式渠道的 /v1/messages 请求体做
// 官方化补全与过滤；非 ZCode 模式或 nil 输入直接放行。
func applyZCodeBodyFingerprint(info *relaycommon.RelayInfo, req *dto.ClaudeRequest) {
	if info == nil || req == nil || !isZhipuZcodeMode(info) {
		return
	}
	replaceZCodeMetadata(info, req)
	applyZCodeCacheBreakpoints(req)
	stripNonZCodeFields(req)
}

// replaceZCodeMetadata 以官方 user_id 覆盖整个 metadata：官方恒为
// {"device_id":"<持久设备UUID>","account_uuid":"","session_id":"<会话ID>"}
// 序列化后的 JSON 字符串。
func replaceZCodeMetadata(info *relaycommon.RelayInfo, req *dto.ClaudeRequest) {
	userID, err := common.Marshal(map[string]string{
		"device_id":    zcodeDeviceMid(),
		"account_uuid": "",
		"session_id":   zcodeSessionId(info),
	})
	if err != nil {
		return
	}
	metadata, err := common.Marshal(map[string]string{"user_id": string(userID)})
	if err != nil {
		return
	}
	req.Metadata = metadata
}

// applyZCodeCacheBreakpoints 在末尾 system 块与末尾消息块补 ephemeral 断点
// （官方 ContextBuilder 行为）；字符串 content 转块数组以承载断点。
func applyZCodeCacheBreakpoints(req *dto.ClaudeRequest) {
	switch sys := req.System.(type) {
	case string:
		if sys != "" {
			text := sys
			req.System = []dto.ClaudeMediaMessage{{
				Type:         dto.ContentTypeText,
				Text:         &text,
				CacheControl: zcodeEphemeralCacheControl,
			}}
		}
	case []dto.ClaudeMediaMessage:
		if len(sys) > 0 {
			sys[len(sys)-1].CacheControl = zcodeEphemeralCacheControl
		}
	}

	if len(req.Messages) == 0 {
		return
	}
	last := &req.Messages[len(req.Messages)-1]
	switch content := last.Content.(type) {
	case string:
		if content != "" {
			text := content
			last.Content = []dto.ClaudeMediaMessage{{
				Type:         dto.ContentTypeText,
				Text:         &text,
				CacheControl: zcodeEphemeralCacheControl,
			}}
		}
	case []dto.ClaudeMediaMessage:
		if len(content) > 0 {
			content[len(content)-1].CacheControl = zcodeEphemeralCacheControl
		}
	}
}

// stripNonZCodeFields 清空官方客户端不发送的顶层字段。
func stripNonZCodeFields(req *dto.ClaudeRequest) {
	req.Prompt = ""
	req.MaxTokensToSample = nil
	req.Speed = nil
	req.InferenceGeo = ""
	req.ServiceTier = ""
	req.ContextManagement = nil
	req.Container = nil
	req.OutputFormat = nil
	req.OutputConfig = nil
}
