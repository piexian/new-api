package controller

import (
	"crypto/sha256"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	gorillasessions "github.com/gorilla/sessions"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const loginAuditTestPassword = "login-audit-password"

// loginAuditFailingStore 只让会话保存失败：gin 的 session.Save 走
// gorilla Session.Save → store.Save，因此在这里返回错误即可。
type loginAuditFailingStore struct {
	sessions.Store
}

func (store loginAuditFailingStore) Get(_ *http.Request, name string) (*gorillasessions.Session, error) {
	return gorillasessions.NewSession(store, name), nil
}

func (store loginAuditFailingStore) New(_ *http.Request, name string) (*gorillasessions.Session, error) {
	return gorillasessions.NewSession(store, name), nil
}

func (store loginAuditFailingStore) Save(_ *http.Request, _ http.ResponseWriter, _ *gorillasessions.Session) error {
	return errors.New("session store unavailable")
}

func setupLoginAuditControllerTest(t *testing.T, failingSession bool) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "login.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TwoFA{}, &model.TwoFABackupCode{}, &model.Log{}))

	previousDB, previousLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, db
	previousPasswordLogin := common.PasswordLoginEnabled
	previousEncryption := common.PasswordLoginEncryptionEnabled
	previousTurnstile := common.TurnstileLoginEnabled
	common.PasswordLoginEnabled = true
	common.PasswordLoginEncryptionEnabled = false
	common.TurnstileLoginEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.PasswordLoginEnabled = previousPasswordLogin
		common.PasswordLoginEncryptionEnabled = previousEncryption
		common.TurnstileLoginEnabled = previousTurnstile
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	store := cookie.NewStore([]byte("login-audit-secret"))
	var sessionStore sessions.Store = store
	if failingSession {
		sessionStore = loginAuditFailingStore{Store: store}
	}

	router := gin.New()
	router.Use(middleware.RequestId(), middleware.LoginAudit(), sessions.Sessions("session", sessionStore))
	router.POST("/api/user/login", Login)
	router.POST("/api/user/login/2fa", Verify2FALogin)
	return router, db
}

