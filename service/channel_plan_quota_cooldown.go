package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	planQuotaCooldownTickInterval              = time.Minute
	planQuotaCooldownBatchSize                 = 500
	planQuotaCooldownAlreadyActiveDedupeSecond = int64(300)
)

var (
	planQuotaCooldownOnce    sync.Once
	planQuotaCooldownRunning atomic.Bool

	planQuotaResetDateTimePattern = `[0-9]{4}-[0-9]{2}-[0-9]{2}[T ][0-9]{2}:[0-9]{2}:[0-9]{2}(?:\s*[+-][0-9]{2}:?[0-9]{2}(?:\s*[A-Z]{2,5})?)?`
	planQuotaResetAtPattern       = regexp.MustCompile(`(?i)(?:resets? at|will reset at|quota will reset at|it will reset at)\s+(` + planQuotaResetDateTimePattern + `)`)
	planQuotaChineseResetPattern  = regexp.MustCompile(`(?:限额|限額|额度|額度)[^0-9]{0,16}(?:将在|將在)\s+(` + planQuotaResetDateTimePattern + `)\s*(?:重置|重設|恢复|恢復)?`)
	planQuotaDurationPattern      = regexp.MustCompile(`(?i)(?:reset after|resets in)\s+([0-9]+(?:h|m|s)(?:[0-9]+(?:h|m|s))*)`)
)

func ParsePlanQuotaResetUntil(message string, now time.Time) (int64, bool) {
	message = strings.TrimSpace(message)
	if message == "" {
		return 0, false
	}

	for _, pattern := range []*regexp.Regexp{planQuotaResetAtPattern, planQuotaChineseResetPattern} {
		if matches := pattern.FindStringSubmatch(message); len(matches) == 2 {
			if t, ok := parsePlanQuotaResetTime(matches[1], now.Location()); ok && t.After(now) {
				return t.Unix(), true
			}
		}
	}

	if matches := planQuotaDurationPattern.FindStringSubmatch(message); len(matches) == 2 {
		duration, err := time.ParseDuration(strings.TrimSpace(matches[1]))
		if err == nil && duration > 0 {
			return now.Add(duration).Unix(), true
		}
	}

	return 0, false
}

func parsePlanQuotaResetTime(value string, location *time.Location) (time.Time, bool) {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-0700",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		var (
			t   time.Time
			err error
		)
		if layout == "2006-01-02 15:04:05" {
			t, err = time.ParseInLocation(layout, value, location)
		} else {
			t, err = time.Parse(layout, value)
		}
		if err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func DisableChannelUntil(channelError types.ChannelError, reason string, until int64) {
	if until <= common.GetTimestamp() {
		return
	}
	disabledUntilText := time.Unix(until, 0).Format(time.RFC3339)
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）触发套餐限额冷却，禁用至 %s，原因：%s",
		channelError.ChannelName,
		channelError.ChannelId,
		disabledUntilText,
		reason,
	))

	success := model.UpdateChannelStatusUntil(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason, until)
	if success {
		recordPlanQuotaCooldownManageLog(channelError, reason, until, disabledUntilText, "entered", true, 0)
		subject := fmt.Sprintf("通道「%s」（#%d）已按套餐限额冷却", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已按套餐限额冷却至 %s，原因：%s",
			channelError.ChannelName,
			channelError.ChannelId,
			disabledUntilText,
			reason,
		)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
		return
	}
	if isPlanQuotaCooldownAlreadyActive(channelError, until) {
		recordPlanQuotaCooldownManageLog(channelError, reason, until, disabledUntilText, "already_active", false, planQuotaCooldownAlreadyActiveDedupeSecond)
	}
}

func DisableChannelModelUntil(channelError types.ChannelError, modelName string, reason string, until int64) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" || until <= common.GetTimestamp() {
		return
	}
	disabledUntilText := time.Unix(until, 0).Format(time.RFC3339)
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）模型「%s」触发套餐限额冷却，禁用至 %s，原因：%s",
		channelError.ChannelName,
		channelError.ChannelId,
		modelName,
		disabledUntilText,
		reason,
	))

	success := model.UpdateChannelModelStatusUntil(channelError.ChannelId, modelName, reason, until)
	if success {
		recordPlanQuotaCooldownManageLog(channelError, reason, until, disabledUntilText, "entered", true, 0, modelName)
		return
	}
	if isPlanQuotaModelCooldownAlreadyActive(channelError, modelName, until) {
		recordPlanQuotaCooldownManageLog(channelError, reason, until, disabledUntilText, "already_active", false, planQuotaCooldownAlreadyActiveDedupeSecond, modelName)
	}
}

