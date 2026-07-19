package controller

import (
	"bytes"
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
	"github.com/QuantumNous/new-api/relay/channel/qwentokenplan"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	qwenTokenPlanUsageURL      = "https://cli.qianwenai.com/data/v2/api.json"
	qwenTokenPlanCommodityCode = "sfm_tokenplanpersonal_dp_cn"
)

type qwenTokenPlanGatewayRequest struct {
	Product string            `json:"product"`
	Action  string            `json:"action"`
	Region  string            `json:"region"`
	Params  map[string]string `json:"params"`
}

type qwenTokenPlanGatewayEnvelope struct {
	Code    any                       `json:"code"`
	Message string                    `json:"message"`
	Data    qwenTokenPlanInstancePage `json:"data"`
}

type qwenTokenPlanInstancePage struct {
	Data []qwenTokenPlanInstance `json:"Data"`
}

type qwenTokenPlanInstance struct {
	CommodityName           string `json:"CommodityName"`
	TemplateName            string `json:"TemplateName"`
	Status                  any    `json:"Status"`
	StatusCode              string `json:"StatusCode"`
	InitCapacityBaseValue   any    `json:"InitCapacityBaseValue"`
	CurrCapacityBaseValue   any    `json:"CurrCapacityBaseValue"`
	PeriodCapacityBaseValue any    `json:"periodCapacityBaseValue"`
	CapacityTypeCode        string `json:"CapacityTypeCode"`
	EndTime                 any    `json:"EndTime"`
}

type qwenTokenPlanUsage struct {
	Subscribed       bool    `json:"subscribed"`
	PlanName         string  `json:"plan_name,omitempty"`
	Status           string  `json:"status,omitempty"`
	TotalCredits     float64 `json:"total_credits"`
	RemainingCredits float64 `json:"remaining_credits"`
	UsedCredits      float64 `json:"used_credits"`
	UsedPercent      float64 `json:"used_percent"`
	ResetAt          int64   `json:"reset_at,omitempty"`
	CapacityType     string  `json:"capacity_type,omitempty"`
}

