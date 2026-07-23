package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/stretchr/testify/require"
)

func validErrorBanTestSetting() risk_setting.ErrorBanSetting {
	return risk_setting.ErrorBanSetting{
		WindowSeconds:    300,
		DefaultDimension: risk_setting.DimensionIP,
		Rules: []risk_setting.ErrorBanRule{{
			Id:         "quota_error",
			Name:       "Quota error",
			Pattern:    "quota",
			Keywords:   []string{"exceeded"},
			ErrorCodes: []string{"insufficient_quota"},
			Enabled:    true,
			Threshold:  3,
			Tiers: []risk_setting.ErrorBanTier{{
				OffenseCount:    1,
				Action:          risk_setting.TierActionTempIPBan,
				DurationMinutes: 30,
			}},
		}},
	}
}

func TestValidateErrorBanSettingAcceptsCombinedMatchers(t *testing.T) {
	setting := validErrorBanTestSetting()
	require.NoError(t, validateErrorBanSetting(&setting))
}

func TestValidateErrorBanSettingAcceptsWildcardOnlyMatcher(t *testing.T) {
	setting := validErrorBanTestSetting()
	setting.Rules[0].Pattern = ""
	setting.Rules[0].Keywords = nil
	setting.Rules[0].ErrorCodes = []string{"*"}
	require.NoError(t, validateErrorBanSetting(&setting))
}

func TestValidateErrorBanSettingRejectsEnabledRuleWithoutMatcher(t *testing.T) {
	setting := validErrorBanTestSetting()
	setting.Rules[0].Pattern = ""
	setting.Rules[0].Keywords = nil
	setting.Rules[0].ErrorCodes = nil
	require.ErrorContains(t, validateErrorBanSetting(&setting), "至少需要")
}

func TestValidateErrorBanSettingRejectsInvalidPerRuleTier(t *testing.T) {
	setting := validErrorBanTestSetting()
	setting.Rules[0].Tiers[0].DurationMinutes = 0
	require.ErrorContains(t, validateErrorBanSetting(&setting), "必须大于 0")
}

func TestValidateErrorBanSettingRejectsTrimmedDuplicateRuleIDs(t *testing.T) {
	setting := validErrorBanTestSetting()
	duplicate := setting.Rules[0]
	duplicate.Id = " quota_error "
	setting.Rules = append(setting.Rules, duplicate)
	require.ErrorContains(t, validateErrorBanSetting(&setting), "重复")
}
