package service

// MiniMax Code OAuth：设备码登录（PKCE）、令牌刷新与撤销，支持国内/海外双区。
// 协议逆向自 MiniMax Code 客户端（@mavis/oauth-core）。

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	MiniMaxCodeOAuthClientID = "mcode-public"
	MiniMaxCodeOAuthScope    = "agent.default"
	MiniMaxCodeOAuthAudience = "agent-backend"

	minimaxCodeDeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"
	minimaxCodeHTTPTimeout     = 20 * time.Second

	minimaxCodeDeviceCodePath = "/oauth2/device/code"
	minimaxCodeTokenPath      = "/oauth2/token"
	minimaxCodeRevokePath     = "/oauth2/revoke"
)

// MiniMaxCodeAccountOrigin 返回 OAuth 账户域名。
func MiniMaxCodeAccountOrigin(region string) string {
	if region == "global" {
		return "https://account.minimax.io"
	}
	return "https://account.minimax.cn"
}

type MiniMaxCodeTokenGrant struct {
	AccessToken  string
	RefreshToken string
	Scope        string
	Subject      string
	AccountID    string
	ExpiresAt    time.Time
}

// MiniMaxCodeDeviceFlow 是一次设备码授权会话。
type MiniMaxCodeDeviceFlow struct {
	Region          string
	DeviceCode      string
	UserCode        string
	VerificationURL string
	Verifier        string
	Interval        time.Duration
	ExpiresAt       time.Time
	UserCodePolling bool
}

type StartMiniMaxCodeDeviceFlowResult struct {
	Region                  string `json:"region"`
	UserCode                string `json:"user_code"`
	VerificationURL         string `json:"verification_url"`
	VerificationURLComplete string `json:"verification_url_complete"`
	IntervalMs              int64  `json:"interval_ms"`
	ExpiresInSec            int64  `json:"expires_in"`
}

