package system_setting

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type FetchSetting struct {
	EnableSSRFProtection   bool     `json:"enable_ssrf_protection"` // 是否启用SSRF防护
	AllowPrivateIp         bool     `json:"allow_private_ip"`
	DomainFilterMode       bool     `json:"domain_filter_mode"`         // 域名过滤模式，true: 白名单模式，false: 黑名单模式
	IpFilterMode           bool     `json:"ip_filter_mode"`             // IP过滤模式，true: 白名单模式，false: 黑名单模式
	DomainList             []string `json:"domain_list"`                // domain format, e.g. example.com, *.example.com
	IpList                 []string `json:"ip_list"`                    // CIDR format
	AllowedPorts           []string `json:"allowed_ports"`              // port range format, e.g. 80, 443, 8000-9000
	ApplyIPFilterForDomain bool     `json:"apply_ip_filter_for_domain"` // 对域名启用IP过滤（实验性）
}

var defaultFetchSetting = FetchSetting{
	EnableSSRFProtection:   true, // 默认开启SSRF防护
	AllowPrivateIp:         false,
	DomainFilterMode:       false,
	IpFilterMode:           false,
	DomainList:             []string{},
	IpList:                 []string{},
	AllowedPorts:           []string{"80", "443", "8080", "8443"},
	ApplyIPFilterForDomain: true,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("fetch_setting", &defaultFetchSetting)
}

func GetFetchSetting() *FetchSetting {
	return &defaultFetchSetting
}

// ValidateFetchSettingOption validates values before the generic option API
// persists them. The config manager otherwise skips incompatible JSON values,
// which can make an update appear successful without changing runtime policy.
func ValidateFetchSettingOption(key, value string) error {
	switch key {
	case "fetch_setting.enable_ssrf_protection",
		"fetch_setting.allow_private_ip",
		"fetch_setting.domain_filter_mode",
		"fetch_setting.ip_filter_mode",
		"fetch_setting.apply_ip_filter_for_domain":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("invalid boolean value for %s", key)
		}
	case "fetch_setting.domain_list", "fetch_setting.ip_list":
		var entries []string
		if err := common.Unmarshal([]byte(value), &entries); err != nil {
			return fmt.Errorf("%s must be a JSON string array", key)
		}
	case "fetch_setting.allowed_ports":
		var ports []string
		if err := common.Unmarshal([]byte(value), &ports); err != nil {
			return fmt.Errorf("%s must be a JSON string array", key)
		}
		if _, err := common.NewSSRFProtectionFromFetchSetting(false, false, false, nil, nil, ports, false); err != nil {
			return err
		}
	}
	return nil
}