func createLoginAuditTestUser(t *testing.T, db *gorm.DB, username string, status int) *model.User {
	t.Helper()
	hashed, err := common.Password2Hash(loginAuditTestPassword)
	require.NoError(t, err)
	user := &model.User{
		Username: username,
		Password: hashed,
		Status:   status,
		Role:     common.RoleCommonUser,
		AffCode:  strings.ToUpper(username),
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func performLoginAuditLogin(t *testing.T, router *gin.Engine, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LoginAuditTest/1.0")
	req.Header.Set("CF-Ray", "cafe1234beef5678")
	req.RemoteAddr = "198.51.100.90:5555"
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func performLoginAuditTwoFactor(t *testing.T, router *gin.Engine, code string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/user/login/2fa", strings.NewReader(`{"code":"`+code+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LoginAuditTest/1.0")
	req.RemoteAddr = "198.51.100.90:5555"
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func loginAuditSecurityRows(t *testing.T, db *gorm.DB) []model.Log {
	t.Helper()
	middleware.FlushLoginAudit()
	var rows []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeLoginSecurity).Order("id ASC").Find(&rows).Error)
	return rows
}

func loginAuditSuccessRows(t *testing.T, db *gorm.DB) []model.Log {
	t.Helper()
	var rows []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeLogin).Order("id ASC").Find(&rows).Error)
	return rows
}

func loginAuditRowAudit(t *testing.T, log model.Log) map[string]interface{} {
	t.Helper()
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	audit, ok := other["auth_audit"].(map[string]interface{})
	require.True(t, ok)
	return audit
}

func loginAuditRowAdminInfo(t *testing.T, log model.Log) map[string]interface{} {
	t.Helper()
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	return adminInfo
}

func loginAuditRowOther(t *testing.T, log model.Log) map[string]interface{} {
	t.Helper()
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	return other
}

// installHasUserPasswordFailure 让登录成功后 HasUserPassword 查询失败，
// 用来验证「已建立会话后的资料查询错误不补记口令失败」。
func installHasUserPasswordFailure(t *testing.T, db *gorm.DB) {
	t.Helper()
	const callbackName = "test:fail_has_user_password"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Model.(*model.User); !ok {
			return
		}
		for _, column := range tx.Statement.Selects {
			if strings.EqualFold(strings.TrimSpace(column), "password") {
				_ = tx.AddError(errors.New("users.password unavailable in test"))
				return
			}
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(callbackName)
	})
}

func TestLoginAuditRecordsWrongPasswordOnce(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "audit_wrong_password", common.UserStatusEnabled)

	recorder := performLoginAuditLogin(t, router, `{"username":"audit_wrong_password","password":"wrong-password"}`, nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	log := rows[0]
	require.Equal(t, 0, log.UserId)
	require.NotEqual(t, 0, user.Id)
	require.Equal(t, recorder.Header().Get(common.RequestIdKey), log.RequestId)
	require.Equal(t, "198.51.100.90", log.Ip)
	require.Equal(t, "LoginAuditTest/1.0", log.UserAgent)

	audit := loginAuditRowAudit(t, log)
	require.Equal(t, "failure", audit["outcome"])
	require.Equal(t, "credentials", audit["stage"])
	require.Equal(t, "invalid_credentials", audit["reason"])
	require.EqualValues(t, 1, audit["count"])

	// 密码错误也必须记录已查到的 UID 与规范用户名，便于按 UID 聚合。
	adminInfo := loginAuditRowAdminInfo(t, log)
	require.Equal(t, float64(user.Id), adminInfo["target_user_id"])
	require.Equal(t, "audit_wrong_password", adminInfo["target_username"])
	require.Equal(t, "audit_wrong_password", adminInfo["attempted_identifier"])

	require.Empty(t, loginAuditSuccessRows(t, db))
	// 失败事件同样记录不含 query 的请求路径。
	require.Equal(t, "/api/user/login", loginAuditRowOther(t, log)["request_path"])
	// 口令本身绝不会落库。
	require.NotContains(t, strings.ToLower(log.Other), "wrong-password")
}

func TestLoginAuditRecordsUnknownAndDisabledAccounts(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	disabled := createLoginAuditTestUser(t, db, "audit_disabled_user", common.UserStatusDisabled)

	performLoginAuditLogin(t, router, `{"username":"audit_missing_user","password":"whatever"}`, nil)
	performLoginAuditLogin(t, router, `{"username":"audit_disabled_user","password":"`+loginAuditTestPassword+`"}`, nil)

	middleware.FlushLoginAudit()
	var rows []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeLoginSecurity).Order("id ASC").Find(&rows).Error)
	require.Len(t, rows, 2)

	notFound := loginAuditRowAudit(t, rows[0])
	require.Equal(t, "user_not_found", notFound["reason"])
	require.Equal(t, "audit_missing_user", loginAuditRowAdminInfo(t, rows[0])["attempted_identifier"])

	disabledAudit := loginAuditRowAudit(t, rows[1])
	require.Equal(t, "blocked", disabledAudit["outcome"])
	require.Equal(t, "account_disabled", disabledAudit["reason"])
	adminInfo := loginAuditRowAdminInfo(t, rows[1])
	require.Equal(t, float64(disabled.Id), adminInfo["target_user_id"])
	require.Equal(t, "audit_disabled_user", adminInfo["target_username"])
	// 目标账号只作为目标，actor 永远不是真实用户。
	require.Equal(t, 0, rows[1].UserId)
}

func TestLoginAuditRequiresEncryptedPasswordWithoutRecordingSecret(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	createLoginAuditTestUser(t, db, "audit_encryption_user", common.UserStatusEnabled)

	previousEncryption := common.PasswordLoginEncryptionEnabled
	common.PasswordLoginEncryptionEnabled = true
	t.Cleanup(func() { common.PasswordLoginEncryptionEnabled = previousEncryption })

	performLoginAuditLogin(t, router, `{"username":"audit_encryption_user","password":"plain-secret"}`, nil)

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditRowAudit(t, rows[0])
	// 字段校验问题不是凭证不匹配。
	require.Equal(t, "blocked", audit["outcome"])
	require.Equal(t, "request", audit["stage"])
	require.Equal(t, "password_encryption_required", audit["reason"])
	// 解析成功即记录输入账号，加密字段缺失也查得到账号。
	require.Equal(t, "audit_encryption_user", loginAuditRowAdminInfo(t, rows[0])["attempted_identifier"])
	require.NotContains(t, rows[0].Other, "plain-secret")
}

func TestLoginAuditTwoFactorChallengeIsNotAFailureOrSuccess(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "audit_2fa_user", common.UserStatusEnabled)
	require.NoError(t, db.Create(&model.TwoFA{UserId: user.Id, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true}).Error)

	recorder := performLoginAuditLogin(t, router, `{"username":"audit_2fa_user","password":"`+loginAuditTestPassword+`"}`, nil)
	require.Contains(t, recorder.Body.String(), "require_2fa")

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditRowAudit(t, rows[0])
	require.Equal(t, "challenge", audit["outcome"])
	require.Equal(t, "two_factor", audit["stage"])
	require.Equal(t, "two_factor_required", audit["reason"])
	require.Equal(t, float64(user.Id), loginAuditRowAdminInfo(t, rows[0])["target_user_id"])
	// 等待 2FA 既不是成功登录，也不是口令失败。
	require.Empty(t, loginAuditSuccessRows(t, db))
}

func TestLoginAuditTwoFactorInvalidCodeAndBackupCodeSuccess(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "audit_2fa_backup", common.UserStatusEnabled)
	require.NoError(t, db.Create(&model.TwoFA{UserId: user.Id, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true}).Error)
	// 备用码固定 8 位，形如 ABCD-1234。
	require.NoError(t, model.CreateBackupCodes(user.Id, []string{"ABCD1234"}))

	challenge := performLoginAuditLogin(t, router, `{"username":"audit_2fa_backup","password":"`+loginAuditTestPassword+`"}`, nil)
	cookies := challenge.Result().Cookies()
	require.NotEmpty(t, cookies)

	invalid := performLoginAuditTwoFactor(t, router, "000000", cookies)
	require.Contains(t, invalid.Body.String(), `"success":false`)

	success := performLoginAuditTwoFactor(t, router, "ABCD1234", cookies)
	require.Contains(t, success.Body.String(), `"success":true`)

	middleware.FlushLoginAudit()
	var securityRows []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeLoginSecurity).Order("id ASC").Find(&securityRows).Error)
	// challenge + 一次失败；成功不会再写 type=8。
	require.Len(t, securityRows, 2)
	require.Equal(t, "challenge", loginAuditRowAudit(t, securityRows[0])["outcome"])
	invalidAudit := loginAuditRowAudit(t, securityRows[1])
	require.Equal(t, "failure", invalidAudit["outcome"])
	require.Equal(t, "two_factor", invalidAudit["stage"])
	require.Equal(t, "two_factor_invalid", invalidAudit["reason"])
	require.Equal(t, float64(user.Id), loginAuditRowAdminInfo(t, securityRows[1])["target_user_id"])
	require.Equal(t, "/api/user/login/2fa", loginAuditRowOther(t, securityRows[1])["request_path"])

	successRows := loginAuditSuccessRows(t, db)
	require.Len(t, successRows, 1)
	require.Equal(t, user.Id, successRows[0].UserId)
	require.NotEmpty(t, successRows[0].RequestId)
	// 成功日志保留原有的登录方式元数据，并补充管理员诊断字段。
	require.Contains(t, successRows[0].Other, "login_method")
	require.Equal(t, "/api/user/login/2fa", loginAuditRowOther(t, successRows[0])["request_path"])
	adminInfo := loginAuditRowAdminInfo(t, successRows[0])
	require.NotEmpty(t, adminInfo["peer_ip"])
}

func TestLoginAuditTwoFactorSessionExpired(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)

	recorder := performLoginAuditTwoFactor(t, router, "000000", nil)
	require.Contains(t, recorder.Body.String(), "会话已过期")

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditRowAudit(t, rows[0])
	// 失效的 pending 会话不是验证码失败。
	require.Equal(t, "blocked", audit["outcome"])
	require.Equal(t, "two_factor", audit["stage"])
	require.Equal(t, "two_factor_session_expired", audit["reason"])
}

func TestLoginAuditSessionSaveFailure(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, true)
	createLoginAuditTestUser(t, db, "audit_session_user", common.UserStatusEnabled)

	performLoginAuditLogin(t, router, `{"username":"audit_session_user","password":"`+loginAuditTestPassword+`"}`, nil)

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditRowAudit(t, rows[0])
	// 会话存储故障是内部错误，不计入凭证失败。
	require.Equal(t, "error", audit["outcome"])
	require.Equal(t, "session", audit["stage"])
	require.Equal(t, "session_save_failed", audit["reason"])
	// 会话没有建立，就不存在成功登录日志。
	require.Empty(t, loginAuditSuccessRows(t, db))
}

func TestLoginAuditScopeIsolationAndRejectsInvalidScope(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	createLoginAuditTestUser(t, db, "audit_scope_user", common.UserStatusEnabled)

	performLoginAuditLogin(t, router, `{"username":"audit_scope_user","password":"wrong"}`, nil)
	middleware.FlushLoginAudit()

	gin.SetMode(gin.TestMode)
	adminRouter := gin.New()
	adminRouter.GET("/api/log/", func(c *gin.Context) {
		c.Set("id", 1)
		c.Set("role", common.RoleRootUser)
		GetAllLogs(c)
	})
	adminRouter.GET("/api/log/self", func(c *gin.Context) {
		c.Set("id", 3)
		c.Set("role", common.RoleCommonUser)
		GetUserLogs(c)
	})

	adminAll := httptest.NewRecorder()
	adminRouter.ServeHTTP(adminAll, httptest.NewRequest(http.MethodGet, "/api/log/?scope=login&page_size=50", nil))
	require.Contains(t, adminAll.Body.String(), `"success":true`)
	require.Contains(t, adminAll.Body.String(), `"type":8`)

	adminConsume := httptest.NewRecorder()
	adminRouter.ServeHTTP(adminConsume, httptest.NewRequest(http.MethodGet, "/api/log/?scope=login&type=2", nil))
	require.Contains(t, adminConsume.Body.String(), `"total":0`)

	self := httptest.NewRecorder()
	adminRouter.ServeHTTP(self, httptest.NewRequest(http.MethodGet, "/api/log/self?scope=login", nil))
	require.Contains(t, self.Body.String(), `"success":true`)
	// 普通用户在登录审计视图里看不到任何失败事件。
	require.NotContains(t, self.Body.String(), `"type":8`)

	invalid := httptest.NewRecorder()
	adminRouter.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/log/?scope=consume", nil))
	require.Contains(t, invalid.Body.String(), `"success":false`)
}

// 真实 Login 处理器的加密密码解密失败分支。
func TestLoginAuditRecordsPasswordDecryptionFailure(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	require.NoError(t, db.AutoMigrate(&model.LoginEncryptionKey{}))
	require.NoError(t, model.InitPasswordEncryption())

	previousEncryption := common.PasswordLoginEncryptionEnabled
	common.PasswordLoginEncryptionEnabled = true
	t.Cleanup(func() { common.PasswordLoginEncryptionEnabled = previousEncryption })

	createLoginAuditTestUser(t, db, "audit_decrypt_user", common.UserStatusEnabled)
	keyID, _ := common.PasswordEncryptionPublicKey()
	require.NotEmpty(t, keyID)

	body := `{"username":"audit_decrypt_user","password_encrypted":"aW52YWxpZA==","encryption_key_id":"` + keyID + `"}`
	recorder := performLoginAuditLogin(t, router, body, nil)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditRowAudit(t, rows[0])
	require.Equal(t, "blocked", audit["outcome"])
	require.Equal(t, "request", audit["stage"])
	require.Equal(t, "password_decryption_failed", audit["reason"])
	require.Equal(t, "/api/user/login", loginAuditRowOther(t, rows[0])["request_path"])
	require.Equal(t, "audit_decrypt_user", loginAuditRowAdminInfo(t, rows[0])["attempted_identifier"])
	require.Empty(t, loginAuditSuccessRows(t, db))
	// 密文本身不会落库。
	require.NotContains(t, rows[0].Other, "aW52YWxpZA==")
}

// 会话已建立后资料查询失败，不得追加口令失败事件。
func TestLoginAuditDoesNotReportFailureAfterSessionEstablished(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "audit_profile_user", common.UserStatusEnabled)
	installHasUserPasswordFailure(t, db)

	recorder := performLoginAuditLogin(t, router, `{"username":"audit_profile_user","password":"`+loginAuditTestPassword+`"}`, nil)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	require.Empty(t, loginAuditSecurityRows(t, db))

	successRows := loginAuditSuccessRows(t, db)
	require.Len(t, successRows, 1)
	require.Equal(t, user.Id, successRows[0].UserId)
	// 成功事件同样补齐不含 query 的请求路径。
	require.Equal(t, "/api/user/login", loginAuditRowOther(t, successRows[0])["request_path"])
}

// 2FA 锁定分支：TOTP 与备用码都因锁定被拒绝。
func TestLoginAuditTwoFactorLocked(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "audit_2fa_locked", common.UserStatusEnabled)
	lockedUntil := time.Now().Add(10 * time.Minute)
	require.NoError(t, db.Create(&model.TwoFA{
		UserId:      user.Id,
		Secret:      "JBSWY3DPEHPK3PXP",
		IsEnabled:   true,
		LockedUntil: &lockedUntil,
	}).Error)

	challenge := performLoginAuditLogin(t, router, `{"username":"audit_2fa_locked","password":"`+loginAuditTestPassword+`"}`, nil)
	cookies := challenge.Result().Cookies()
	require.NotEmpty(t, cookies)

	recorder := performLoginAuditTwoFactor(t, router, "000000", cookies)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	rows := loginAuditSecurityRows(t, db)
	// challenge + 锁定拒绝。
	require.Len(t, rows, 2)
	lockedAudit := loginAuditRowAudit(t, rows[1])
	require.Equal(t, "blocked", lockedAudit["outcome"])
	require.Equal(t, "two_factor", lockedAudit["stage"])
	require.Equal(t, "two_factor_locked", lockedAudit["reason"])
	require.Empty(t, loginAuditSuccessRows(t, db))
}

// OAuth 账号缺少邮箱时，密码登录会被要求先补全邮箱，并记为 email_repair_required。
func TestLoginAuditRecordsEmailRepairRequired(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	hashed, err := common.Password2Hash(loginAuditTestPassword)
	require.NoError(t, err)
	user := &model.User{
		Username: "audit_email_repair",
		Password: hashed,
		GitHubId: "990001",
		Email:    "",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		AffCode:  "EMAILREPAIR",
	}
	require.NoError(t, db.Create(user).Error)

	recorder := performLoginAuditLogin(t, router, `{"username":"audit_email_repair","password":"`+loginAuditTestPassword+`"}`, nil)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditRowAudit(t, rows[0])
	require.Equal(t, "blocked", audit["outcome"])
	require.Equal(t, "credentials", audit["stage"])
	require.Equal(t, "email_repair_required", audit["reason"])
	require.Equal(t, float64(user.Id), loginAuditRowAdminInfo(t, rows[0])["target_user_id"])
	require.Empty(t, loginAuditSuccessRows(t, db))
}

// 已进入 2FA 流程但记录被删除/禁用 → blocked + two_factor_unavailable。
func TestLoginAuditTwoFactorUnavailable(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "audit_2fa_unavailable", common.UserStatusEnabled)
	require.NoError(t, db.Create(&model.TwoFA{UserId: user.Id, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true}).Error)

	challenge := performLoginAuditLogin(t, router, `{"username":"audit_2fa_unavailable","password":"`+loginAuditTestPassword+`"}`, nil)
	cookies := challenge.Result().Cookies()
	require.NotEmpty(t, cookies)

	// 进流程后把 2FA 记录改成未启用。
	require.NoError(t, db.Model(&model.TwoFA{}).Where("user_id = ?", user.Id).Update("is_enabled", false).Error)

	recorder := performLoginAuditTwoFactor(t, router, "000000", cookies)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 2)
	unavailable := loginAuditRowAudit(t, rows[1])
	require.Equal(t, "blocked", unavailable["outcome"])
	require.Equal(t, "two_factor", unavailable["stage"])
	require.Equal(t, "two_factor_unavailable", unavailable["reason"])
	require.Contains(t, rows[1].Content, "无法使用两步验证")
	require.Empty(t, loginAuditSuccessRows(t, db))
}

// 2FA 记录读取失败属于内部故障，不能记成验证码错误。
func TestLoginAuditTwoFactorDatabaseFailureIsError(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "audit_2fa_dbfail", common.UserStatusEnabled)
	require.NoError(t, db.Create(&model.TwoFA{UserId: user.Id, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true}).Error)

	challenge := performLoginAuditLogin(t, router, `{"username":"audit_2fa_dbfail","password":"`+loginAuditTestPassword+`"}`, nil)
	cookies := challenge.Result().Cookies()
	require.NotEmpty(t, cookies)

	installLoginAuditQueryFailure(t, db, func(dest any) bool {
		_, ok := dest.(*model.TwoFA)
		return ok
	})

	recorder := performLoginAuditTwoFactor(t, router, "000000", cookies)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 2)
	audit := loginAuditRowAudit(t, rows[1])
	require.Equal(t, "error", audit["outcome"])
	require.Equal(t, "two_factor", audit["stage"])
	require.Equal(t, "database_error", audit["reason"])
}

// 登录时用户表读取失败：响应仍是数据库错误，审计记为 error 而不是 user_not_found。
func TestLoginAuditDatabaseFailureIsNotCountedAsCredentialFailure(t *testing.T) {
	router, db := setupLoginAuditControllerTest(t, false)
	createLoginAuditTestUser(t, db, "audit_db_failure", common.UserStatusEnabled)

	installLoginAuditQueryFailure(t, db, func(dest any) bool {
		_, ok := dest.(*model.User)
		return ok
	})

	recorder := performLoginAuditLogin(t, router, `{"username":"audit_db_failure","password":"`+loginAuditTestPassword+`"}`, nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditRowAudit(t, rows[0])
	require.Equal(t, "error", audit["outcome"])
	require.Equal(t, "credentials", audit["stage"])
	require.Equal(t, "database_error", audit["reason"])
	// 输入账号仍被记录，便于排查。
	require.Equal(t, "audit_db_failure", loginAuditRowAdminInfo(t, rows[0])["attempted_identifier"])
}

// installLoginAuditQueryFailure 让匹配的查询直接失败，用于模拟数据库故障。
func installLoginAuditQueryFailure(t *testing.T, db *gorm.DB, match func(dest any) bool) {
	t.Helper()
	const callbackName = "test:login_audit_query_failure"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if match(tx.Statement.Dest) {
			_ = tx.AddError(errors.New("table unavailable in test"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })
}

// loginAuditTestSourceIP 按测试名派生确定性文档 IPv6，避免不同测试共用聚合桶。
func loginAuditTestSourceIP(t *testing.T) string {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name()))
	ip := net.IP(make([]byte, net.IPv6len))
	// 2001:db8::/32 才是文档保留前缀。
	ip[0], ip[1], ip[2], ip[3] = 0x20, 0x01, 0x0d, 0xb8
	copy(ip[4:], sum[:12])
	return ip.String()
}

// performLoginAuditRawRequest 允许完全省略 User-Agent（userAgent 为 nil）。
func performLoginAuditRawRequest(
	t *testing.T,
	router *gin.Engine,
	method string,
	path string,
	body string,
	userAgent *string,
	cookies []*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userAgent != nil {
		req.Header.Set("User-Agent", *userAgent)
	}
	req.Header.Set("CF-Ray", "cafe1234beef5678")
	req.RemoteAddr = "[" + loginAuditTestSourceIP(t) + "]:5555"
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func loginAuditOtherMap(t *testing.T, log model.Log) map[string]interface{} {
	t.Helper()
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	return other
}

// 缺少 UA（完全缺失或全空白）时，即使密码正确也不能建立登录会话。
func TestLoginAuditBlocksMissingUserAgentAtLoginEntry(t *testing.T) {
	require.NoError(t, newapii18n.Init())
	emptyUA := ""
	blankUA := "   \t  "

	for name, userAgent := range map[string]*string{
		"missing": nil,
		"blank":   &blankUA,
		"empty":   &emptyUA,
	} {
		t.Run(name, func(t *testing.T) {
			router, db := setupLoginAuditControllerTest(t, false)
			user := createLoginAuditTestUser(t, db, "audit_ua_"+name, common.UserStatusEnabled)

			recorder := performLoginAuditRawRequest(t, router, http.MethodPost, "/api/user/login",
				`{"username":"audit_ua_`+name+`","password":"`+loginAuditTestPassword+`"}`, userAgent, nil)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"success":false`)
			require.Contains(t, recorder.Body.String(), "浏览器环境异常，请检查浏览器设置后重试")
			// 没有新会话 cookie。
			require.Empty(t, recorder.Header().Values("Set-Cookie"))

			rows := loginAuditSecurityRows(t, db)
			require.Len(t, rows, 1)
			log := rows[0]
			require.Equal(t, 0, log.UserId)
			require.Equal(t, loginAuditTestSourceIP(t), log.Ip)
			audit := loginAuditRowAudit(t, log)
			require.Equal(t, "blocked", audit["outcome"])
			require.Equal(t, "precheck", audit["stage"])
			require.Equal(t, "user_agent_missing", audit["reason"])
			require.Contains(t, log.Content, "浏览器环境异常")
			require.Equal(t, "password", loginAuditOtherMap(t, log)["login_method"])
			// 入口阶段尚未解析账号：admin_info 只有请求诊断字段，没有目标账号。
			adminInfo := loginAuditRowAdminInfo(t, log)
			require.Equal(t, "cafe1234beef5678", adminInfo["cf_ray"])
			require.Equal(t, loginAuditTestSourceIP(t), adminInfo["peer_ip"])
			require.NotContains(t, adminInfo, "target_user_id")
			require.NotContains(t, adminInfo, "target_username")
			require.NotContains(t, adminInfo, "attempted_identifier")

			require.Empty(t, loginAuditSuccessRows(t, db))
			var stored model.User
			require.NoError(t, db.First(&stored, user.Id).Error)
			require.Zero(t, stored.LastLoginAt)
		})
	}
}

