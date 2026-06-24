package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	miniMaxTokenPlanLimitCode           = "2056"
	miniMaxTokenPlanExhaustedStatus     = 2
	miniMaxTokenPlanUsageRequestTimeout = 15 * time.Second
)

type miniMaxUsageEnvelope struct {
	BaseResp struct {
		StatusCode int64  `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

type miniMaxTokenPlanUsagePayload struct {
	BaseResp struct {
		StatusCode int64  `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
	ModelRemains []miniMaxTokenPlanModelRemain `json:"model_remains"`
}

type miniMaxTokenPlanModelRemain struct {
	ModelName                       string  `json:"model_name"`
	CurrentIntervalRemainingPercent float64 `json:"current_interval_remaining_percent"`
	CurrentIntervalStatus           int     `json:"current_interval_status"`
	CurrentIntervalTotalCount       int64   `json:"current_interval_total_count"`
	CurrentIntervalUsageCount       int64   `json:"current_interval_usage_count"`
	EndTime                         int64   `json:"end_time"`
	RemainsTime                     int64   `json:"remains_time"`
	CurrentWeeklyRemainingPercent   float64 `json:"current_weekly_remaining_percent"`
	CurrentWeeklyStatus             int     `json:"current_weekly_status"`
	CurrentWeeklyTotalCount         int64   `json:"current_weekly_total_count"`
	CurrentWeeklyUsageCount         int64   `json:"current_weekly_usage_count"`
	WeeklyEndTime                   int64   `json:"weekly_end_time"`
	WeeklyRemainsTime               int64   `json:"weekly_remains_time"`
}

func GetMiniMaxChannelUsage(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if ch.Type != constant.ChannelTypeMiniMax {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not MiniMax"})
		return
	}

	keySelection, err := resolveChannelUsageKeySelection(ch, c.Query("key_index"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	statusCode, body, requestURL, err := fetchMiniMaxTokenPlanUsage(ctx, client, ch, keySelection.Key)
	if err != nil {
		common.SysError("failed to fetch minimax token plan usage: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取 MiniMax Token Plan 用量失败，请稍后重试",
		})
		return
	}

	payload, success, message := parseMiniMaxUsageResponse(statusCode, body)
	c.JSON(http.StatusOK, gin.H{
		"success":         success,
		"message":         message,
		"multi_key":       ch.ChannelInfo.IsMultiKey,
		"key_index":       keySelection.KeyIndex,
		"key_count":       keySelection.KeyCount,
		"key_label":       keySelection.KeyLabel,
		"key_status":      keySelection.KeyStatus,
		"disabled_reason": keySelection.DisabledReason,
		"disabled_time":   keySelection.DisabledTime,
		"upstream_status": statusCode,
		"request_url":     requestURL,
		"data":            payload,
	})
}

func parseMiniMaxUsageResponse(statusCode int, body []byte) (payload any, success bool, message string) {
	if common.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}

	success = statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
	var envelope miniMaxUsageEnvelope
	if common.Unmarshal(body, &envelope) == nil && envelope.BaseResp.StatusCode != 0 {
		success = false
		message = strings.TrimSpace(envelope.BaseResp.StatusMsg)
		if message == "" {
			message = fmt.Sprintf("MiniMax status code: %d", envelope.BaseResp.StatusCode)
		}
	}
	if !success && message == "" {
		message = fmt.Sprintf("upstream status: %d", statusCode)
	}
	return payload, success, message
}

func fetchMiniMaxTokenPlanUsage(ctx context.Context, client *http.Client, channel *model.Channel, apiKey string) (statusCode int, body []byte, requestURL string, err error) {
	var lastErr error
	var lastStatusCode int
	var lastBody []byte
	var lastRequestURL string

	for _, candidateURL := range miniMaxTokenPlanRequestURLs(channel) {
		statusCode, body, err = doMiniMaxTokenPlanUsageRequest(ctx, client, candidateURL, apiKey)
		if err != nil {
			lastErr = err
			continue
		}

		lastStatusCode = statusCode
		lastBody = body
		lastRequestURL = candidateURL
		if statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed {
			continue
		}
		return statusCode, body, candidateURL, nil
	}

	if lastRequestURL != "" {
		return lastStatusCode, lastBody, lastRequestURL, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no reachable MiniMax Token Plan endpoint")
	}
	return 0, nil, "", lastErr
}

func doMiniMaxTokenPlanUsageRequest(ctx context.Context, client *http.Client, requestURL string, apiKey string) (statusCode int, body []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header = GetAuthHeader(apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func miniMaxTokenPlanRequestURLs(channel *model.Channel) []string {
	candidates := make([]string, 0, 4)
	addCandidate := func(host string) {
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		host = strings.TrimRight(host, "/")
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		url := host + "/v1/token_plan/remains"
		for _, existing := range candidates {
			if existing == url {
				return
			}
		}
		candidates = append(candidates, url)
	}

	baseURL := strings.ToLower(strings.TrimSpace(channel.GetBaseURL()))
	switch {
	case strings.Contains(baseURL, "minimax.io"):
		addCandidate("https://www.minimax.io")
	case strings.Contains(baseURL, "minimax.com"):
		addCandidate("https://www.minimax.com")
	case strings.Contains(baseURL, "minimaxi.com"):
		addCandidate("https://www.minimaxi.com")
	}

	addCandidate("https://www.minimaxi.com")
	addCandidate("https://www.minimax.io")
	addCandidate("https://www.minimax.com")
	addLegacyMiniMaxTokenPlanCandidates(&candidates)
	return candidates
}

func isMiniMaxTokenPlanLimitError(channelError types.ChannelError, err *types.NewAPIError) bool {
	if channelError.ChannelType != constant.ChannelTypeMiniMax || err == nil {
		return false
	}
	message := strings.ToLower(err.ErrorWithStatusCode())
	return strings.Contains(message, "token plan") &&
		strings.Contains(message, miniMaxTokenPlanLimitCode) &&
		(strings.Contains(message, "用量上限") || strings.Contains(message, "usage limit") || strings.Contains(message, "limit"))
}

func resolveAndLimitMiniMaxTokenPlanCooldown(channelError types.ChannelError, reason string, modelName string) {
	ctx, cancel := context.WithTimeout(context.Background(), miniMaxTokenPlanUsageRequestTimeout)
	defer cancel()

	until, detail, ok, err := resolveMiniMaxTokenPlanCooldownUntil(ctx, channelError, modelName, time.Now())
	if err != nil {
		common.SysError(fmt.Sprintf("failed to resolve MiniMax Token Plan cooldown: channel_id=%d, error=%v", channelError.ChannelId, err))
		return
	}
	if !ok {
		common.SysLog(fmt.Sprintf("MiniMax Token Plan limit did not enter cooldown: channel_id=%d, model=%s, no exhausted quota window found", channelError.ChannelId, modelName))
		return
	}
	if detail != "" {
		reason = reason + "；MiniMax套餐查询：" + detail
	}
	service.DisableChannelUntil(channelError, reason, until)
}

func resolveMiniMaxTokenPlanCooldownUntil(ctx context.Context, channelError types.ChannelError, modelName string, now time.Time) (until int64, detail string, ok bool, err error) {
	ch, err := model.GetChannelById(channelError.ChannelId, true)
	if err != nil {
		return 0, "", false, err
	}
	if ch == nil {
		return 0, "", false, errors.New("channel not found")
	}
	if ch.Type != constant.ChannelTypeMiniMax {
		return 0, "", false, fmt.Errorf("channel type is not MiniMax: %d", ch.Type)
	}

	apiKey := channelError.UsingKey
	if strings.TrimSpace(apiKey) == "" {
		nextKey, _, keyErr := ch.GetNextEnabledKey()
		if keyErr != nil {
			return 0, "", false, keyErr
		}
		apiKey = nextKey
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		return 0, "", false, err
	}

	statusCode, body, requestURL, err := fetchMiniMaxTokenPlanUsage(ctx, client, ch, apiKey)
	if err != nil {
		return 0, "", false, err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return 0, "", false, fmt.Errorf("MiniMax Token Plan usage upstream status %d from %s", statusCode, requestURL)
	}
	return miniMaxTokenPlanCooldownUntil(body, now, modelName)
}

func miniMaxTokenPlanCooldownUntil(body []byte, now time.Time, modelName string) (until int64, detail string, ok bool, err error) {
	var payload miniMaxTokenPlanUsagePayload
	if err := common.Unmarshal(body, &payload); err != nil {
		return 0, "", false, err
	}
	if payload.BaseResp.StatusCode != 0 {
		message := strings.TrimSpace(payload.BaseResp.StatusMsg)
		if message == "" {
			message = fmt.Sprintf("status_code=%d", payload.BaseResp.StatusCode)
		}
		return 0, "", false, errors.New(message)
	}
	if len(payload.ModelRemains) == 0 {
		return 0, "", false, nil
	}

	preferredBuckets := inferMiniMaxTokenPlanBuckets(modelName)
	candidates := selectMiniMaxTokenPlanRemains(payload.ModelRemains, preferredBuckets)
	var selectedUntil int64
	selectedDetail := ""
	for _, item := range candidates {
		itemUntil, itemDetail, exhausted := miniMaxTokenPlanRemainCooldownUntil(item, now)
		if !exhausted || itemUntil <= now.Unix() {
			continue
		}
		if itemUntil > selectedUntil {
			selectedUntil = itemUntil
			selectedDetail = itemDetail
		}
	}
	if selectedUntil == 0 {
		return 0, "", false, nil
	}
	return selectedUntil, selectedDetail, true, nil
}

func selectMiniMaxTokenPlanRemains(items []miniMaxTokenPlanModelRemain, preferredBuckets []string) []miniMaxTokenPlanModelRemain {
	if len(preferredBuckets) == 0 {
		return items
	}
	preferred := make(map[string]bool, len(preferredBuckets))
	for _, bucket := range preferredBuckets {
		preferred[strings.ToLower(bucket)] = true
	}
	matched := make([]miniMaxTokenPlanModelRemain, 0, len(items))
	for _, item := range items {
		if preferred[strings.ToLower(strings.TrimSpace(item.ModelName))] {
			matched = append(matched, item)
		}
	}
	if len(matched) > 0 {
		return matched
	}
	return items
}

func miniMaxTokenPlanRemainCooldownUntil(item miniMaxTokenPlanModelRemain, now time.Time) (until int64, detail string, exhausted bool) {
	windowUntil := int64(0)
	windows := make([]string, 0, 2)
	if miniMaxTokenPlanWindowExhausted(item.CurrentIntervalStatus, item.CurrentIntervalRemainingPercent, item.CurrentIntervalTotalCount, item.CurrentIntervalUsageCount) {
		if candidate := miniMaxTokenPlanResetUnix(item.EndTime, item.RemainsTime, now); candidate > windowUntil {
			windowUntil = candidate
		}
		windows = append(windows, "interval")
	}
	if miniMaxTokenPlanWindowExhausted(item.CurrentWeeklyStatus, item.CurrentWeeklyRemainingPercent, item.CurrentWeeklyTotalCount, item.CurrentWeeklyUsageCount) {
		if candidate := miniMaxTokenPlanResetUnix(item.WeeklyEndTime, item.WeeklyRemainsTime, now); candidate > windowUntil {
			windowUntil = candidate
		}
		windows = append(windows, "weekly")
	}
	if windowUntil <= 0 || len(windows) == 0 {
		return 0, "", false
	}
	return windowUntil, fmt.Sprintf("model=%s window=%s reset_at=%s", item.ModelName, strings.Join(windows, "+"), time.Unix(windowUntil, 0).Format(time.RFC3339)), true
}

func miniMaxTokenPlanWindowExhausted(status int, remainingPercent float64, totalCount int64, usageCount int64) bool {
	if status == miniMaxTokenPlanExhaustedStatus {
		return true
	}
	return remainingPercent <= 0 && totalCount > 0 && usageCount >= totalCount
}

func miniMaxTokenPlanResetUnix(endTime int64, remainsTime int64, now time.Time) int64 {
	if endTime > 0 {
		switch {
		case endTime > 1_000_000_000_000:
			return endTime / 1000
		case endTime > 1_000_000_000:
			return endTime
		}
	}
	if remainsTime > 0 {
		return now.Add(time.Duration(remainsTime) * time.Millisecond).Unix()
	}
	return 0
}

func inferMiniMaxTokenPlanBuckets(modelName string) []string {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case modelName == "":
		return []string{"general"}
	case strings.Contains(modelName, "hailuo") ||
		strings.HasPrefix(modelName, "t2v-") ||
		strings.HasPrefix(modelName, "i2v-") ||
		strings.HasPrefix(modelName, "s2v-"):
		return []string{"video"}
	case strings.HasPrefix(modelName, "music-") || strings.Contains(modelName, "music"):
		return []string{"music", "general"}
	case strings.HasPrefix(modelName, "speech-"):
		return []string{"audio", "general"}
	case strings.HasPrefix(modelName, "image-"):
		return []string{"image", "general"}
	default:
		return []string{"general"}
	}
}

func addLegacyMiniMaxTokenPlanCandidates(candidates *[]string) {
	for _, host := range []string{
		"https://www.minimaxi.com",
		"https://www.minimax.io",
		"https://www.minimax.com",
	} {
		url := host + "/v1/api/openplatform/coding_plan/remains"
		exists := false
		for _, existing := range *candidates {
			if existing == url {
				exists = true
				break
			}
		}
		if !exists {
			*candidates = append(*candidates, url)
		}
	}
}