func GetQwenTokenPlanUsage(c *gin.Context) {
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
	if ch.Type != constant.ChannelTypeQwenTokenPlan {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Qwen Token Plan"})
		return
	}

	keySelection, err := resolveChannelUsageKeySelection(ch, c.Query("key_index"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	response := qwenTokenPlanUsageResponse(ch, keySelection)
	credential, err := qwentokenplan.ParseCredential(keySelection.Key)
	if err != nil {
		response["success"] = false
		response["message"] = err.Error()
		c.JSON(http.StatusOK, response)
		return
	}
	if credential.OAuthExpired(time.Now()) {
		response["success"] = false
		response["message"] = "QianWen OAuth credential has expired; authorize the channel again"
		c.JSON(http.StatusOK, response)
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	statusCode, body, err := doQwenTokenPlanUsageRequest(ctx, client, qwenTokenPlanUsageURL, credential.AccessToken)
	if err != nil {
		common.SysError("failed to fetch qwen token plan usage: " + err.Error())
		response["success"] = false
		response["message"] = "获取 Qwen Token Plan 额度失败，请稍后重试"
		c.JSON(http.StatusOK, response)
		return
	}

	payload, success, message := parseQwenTokenPlanUsageResponse(statusCode, body)
	response["success"] = success
	response["message"] = message
	response["upstream_status"] = statusCode
	response["request_url"] = qwenTokenPlanUsageURL
	response["data"] = payload
	c.JSON(http.StatusOK, response)
}

func qwenTokenPlanUsageResponse(ch *model.Channel, selection *channelUsageKeySelection) gin.H {
	return gin.H{
		"success":         false,
		"message":         "",
		"multi_key":       ch.ChannelInfo.IsMultiKey,
		"key_index":       selection.KeyIndex,
		"key_count":       selection.KeyCount,
		"key_label":       selection.KeyLabel,
		"key_status":      selection.KeyStatus,
		"disabled_reason": selection.DisabledReason,
		"disabled_time":   selection.DisabledTime,
	}
}

func doQwenTokenPlanUsageRequest(ctx context.Context, client *http.Client, requestURL string, accessToken string) (statusCode int, body []byte, err error) {
	payload := qwenTokenPlanGatewayRequest{
		Product: "BssOpenAPI-V3",
		Action:  "DescribeFrInstances",
		Region:  "cn-beijing",
		Params: map[string]string{
			"Group":         "tokenPlan",
			"CommodityCode": qwenTokenPlanCommodityCode,
			"PageNum":       "1",
			"PageSize":      "10",
		},
	}
	requestBody, err := common.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(requestBody))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
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

func parseQwenTokenPlanUsageResponse(statusCode int, body []byte) (qwenTokenPlanUsage, bool, string) {
	usage := qwenTokenPlanUsage{}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return usage, false, fmt.Sprintf("upstream status: %d", statusCode)
	}

	var envelope qwenTokenPlanGatewayEnvelope
	if err := common.Unmarshal(body, &envelope); err != nil {
		return usage, false, "invalid Qwen CLI usage response"
	}
	if strings.TrimSpace(fmt.Sprint(envelope.Code)) != "200" {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = fmt.Sprintf("Qwen CLI gateway code: %v", envelope.Code)
		}
		return usage, false, message
	}
	if len(envelope.Data.Data) == 0 {
		return usage, true, ""
	}

	instance := envelope.Data.Data[0]
	for _, candidate := range envelope.Data.Data {
		if qwenTokenPlanStatus(candidate) == "valid" {
			instance = candidate
			break
		}
	}
	status := qwenTokenPlanStatus(instance)
	total := qwenTokenPlanNumber(instance.InitCapacityBaseValue)
	remaining := qwenTokenPlanNumber(instance.CurrCapacityBaseValue)
	if instance.CapacityTypeCode == "periodMonthlyShift" {
		periodRemaining := qwenTokenPlanNumber(instance.PeriodCapacityBaseValue)
		if periodRemaining != 0 || !isEmptyQwenTokenPlanValue(instance.PeriodCapacityBaseValue) {
			remaining = periodRemaining
		}
	}
	used := math.Max(total-remaining, 0)
	usedPercent := 0.0
	if total > 0 {
		usedPercent = used / total * 100
	}

	usage = qwenTokenPlanUsage{
		Subscribed:       status == "valid",
		PlanName:         firstNonEmptyString(instance.TemplateName, instance.CommodityName),
		Status:           status,
		TotalCredits:     total,
		RemainingCredits: remaining,
		UsedCredits:      used,
		UsedPercent:      usedPercent,
		ResetAt:          qwenTokenPlanTimestamp(instance.EndTime),
		CapacityType:     instance.CapacityTypeCode,
	}
	return usage, true, ""
}

func qwenTokenPlanStatus(instance qwenTokenPlanInstance) string {
	if status, ok := instance.Status.(string); ok {
		return strings.TrimSpace(status)
	}
	if status, ok := instance.Status.(map[string]any); ok {
		if code, ok := status["Code"]; ok {
			return strings.TrimSpace(fmt.Sprint(code))
		}
	}
	return strings.TrimSpace(instance.StatusCode)
}

func qwenTokenPlanNumber(value any) float64 {
	if value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(typed)), 64)
		return parsed
	}
}

func qwenTokenPlanTimestamp(value any) int64 {
	numeric := qwenTokenPlanNumber(value)
	if numeric > 0 {
		return int64(numeric)
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" {
		return 0
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, text)
		if err == nil {
			return parsed.UnixMilli()
		}
	}
	return 0
}

func isEmptyQwenTokenPlanValue(value any) bool {
	return value == nil || strings.TrimSpace(fmt.Sprint(value)) == ""
}