// 2FA 入口同样先做空 UA 校验。
func TestLoginAuditBlocksMissingUserAgentAtTwoFactorEntry(t *testing.T) {
	require.NoError(t, newapii18n.Init())
	router, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "audit_ua_2fa", common.UserStatusEnabled)
	require.NoError(t, db.Create(&model.TwoFA{UserId: user.Id, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true}).Error)

	challenge := performLoginAuditLogin(t, router, `{"username":"audit_ua_2fa","password":"`+loginAuditTestPassword+`"}`, nil)
	cookies := challenge.Result().Cookies()
	require.NotEmpty(t, cookies)

	recorder := performLoginAuditRawRequest(t, router, http.MethodPost, "/api/user/login/2fa", `{"code":"000000"}`, nil, cookies)
	require.Contains(t, recorder.Body.String(), "浏览器环境异常")

	middleware.FlushLoginAudit()
	var rows []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeLoginSecurity).Order("id ASC").Find(&rows).Error)
	require.Len(t, rows, 2)
	blocked := loginAuditRowAudit(t, rows[1])
	require.Equal(t, "blocked", blocked["outcome"])
	require.Equal(t, "user_agent_missing", blocked["reason"])
	require.Equal(t, "2fa", loginAuditOtherMap(t, rows[1])["login_method"])
	require.Empty(t, loginAuditSuccessRows(t, db))
	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Zero(t, stored.LastLoginAt)
}