func StartMiniMaxCodeDeviceFlow(ctx context.Context, proxyURL, region string) (*MiniMaxCodeDeviceFlow, *StartMiniMaxCodeDeviceFlowResult, error) {
	region = normalizeMiniMaxCodeRegion(region)
	verifier, err := randomBase64URL(32)
	if err != nil {
		return nil, nil, err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	form := url.Values{}
	form.Set("client_id", MiniMaxCodeOAuthClientID)
	form.Set("scope", MiniMaxCodeOAuthScope)
	form.Set("audience", MiniMaxCodeOAuthAudience)
	form.Set("code_challenge", challenge)
	form.Set("code_challenge_method", "S256")

	var payload map[string]any
	if err := doMiniMaxCodeFormRequest(ctx, proxyURL, http.MethodPost,
		MiniMaxCodeAccountOrigin(region)+minimaxCodeDeviceCodePath, form, &payload); err != nil {
		return nil, nil, err
	}

	userCode := payloadString(payload, "user_code")
	deviceCode := payloadString(payload, "device_code")
	verificationURL := payloadString(payload, "verification_uri")
	if verificationURL == "" {
		verificationURL = payloadString(payload, "verification_url")
	}
	if userCode == "" || verificationURL == "" {
		return nil, nil, errors.New("device authorization response missing fields")
	}

	// 账户侧可能不下发 device_code，此时用 user_code 轮询且 interval 单位为毫秒。
	userCodePolling := deviceCode == ""
	if userCodePolling {
		deviceCode = userCode
	}

	expiresInSec := payloadPositiveNumber(payload, "expires_in")
	if raw := payloadPositiveNumber(payload, "expired_in"); raw > 0 {
		// expired_in 为账户过期毫秒时间戳。
		if raw >= 1_000_000_000_000 {
			if remain := int64(time.Until(time.UnixMilli(raw)).Seconds()); remain > 0 {
				expiresInSec = remain
			}
		} else {
			expiresInSec = raw
		}
	}
	if expiresInSec <= 0 {
		expiresInSec = 600
	}

	intervalSec := int64(5)
	if raw := payloadPositiveNumber(payload, "interval"); raw > 0 {
		if userCodePolling {
			intervalSec = raw / 1_000
			if intervalSec <= 0 {
				intervalSec = 1
			}
		} else {
			intervalSec = raw
		}
	}

	flow := &MiniMaxCodeDeviceFlow{
		Region:          region,
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURL: verificationURL,
		Verifier:        verifier,
		Interval:        time.Duration(intervalSec) * time.Second,
		ExpiresAt:       time.Now().Add(time.Duration(expiresInSec) * time.Second),
		UserCodePolling: userCodePolling,
	}
	// 对齐官方桌面端跳转格式：
	// {verification_uri}?client_id=mcode-public&user_code=X&client_surface=desktop&download_source=default
	verificationURLComplete := verificationURL
	if userCode != "" {
		separator := "?"
		if strings.Contains(verificationURL, "?") {
			separator = "&"
		}
		verificationURLComplete = verificationURL + separator + url.Values{
			"client_id":       []string{MiniMaxCodeOAuthClientID},
			"user_code":       []string{userCode},
			"client_surface":  []string{"desktop"},
			"download_source": []string{"default"},
		}.Encode()
	}
	result := &StartMiniMaxCodeDeviceFlowResult{
		Region:                  region,
		UserCode:                userCode,
		VerificationURL:         verificationURL,
		VerificationURLComplete: verificationURLComplete,
		IntervalMs:              intervalSec * 1000,
		ExpiresInSec:            expiresInSec,
	}
	return flow, result, nil
}

// PollMiniMaxCodeDeviceFlow 单次轮询令牌端点；pending 返回 status=pending。
type MiniMaxCodePollStatus struct {
	Status string                 `json:"status"` // pending | ready
	Grant  *MiniMaxCodeTokenGrant `json:"-"`
}

func PollMiniMaxCodeDeviceFlow(ctx context.Context, proxyURL, region string, flow *MiniMaxCodeDeviceFlow) (*MiniMaxCodePollStatus, error) {
	if flow == nil || flow.DeviceCode == "" || flow.Verifier == "" {
		return nil, errors.New("device flow is not initialized")
	}
	if time.Now().After(flow.ExpiresAt) {
		return &MiniMaxCodePollStatus{Status: "expired"}, nil
	}
	region = normalizeMiniMaxCodeRegion(region)
	form := url.Values{}
	form.Set("grant_type", minimaxCodeDeviceGrantType)
	if flow.UserCodePolling {
		form.Set("user_code", flow.UserCode)
	} else {
		form.Set("device_code", flow.DeviceCode)
	}
	form.Set("client_id", MiniMaxCodeOAuthClientID)
	form.Set("code_verifier", flow.Verifier)

	var payload map[string]any
	err := doMiniMaxCodeFormRequest(ctx, proxyURL, http.MethodPost,
		MiniMaxCodeAccountOrigin(region)+minimaxCodeTokenPath, form, &payload, true)
	if err == nil {
		grant, grantErr := parseMiniMaxCodeTokenGrant(payload, "")
		if grantErr != nil {
			return nil, grantErr
		}
		return &MiniMaxCodePollStatus{Status: "ready", Grant: grant}, nil
	}

	var protoErr *miniMaxCodeOAuthError
	if errors.As(err, &protoErr) {
		switch protoErr.Code {
		case "authorization_pending":
			return &MiniMaxCodePollStatus{Status: "pending"}, nil
		case "slow_down":
			flow.Interval += 5 * time.Second
			return &MiniMaxCodePollStatus{Status: "pending"}, nil
		case "access_denied", "denied":
			return &MiniMaxCodePollStatus{Status: "denied"}, nil
		case "expired_token", "expired":
			return &MiniMaxCodePollStatus{Status: "expired"}, nil
		}
	}
	return nil, err
}

func RefreshMiniMaxCodeToken(ctx context.Context, proxyURL, region, refreshToken string) (*MiniMaxCodeTokenGrant, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, errors.New("empty refresh_token")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", MiniMaxCodeOAuthClientID)
	form.Set("scope", MiniMaxCodeOAuthScope)
	form.Set("audience", MiniMaxCodeOAuthAudience)

	var payload map[string]any
	if err := doMiniMaxCodeFormRequest(ctx, proxyURL, http.MethodPost,
		MiniMaxCodeAccountOrigin(normalizeMiniMaxCodeRegion(region))+minimaxCodeTokenPath, form, &payload); err != nil {
		return nil, err
	}
	return parseMiniMaxCodeTokenGrant(payload, refreshToken)
}

func RevokeMiniMaxCodeToken(ctx context.Context, proxyURL, region, refreshToken string) error {
	form := url.Values{}
	form.Set("token", refreshToken)
	form.Set("token_type_hint", "refresh_token")
	form.Set("client_id", MiniMaxCodeOAuthClientID)
	return doMiniMaxCodeFormRequest(ctx, proxyURL, http.MethodPost,
		MiniMaxCodeAccountOrigin(normalizeMiniMaxCodeRegion(region))+minimaxCodeRevokePath, form, nil)
}

func parseMiniMaxCodeTokenGrant(payload map[string]any, fallbackRefreshToken string) (*MiniMaxCodeTokenGrant, error) {
	accessToken := payloadString(payload, "access_token")
	refreshToken := payloadString(payload, "refresh_token")
	if refreshToken == "" {
		refreshToken = fallbackRefreshToken
	}
	tokenType := payloadString(payload, "token_type")
	expiresIn := payloadPositiveNumber(payload, "expires_in")
	if accessToken == "" || refreshToken == "" || expiresIn <= 0 ||
		(tokenType != "" && !strings.EqualFold(tokenType, "bearer")) {
		return nil, errors.New("invalid token response")
	}
	grant := &MiniMaxCodeTokenGrant{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Scope:        payloadString(payload, "scope"),
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	if claims, ok := decodeJWTClaimsMap(accessToken); ok {
		grant.Subject = claimsString(claims, "sub")
		grant.AccountID = claimsString(claims, "account_id")
		if grant.Scope == "" {
			grant.Scope = claimsString(claims, "scope")
		}
	}
	return grant, nil
}

type miniMaxCodeOAuthError struct {
	Code       string
	HTTPStatus int
}

func (e *miniMaxCodeOAuthError) Error() string {
	return fmt.Sprintf("minimax code oauth error %s (http %d)", e.Code, e.HTTPStatus)
}

func doMiniMaxCodeFormRequest(ctx context.Context, proxyURL, method, endpoint string, form url.Values, out *map[string]any, allowOAuthError ...bool) error {
	client, err := GetHttpClientWithProxy(strings.TrimSpace(proxyURL))
	if err != nil {
		return err
	}
	if client == nil {
		client = &http.Client{Timeout: minimaxCodeHTTPTimeout}
	} else {
		clientCopy := *client
		clientCopy.Timeout = minimaxCodeHTTPTimeout
		client = &clientCopy
	}

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return err
	}
	var payload map[string]any
	if common.Unmarshal(raw, &payload) != nil || payload == nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 && len(allowOAuthError) > 0 && allowOAuthError[0] {
			return nil // revoke 允许空响应
		}
		return fmt.Errorf("minimax code oauth invalid response (http %d)", resp.StatusCode)
	}
	if errCode := payloadString(payload, "error"); errCode != "" {
		return &miniMaxCodeOAuthError{Code: errCode, HTTPStatus: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("minimax code oauth request failed: http %d", resp.StatusCode)
	}
	if out != nil {
		*out = payload
	}
	return nil
}

func normalizeMiniMaxCodeRegion(region string) string {
	switch strings.ToLower(strings.TrimSpace(region)) {
	case "global", "io", "en", "overseas":
		return "global"
	default:
		return "cn"
	}
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func payloadPositiveNumber(payload map[string]any, key string) int64 {
	if payload == nil {
		return 0
	}
	switch v := payload[key].(type) {
	case float64:
		if v > 0 {
			return int64(v)
		}
	case json.Number:
		if n, err := v.Int64(); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func claimsString(claims map[string]any, key string) string {
	if v, ok := claims[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func decodeJWTClaimsMap(token string) (map[string]any, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, false
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false
	}
	var claims map[string]any
	if common.Unmarshal(payloadRaw, &claims) != nil {
		return nil, false
	}
	return claims, true
}

func randomBase64URL(byteCount int) (string, error) {
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
