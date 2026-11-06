package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const loginUAGuardBrowserAgent = "Mozilla/5.0 (Test Browser)"

func setupLoginUAGuardTestDB(t *testing.T) {
	t.Helper()
	require.NoError(t, newapii18n.Init())
	_, db := setupLoginAuditControllerTest(t, false)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.RiskBanLog{}, &model.OAuthOwnershipTransfer{}))
}

func performLoginUAGuardRequest(
	t *testing.T,
	router *gin.Engine,
	method string,
	path string,
	body string,
	userAgent string,
	cookies []*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	// 与 RawRequest 一致：按测试名派生来源 IP，避免复用其他测试的预拦截聚合桶。
	req.RemoteAddr = "[" + loginAuditTestSourceIP(t) + "]:5555"
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// loginUAGuardFakeProvider 是可控的 OAuth provider，用于断言空 UA 不会兑换授权码。
type loginUAGuardFakeProvider struct {
	name          string
	enabled       bool
	exchangeCalls atomic.Int64
	userInfoCalls atomic.Int64
}

func (p *loginUAGuardFakeProvider) GetName() string           { return "FakeOAuth" }
func (p *loginUAGuardFakeProvider) GetProviderPrefix() string { return "fakeoauth_" }
func (p *loginUAGuardFakeProvider) IsEnabled() bool           { return p.enabled }

func (p *loginUAGuardFakeProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	p.exchangeCalls.Add(1)
	return &oauth.OAuthToken{AccessToken: "fake-token"}, nil
}

func (p *loginUAGuardFakeProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	p.userInfoCalls.Add(1)
	return &oauth.OAuthUser{ProviderUserID: "fake-oauth-user-1", Username: "fake-oauth-user-1"}, nil
}

func (p *loginUAGuardFakeProvider) IsUserIDTaken(providerUserID string) bool {
	return providerUserID == "fake-oauth-user-1"
}

func (p *loginUAGuardFakeProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	return model.DB.Where("oidc_id = ?", providerUserID).First(user).Error
}

func (p *loginUAGuardFakeProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.OidcId = providerUserID
}

func registerLoginUAGuardFakeProvider(t *testing.T, provider *loginUAGuardFakeProvider) {
	t.Helper()
	oauth.Register(provider.name, provider)
	t.Cleanup(func() { oauth.Unregister(provider.name) })
}

// 空 UA 的 OAuth 登录不得兑换授权码、读上游或创建账号；带 UA 正常登录，绑定分支不受影响。
func TestHandleOAuthBlocksMissingUserAgentBeforeCodeExchange(t *testing.T) {
	setupLoginUAGuardTestDB(t)
	provider := &loginUAGuardFakeProvider{name: "loginuaoauth", enabled: true}
	registerLoginUAGuardFakeProvider(t, provider)

	store := cookie.NewStore([]byte("login-ua-oauth"))
	router := gin.New()
	router.Use(middleware.RequestId(), middleware.LoginAudit(), sessions.Sessions("session", store))
	router.POST("/seed", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("oauth_state", "valid")
		require.NoError(t, session.Save())
		c.Status(http.StatusOK)
	})
	router.GET("/api/oauth/:provider", HandleOAuth)

	// 预置一个已绑定的账号，带 UA 时应能直接登录。
	bound := createLoginAuditTestUser(t, model.DB, "fake_oauth_bound", common.UserStatusEnabled)
	require.NoError(t, model.DB.Model(bound).Update("oidc_id", "fake-oauth-user-1").Error)

	seedRecorder := httptest.NewRecorder()
	router.ServeHTTP(seedRecorder, httptest.NewRequest(http.MethodPost, "/seed", nil))
	cookies := seedRecorder.Result().Cookies()
	require.NotEmpty(t, cookies)

	// 1) 空 UA：在 ExchangeToken 之前被拦下。
	blocked := performLoginUAGuardRequest(t, router, http.MethodGet, "/api/oauth/loginuaoauth?state=valid&code=code", "", "", cookies)
	require.Contains(t, blocked.Body.String(), `"success":false`)
	require.Contains(t, blocked.Body.String(), "浏览器环境异常")
	require.EqualValues(t, 0, provider.exchangeCalls.Load())
	require.EqualValues(t, 0, provider.userInfoCalls.Load())
	require.Empty(t, loginAuditSuccessRows(t, model.DB))

	rows := loginAuditSecurityRows(t, model.DB)
	require.Len(t, rows, 1)
	audit := loginAuditRowAudit(t, rows[0])
	require.Equal(t, "blocked", audit["outcome"])
	require.Equal(t, "precheck", audit["stage"])
	require.Equal(t, "user_agent_missing", audit["reason"])
	require.Equal(t, "oauth:loginuaoauth", loginAuditOtherMap(t, rows[0])["login_method"])

	// 2) 同一会话带 UA：正常兑换并登录。
	ok := performLoginUAGuardRequest(t, router, http.MethodGet, "/api/oauth/loginuaoauth?state=valid&code=code", "", loginUAGuardBrowserAgent, cookies)
	require.Contains(t, ok.Body.String(), `"success":true`, ok.Body.String())
	require.EqualValues(t, 1, provider.exchangeCalls.Load())
	require.EqualValues(t, 1, provider.userInfoCalls.Load())
	require.Len(t, loginAuditSuccessRows(t, model.DB), 1)
}

