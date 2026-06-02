package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

const (
	ViolationFeeCodePrefix           = "violation_fee."
	CSAMViolationMarker              = "Failed check: SAFETY_CHECK_TYPE"
	ContentViolatesUsageMarker       = "Content violates usage guidelines"
	XAIImageModerationMarker         = "Generated image rejected by content moderation"
	XAICostInUSDTicksPerUSD    int64 = 10_000_000_000
)

func IsViolationFeeCode(code types.ErrorCode) bool {
	return strings.HasPrefix(string(code), ViolationFeeCodePrefix)
}

func HasCSAMViolationMarker(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), CSAMViolationMarker) || strings.Contains(err.Error(), ContentViolatesUsageMarker) {
		return true
	}
	msg := err.ToOpenAIError().Message
	return strings.Contains(msg, CSAMViolationMarker) || strings.Contains(msg, ContentViolatesUsageMarker)
}

func HasXAIImageModerationMarker(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if hasXAIImageModerationMarkerText(err.Error()) || hasXAIImageModerationMarkerText(err.ToOpenAIError().Message) {
		return true
	}
	return hasXAIImageModerationMarkerText(xAIErrorMessageFromMetadata(err.Metadata))
}

func hasXAIImageModerationMarkerText(value string) bool {
	normalized := strings.ToLower(value)
	return strings.Contains(normalized, strings.ToLower(XAIImageModerationMarker))
}

func WrapAsViolationFeeGrokCSAM(err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		return nil
	}
	oai := err.ToOpenAIError()
	oai.Type = string(types.ErrorCodeViolationFeeGrokCSAM)
	oai.Code = string(types.ErrorCodeViolationFeeGrokCSAM)
	return types.WithOpenAIError(oai, err.StatusCode, types.ErrOptionWithSkipRetry(), types.ErrOptionWithMetadata(err.Metadata))
}

func WrapAsViolationFeeGrokModeration(err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		return nil
	}
	oai := err.ToOpenAIError()
	oai.Type = string(types.ErrorCodeViolationFeeGrokModeration)
	oai.Code = string(types.ErrorCodeViolationFeeGrokModeration)
	return types.WithOpenAIError(oai, err.StatusCode, types.ErrOptionWithSkipRetry(), types.ErrOptionWithMetadata(err.Metadata))
}

// NormalizeViolationFeeError ensures:
// - if the CSAM marker is present, error.code is set to a stable violation-fee code and skip-retry is enabled.
// - if error.code already has the violation-fee prefix, skip-retry is enabled.
//
// It must be called before retry decision logic.
func NormalizeViolationFeeError(err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		return nil
	}

	if HasCSAMViolationMarker(err) {
		return WrapAsViolationFeeGrokCSAM(err)
	}
	if HasXAIImageModerationMarker(err) {
		return WrapAsViolationFeeGrokModeration(err)
	}

	if IsViolationFeeCode(err.GetErrorCode()) {
		oai := err.ToOpenAIError()
		return types.WithOpenAIError(oai, err.StatusCode, types.ErrOptionWithSkipRetry(), types.ErrOptionWithMetadata(err.Metadata))
	}

	return err
}

func shouldChargeViolationFee(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if IsViolationFeeCode(err.GetErrorCode()) {
		return true
	}
	// In case some callers didn't normalize, keep a safety net.
	return HasCSAMViolationMarker(err) || HasXAIImageModerationMarker(err)
}

func calcViolationFeeQuota(amount, groupRatio float64) int {
	return calcViolationFeeQuotaDecimal(decimal.NewFromFloat(amount), groupRatio)
}

func calcViolationFeeQuotaDecimal(amount decimal.Decimal, groupRatio float64) int {
	if amount.LessThanOrEqual(decimal.Zero) {
		return 0
	}
	if groupRatio <= 0 {
		return 0
	}
	quota := amount.
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(decimal.NewFromFloat(groupRatio)).
		Round(0).
		IntPart()
	if quota <= 0 {
		return 0
	}
	return int(quota)
}

func xAIUsageCostInUSDTicks(err *types.NewAPIError) (decimal.Decimal, bool) {
	if err == nil || len(err.Metadata) == 0 {
		return decimal.Zero, false
	}
	var payload struct {
		Usage struct {
			CostInUSDTicks json.RawMessage `json:"cost_in_usd_ticks"`
		} `json:"usage"`
	}
	if unmarshalErr := common.Unmarshal(err.Metadata, &payload); unmarshalErr != nil {
		return decimal.Zero, false
	}
	return positiveDecimalFromRaw(payload.Usage.CostInUSDTicks)
}

