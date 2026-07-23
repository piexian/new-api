package risk_setting

import "testing"

func TestErrorBanSettingNormalizeInitializesSlices(t *testing.T) {
	setting := ErrorBanSetting{}

	setting.Normalize()

	if setting.ExcludeStatusCodes == nil {
		t.Fatal("ExcludeStatusCodes must serialize as an empty array, not null")
	}
	if setting.WhitelistGroups == nil {
		t.Fatal("WhitelistGroups must serialize as an empty array, not null")
	}
	if setting.Rules == nil {
		t.Fatal("Rules must serialize as an empty array, not null")
	}
	if setting.Tiers == nil {
		t.Fatal("Tiers must serialize as an array, not null")
	}
}

func TestRiskSettingNormalizesWhitelistGroups(t *testing.T) {
	probe := ProbeGuardSetting{WhitelistGroups: []string{" trusted ", "", "trusted", "premium"}}
	probe.Normalize()
	if len(probe.WhitelistGroups) != 2 || probe.WhitelistGroups[0] != "trusted" || probe.WhitelistGroups[1] != "premium" {
		t.Fatalf("unexpected probe whitelist groups: %#v", probe.WhitelistGroups)
	}
	if !probe.IsGroupWhitelisted(" trusted ") {
		t.Fatal("normalized probe group should be whitelisted")
	}

	errorBan := ErrorBanSetting{WhitelistGroups: []string{" premium ", "premium"}}
	errorBan.Normalize()
	if len(errorBan.WhitelistGroups) != 1 || !errorBan.IsGroupWhitelisted("premium") {
		t.Fatalf("unexpected error-ban whitelist groups: %#v", errorBan.WhitelistGroups)
	}
}

func TestErrorBanSettingNormalizesUserBanDurations(t *testing.T) {
	errorBan := ErrorBanSetting{Tiers: []ErrorBanTier{
		{OffenseCount: 1, Action: TierActionTempIPBan, DurationMinutes: 0},
		{OffenseCount: 2, Action: TierActionDisableUser, DurationMinutes: -1},
		{OffenseCount: 3, Action: TierActionBoth, DurationMinutes: 999999},
	}}
	errorBan.Normalize()
	if errorBan.Tiers[0].DurationMinutes != 1 {
		t.Fatalf("temporary IP ban must be at least one minute, got %d", errorBan.Tiers[0].DurationMinutes)
	}
	if errorBan.Tiers[1].DurationMinutes != 0 {
		t.Fatalf("negative user duration should become permanent, got %d", errorBan.Tiers[1].DurationMinutes)
	}
	if errorBan.Tiers[2].DurationMinutes != 525600 {
		t.Fatalf("user duration should be capped, got %d", errorBan.Tiers[2].DurationMinutes)
	}
}

func TestProbeGuardSettingMigratesLegacyBanDimension(t *testing.T) {
	legacyIP := ProbeGuardSetting{}
	legacyIP.Normalize()
	if legacyIP.BanDimension != DimensionIP || !legacyIP.BansIP() || legacyIP.BansUser() {
		t.Fatalf("legacy IP-only config migrated incorrectly: %#v", legacyIP)
	}

	legacyBoth := ProbeGuardSetting{UserBanEnabled: true}
	legacyBoth.Normalize()
	if legacyBoth.BanDimension != ProbeBanDimensionBoth || !legacyBoth.BansIP() || !legacyBoth.BansUser() {
		t.Fatalf("legacy user-ban config migrated incorrectly: %#v", legacyBoth)
	}

	userOnly := ProbeGuardSetting{BanDimension: DimensionUser}
	userOnly.Normalize()
	if userOnly.BansIP() || !userOnly.BansUser() || !userOnly.UserBanEnabled {
		t.Fatalf("explicit user-only config normalized incorrectly: %#v", userOnly)
	}
}
