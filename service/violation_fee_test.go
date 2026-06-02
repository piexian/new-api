package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

const xAIImageModerationErrorWithUsage = `{"code":"Client specified an invalid argument","error":"Generated image rejected by content moderation.","usage":{"cost_in_usd_ticks":2100000000}}`

func TestRelayErrorHandlerPreservesXAIUsageCostMetadata(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(xAIImageModerationErrorWithUsage)),
	}

	apiErr := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeBadResponseStatusCode, apiErr.GetErrorCode())
	require.Equal(t, xAIImageModerationErrorWithUsage, string(apiErr.Metadata))

	normalized := NormalizeViolationFeeError(apiErr)
	require.Equal(t, types.ErrorCodeViolationFeeGrokModeration, normalized.GetErrorCode())
	require.True(t, types.IsSkipRetryError(normalized))
	require.Equal(t, xAIImageModerationErrorWithUsage, string(normalized.Metadata))

	costInUSDTicks, ok := xAIUsageCostInUSDTicks(normalized)
	require.True(t, ok)
	require.Equal(t, "2100000000", costInUSDTicks.String())
}

func TestChargeViolationFeeUsesXAIUsageCostTicks(t *testing.T) {
	resetViolationFeeTestTables(t)
	restoreGrokSettings := setViolationFeeSettings(t, true, 0.05)
	defer restoreGrokSettings()

	initialQuota := 1_000_000
	groupRatio := 2.0
	expectedQuota := quotaFromUSDTicksForTest(2_100_000_000, groupRatio)
	apiErr := NormalizeViolationFeeError(types.NewOpenAIError(
		errors.New("Generated image rejected by content moderation."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
		types.ErrOptionWithMetadata([]byte(xAIImageModerationErrorWithUsage)),
	))

	ctx, relayInfo := setupViolationFeeChargeTest(t, initialQuota, groupRatio)

	require.True(t, ChargeViolationFeeIfNeeded(ctx, relayInfo, apiErr))
	assertViolationFeeCharge(t, initialQuota, expectedQuota, "upstream_usage_cost", "0.21")
}

func TestChargeViolationFeeFallsBackToConfiguredAmountWithoutUsageCost(t *testing.T) {
	resetViolationFeeTestTables(t)
	restoreGrokSettings := setViolationFeeSettings(t, true, 0.05)
	defer restoreGrokSettings()

	initialQuota := 1_000_000
	groupRatio := 2.0
	expectedQuota := calcViolationFeeQuota(0.05, groupRatio)
	apiErr := NormalizeViolationFeeError(types.NewOpenAIError(
		errors.New("Generated image rejected by content moderation."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	))

	ctx, relayInfo := setupViolationFeeChargeTest(t, initialQuota, groupRatio)

	require.True(t, ChargeViolationFeeIfNeeded(ctx, relayInfo, apiErr))
	assertViolationFeeCharge(t, initialQuota, expectedQuota, "configured_amount", "")
}

func resetViolationFeeTestTables(t *testing.T) {
	t.Helper()
	for _, table := range []string{"logs", "tokens", "users", "channels"} {
		require.NoError(t, model.DB.Exec("DELETE FROM "+table).Error)
	}
	t.Cleanup(func() {
		for _, table := range []string{"logs", "tokens", "users", "channels"} {
			_ = model.DB.Exec("DELETE FROM " + table).Error
		}
	})
}

func setViolationFeeSettings(t *testing.T, enabled bool, amount float64) func() {
	t.Helper()
	settings := model_setting.GetGrokSettings()
	original := *settings
	settings.ViolationDeductionEnabled = enabled
	settings.ViolationDeductionAmount = amount
	return func() {
		*settings = original
	}
}

func setupViolationFeeChargeTest(t *testing.T, initialQuota int, groupRatio float64) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()

	seedUser(t, 1, initialQuota)
	seedToken(t, 1, 1, "test-token", initialQuota)
	seedChannel(t, 1)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	ctx.Set("username", "test_user")
	ctx.Set("token_name", "test_token")

	relayInfo := &relaycommon.RelayInfo{
		UserId:          1,
		TokenId:         1,
		TokenKey:        "test-token",
		OriginModelName: "grok-2-image",
		UsingGroup:      "default",
		StartTime:       time.Now(),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 1},
		PriceData: types.PriceData{
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: groupRatio},
		},
	}
	return ctx, relayInfo
}

func quotaFromUSDTicksForTest(costInUSDTicks int64, groupRatio float64) int {
	quota := decimal.NewFromInt(costInUSDTicks).
		Div(decimal.NewFromInt(XAICostInUSDTicksPerUSD)).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(decimal.NewFromFloat(groupRatio)).
		Round(0).
		IntPart()
	return int(quota)
}

func assertViolationFeeCharge(t *testing.T, initialQuota int, expectedQuota int, expectedFeeSource string, expectedUpstreamCostUSD string) {
	t.Helper()

	var user model.User
	require.NoError(t, model.DB.First(&user, 1).Error)
	require.Equal(t, initialQuota-expectedQuota, user.Quota)
	require.Equal(t, expectedQuota, user.UsedQuota)
	require.Equal(t, 1, user.RequestCount)

	var token model.Token
	require.NoError(t, model.DB.First(&token, 1).Error)
	require.Equal(t, initialQuota-expectedQuota, token.RemainQuota)
	require.Equal(t, expectedQuota, token.UsedQuota)

	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, 1).Error)
	require.Equal(t, int64(expectedQuota), channel.UsedQuota)

	var log model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeConsume).First(&log).Error)
	require.Equal(t, expectedQuota, log.Quota)
	require.Equal(t, "Violation fee charged", log.Content)

	var other map[string]any
	require.NoError(t, common.Unmarshal([]byte(log.Other), &other))
	require.Equal(t, true, other["violation_fee"])
	require.Equal(t, string(types.ErrorCodeViolationFeeGrokModeration), other["violation_fee_code"])
	require.Equal(t, expectedFeeSource, other["fee_source"])
	require.InDelta(t, float64(expectedQuota), other["fee_quota"], 0)
	if expectedUpstreamCostUSD != "" {
		require.Equal(t, "2100000000", other["upstream_cost_in_usd_ticks"])
		require.Equal(t, expectedUpstreamCostUSD, other["upstream_cost_usd"])
	} else {
		require.NotContains(t, other, "upstream_cost_in_usd_ticks")
		require.NotContains(t, other, "upstream_cost_usd")
	}
}
