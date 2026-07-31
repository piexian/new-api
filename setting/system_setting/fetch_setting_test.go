package system_setting

import (
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

func TestValidateFetchSettingOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{
			name:  "boolean",
			key:   "fetch_setting.enable_ssrf_protection",
			value: "true",
		},
		{
			name:    "invalid boolean",
			key:     "fetch_setting.allow_private_ip",
			value:   "enabled",
			wantErr: true,
		},
		{
			name:  "domain string list",
			key:   "fetch_setting.domain_list",
			value: `["example.com","*.example.org"]`,
		},
		{
			name:    "domain list rejects non-string entries",
			key:     "fetch_setting.domain_list",
			value:   `["example.com",1]`,
			wantErr: true,
		},
		{
			name:  "ports and ranges",
			key:   "fetch_setting.allowed_ports",
			value: `["80","443","8000-8002"]`,
		},
		{
			name:    "numeric port array is incompatible",
			key:     "fetch_setting.allowed_ports",
			value:   `[80,443]`,
			wantErr: true,
		},
		{
			name:    "invalid port range",
			key:     "fetch_setting.allowed_ports",
			value:   `["9000-8000"]`,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateFetchSettingOption(test.key, test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateFetchSettingOption() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestFetchSettingAllowedPortsRuntimeContract(t *testing.T) {
	t.Parallel()

	const value = `["80","443","8000-8002"]`
	if err := ValidateFetchSettingOption("fetch_setting.allowed_ports", value); err != nil {
		t.Fatalf("ValidateFetchSettingOption() error = %v", err)
	}

	settings := FetchSetting{}
	if err := config.UpdateConfigFromMap(&settings, map[string]string{"allowed_ports": value}); err != nil {
		t.Fatalf("UpdateConfigFromMap() error = %v", err)
	}
	want := []string{"80", "443", "8000-8002"}
	if !reflect.DeepEqual(settings.AllowedPorts, want) {
		t.Fatalf("AllowedPorts = %#v, want %#v", settings.AllowedPorts, want)
	}

	protection, err := common.NewSSRFProtectionFromFetchSetting(
		true,
		false,
		false,
		nil,
		nil,
		settings.AllowedPorts,
		false,
	)
	if err != nil {
		t.Fatalf("NewSSRFProtectionFromFetchSetting() error = %v", err)
	}
	if err := protection.ValidateNetworkTarget("8.8.8.8", 8001); err != nil {
		t.Fatalf("ValidateNetworkTarget() rejected configured range port: %v", err)
	}
	if err := protection.ValidateNetworkTarget("8.8.8.8", 9000); err == nil {
		t.Fatal("ValidateNetworkTarget() accepted a port outside the configured range")
	}
}