// 共享会话出口（归属转移确认直接调用 setupLoginSession）：非空 UA 正常建立会话。
func TestSharedLoginExitCreatesSessionForBrowserUserAgent(t *testing.T) {
	_, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "shared_exit_ok", common.UserStatusEnabled)

	exitRouter := gin.New()
	exitRouter.Use(middleware.RequestId(), middleware.LoginAudit(),
		sessions.Sessions("session", cookie.NewStore([]byte("login-audit-secret"))))
	// 与 controller/oauth_ownership_transfer.go 的确认分支一致：直接调用共享创建器。
	exitRouter.POST("/api/oauth/ownership/confirm", func(c *gin.Context) {
		verified, err := model.GetUserById(user.Id, false)
		require.NoError(t, err)
		setupLoginSession(verified, c)
	})

	browserUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"
	recorder := performLoginAuditRawRequest(t, exitRouter, http.MethodPost, "/api/oauth/ownership/confirm", "", &browserUA, nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NotEmpty(t, recorder.Header().Values("Set-Cookie"))

	successRows := loginAuditSuccessRows(t, db)
	require.Len(t, successRows, 1)
	require.Equal(t, user.Id, successRows[0].UserId)
	require.Equal(t, "/api/oauth/ownership/confirm", loginAuditOtherMap(t, successRows[0])["request_path"])
	require.Equal(t, "oauth_ownership_transfer", loginAuditOtherMap(t, successRows[0])["login_method"])

	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.NotZero(t, stored.LastLoginAt)

	// 成功路径不写 type=8。
	require.Empty(t, loginAuditSecurityRows(t, db))
}

