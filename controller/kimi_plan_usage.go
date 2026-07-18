package controller

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	kimiCodingPlanBaseURL   = "kimi-coding-plan"
	kimiCodingPlanQuotaPath = "/usages"
	kimiCodingPlanFallback  = "https://api.kimi.com/coding/v1"
)

type kimiCodingPlanErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
	Message string `json:"message"`
}

type kimiCodingPlanExtraUsage struct {
	BalanceCents              int64  `json:"balance_cents"`
	TotalCents                int64  `json:"total_cents"`
	MonthlyChargeLimitEnabled bool   `json:"monthly_charge_limit_enabled"`
	MonthlyChargeLimitCents   int64  `json:"monthly_charge_limit_cents"`
	MonthlyUsedCents          int64  `json:"monthly_used_cents"`
	Currency                  string `json:"currency"`
}

type kimiCodingPlanUsagePayload struct {
	BoosterWallet *kimiCodingPlanBoosterWallet `json:"boosterWallet"`
}

type kimiCodingPlanBoosterWallet struct {
	Balance                   *kimiCodingPlanBoosterBalance `json:"balance"`
	MonthlyChargeLimit        *kimiCodingPlanMoney          `json:"monthlyChargeLimit"`
	MonthlyUsed               *kimiCodingPlanMoney          `json:"monthlyUsed"`
	MonthlyChargeLimitEnabled bool                          `json:"monthlyChargeLimitEnabled"`
}

type kimiCodingPlanBoosterBalance struct {
	Type       string                 `json:"type"`
	Amount     *kimiCodingPlanInteger `json:"amount"`
	AmountLeft *kimiCodingPlanInteger `json:"amountLeft"`
}

type kimiCodingPlanMoney struct {
	PriceInCents *kimiCodingPlanInteger `json:"priceInCents"`
	Currency     string                 `json:"currency"`
}

type kimiCodingPlanInteger int64

func (value *kimiCodingPlanInteger) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		var decoded string
		if err := common.Unmarshal(data, &decoded); err != nil {
			return err
		}
		text = strings.TrimSpace(decoded)
	}
	if text == "" {
		return fmt.Errorf("empty integer")
	}

	if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
		*value = kimiCodingPlanInteger(parsed)
		return nil
	}

	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return fmt.Errorf("invalid integer %q", text)
	}
	if parsed >= float64(math.MaxInt64) || parsed <= float64(math.MinInt64) {
		return fmt.Errorf("integer %q is out of range", text)
	}
	*value = kimiCodingPlanInteger(math.Trunc(parsed))
	return nil
}

// GetKimiCodingPlanUsage fetches the usage/quota information for a Moonshot
// channel that is configured to use the Kimi Coding Plan endpoint
// (https://api.kimi.com/coding/v1/usages).
func GetKimiCodingPlanUsage(c *gin.Context) {
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
	if ch.Type != constant.ChannelTypeMoonshot {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Moonshot"})
		return
	}

	keySelection, err := resolveChannelUsageKeySelection(ch, c.Query("key_index"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	if _, ok := kimiCodingPlanAPIBase(ch.GetBaseURL()); !ok {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "channel is not a Kimi Coding Plan base",
		})
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	statusCode, body, requestURL, err := fetchKimiCodingPlanUsage(ctx, client, ch, keySelection.Key)
	if err != nil {
		common.SysError("failed to fetch kimi coding plan usage: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取 Kimi Coding Plan 额度失败，请稍后重试",
		})
		return
	}

	var payload any
	if common.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}
	extraUsage := parseKimiCodingPlanExtraUsage(body)

	success := statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
	message := ""

	if !success {
		var envelope kimiCodingPlanErrorEnvelope
		if common.Unmarshal(body, &envelope) == nil {
			message = firstNonEmptyString(envelope.Error.Message, envelope.Message)
		}
		if message == "" {
			message = fmt.Sprintf("upstream status: %d", statusCode)
		}
	}

	response := gin.H{
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
	}
	if extraUsage != nil {
		response["extra_usage"] = extraUsage
	}
	c.JSON(http.StatusOK, response)
}

