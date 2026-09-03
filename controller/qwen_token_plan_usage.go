package controller

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/qwentokenplan"
	"github.com/QuantumNous/new-api/service"
	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

// 额度查询走阿里云百炼 console 网关（bailian-cli `bl usage token-plan` 同源协议）：
// POST /cli/api.json?action=BroadScopeAspnGateway&product=sfm_bailian&api=zeldaHttp.apikeyMgr./tokenplan/personal/api/v2/usage
// Bearer 为阿里云账号 console 凭证；凭证可用 AK/SK 经 GenerateCLIAccessToken 按需换签（保活）。
const (
	qwenTokenPlanGatewayAction  = "BroadScopeAspnGateway"
	qwenTokenPlanGatewayProduct = "sfm_bailian"
	qwenTokenPlanUsageAPI       = "zeldaHttp.apikeyMgr./tokenplan/personal/api/v2/usage"
	qwenTokenPlanConsoleRegion  = "cn-beijing"
	qwenTokenPlanTokenHost      = "modelstudio.cn-beijing.aliyuncs.com"
	qwenTokenPlanTokenPath      = "/modelstudio/cli/generateAccessToken"
	qwenTokenPlanTokenAction    = "GenerateCLIAccessToken"
	qwenTokenPlanTokenVersion   = "2026-02-10"
	qwenTokenPlanRequestTimeout = 15 * time.Second
)

// 测试会将其覆写指向 httptest 服务器
var (
	qwenTokenPlanGatewayURL = "https://bailian-cs.console.aliyun.com/cli/api.json"
	qwenTokenPlanTokenURL   = "https://modelstudio.cn-beijing.aliyuncs.com/modelstudio/cli/generateAccessToken"
)

type qwenTokenPlanUsageWindow struct {
	Present     bool    `json:"present"`
	UsedPercent float64 `json:"used_percent"`
	ResetAt     int64   `json:"reset_at,omitempty"`
}

type qwenTokenPlanUsage struct {
	Subscribed bool                     `json:"subscribed"`
	Per5Hour   qwenTokenPlanUsageWindow `json:"per_5_hour"`
	Per1Week   qwenTokenPlanUsageWindow `json:"per_1_week"`
}

