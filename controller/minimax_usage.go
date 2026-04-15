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

type miniMaxUsageEnvelope struct {
	BaseResp struct {
		StatusCode int64  `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
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
	if ch.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "multi-key channel is not supported"})
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	statusCode, body, requestURL, err := fetchMiniMaxTokenPlanUsage(ctx, client, ch)
	if err != nil {
		common.SysError("failed to fetch minimax token plan usage: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取 MiniMax Token Plan 用量失败，请稍后重试",
		})
		return
	}

	var payload any
	if common.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}

	success := statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
	message := ""

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

	c.JSON(http.StatusOK, gin.H{
		"success":         success,
		"message":         message,
		"upstream_status": statusCode,
		"request_url":     requestURL,
		"data":            payload,
	})
}

func fetchMiniMaxTokenPlanUsage(ctx context.Context, client *http.Client, channel *model.Channel) (statusCode int, body []byte, requestURL string, err error) {
	var lastErr error
	var lastStatusCode int
	var lastBody []byte
	var lastRequestURL string

	for _, candidateURL := range miniMaxTokenPlanRequestURLs(channel) {
		statusCode, body, err = doMiniMaxTokenPlanUsageRequest(ctx, client, candidateURL, strings.TrimSpace(channel.Key))
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
		url := host + "/v1/api/openplatform/coding_plan/remains"
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
	return candidates
}