// 共享会话出口遇到空 UA：不建立会话、不更新时间、无 type7，留 blocked type8（带目标）。
func TestSharedLoginExitBlocksMissingUserAgent(t *testing.T) {
	_, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "shared_exit_blocked", common.UserStatusEnabled)

	exitRouter := gin.New()
	exitRouter.Use(middleware.RequestId(), middleware.LoginAudit(),
		sessions.Sessions("session", cookie.NewStore([]byte("login-audit-secret"))))
	exitRouter.POST("/api/oauth/ownership/confirm", func(c *gin.Context) {
		setupLoginSession(user, c)
	})

	recorder := performLoginAuditRawRequest(t, exitRouter, http.MethodPost, "/api/oauth/ownership/confirm", "", nil, nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
	require.Empty(t, recorder.Header().Values("Set-Cookie"))

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	log := rows[0]
	require.Equal(t, 0, log.UserId)
	audit := loginAuditRowAudit(t, log)
	require.Equal(t, "blocked", audit["outcome"])
	require.Equal(t, "precheck", audit["stage"])
	require.Equal(t, "user_agent_missing", audit["reason"])
	// 已知目标逐条保留在 admin_info，不作为 actor。
	adminInfo := loginAuditRowAdminInfo(t, log)
	require.Equal(t, float64(user.Id), adminInfo["target_user_id"])
	require.Equal(t, "shared_exit_blocked", adminInfo["target_username"])

	require.Empty(t, loginAuditSuccessRows(t, db))
	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Zero(t, stored.LastLoginAt)
}