func recordPlanQuotaCooldownManageLog(channelError types.ChannelError, reason string, until int64, disabledUntilText string, state string, statusChanged bool, dedupeSeconds int64, modelNames ...string) {
	modelName := ""
	if len(modelNames) > 0 {
		modelName = strings.TrimSpace(modelNames[0])
	}
	scope := "channel"
	if channelError.IsMultiKey {
		scope = "current_key"
	}
	if modelName != "" {
		scope = "model"
	}
	adminInfo := map[string]interface{}{
		"event":               "channel_plan_quota_cooldown",
		"channel_id":          channelError.ChannelId,
		"channel_type":        channelError.ChannelType,
		"channel_name":        channelError.ChannelName,
		"is_multi_key":        channelError.IsMultiKey,
		"scope":               scope,
		"disabled_until":      until,
		"disabled_until_text": disabledUntilText,
		"reason":              reason,
		"state":               state,
		"status_changed":      statusChanged,
		"version":             common.Version,
	}
	if modelName != "" {
		adminInfo["model_name"] = modelName
	}
	keyIndex, hasKeyIndex := resolvePlanQuotaCooldownKeyIndex(channelError)
	if hasKeyIndex {
		adminInfo["multi_key_index"] = keyIndex
	}

	content := buildPlanQuotaCooldownManageLogContent(channelError, reason, disabledUntilText, state, keyIndex, hasKeyIndex, modelName)
	if dedupeSeconds > 0 && model.ChannelManageLogExistsSince(channelError.ChannelId, content, common.GetTimestamp()-dedupeSeconds) {
		return
	}
	model.RecordChannelManageLog(channelError.ChannelId, content, adminInfo)
}

func buildPlanQuotaCooldownManageLogContent(channelError types.ChannelError, reason string, disabledUntilText string, state string, keyIndex int, hasKeyIndex bool, modelName string) string {
	keyText := ""
	if hasKeyIndex {
		keyText = fmt.Sprintf("密钥 #%d ", keyIndex+1)
	}
	modelText := ""
	if modelName != "" {
		modelText = fmt.Sprintf("模型「%s」", modelName)
	}
	if state == "already_active" {
		return fmt.Sprintf("通道「%s」（#%d）%s%s已处于套餐限额冷却，禁用至 %s，原因：%s",
			channelError.ChannelName,
			channelError.ChannelId,
			keyText,
			modelText,
			disabledUntilText,
			reason,
		)
	}
	return fmt.Sprintf("通道「%s」（#%d）%s%s进入套餐限额冷却，禁用至 %s，原因：%s",
		channelError.ChannelName,
		channelError.ChannelId,
		keyText,
		modelText,
		disabledUntilText,
		reason,
	)
}

func resolvePlanQuotaCooldownKeyIndex(channelError types.ChannelError) (int, bool) {
	if !channelError.IsMultiKey || channelError.UsingKey == "" {
		return 0, false
	}
	channel, err := model.GetChannelById(channelError.ChannelId, true)
	if err != nil {
		return 0, false
	}
	return resolvePlanQuotaCooldownKeyIndexFromChannel(channel, channelError)
}

func resolvePlanQuotaCooldownKeyIndexFromChannel(channel *model.Channel, channelError types.ChannelError) (int, bool) {
	if channel == nil || !channel.ChannelInfo.IsMultiKey || channelError.UsingKey == "" {
		return 0, false
	}
	for idx, key := range channel.GetKeys() {
		if key == channelError.UsingKey {
			return idx, true
		}
	}
	return 0, false
}

func isPlanQuotaCooldownAlreadyActive(channelError types.ChannelError, until int64) bool {
	channel, err := model.GetChannelById(channelError.ChannelId, true)
	if err != nil || channel == nil {
		return false
	}
	if channel.ChannelInfo.IsMultiKey || channelError.IsMultiKey {
		keyIndex, ok := resolvePlanQuotaCooldownKeyIndexFromChannel(channel, channelError)
		if !ok || channel.ChannelInfo.MultiKeyStatusList == nil || channel.ChannelInfo.MultiKeyDisabledUntil == nil {
			return false
		}
		return channel.ChannelInfo.MultiKeyStatusList[keyIndex] == common.ChannelStatusAutoDisabled &&
			channel.ChannelInfo.MultiKeyDisabledUntil[keyIndex] == until
	}
	return channel.Status == common.ChannelStatusAutoDisabled && channel.GetStatusUntil() == until
}

func isPlanQuotaModelCooldownAlreadyActive(channelError types.ChannelError, modelName string, until int64) bool {
	channel, err := model.GetChannelById(channelError.ChannelId, true)
	if err != nil || channel == nil {
		return false
	}
	return channel.GetModelStatusUntil(modelName) == until
}

func StartChannelPlanQuotaCooldownTask() {
	planQuotaCooldownOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("channel plan quota cooldown task started: tick=%s", planQuotaCooldownTickInterval))
			ticker := time.NewTicker(planQuotaCooldownTickInterval)
			defer ticker.Stop()

			runChannelPlanQuotaCooldownOnce()
			for range ticker.C {
				runChannelPlanQuotaCooldownOnce()
			}
		})
	})
}

func runChannelPlanQuotaCooldownOnce() {
	if !planQuotaCooldownRunning.CompareAndSwap(false, true) {
		return
	}
	defer planQuotaCooldownRunning.Store(false)

	releasedChannels, releasedKeys, releasedModels, err := model.ReleaseExpiredPlanQuotaCooldowns(common.GetTimestamp(), planQuotaCooldownBatchSize)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("channel plan quota cooldown release failed: %v", err))
		return
	}
	if releasedChannels > 0 || releasedKeys > 0 || releasedModels > 0 {
		model.InitChannelCache()
		logger.LogInfo(context.Background(), fmt.Sprintf("channel plan quota cooldown released: channels=%d keys=%d models=%d", releasedChannels, releasedKeys, releasedModels))
	}
}
