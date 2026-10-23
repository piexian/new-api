package service

// MiniMax Code 账户类接口：身份/分组（matrix）、套餐余量（open platform）、
// 重置卡与积分（backend/account）、签到。签名算法逆向自官方客户端 transport.js。

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	minimaxCodeSignatureSalt = "I*7Cf%WZ#S&%1RlZJ&C2"
	minimaxCodeYYSuffix      = "ooui"

	minimaxCodeMatrixOriginCN     = "https://agent.minimax.cn"
	minimaxCodeMatrixOriginGlobal = "https://agent.minimax.io"
	minimaxCodeOpenOriginCN       = "https://www.minimaxi.com"
	minimaxCodeOpenOriginGlobal   = "https://platform.minimax.io"
)

// MiniMaxCodeMatrixOrigin 返回账户/签到等 matrix 域名。
func MiniMaxCodeMatrixOrigin(region string) string {
	if region == "global" {
		return minimaxCodeMatrixOriginGlobal
	}
	return minimaxCodeMatrixOriginCN
}

// MiniMaxCodeOpenPlatformOrigin 返回套餐余量等 open platform 域名。
func MiniMaxCodeOpenPlatformOrigin(region string) string {
	if region == "global" {
		return minimaxCodeOpenOriginGlobal
	}
	return minimaxCodeOpenOriginCN
}

type MiniMaxCodeAccountClient struct {
	client      *http.Client
	region      string
	accessToken string
	userID      string // realUserID，未知时为 0
}

func NewMiniMaxCodeAccountClient(proxyURL, region, accessToken, userID string) (*MiniMaxCodeAccountClient, error) {
	client, err := NewProxyHttpClient(proxyURL)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: minimaxCodeHTTPTimeout}
	} else {
		clientCopy := *client
		clientCopy.Timeout = minimaxCodeHTTPTimeout
		client = &clientCopy
	}
	return &MiniMaxCodeAccountClient{
		client:      client,
		region:      normalizeMiniMaxCodeRegion(region),
		accessToken: strings.TrimSpace(accessToken),
		userID:      strings.TrimSpace(userID),
	}, nil
}

