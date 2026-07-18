package common

import "github.com/QuantumNous/new-api/setting/model_setting"

func IsRequestPassThroughEnabled(info *RelayInfo) bool {
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled {
		return true
	}
	return info != nil && info.ChannelMeta != nil && info.ChannelSetting.PassThroughBodyEnabled
}