func GetQwenTokenPlanUsage(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgChannelIdFormatError)
		return
	}

	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": i18n.T(c, i18n.MsgChannelNotExists)})
		return
	}
	if ch.Type != constant.ChannelTypeQwenTokenPlan {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": i18n.T(c, i18n.MsgChannelTypeNotMatched)})
		return
	}

	keySelection, err := resolveChannelUsageKeySelection(ch, c.Query("key_index"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	response := qwenTokenPlanUsageResponse(ch, keySelection)

	credential, err := qwentokenplan.ParseCredential(keySelection.Key)
	if err != nil || !credential.HasConsoleCredential() {
		response["success"] = false
		response["message"] = i18n.T(c, i18n.MsgChannelQwenConsoleCredMissing)
		c.JSON(http.StatusOK, response)
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), qwenTokenPlanRequestTimeout)
	defer cancel()

	accessToken := credential.ConsoleToken
	if credential.AccessKeyID != "" && credential.AccessKeySecret != "" {
		accessToken, err = cachedQwenConsoleToken(ctx, client, credential.AccessKeyID, credential.AccessKeySecret)
		if err != nil {
			response["success"] = false
			response["message"] = i18n.T(c, i18n.MsgChannelQwenConsoleTokenFailed)
			c.JSON(http.StatusOK, response)
			return
		}
	}

	statusCode, body, err := doQwenTokenPlanUsageRequest(ctx, client, accessToken)
	if err != nil {
		common.SysError("failed to fetch qwen token plan usage: " + err.Error())
		response["success"] = false
		response["message"] = i18n.T(c, i18n.MsgRetryLater)
		c.JSON(http.StatusOK, response)
		return
	}
	payload, notLogined, success, message := parseQwenTokenPlanUsageResponse(statusCode, body)

	// console token 失效且有 AK/SK：作废缓存重新换签一次（凭证保活）
	if notLogined && credential.AccessKeyID != "" && credential.AccessKeySecret != "" {
		accessToken, err = invalidateQwenConsoleToken(ctx, client, credential.AccessKeyID, credential.AccessKeySecret)
		if err == nil {
			statusCode, body, err = doQwenTokenPlanUsageRequest(ctx, client, accessToken)
			if err == nil {
				payload, notLogined, success, message = parseQwenTokenPlanUsageResponse(statusCode, body)
			}
		}
	}

	if notLogined {
		message = i18n.T(c, i18n.MsgChannelQwenConsoleTokenExpired)
	}
	response["success"] = success
	response["message"] = message
	response["upstream_status"] = statusCode
	response["request_url"] = qwenTokenPlanGatewayURL
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

func doQwenTokenPlanUsageRequest(ctx context.Context, client *http.Client, accessToken string) (statusCode int, body []byte, err error) {
	envelope := fmt.Sprintf(
		`{"Api":%s,"V":"1.0","Data":{"cornerstoneParam":{"protocol":"V2","console":"ONE_CONSOLE","productCode":"p_efm","switchUserType":3,"consoleSite":"BAILIAN_ALIYUN"}}}`,
		strconv.Quote(qwenTokenPlanUsageAPI),
	)
	form := url.Values{}
	form.Set("params", envelope)
	form.Set("region", qwenTokenPlanConsoleRegion)

	requestURL := fmt.Sprintf(
		"%s?action=%s&product=%s&api=%s",
		qwenTokenPlanGatewayURL, qwenTokenPlanGatewayAction, qwenTokenPlanGatewayProduct,
		url.QueryEscape(qwenTokenPlanUsageAPI),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

type qwenTokenPlanGatewayData struct {
	Success   bool   `json:"success"`
	ErrorCode string `json:"errorCode"`
	ErrorMsg  string `json:"errorMsg"`
	DataV2    *struct {
		Data struct {
			Success bool                   `json:"success"`
			Code    string                 `json:"code"`
			Msg     string                 `json:"msg"`
			Data    map[string]interface{} `json:"data"`
		} `json:"data"`
	} `json:"DataV2"`
}

func parseQwenTokenPlanUsageResponse(statusCode int, body []byte) (usage qwenTokenPlanUsage, notLogined bool, success bool, message string) {
	usage = qwenTokenPlanUsage{}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return usage, false, false, fmt.Sprintf("upstream status: %d", statusCode)
	}

	var envelope struct {
		Data qwenTokenPlanGatewayData `json:"data"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return usage, false, false, "invalid Qwen console gateway response"
	}
	if !envelope.Data.Success {
		message = strings.TrimSpace(envelope.Data.ErrorMsg)
		if message == "" {
			message = fmt.Sprintf("Qwen console gateway error: %s", envelope.Data.ErrorCode)
		}
		if strings.Contains(envelope.Data.ErrorCode, "NotLogined") {
			return usage, true, false, message
		}
		return usage, false, false, message
	}
	if envelope.Data.DataV2 == nil {
		return usage, false, false, "invalid Qwen console gateway response"
	}

	raw := envelope.Data.DataV2.Data.Data
	if !envelope.Data.DataV2.Data.Success && raw == nil {
		detail := strings.TrimSpace(envelope.Data.DataV2.Data.Msg)
		if detail == "" {
			detail = envelope.Data.DataV2.Data.Code
		}
		return usage, false, false, fmt.Sprintf("Qwen token plan usage error: %s", detail)
	}

	usage.Per5Hour = qwenTokenPlanWindow(raw, "per5Hour")
	usage.Per1Week = qwenTokenPlanWindow(raw, "per1Week")
	usage.Subscribed = usage.Per5Hour.Present || usage.Per1Week.Present
	return usage, false, true, ""
}

func qwenTokenPlanWindow(raw map[string]interface{}, prefix string) qwenTokenPlanUsageWindow {
	window := qwenTokenPlanUsageWindow{}
	percent, ok := qwenTokenPlanFloat(raw, prefix+"Percentage")
	if !ok {
		return window
	}
	window.Present = true
	window.UsedPercent = mathClampPercent(percent * 100)
	if reset, ok := qwenTokenPlanFloat(raw, prefix+"ResetTime"); ok {
		window.ResetAt = int64(reset)
	}
	return window
}

func qwenTokenPlanFloat(raw map[string]interface{}, key string) (float64, bool) {
	value, ok := raw[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(typed)), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	}
}

func mathClampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

// ---- console token 保活：AK/SK 换签短时 token，NotLogined 时作废重签 ----

var qwenConsoleTokenCache sync.Map // accessKeyID -> console access token

func cachedQwenConsoleToken(ctx context.Context, client *http.Client, accessKeyID string, accessKeySecret string) (string, error) {
	if cached, ok := qwenConsoleTokenCache.Load(accessKeyID); ok {
		if token, ok := cached.(string); ok && token != "" {
			return token, nil
		}
	}
	return invalidateQwenConsoleToken(ctx, client, accessKeyID, accessKeySecret)
}

func invalidateQwenConsoleToken(ctx context.Context, client *http.Client, accessKeyID string, accessKeySecret string) (string, error) {
	token, err := generateQwenConsoleToken(ctx, client, accessKeyID, accessKeySecret)
	if err != nil {
		return "", err
	}
	qwenConsoleTokenCache.Store(accessKeyID, token)
	return token, nil
}

func generateQwenConsoleToken(ctx context.Context, client *http.Client, accessKeyID string, accessKeySecret string) (string, error) {
	body := []byte{}
	headers := qwenAcs3SignHeaders(http.MethodPost, qwenTokenPlanTokenHost, qwenTokenPlanTokenPath, "",
		qwenTokenPlanTokenAction, qwenTokenPlanTokenVersion, accessKeyID, accessKeySecret, "", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, qwenTokenPlanTokenURL, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("generate access token status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload struct {
		CliAccessToken string `json:"cliAccessToken"`
	}
	if err := common.Unmarshal(responseBody, &payload); err != nil {
		return "", err
	}
	token := strings.TrimSpace(payload.CliAccessToken)
	if token == "" {
		return "", fmt.Errorf("generate access token response missing cliAccessToken")
	}
	return token, nil
}

// qwenAcs3SignHeaders 按阿里云 ACS3-HMAC-SHA256 规范生成签名请求头（时间与 nonce 自动生成）。
func qwenAcs3SignHeaders(method string, host string, pathName string, queryString string,
	action string, version string, accessKeyID string, accessKeySecret string, securityToken string, body []byte) map[string]string {
	return qwenAcs3Sign(method, host, pathName, queryString, action, version,
		accessKeyID, accessKeySecret, securityToken, body,
		time.Now().UTC().Format("2006-01-02T15:04:05Z"), uuid.NewString())
}

// qwenAcs3Sign 为 ACS3 签名核心，signedAt/nonce 由调用方注入以便测试与复现。
func qwenAcs3Sign(method string, host string, pathName string, queryString string,
	action string, version string, accessKeyID string, accessKeySecret string, securityToken string, body []byte,
	signedAt string, nonce string) map[string]string {
	payloadHash := sha256.Sum256(body)
	headers := map[string]string{
		"host":                  host,
		"x-acs-action":          action,
		"x-acs-version":         version,
		"x-acs-date":            signedAt,
		"x-acs-signature-nonce": nonce,
		"x-acs-content-sha256":  hex.EncodeToString(payloadHash[:]),
		"content-type":          "application/json",
	}
	if securityToken != "" {
		headers["x-acs-security-token"] = securityToken
	}

	signedKeys := make([]string, 0, len(headers))
	for key := range headers {
		if key == "host" || key == "content-type" || strings.HasPrefix(key, "x-acs-") {
			signedKeys = append(signedKeys, key)
		}
	}
	sort.Strings(signedKeys)

	canonicalHeaders := strings.Builder{}
	for _, key := range signedKeys {
		canonicalHeaders.WriteString(key)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(headers[key])
		canonicalHeaders.WriteString("\n")
	}
	canonicalRequest := strings.Join([]string{
		method,
		pathName,
		queryString,
		canonicalHeaders.String(),
		strings.Join(signedKeys, ";"),
		headers["x-acs-content-sha256"],
	}, "\n")

	canonicalHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "ACS3-HMAC-SHA256\n" + hex.EncodeToString(canonicalHash[:])
	mac := hmac.New(sha256.New, []byte(accessKeySecret))
	mac.Write([]byte(stringToSign))
	signature := hex.EncodeToString(mac.Sum(nil))

	headers["authorization"] = fmt.Sprintf(
		"ACS3-HMAC-SHA256 Credential=%s,SignedHeaders=%s,Signature=%s",
		accessKeyID, strings.Join(signedKeys, ";"), signature,
	)
	return headers
}
