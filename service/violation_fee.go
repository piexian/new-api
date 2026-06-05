package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

const (
	ViolationFeeCodePrefix           = "violation_fee."
	CSAMViolationMarker              = "Failed check: SAFETY_CHECK_TYPE"
	ContentViolatesUsageMarker       = "Content violates usage guidelines"
	XAIImageModerationMarker         = "Generated image rejected by content moderation"
	usageGuidelinesMarker            = "usage guidelines"
	usageGuidelineMarker             = "usage guideline"
	violationMarker                  = "violation"
	XAICostInUSDTicksPerUSD    int64 = 10_000_000_000
)

func IsViolationFeeCode(code types.ErrorCode) bool {
	return strings.HasPrefix(string(code), ViolationFeeCodePrefix)
}

func IsGrokViolationFeeContext(relayInfo *relaycommon.RelayInfo) bool {
	if relayInfo == nil {
		return false
	}
	channelType := 0
	if relayInfo.ChannelMeta != nil && relayInfo.ChannelMeta.ChannelType != 0 {
		channelType = relayInfo.ChannelMeta.ChannelType
	}
	if channelType == constant.ChannelTypeXai {
		return true
	}
	return common.IsGrokModel(relayInfo.OriginModelName) ||
		common.IsGrokModel(relayInfo.UpstreamModelName) ||
		(relayInfo.ChannelMeta != nil && common.IsGrokModel(relayInfo.ChannelMeta.UpstreamModelName))
}

func IsGrokViolationFeeContextFromFields(channelType int, modelNames ...string) bool {
	if channelType == constant.ChannelTypeXai {
		return true
	}
	for _, modelName := range modelNames {
		if common.IsGrokModel(modelName) {
			return true
		}
	}
	return false
}

func HasViolationFeeMarkerText(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	if strings.Contains(text, strings.ToLower(CSAMViolationMarker)) ||
		strings.Contains(text, strings.ToLower(ContentViolatesUsageMarker)) ||
		strings.Contains(text, strings.ToLower(XAIImageModerationMarker)) {
		return true
	}
	if strings.Contains(text, violationMarker) &&
		(strings.Contains(text, usageGuidelinesMarker) || strings.Contains(text, usageGuidelineMarker)) {
		return true
	}
	if strings.Contains(text, "violates") &&
		(strings.Contains(text, usageGuidelinesMarker) || strings.Contains(text, usageGuidelineMarker)) {
		return true
	}
	return false
}

func HasCSAMViolationMarker(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(CSAMViolationMarker)) {
		return true
	}
	oai := err.ToOpenAIError()
	return strings.Contains(strings.ToLower(oai.Message), strings.ToLower(CSAMViolationMarker)) ||
		strings.Contains(strings.ToLower(oai.Type), strings.ToLower(CSAMViolationMarker)) ||
		strings.Contains(strings.ToLower(fmt.Sprintf("%v", oai.Code)), strings.ToLower(CSAMViolationMarker)) ||
		strings.Contains(strings.ToLower(string(oai.Metadata)), strings.ToLower(CSAMViolationMarker)) ||
		strings.Contains(strings.ToLower(string(err.Metadata)), strings.ToLower(CSAMViolationMarker))
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

func NormalizeViolationFeeErrorForRelay(relayInfo *relaycommon.RelayInfo, err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		return nil
	}
	if !IsGrokViolationFeeContext(relayInfo) {
		return err
	}
	return NormalizeViolationFeeError(err)
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

	if HasXAIImageModerationMarker(err) {
		return WrapAsViolationFeeGrokModeration(err)
	}
	if HasCSAMViolationMarker(err) {
		return WrapAsViolationFeeGrokCSAM(err)
	}

	if IsViolationFeeCode(err.GetErrorCode()) {
		oai := err.ToOpenAIError()
		return types.WithOpenAIError(oai, err.StatusCode, types.ErrOptionWithSkipRetry(), types.ErrOptionWithMetadata(err.Metadata))
	}

	return err
}

func shouldChargeViolationFee(relayInfo *relaycommon.RelayInfo, err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if !IsGrokViolationFeeContext(relayInfo) {
		return false
	}
	if IsViolationFeeCode(err.GetErrorCode()) {
		return true
	}
	// In case some callers didn't normalize, keep a safety net.
	return HasXAIImageModerationMarker(err) || HasCSAMViolationMarker(err)
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
	if !shouldChargeViolationFee(relayInfo, apiErr) {
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

func ShouldChargeTaskViolationFee(channelType int, task *model.Task, reason string) bool {
	if task == nil || !HasViolationFeeMarkerText(reason) {
		return false
	}
	return IsGrokViolationFeeContextFromFields(
		channelType,
		taskModelName(task),
		task.Properties.OriginModelName,
		task.Properties.UpstreamModelName,
	)
}

func taskViolationGroupRatio(task *model.Task) float64 {
	if task == nil {
		return 0
	}
	if bc := task.PrivateData.BillingContext; bc != nil && bc.GroupRatio > 0 {
		return bc.GroupRatio
	}
	if task.Group != "" {
		return ratio_setting.GetGroupRatio(task.Group)
	}
	return 1
}

func ChargeTaskViolationFeeIfNeeded(ctx context.Context, task *model.Task, channelType int, statusCode int, reason string) bool {
	if task == nil || !ShouldChargeTaskViolationFee(channelType, task, reason) {
		return false
	}

	settings := model_setting.GetGrokSettings()
	if settings == nil || !settings.ViolationDeductionEnabled {
		return false
	}

	groupRatio := taskViolationGroupRatio(task)
	feeQuota := calcViolationFeeQuota(settings.ViolationDeductionAmount, groupRatio)
	if feeQuota <= 0 {
		return false
	}

	if err := taskAdjustFunding(task, feeQuota); err != nil {
		logger.LogError(ctx, fmt.Sprintf("failed to charge task violation fee: %s", err.Error()))
		return false
	}

	taskAdjustTokenQuota(ctx, task, feeQuota)
	model.UpdateUserUsedQuotaAndRequestCount(task.UserId, feeQuota)
	model.UpdateChannelUsedQuota(task.ChannelId, feeQuota)

	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["violation_fee"] = true
	other["violation_fee_code"] = string(types.ErrorCodeViolationFeeGrokCSAM)
	other["fee_quota"] = feeQuota
	other["base_amount"] = settings.ViolationDeductionAmount
	other["group_ratio"] = groupRatio
	other["status_code"] = statusCode
	other["channel_type"] = channelType
	other["violation_fee_marker"] = CSAMViolationMarker
	if reason != "" {
		other["reason"] = reason
	}

	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeConsume,
		Content:   "Violation fee charged",
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     feeQuota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
	return true
}
