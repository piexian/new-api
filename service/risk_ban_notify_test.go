package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestRiskBanInfoUserNotificationOmitsInternalReportDetails(t *testing.T) {
	info := RiskBanInfo{
		Source:          "secret-source",
		Dimension:       model.RiskBanDimensionUser,
		TriggerIP:       "203.0.113.42",
		UserId:          42,
		Username:        "secret-user",
		DisplayName:     "Secret User",
		Reason:          "secret-reason",
		DurationMinutes: 90,
		BannedAt:        1785650400,
		UnbanAt:         1785655800,
		OffenseCount:    3,
		TierLevel:       2,
		TierAction:      "secret-action",
		RuleId:          "secret-rule-id",
		RuleName:        "secret-rule-name",
		ErrorSample:     "secret-error-sample",
		TriggeredModels: "secret-model",
		AppealHint:      "Contact support for review.",
	}

	for _, language := range []string{i18n.LangEn, i18n.LangZhCN, i18n.LangZhTW} {
		t.Run(language, func(t *testing.T) {
			subject, content := info.userNotification(language)
			combined := subject + "\n" + content

			for _, internalValue := range []string{
				info.Source,
				info.TriggerIP,
				info.Username,
				info.DisplayName,
				info.Reason,
				info.TierAction,
				info.RuleId,
				info.RuleName,
				info.ErrorSample,
				info.TriggeredModels,
			} {
				assert.NotContains(t, combined, internalValue)
			}
			assert.Contains(t, content, "90")
			assert.Contains(t, content, formatRiskTime(info.UnbanAt))
			assert.Contains(t, content, info.AppealHint)
		})
	}
}

func TestRiskBanInfoUserEmailVariablesAreMinimal(t *testing.T) {
	info := RiskBanInfo{
		Source:          "error_ban",
		TriggerIP:       "203.0.113.42",
		UserId:          42,
		Username:        "example-user",
		DisplayName:     "Example User",
		Reason:          "raw risk reason",
		DurationMinutes: 90,
		UnbanAt:         1785655800,
		OffenseCount:    3,
		TierAction:      "disable_user",
		RuleId:          "invalid-key",
		RuleName:        "Invalid key",
		ErrorSample:     "raw upstream error",
		TriggeredModels: "secret-model",
		AppealHint:      "Contact support for review.",
	}

	variables := info.toUserEmailVariables(i18n.LangEn)
	assert.Equal(t, map[string]string{
		"ban_type":     "Temporary",
		"ban_duration": "90 minutes",
		"unban_at":     formatRiskTime(info.UnbanAt),
		"appeal_hint":  info.AppealHint,
	}, variables)
}

func TestRiskBanInfoAdminContentRetainsAuditDetails(t *testing.T) {
	info := RiskBanInfo{
		Source:          "error_ban",
		Dimension:       model.RiskBanDimensionIP,
		TriggerIP:       "203.0.113.42",
		Reason:          "risk reason",
		RuleId:          "invalid-key",
		RuleName:        "Invalid key",
		ErrorSample:     "raw upstream error",
		TriggeredModels: "secret-model",
	}

	content := info.adminContent()
	for _, detail := range []string{
		info.Source,
		info.TriggerIP,
		info.Reason,
		info.RuleId,
		info.RuleName,
		info.ErrorSample,
		info.TriggeredModels,
	} {
		assert.True(t, strings.Contains(content, detail), detail)
	}
}