// 绑定分支在 UA 守卫之前，空 UA 的绑定流程不能被挡住。
func TestHandleOAuthBindIsNotBlockedByMissingUserAgent(t *testing.T) {
	setupLoginUAGuardTestDB(t)
	provider := &loginUAGuardFakeProvider{name: "loginuaoauthbind", enabled: true}
	registerLoginUAGuardFakeProvider(t, provider)
	bound := createLoginAuditTestUser(t, model.DB, "fake_oauth_bind_actor", common.UserStatusEnabled)

	store := cookie.NewStore([]byte("login-ua-oauth-bind"))
	router := gin.New()
	router.Use(middleware.RequestId(), middleware.LoginAudit(), sessions.Sessions("session", store))
	router.POST("/seed", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("oauth_state", "valid")
		session.Set("id", bound.Id)
		session.Set("username", bound.Username)
		require.NoError(t, session.Save())
		c.Status(http.StatusOK)
	})
	router.GET("/api/oauth/:provider", HandleOAuth)

	seedRecorder := httptest.NewRecorder()
	router.ServeHTTP(seedRecorder, httptest.NewRequest(http.MethodPost, "/seed", nil))
	cookies := seedRecorder.Result().Cookies()
	require.NotEmpty(t, cookies)

	recorder := performLoginUAGuardRequest(t, router, http.MethodGet, "/api/oauth/loginuaoauthbind?state=valid&code=code", "", "", cookies)
	require.NotContains(t, recorder.Body.String(), "浏览器环境异常")
	require.EqualValues(t, 1, provider.exchangeCalls.Load())
	require.Empty(t, loginAuditSecurityRows(t, model.DB))
}

// 归属转移确认：空 UA 必须在任何 Expire/Claim/Verify/Complete 之前拒绝。
func TestConfirmOAuthOwnershipTransferBlocksMissingUserAgentWithoutSideEffects(t *testing.T) {
	setupLoginUAGuardTestDB(t)
	db := model.DB
	challenge, previous, target := prepareControllerOAuthOwnershipTransfer(t, db, common.GetTimestamp())
	common.RegisterVerificationCodeWithKey(challenge.PairKey, "abc123", common.OAuthOwnershipTransferPurpose)
	t.Cleanup(func() {
		common.DeleteKey(challenge.PairKey, common.OAuthOwnershipTransferPurpose)
	})

	store := cookie.NewStore([]byte("login-ua-ownership"))
	router := gin.New()
	router.Use(middleware.RequestId(), middleware.LoginAudit(), sessions.Sessions("session", store))
	router.POST("/seed", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set(oauthOwnershipTransferSessionKey, challenge.Id)
		require.NoError(t, session.Save())
		c.Status(http.StatusOK)
	})
	router.POST("/api/oauth/ownership/confirm", ConfirmOAuthOwnershipTransfer)

	seedRecorder := httptest.NewRecorder()
	router.ServeHTTP(seedRecorder, httptest.NewRequest(http.MethodPost, "/seed", nil))
	cookies := seedRecorder.Result().Cookies()
	require.NotEmpty(t, cookies)

	// 1) 空 UA：拒绝且不产生任何副作用。
	blocked := performLoginUAGuardRequest(t, router, http.MethodPost, "/api/oauth/ownership/confirm", `{"code":"abc123"}`, "", cookies)
	require.Contains(t, blocked.Body.String(), `"success":false`)
	require.Contains(t, blocked.Body.String(), "浏览器环境异常")
	// 不发放新会话 cookie，也不清掉确认会话。
	require.Empty(t, blocked.Header().Values("Set-Cookie"))

	storedChallenge, err := model.GetOAuthOwnershipTransferById(challenge.Id)
	require.NoError(t, err)
	require.Equal(t, model.OAuthOwnershipTransferStatusCodeSent, storedChallenge.Status)
	require.Zero(t, storedChallenge.FailedAttempts)
	require.Zero(t, storedChallenge.AttemptedAt)
	require.Zero(t, storedChallenge.CompletedAt)
	// 验证码未被消耗。
	require.True(t, common.VerifyCodeWithKey(challenge.PairKey, "abc123", common.OAuthOwnershipTransferPurpose))

	var storedPrevious, storedTarget model.User
	require.NoError(t, db.First(&storedPrevious, previous.Id).Error)
	require.NoError(t, db.First(&storedTarget, target.Id).Error)
	require.Equal(t, common.UserStatusEnabled, storedPrevious.Status)
	require.Equal(t, previous.Email, storedPrevious.Email)
	require.Empty(t, storedTarget.Email)

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditRowAudit(t, rows[0])
	require.Equal(t, "blocked", audit["outcome"])
	require.Equal(t, "precheck", audit["stage"])
	require.Equal(t, "user_agent_missing", audit["reason"])
	// 已知目标只写 admin_info，且不作为 actor。
	require.Equal(t, float64(target.Id), loginAuditRowAdminInfo(t, rows[0])["target_user_id"])
	require.Equal(t, 0, rows[0].UserId)

	// 2) 同一挑战带 UA 重试：仍可完成转移并建立会话。
	ok := performLoginUAGuardRequest(t, router, http.MethodPost, "/api/oauth/ownership/confirm", `{"code":"abc123"}`, loginUAGuardBrowserAgent, cookies)
	require.Contains(t, ok.Body.String(), `"success":true`, ok.Body.String())
	require.NotEmpty(t, ok.Header().Values("Set-Cookie"))

	require.NoError(t, db.First(&storedPrevious, previous.Id).Error)
	require.NoError(t, db.First(&storedTarget, target.Id).Error)
	require.Equal(t, common.UserStatusDisabled, storedPrevious.Status)
	require.Empty(t, storedPrevious.Email)
	require.Equal(t, previous.Email, storedTarget.Email)
}