type miniMaxCodeEnvelope struct {
	Data     json.RawMessage `json:"data"`
	BaseResp struct {
		StatusCode int64  `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
	StatusInfo struct {
		Code int64  `json:"code"`
		Msg  string `json:"msg"`
	} `json:"statusInfo"`
}

func (e *miniMaxCodeEnvelope) success() bool {
	return e.BaseResp.StatusCode == 0 && e.StatusInfo.Code == 0
}

func (e *miniMaxCodeEnvelope) message() string {
	if e.StatusInfo.Code != 0 && e.StatusInfo.Msg != "" {
		return e.StatusInfo.Msg
	}
	if e.BaseResp.StatusCode != 0 {
		if e.BaseResp.StatusMsg != "" {
			return e.BaseResp.StatusMsg
		}
		return fmt.Sprintf("status_code=%d", e.BaseResp.StatusCode)
	}
	return ""
}

// matrixSignedGet 发起带公共参数与签名的 GET 请求。
func (cl *MiniMaxCodeAccountClient) matrixSignedGet(ctx context.Context, path string, query url.Values, out any) error {
	if query == nil {
		query = url.Values{}
	}
	for k, v := range cl.commonQuery() {
		query[k] = v
	}
	return cl.doSignedJSON(ctx, http.MethodGet, MiniMaxCodeMatrixOrigin(cl.region)+path+"?"+query.Encode(), nil, out)
}

// matrixSignedPost 发起带公共参数与签名的 POST 请求。
func (cl *MiniMaxCodeAccountClient) matrixSignedPost(ctx context.Context, path string, payload any, out any) error {
	query := url.Values{}
	for k, v := range cl.commonQuery() {
		query[k] = v
	}
	return cl.doSignedJSON(ctx, http.MethodPost, MiniMaxCodeMatrixOrigin(cl.region)+path+"?"+query.Encode(), payload, out)
}

// backendSigned 调用 www.minimaxi.com /backend/account/*。
func (cl *MiniMaxCodeAccountClient) backendSigned(ctx context.Context, method, path string, payload any, out any) error {
	return cl.doSignedJSON(ctx, method, MiniMaxCodeOpenPlatformOrigin(cl.region)+path, payload, out)
}

// openPlatformGet 调用 coding_plan/remains（仅需 Bearer + X-Group-Id，无签名）。
func (cl *MiniMaxCodeAccountClient) openPlatformRemains(ctx context.Context, groupID string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		MiniMaxCodeOpenPlatformOrigin(cl.region)+"/v1/api/openplatform/coding_plan/remains", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cl.accessToken)
	req.Header.Set("X-Group-Id", groupID)
	return cl.doRequest(req, out)
}

func (cl *MiniMaxCodeAccountClient) commonQuery() url.Values {
	language := "zh"
	if cl.region == "global" {
		language = "en"
	}
	now := time.Now()
	query := url.Values{}
	query.Set("device_platform", "web")
	query.Set("biz_id", "3")
	query.Set("app_id", "3001")
	query.Set("version_code", "22201")
	query.Set("unix", strconv.FormatInt(now.UnixMilli(), 10))
	_, zoneOffset := now.Zone()
	query.Set("timezone_offset", strconv.Itoa(zoneOffset))
	query.Set("sys_language", language)
	query.Set("lang", language)
	query.Set("device_id", "0")
	query.Set("user_id", cl.userIDOrZero())
	query.Set("client", "desktop")
	query.Set("region", cl.region)
	query.Set("is_desktop", "1")
	return query
}

func (cl *MiniMaxCodeAccountClient) userIDOrZero() string {
	if cl.userID == "" {
		return "0"
	}
	return cl.userID
}

func (cl *MiniMaxCodeAccountClient) doSignedJSON(ctx context.Context, method, rawURL string, payload any, out any) error {
	var body []byte
	if payload != nil {
		encoded, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		body = encoded
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	nowMs := time.Now().UnixMilli()
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header = miniMaxCodeSignedHeaders(nowMs, parsed.Path+"?"+parsed.RawQuery, body)
	req.Header.Set("Authorization", "Bearer "+cl.accessToken)
	return cl.doRequest(req, out)
}

func (cl *MiniMaxCodeAccountClient) doRequest(req *http.Request, out any) error {
	resp, err := cl.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("minimax code account unauthorized (http 401)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("minimax code account request failed: http %d", resp.StatusCode)
	}
	var envelope miniMaxCodeEnvelope
	if err := common.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("minimax code account invalid response: %w", err)
	}
	if !envelope.success() {
		return fmt.Errorf("minimax code account error: %s", envelope.message())
	}
	if out == nil {
		return nil
	}
	// 实测：/v1/api/user/info 有 data 包装，get_user_extra_info / token_plan_credit /
	// reset_card / signin 的业务字段在顶层。优先取 data，缺失时回退整体 body。
	data := envelope.Data
	if len(data) == 0 {
		data = raw
	}
	return common.Unmarshal(data, out)
}

// miniMaxCodeSignedHeaders 计算官方客户端同款签名头。
// yy = md5(enc(path+query) + '_' + body或'{}' + md5(nowMs) + 'ooui')
// x-signature = md5(秒 + 盐 + body)
func miniMaxCodeSignedHeaders(nowMs int64, pathWithQuery string, body []byte) http.Header {
	bodyOrBraces := string(body)
	if bodyOrBraces == "" {
		bodyOrBraces = "{}"
	}
	second := strconv.FormatInt(nowMs/1000, 10)
	nowStr := strconv.FormatInt(nowMs, 10)
	header := http.Header{}
	header.Set("Accept", "application/json")
	header.Set("Content-Type", "application/json")
	header.Set("User-Agent", "MiniMaxAgent")
	header.Set("yy", md5Hex(encodeURIComponent(pathWithQuery)+"_"+bodyOrBraces+md5Hex(nowStr)+minimaxCodeYYSuffix))
	header.Set("x-timestamp", second)
	header.Set("x-signature", md5Hex(second+minimaxCodeSignatureSalt+string(body)))
	return header
}

// encodeURIComponent 对齐 JS encodeURIComponent。
func encodeURIComponent(s string) string {
	var b strings.Builder
	for _, r := range []byte(s) {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' || r == '!' || r == '~' || r == '*' || r == '\'' || r == '(' || r == ')' || r == '/' {
			b.WriteByte(r)
		} else {
			fmt.Fprintf(&b, "%%%02X", r)
		}
	}
	return b.String()
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ---- 高层账户操作 ----

type MiniMaxCodeIdentity struct {
	RealUserID string `json:"real_user_id"`
	UserID     string `json:"user_id"`
	UserName   string `json:"user_name"`
	Email      string `json:"email"`
	GroupID    string `json:"group_id"`
	// CreditBalance 取个人 workspace 的积分余额，字段优先级与官方客户端一致。
	CreditBalance string `json:"credit_balance"`
}

// FetchMiniMaxCodeIdentity 拉取身份与套餐分组（op_group_id）。
func (cl *MiniMaxCodeAccountClient) FetchMiniMaxCodeIdentity(ctx context.Context) (*MiniMaxCodeIdentity, error) {
	var info struct {
		UserInfo *struct {
			RealUserID      string `json:"realUserID"`
			RealUserIDSnake string `json:"real_user_id"`
			UserID          any    `json:"userID"`
			UserName        string `json:"userName"`
			UserNameSnake   string `json:"user_name"`
			Email           string `json:"email"`
		} `json:"userInfo"`
	}
	if err := cl.matrixSignedGet(ctx, "/v1/api/user/info", nil, &info); err != nil {
		return nil, err
	}
	identity := &MiniMaxCodeIdentity{}
	if info.UserInfo != nil {
		identity.RealUserID = firstNonEmpty(info.UserInfo.RealUserID, info.UserInfo.RealUserIDSnake)
		identity.UserID = anyToString(info.UserInfo.UserID)
		identity.UserName = firstNonEmpty(info.UserInfo.UserName, info.UserInfo.UserNameSnake)
		identity.Email = info.UserInfo.Email
	}
	if identity.RealUserID == "" {
		return nil, fmt.Errorf("minimax code identity missing real_user_id")
	}

	var extra struct {
		Workspaces []struct {
			WorkspaceID   any    `json:"workspace_id"`
			WorkspaceType any    `json:"workspace_type"`
			HasTokenPlan  any    `json:"has_token_plan"`
			OpGroupID     string `json:"op_group_id"`
			CreditSummary *struct {
				TotalRemainingAmount any `json:"total_remaining_amount"`
			} `json:"op_credit_summary"`
			OpCreditBalance    any `json:"opcredit_balance"`
			TotalRemainsCredit any `json:"total_remains_credit"`
		} `json:"workspaces"`
	}
	if err := cl.matrixSignedPost(ctx, "/matrix/api/v1/user/get_user_extra_info", struct{}{}, &extra); err == nil {
		for _, ws := range extra.Workspaces {
			if fmt.Sprintf("%v", ws.WorkspaceType) != "0" {
				continue
			}
			identity.GroupID = ws.OpGroupID
			var summaryAmount any
			if ws.CreditSummary != nil {
				summaryAmount = ws.CreditSummary.TotalRemainingAmount
			}
			identity.CreditBalance = firstNonEmpty(
				miniMaxCodeAmountString(summaryAmount),
				miniMaxCodeAmountString(ws.OpCreditBalance),
				miniMaxCodeAmountString(ws.TotalRemainsCredit),
			)
			break
		}
	}
	return identity, nil
}

// FetchMiniMaxCodeRemains 拉取套餐余量。实测 X-Group-Id 可为空（走默认个人分组）。
func (cl *MiniMaxCodeAccountClient) FetchMiniMaxCodeRemains(ctx context.Context, groupID string, out any) error {
	return cl.openPlatformRemains(ctx, strings.TrimSpace(groupID), out)
}

// ResetCardList / ResetCardUse / ResetCardFlows 重置卡。
// 实测 list 必须带 page+size 查询参数，否则上游返回 2013 invalid params。
func (cl *MiniMaxCodeAccountClient) ResetCardList(ctx context.Context, out any) error {
	return cl.backendSigned(ctx, http.MethodGet, "/backend/account/reset_card/list?page=1&size=20", nil, out)
}

// sourceType 原样透传卡片列表中的值，上游可能是数字也可能是字符串。
func (cl *MiniMaxCodeAccountClient) ResetCardUse(ctx context.Context, sourceType any, outBizNo string, out any) error {
	return cl.backendSigned(ctx, http.MethodPost, "/backend/account/reset_card/use", map[string]any{
		"source_type": sourceType,
		"out_biz_no":  outBizNo,
	}, out)
}

func (cl *MiniMaxCodeAccountClient) ResetCardFlows(ctx context.Context, out any) error {
	return cl.backendSigned(ctx, http.MethodGet, "/backend/account/reset_card/flows", nil, out)
}

// CreditPool / CreditUpdateFallback / CreditDetails 积分。
func (cl *MiniMaxCodeAccountClient) CreditPool(ctx context.Context, out any) error {
	return cl.backendSigned(ctx, http.MethodGet, "/backend/account/token_plan_credit", nil, out)
}

func (cl *MiniMaxCodeAccountClient) CreditUpdateFallback(ctx context.Context, enabled bool, out any) error {
	return cl.backendSigned(ctx, http.MethodPut, "/backend/account/token_plan_credit/fallback", map[string]bool{"enabled": enabled}, out)
}

func (cl *MiniMaxCodeAccountClient) CreditDetails(ctx context.Context, out any) error {
	query := cl.commonQuery()
	return cl.signedGetWithQuery(ctx, MiniMaxCodeMatrixOrigin(cl.region)+"/minimax-cloud/api/v1/credit/details?"+query.Encode(), out)
}

func (cl *MiniMaxCodeAccountClient) SigninStatus(ctx context.Context, out any) error {
	query := cl.commonQuery()
	return cl.signedGetWithQuery(ctx, MiniMaxCodeMatrixOrigin(cl.region)+"/minimax-cloud/api/v1/signin/status?"+query.Encode(), out)
}

func (cl *MiniMaxCodeAccountClient) SigninClaim(ctx context.Context, out any) error {
	query := cl.commonQuery()
	return cl.doSignedJSON(ctx, http.MethodPost, MiniMaxCodeMatrixOrigin(cl.region)+"/minimax-cloud/api/v1/signin/claim?"+query.Encode(), struct{}{}, out)
}

func (cl *MiniMaxCodeAccountClient) signedGetWithQuery(ctx context.Context, rawURL string, out any) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	nowMs := time.Now().UnixMilli()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header = miniMaxCodeSignedHeaders(nowMs, parsed.Path+"?"+parsed.RawQuery, nil)
	req.Header.Set("Authorization", "Bearer "+cl.accessToken)
	return cl.doRequest(req, out)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// miniMaxCodeAmountString 保留积分金额的小数位；上游可能返回数字或字符串。
func miniMaxCodeAmountString(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case json.Number:
		return value.String()
	default:
		return ""
	}
}

func anyToString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		return strconv.FormatInt(int64(value), 10)
	case json.Number:
		return value.String()
	default:
		return ""
	}
}

// MiniMaxCodeCreditFallbackEnabled 查询积分池，判断积分兜底是否开启。
// token_plan_credit_fallback_enabled 为实测字段名；其余键防御式保留。
func MiniMaxCodeCreditFallbackEnabled(ctx context.Context, client *MiniMaxCodeAccountClient) bool {
	if client == nil {
		return false
	}
	fallbackCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var pool map[string]any
	if err := client.CreditPool(fallbackCtx, &pool); err != nil {
		return false
	}
	for _, key := range []string{"token_plan_credit_fallback_enabled", "fallback_enabled", "fallback", "use_fallback", "enable_fallback", "use_credit_fallback"} {
		if v, ok := pool[key].(bool); ok && v {
			return true
		}
	}
	return false
}
