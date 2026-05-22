package oauth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

const (
	qqTokenEndpoint    = "https://graph.qq.com/oauth2.0/token"
	qqOpenIDEndpoint   = "https://graph.qq.com/oauth2.0/me"
	qqUserInfoEndpoint = "https://graph.qq.com/user/get_user_info"
)

func init() {
	Register("qq", &QQProvider{})
}

// QQProvider implements OAuth for QQ Connect.
type QQProvider struct{}

type qqTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	ClientID     string `json:"client_id"`
	Error        any    `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

type qqUserInfo struct {
	Ret          int    `json:"ret"`
	Msg          string `json:"msg"`
	Nickname     string `json:"nickname"`
	FigureURL    string `json:"figureurl"`
	FigureURL1   string `json:"figureurl_1"`
	FigureURL2   string `json:"figureurl_2"`
	FigureURLQQ1 string `json:"figureurl_qq_1"`
	FigureURLQQ2 string `json:"figureurl_qq_2"`
	Gender       string `json:"gender"`
	Province     string `json:"province"`
	City         string `json:"city"`
	Year         string `json:"year"`
}

func (p *QQProvider) GetName() string {
	return "QQ"
}

func (p *QQProvider) IsEnabled() bool {
	return common.QQOAuthEnabled
}

func (p *QQProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	logger.LogDebug(ctx, "[OAuth-QQ] ExchangeToken: code=%s...", code[:min(len(code), 10)])

	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("client_id", common.QQClientId)
	values.Set("client_secret", common.QQClientSecret)
	values.Set("code", code)
	values.Set("redirect_uri", fmt.Sprintf("%s/oauth/qq", system_setting.ServerAddress))
	values.Set("fmt", "json")
	values.Set("need_openid", "1")

	reqURL := qqTokenEndpoint + "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] ExchangeToken error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "QQ"}, err.Error())
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] ExchangeToken read body error: %s", err.Error()))
		return nil, err
	}
	bodyStr := string(body)
	logger.LogDebug(ctx, "[OAuth-QQ] ExchangeToken response status=%d body=%s", res.StatusCode, bodyStr[:min(len(bodyStr), 500)])

	tokenResponse, err := parseQQTokenResponse(body)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] ExchangeToken parse error: %s", err.Error()))
		return nil, err
	}
	if common.Interface2String(tokenResponse.Error) != "" {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] ExchangeToken OAuth error: %s - %s", common.Interface2String(tokenResponse.Error), tokenResponse.ErrorDesc))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "QQ"}, tokenResponse.ErrorDesc)
	}
	if tokenResponse.AccessToken == "" {
		logger.LogError(ctx, "[OAuth-QQ] ExchangeToken failed: empty access token")
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "QQ"})
	}

	openID := tokenResponse.OpenID
	if openID == "" {
		openID, err = fetchQQOpenID(ctx, tokenResponse.AccessToken)
		if err != nil {
			return nil, err
		}
	}
	if openID == "" {
		logger.LogError(ctx, "[OAuth-QQ] ExchangeToken failed: empty openid")
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "QQ"})
	}

	logger.LogDebug(ctx, "[OAuth-QQ] ExchangeToken success")

	return &OAuthToken{
		AccessToken:  tokenResponse.AccessToken,
		TokenType:    "Bearer",
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresIn:    tokenResponse.ExpiresIn,
		Extra: map[string]any{
			"openid": openID,
		},
	}, nil
}

func (p *QQProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	openID, ok := token.Extra["openid"].(string)
	if !ok || openID == "" {
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "QQ"})
	}

	values := url.Values{}
	values.Set("access_token", token.AccessToken)
	values.Set("oauth_consumer_key", common.QQClientId)
	values.Set("openid", openID)

	reqURL := qqUserInfoEndpoint + "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] GetUserInfo error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "QQ"}, err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] GetUserInfo failed: status=%d", res.StatusCode))
		return nil, NewOAuthError(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": "QQ"})
	}

	var qqUser qqUserInfo
	if err := common.DecodeJson(res.Body, &qqUser); err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] GetUserInfo decode error: %s", err.Error()))
		return nil, err
	}
	if qqUser.Ret != 0 {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] GetUserInfo failed: ret=%d msg=%s", qqUser.Ret, qqUser.Msg))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": "QQ"}, qqUser.Msg)
	}
	if openID == "" {
		logger.LogError(ctx, "[OAuth-QQ] GetUserInfo failed: empty openid")
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "QQ"})
	}

	logger.LogDebug(ctx, "[OAuth-QQ] GetUserInfo success: openid=%s nickname=%s", openID, qqUser.Nickname)

	return &OAuthUser{
		ProviderUserID: openID,
		Username:       "",
		DisplayName:    qqUser.Nickname,
		Extra: map[string]any{
			"figureurl":      qqUser.FigureURL,
			"figureurl_1":    qqUser.FigureURL1,
			"figureurl_2":    qqUser.FigureURL2,
			"figureurl_qq_1": qqUser.FigureURLQQ1,
			"figureurl_qq_2": qqUser.FigureURLQQ2,
			"gender":         qqUser.Gender,
			"province":       qqUser.Province,
			"city":           qqUser.City,
			"year":           qqUser.Year,
		},
	}, nil
}

func (p *QQProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsQQIdAlreadyTaken(providerUserID)
}

func (p *QQProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.QQId = providerUserID
	return user.FillUserByQQId()
}

func (p *QQProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.QQId = providerUserID
}

func (p *QQProvider) GetProviderPrefix() string {
	return "qq_"
}

func parseQQTokenResponse(body []byte) (*qqTokenResponse, error) {
	var raw map[string]any
	if err := common.Unmarshal(body, &raw); err == nil {
		return &qqTokenResponse{
			AccessToken:  common.Interface2String(raw["access_token"]),
			ExpiresIn:    parseQQInt(raw["expires_in"]),
			RefreshToken: common.Interface2String(raw["refresh_token"]),
			OpenID:       common.Interface2String(raw["openid"]),
			ClientID:     common.Interface2String(raw["client_id"]),
			Error:        raw["error"],
			ErrorDesc:    common.Interface2String(raw["error_description"]),
		}, nil
	}

	parsedValues, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	tokenResponse := qqTokenResponse{}
	tokenResponse.AccessToken = parsedValues.Get("access_token")
	tokenResponse.RefreshToken = parsedValues.Get("refresh_token")
	tokenResponse.OpenID = parsedValues.Get("openid")
	tokenResponse.ClientID = parsedValues.Get("client_id")
	tokenResponse.Error = parsedValues.Get("error")
	tokenResponse.ErrorDesc = parsedValues.Get("error_description")
	tokenResponse.ExpiresIn = parseQQInt(parsedValues.Get("expires_in"))
	return &tokenResponse, nil
}

func fetchQQOpenID(ctx context.Context, accessToken string) (string, error) {
	values := url.Values{}
	values.Set("access_token", accessToken)
	values.Set("fmt", "json")

	reqURL := qqOpenIDEndpoint + "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] fetch openid error: %s", err.Error()))
		return "", NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "QQ"}, err.Error())
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	body = trimQQCallback(body)

	var openIDResponse map[string]any
	if err := common.Unmarshal(body, &openIDResponse); err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] fetch openid decode error: %s", err.Error()))
		return "", err
	}
	if errorCode := parseQQInt(openIDResponse["error"]); errorCode != 0 {
		errorDesc := common.Interface2String(openIDResponse["error_description"])
		logger.LogError(ctx, fmt.Sprintf("[OAuth-QQ] fetch openid failed: error=%d desc=%s", errorCode, errorDesc))
		return "", NewOAuthErrorWithRaw(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": "QQ"}, errorDesc)
	}
	return common.Interface2String(openIDResponse["openid"]), nil
}

func trimQQCallback(body []byte) []byte {
	raw := strings.TrimSpace(string(body))
	if strings.HasPrefix(raw, "callback(") && strings.HasSuffix(raw, ");") {
		raw = strings.TrimSuffix(strings.TrimPrefix(raw, "callback("), ");")
	}
	return []byte(strings.TrimSpace(raw))
}

func parseQQInt(value any) int {
	strValue := strings.TrimSpace(common.Interface2String(value))
	if strValue == "" {
		return 0
	}
	intValue, err := strconv.Atoi(strValue)
	if err == nil {
		return intValue
	}
	var floatValue float64
	if _, err := fmt.Sscanf(strValue, "%f", &floatValue); err == nil {
		return int(floatValue)
	}
	return 0
}