func positiveDecimalFromRaw(raw json.RawMessage) (decimal.Decimal, bool) {
	value := strings.TrimSpace(common.JsonRawMessageToString(raw))
	if value == "" || value == "null" {
		return decimal.Zero, false
	}
	amount, err := decimal.NewFromString(value)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, false
	}
	return amount, true
}

func xAIErrorMessageFromMetadata(metadata json.RawMessage) string {
	if len(metadata) == 0 {
		return ""
	}
	var payload struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if err := common.Unmarshal(metadata, &payload); err != nil {
		return ""
	}
	if payload.Message != "" {
		return payload.Message
	}
	return common.JsonRawMessageToString(payload.Error)
}

func violationFeeCodeForError(err *types.NewAPIError) types.ErrorCode {
	if err == nil {
		return types.ErrorCodeViolationFeeGrokCSAM
	}
	if code := err.GetErrorCode(); IsViolationFeeCode(code) {
		return code
	}
	if HasXAIImageModerationMarker(err) {
		return types.ErrorCodeViolationFeeGrokModeration
	}
	return types.ErrorCodeViolationFeeGrokCSAM
}

func violationFeeMarkerForError(err *types.NewAPIError) string {
	if HasXAIImageModerationMarker(err) {
		return XAIImageModerationMarker
	}
	return CSAMViolationMarker
}

// ChargeViolationFeeIfNeeded charges an additional fee after the normal flow finishes (including refund).
// It uses Grok fee settings as the fee policy.
func ChargeViolationFeeIfNeeded(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, apiErr *types.NewAPIError) bool {
	if ctx == nil || relayInfo == nil || apiErr == nil {
		return false
	}
	//if relayInfo.IsPlayground {
	//	return false
	//}
	if !shouldChargeViolationFee(apiErr) {
		return false
	}

	settings := model_setting.GetGrokSettings()
	if settings == nil || !settings.ViolationDeductionEnabled {
		return false
	}

	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	feeAmount := decimal.NewFromFloat(settings.ViolationDeductionAmount)
	feeSource := "configured_amount"
	upstreamCostInUSDTicks := decimal.Zero
	upstreamCostInUSD := decimal.Zero
	if costInUSDTicks, ok := xAIUsageCostInUSDTicks(apiErr); ok {
		upstreamCostInUSDTicks = costInUSDTicks
		upstreamCostInUSD = costInUSDTicks.Div(decimal.NewFromInt(XAICostInUSDTicksPerUSD))
		feeAmount = upstreamCostInUSD
		feeSource = "upstream_usage_cost"
	}

	feeQuota := calcViolationFeeQuotaDecimal(feeAmount, groupRatio)
	if feeQuota <= 0 {
		return false
	}

	if err := PostConsumeQuota(relayInfo, feeQuota, 0, true); err != nil {
		logger.LogError(ctx, fmt.Sprintf("failed to charge violation fee: %s", err.Error()))
		return false
	}

	model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, feeQuota)
	model.UpdateChannelUsedQuota(relayInfo.ChannelId, feeQuota)

	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	tokenName := ctx.GetString("token_name")
	oai := apiErr.ToOpenAIError()
	violationFeeCode := violationFeeCodeForError(apiErr)

	other := map[string]any{
		"violation_fee":        true,
		"violation_fee_code":   string(violationFeeCode),
		"fee_quota":            feeQuota,
		"base_amount":          feeAmount.InexactFloat64(),
		"fee_amount":           feeAmount.String(),
		"fee_source":           feeSource,
		"group_ratio":          groupRatio,
		"status_code":          apiErr.StatusCode,
		"upstream_error_type":  oai.Type,
		"upstream_error_code":  fmt.Sprintf("%v", oai.Code),
		"violation_fee_marker": violationFeeMarkerForError(apiErr),
	}
	if !upstreamCostInUSDTicks.IsZero() {
		other["upstream_cost_in_usd_ticks"] = upstreamCostInUSDTicks.String()
		other["upstream_cost_usd"] = upstreamCostInUSD.String()
		other["configured_amount"] = settings.ViolationDeductionAmount
	}

	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:      relayInfo.ChannelId,
		ModelName:      relayInfo.OriginModelName,
		TokenName:      tokenName,
		Quota:          feeQuota,
		Content:        "Violation fee charged",
		TokenId:        relayInfo.TokenId,
		UseTimeSeconds: int(useTimeSeconds),
		IsStream:       relayInfo.IsStream,
		Group:          relayInfo.UsingGroup,
		Other:          other,
	})

	return true
}