func fetchKimiCodingPlanUsage(ctx context.Context, client *http.Client, channel *model.Channel, apiKey string) (statusCode int, body []byte, requestURL string, err error) {
	requestURL, err = kimiCodingPlanRequestURL(channel)
	if err != nil {
		return 0, nil, "", err
	}
	statusCode, body, err = doKimiCodingPlanUsageRequest(ctx, client, requestURL, apiKey)
	if err != nil {
		return 0, nil, requestURL, err
	}
	return statusCode, body, requestURL, nil
}

func doKimiCodingPlanUsageRequest(ctx context.Context, client *http.Client, requestURL string, apiKey string) (statusCode int, body []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Accept", "application/json")
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

func kimiCodingPlanRequestURL(channel *model.Channel) (string, error) {
	apiBase, ok := kimiCodingPlanAPIBase(channel.GetBaseURL())
	if !ok {
		return "", fmt.Errorf("no kimi coding plan api base for %q", channel.GetBaseURL())
	}
	return strings.TrimRight(apiBase, "/") + kimiCodingPlanQuotaPath, nil
}

// kimiCodingPlanAPIBase resolves the OpenAI-compatible base URL for the Kimi
// Coding Plan endpoint. It accepts the special placeholder or a custom URL
// ending in either /coding or /coding/v1.
func kimiCodingPlanAPIBase(baseURL string) (string, bool) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return "", false
	}
	if trimmed == kimiCodingPlanBaseURL {
		if plan, ok := constant.ChannelSpecialBases[kimiCodingPlanBaseURL]; ok && plan.OpenAIBaseURL != "" {
			return plan.OpenAIBaseURL, true
		}
		return kimiCodingPlanFallback, true
	}

	normalized := strings.ToLower(trimmed)
	if strings.HasSuffix(normalized, "/coding/v1") {
		return trimmed, true
	}
	if strings.HasSuffix(normalized, "/coding") {
		return trimmed + "/v1", true
	}
	return "", false
}

func parseKimiCodingPlanExtraUsage(body []byte) *kimiCodingPlanExtraUsage {
	var payload kimiCodingPlanUsagePayload
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil
	}

	wallet := payload.BoosterWallet
	if wallet == nil || wallet.Balance == nil {
		return nil
	}
	balance := wallet.Balance
	if balance.Type != "BOOSTER" || balance.Amount == nil || int64(*balance.Amount) <= 0 {
		return nil
	}

	monthlyLimitCents, monthlyLimitCurrency := kimiCodingPlanMoneyValue(wallet.MonthlyChargeLimit)
	monthlyUsedCents, monthlyUsedCurrency := kimiCodingPlanMoneyValue(wallet.MonthlyUsed)
	currency := firstNonEmptyString(monthlyLimitCurrency, monthlyUsedCurrency, "USD")
	balanceCents := int64(0)
	if balance.AmountLeft != nil {
		balanceCents = kimiCodingPlanFixedPointToCents(int64(*balance.AmountLeft))
	}

	return &kimiCodingPlanExtraUsage{
		BalanceCents:              balanceCents,
		TotalCents:                kimiCodingPlanFixedPointToCents(int64(*balance.Amount)),
		MonthlyChargeLimitEnabled: wallet.MonthlyChargeLimitEnabled,
		MonthlyChargeLimitCents:   monthlyLimitCents,
		MonthlyUsedCents:          monthlyUsedCents,
		Currency:                  currency,
	}
}

func kimiCodingPlanMoneyValue(money *kimiCodingPlanMoney) (int64, string) {
	if money == nil || money.PriceInCents == nil {
		return 0, ""
	}
	return int64(*money.PriceInCents), strings.TrimSpace(money.Currency)
}

func kimiCodingPlanFixedPointToCents(value int64) int64 {
	cents := float64(value) / 1_000_000
	if cents > 0 && cents < 1 {
		return 1
	}
	return int64(math.Round(cents))
}
