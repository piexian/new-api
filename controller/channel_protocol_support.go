package controller

// 渠道协议支持：默认值与锁定规则只在后端维护，前端读取这里给出的结果。

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	dto "github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
)

// parseChannelSettingsForProtocol 只读解析 setting；不用 Channel.GetSetting，
// 它在解析失败时会回写数据库，而这里的渠道可能是省略了 key 的部分加载结果。
func parseChannelSettingsForProtocol(raw *string) dto.ChannelSettings {
	setting := dto.ChannelSettings{}
	if raw != nil && strings.TrimSpace(*raw) != "" {
		_ = common.UnmarshalJsonStr(*raw, &setting)
	}
	return setting
}

func attachChannelProtocolSupportInfo(channel *model.Channel) {
	setting := parseChannelSettingsForProtocol(channel.Setting)
	info := relaycommon.BuildChannelProtocolSupportInfo(channel.Type, channel.GetBaseURL(), setting)
	channel.ProtocolSupportInfo = &info
}

// normalizeChannelProtocolSupportSetting 把前端提交的目标协议状态整理成存储用的覆盖值，
// 只改 protocol_support 与 upstream_openai_compat_enabled，其余 setting 字段原样保留。
func normalizeChannelProtocolSupportSetting(channel *model.Channel, channelType int, baseURL string) error {
	if channel == nil || channel.Setting == nil || strings.TrimSpace(*channel.Setting) == "" {
		return nil
	}
	var raw map[string]any
	if err := common.UnmarshalJsonStr(*channel.Setting, &raw); err != nil {
		return nil
	}
	if _, ok := raw["protocol_support"]; !ok {
		return nil
	}
	setting := parseChannelSettingsForProtocol(channel.Setting)
	normalized := relaycommon.NormalizeProtocolSupportOverrides(channelType, baseURL, setting)
	if normalized.ProtocolSupport == nil {
		delete(raw, "protocol_support")
	} else {
		raw["protocol_support"] = normalized.ProtocolSupport
	}
	if normalized.UpstreamOpenAICompatEnabled != setting.UpstreamOpenAICompatEnabled {
		if normalized.UpstreamOpenAICompatEnabled {
			raw["upstream_openai_compat_enabled"] = true
		} else {
			delete(raw, "upstream_openai_compat_enabled")
		}
	}
	encoded, err := common.Marshal(raw)
	if err != nil {
		return err
	}
	value := string(encoded)
	channel.Setting = &value
	return nil
}

type resolveChannelProtocolSupportRequest struct {
	Type    int                 `json:"type"`
	BaseURL string              `json:"base_url"`
	Setting dto.ChannelSettings `json:"setting"`
}

// ResolveChannelProtocolSupport 供编辑表单在切换类型或地址时获取默认值与锁定状态。
func ResolveChannelProtocolSupport(c *gin.Context) {
	var request resolveChannelProtocolSupportRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	info := relaycommon.BuildChannelProtocolSupportInfo(request.Type, request.BaseURL, request.Setting)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": info})
}
