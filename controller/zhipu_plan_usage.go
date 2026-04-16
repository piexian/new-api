package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
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
	zhipuCodingPlanBaseURL              = "glm-coding-plan"
	zhipuCodingPlanInternationalBaseURL = "glm-coding-plan-international"
	zhipuCodingPlanQuotaPath            = "/api/monitor/usage/quota/limit"
)

type zhipuCodingPlanEnvelope struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Message string `json:"message"`
	Success *bool  `json:"success"`
}

func GetZhipuCodingPlanUsage(c *gin.Context) {
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
	if ch.Type != constant.ChannelTypeZhipu_v4 && ch.Type != constant.ChannelTypeZhipu {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Zhipu"})
		return
	}

	keySelection, err := resolveChannelUsageKeySelection(ch, c.Query("key_index"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if _, ok := zhipuCodingPlanAPIBase(ch.GetBaseURL()); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel is not a Zhipu Coding Plan base"})
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	statusCode, body, requestURL, err := fetchZhipuCodingPlanUsage(ctx, client, ch, keySelection.Key)
	if err != nil {
		common.SysError("failed to fetch zhipu coding plan usage: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取智谱 Coding Plan 额度失败，请稍后重试",
		})
		return
	}

	var payload any
	if common.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}

	success := statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
	message := ""

	var envelope zhipuCodingPlanEnvelope
	if common.Unmarshal(body, &envelope) == nil {
		if envelope.Success != nil && !*envelope.Success {
			success = false
		}
		if envelope.Code != 0 && envelope.Code != http.StatusOK {
			success = false
		}
		if !success {
			message = firstNonEmptyString(envelope.Message, envelope.Msg)
		}
	}
	if !success && message == "" {
		message = fmt.Sprintf("upstream status: %d", statusCode)
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

func fetchZhipuCodingPlanUsage(ctx context.Context, client *http.Client, channel *model.Channel, apiKey string) (statusCode int, body []byte, requestURL string, err error) {
	requestURL, err = zhipuCodingPlanRequestURL(channel)
	if err != nil {
		return 0, nil, "", err
	}
	statusCode, body, err = doZhipuCodingPlanUsageRequest(ctx, client, requestURL, apiKey)
	if err != nil {
		return 0, nil, requestURL, err
	}
	return statusCode, body, requestURL, nil
}

func doZhipuCodingPlanUsageRequest(ctx context.Context, client *http.Client, requestURL string, apiKey string) (statusCode int, body []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", apiKey)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Content-Type", "application/json")
	if origin := requestOrigin(requestURL); origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Referer", origin+"/")
	}

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

func zhipuCodingPlanRequestURL(channel *model.Channel) (string, error) {
	apiBase, ok := zhipuCodingPlanAPIBase(channel.GetBaseURL())
	if !ok {
		return "", fmt.Errorf("no zhipu coding plan api base for %q", channel.GetBaseURL())
	}
	return strings.TrimRight(apiBase, "/") + zhipuCodingPlanQuotaPath, nil
}

func zhipuCodingPlanAPIBase(baseURL string) (string, bool) {
	trimmed := strings.TrimSpace(baseURL)
	switch trimmed {
	case zhipuCodingPlanBaseURL:
		return "https://open.bigmodel.cn", true
	case zhipuCodingPlanInternationalBaseURL:
		return "https://api.z.ai", true
	}

	lower := strings.ToLower(trimmed)
	switch {
	case strings.Contains(lower, "api.z.ai"):
		return "https://api.z.ai", true
	case strings.Contains(lower, "open.bigmodel.cn"):
		return "https://open.bigmodel.cn", true
	case strings.Contains(lower, "www.bigmodel.cn"):
		return "https://www.bigmodel.cn", true
	default:
		return "", false
	}
}

func requestOrigin(requestURL string) string {
	parsed, err := neturl.Parse(requestURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