// setupLogin 是 Passkey/微信/Telegram/通用 OAuth 的共同出口，空 UA 同样被拒。
func TestSetupLoginBlocksMissingUserAgent(t *testing.T) {
	_, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "passkey_exit_blocked", common.UserStatusEnabled)
	// 归属转移/通用 OAuth 复用同一出口：这里用 passkey 的路径标签。
	exitRouter := gin.New()
	exitRouter.Use(middleware.RequestId(), middleware.LoginAudit(),
		sessions.Sessions("session", cookie.NewStore([]byte("login-audit-secret"))))
	exitRouter.POST("/api/user/passkey/login/finish", func(c *gin.Context) {
		setupLogin(user, c)
	})

	recorder := performLoginAuditRawRequest(t, exitRouter, http.MethodPost, "/api/user/passkey/login/finish", "", nil, nil)
	require.Contains(t, recorder.Body.String(), `"success":false`)
	require.Empty(t, recorder.Header().Values("Set-Cookie"))

	rows := loginAuditSecurityRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditRowAudit(t, rows[0])
	require.Equal(t, "blocked", audit["outcome"])
	require.Equal(t, "user_agent_missing", audit["reason"])
	require.Equal(t, "passkey", loginAuditOtherMap(t, rows[0])["login_method"])
	require.Equal(t, float64(user.Id), loginAuditRowAdminInfo(t, rows[0])["target_user_id"])
	require.Empty(t, loginAuditSuccessRows(t, db))
}

