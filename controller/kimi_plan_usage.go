package controller

import (
	"context"
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
// Coding Plan endpoint. It accepts:
//   - the special placeholder "kimi-coding-plan" (mapped via ChannelSpecialBases);
//   - any non-empty URL — the user fills the address up to "/coding" as
//     instructed in the form description, and we append "/v1" ourselves.
func kimiCodingPlanAPIBase(baseURL string) (string, bool) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return "", false
	}
	if trimmed == kimiCodingPlanBaseURL {
		if plan, ok := constant.ChannelSpecialBases[kimiCodingPlanBaseURL]; ok && plan.OpenAIBaseURL != "" {
			return plan.OpenAIBaseURL, true
		}
		return kimiCodingPlanFallback, true
	}
	return strings.TrimRight(trimmed, "/") + "/v1", true
}