// Passkey / Telegram / 微信登录入口：空 UA 在创建挑战、消费外部 code 之前被拒。
func TestLoginEntriesBlockMissingUserAgentBeforeSideEffects(t *testing.T) {
	require.NoError(t, newapii18n.Init())
	previousPasskey := system_setting.PasskeySettingsSnapshot().Enabled
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"passkey.enabled": "true"}))
	previousTelegram := common.TelegramOAuthEnabled
	previousWeChat := common.WeChatAuthEnabled
	common.TelegramOAuthEnabled = true
	common.WeChatAuthEnabled = true
	t.Cleanup(func() {
		restore := "false"
		if previousPasskey {
			restore = "true"
		}
		_ = config.GlobalConfig.LoadFromDB(map[string]string{"passkey.enabled": restore})
		common.TelegramOAuthEnabled = previousTelegram
		common.WeChatAuthEnabled = previousWeChat
	})

	cases := []struct {
		name        string
		method      string
		path        string
		loginMethod string
	}{
		{"passkey_begin", http.MethodPost, "/api/user/passkey/login/begin", "passkey"},
		{"passkey_finish", http.MethodPost, "/api/user/passkey/login/finish", "passkey"},
		{"telegram", http.MethodGet, "/api/oauth/telegram/login", "telegram"},
		{"wechat", http.MethodGet, "/api/oauth/wechat?code=external-code", "wechat"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 每个子测试独立 DB 与路由：前一个子测试失败也不会污染后一个。
			setupLoginUAGuardTestDB(t)
			db := model.DB

			router := gin.New()
			router.Use(middleware.RequestId(), middleware.LoginAudit(),
				sessions.Sessions("session", cookie.NewStore([]byte("login-ua-entries"))))
			router.POST("/api/user/passkey/login/begin", PasskeyLoginBegin)
			router.POST("/api/user/passkey/login/finish", PasskeyLoginFinish)
			router.GET("/api/oauth/telegram/login", TelegramLogin)
			router.GET("/api/oauth/wechat", WeChatAuth)

			recorder := performLoginUAGuardRequest(t, router, tc.method, tc.path, "", "", nil)
			require.Contains(t, recorder.Body.String(), `"success":false`, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), "浏览器环境异常")
			// 拒绝前不创建挑战、不发放会话 cookie。
			require.Empty(t, recorder.Header().Values("Set-Cookie"))

			middleware.FlushLoginAudit()
			var rows []model.Log
			require.NoError(t, db.Where("type = ?", model.LogTypeLoginSecurity).Order("id ASC").Find(&rows).Error)
			require.Len(t, rows, 1)
			audit := loginAuditRowAudit(t, rows[0])
			require.Equal(t, "blocked", audit["outcome"])
			require.Equal(t, "precheck", audit["stage"])
			require.Equal(t, "user_agent_missing", audit["reason"])
			require.Equal(t, tc.loginMethod, loginAuditOtherMap(t, rows[0])["login_method"])
		})
	}
}