// 非登录请求与已有会话访问不受空 UA 策略影响。
func TestMissingUserAgentPolicyOnlyGatesNewSessions(t *testing.T) {
	_, db := setupLoginAuditControllerTest(t, false)
	user := createLoginAuditTestUser(t, db, "existing_session_user", common.UserStatusEnabled)

	store := cookie.NewStore([]byte("login-audit-secret"))
	router := gin.New()
	router.Use(middleware.RequestId(), middleware.LoginAudit(), sessions.Sessions("session", store))
	router.GET("/api/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	router.GET("/api/user/self", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "id": sessions.Default(c).Get("id")})
	})

	// 非登录请求：不被阻断，也不产生登录事件。
	recorder := performLoginAuditRawRequest(t, router, http.MethodGet, "/api/status", "", nil, nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.Empty(t, loginAuditSecurityRows(t, db))

	// 已有会话 cookie 的访问：空 UA 仍然可用，也不写登录事件。
	seedRouter := gin.New()
	seedRouter.Use(sessions.Sessions("session", store))
	seedRouter.GET("/seed", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		require.NoError(t, session.Save())
		c.Status(http.StatusOK)
	})
	seedRecorder := httptest.NewRecorder()
	seedRouter.ServeHTTP(seedRecorder, httptest.NewRequest(http.MethodGet, "/seed", nil))
	cookies := seedRecorder.Result().Cookies()
	require.NotEmpty(t, cookies)

	recorder = performLoginAuditRawRequest(t, router, http.MethodGet, "/api/user/self", "", nil, cookies)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.Contains(t, recorder.Body.String(), `"id":`+strconv.Itoa(user.Id))
	require.Empty(t, loginAuditSecurityRows(t, db))
}

// 扫描器式的不合规请求（非法 JSON / 缺字段）记为 blocked，不混入凭证失败。
func TestLoginAuditMarksMalformedRequestsAsBlocked(t *testing.T) {
	require.NoError(t, newapii18n.Init())
	router, db := setupLoginAuditControllerTest(t, false)
	createLoginAuditTestUser(t, db, "malformed_user", common.UserStatusEnabled)
	browserUA := "Mozilla/5.0 (Test Browser)"

	// 1) 非法 JSON
	recorder := performLoginAuditRawRequest(t, router, http.MethodPost, "/api/user/login", `{not-json`, &browserUA, nil)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	// 2) 缺密码字段
	recorder = performLoginAuditRawRequest(t, router, http.MethodPost, "/api/user/login", `{"username":"malformed_user"}`, &browserUA, nil)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	// 3) 2FA 非法 JSON（无 pending 会话也存在，字段校验先失败）
	recorder = performLoginAuditRawRequest(t, router, http.MethodPost, "/api/user/login/2fa", `{not-json`, &browserUA, nil)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	middleware.FlushLoginAudit()
	var rows []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeLoginSecurity).Order("id ASC").Find(&rows).Error)
	require.Len(t, rows, 3)

	for i, row := range rows {
		audit := loginAuditRowAudit(t, row)
		require.Equal(t, "blocked", audit["outcome"], "row %d", i)
		require.Equal(t, "request", audit["stage"], "row %d", i)
		require.Equal(t, "invalid_request", audit["reason"], "row %d", i)
	}
	require.Equal(t, "password", loginAuditOtherMap(t, rows[0])["login_method"])
	require.Equal(t, "2fa", loginAuditOtherMap(t, rows[2])["login_method"])
	require.Empty(t, loginAuditSuccessRows(t, db))
}
