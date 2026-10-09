package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 未设置登录密码的用户：会话/AccessToken 只能访问自身信息与设置密码接口；
// 设置成功后同一会话立即恢复；TokenAuth（API key）不受影响。
func TestPasswordSetupGate(t *testing.T) {
	require.NoError(t, newapii18n.Init())
	accessToken := "password-setup-management-token"
	user := model.User{
		Id: 1, Username: "no-password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, AccessToken: &accessToken, Group: "default",
	}
	setupHeaderNavAuthTestDB(t, user)
	db := model.DB

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("password-gate-test"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", user.Username)
		session.Set("role", user.Role)
		session.Set("id", user.Id)
		session.Set("status", user.Status)
		session.Set("group", user.Group)
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	ok := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	router.GET("/api/user/self", UserAuth(), ok)
	router.PUT("/api/user/self", UserAuth(), ok)
	router.PUT("/api/user/self/password", UserAuth(), ok)
	router.GET("/protected", UserAuth(), ok)
	router.GET("/oauth/test", PasswordSetupForSession(), ok)
	router.GET("/mixed", TokenOrUserAuth(), ok)

	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)
	cookies := loginRecorder.Result().Cookies()

	request := func(method, path string, withSession bool, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("New-Api-User", strconv.Itoa(user.Id))
		if withSession {
			for _, cookie := range cookies {
				req.AddCookie(cookie)
			}
		}
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	for _, tc := range []struct{ method, path string }{
		{"GET", "/protected"},
		{"GET", "/mixed"},
		{"GET", "/oauth/test"},
		{"PUT", "/api/user/self"},
	} {
		w := request(tc.method, tc.path, true, "")
		require.Equal(t, http.StatusForbidden, w.Code, tc.path)
		require.Contains(t, w.Body.String(), "PASSWORD_SETUP_REQUIRED")
	}
	// AccessToken 管理接口同样被拦截
	w := request("GET", "/protected", false, accessToken)
	require.Equal(t, http.StatusForbidden, w.Code)
	// 放行：查询自身、设置密码、无会话的 OAuth 回调
	require.Equal(t, http.StatusNoContent, request("GET", "/api/user/self", true, "").Code)
	require.Equal(t, http.StatusNoContent, request("PUT", "/api/user/self/password", true, "").Code)
	require.Equal(t, http.StatusNoContent, request("GET", "/oauth/test", false, "").Code)
	// 设置密码后，同一会话立即恢复访问
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", user.Id).Update("password", "existing-hash").Error)
	require.Equal(t, http.StatusNoContent, request("GET", "/protected", true, "").Code)
}

func TestPasswordStateReadFailureDoesNotAllowAccess(t *testing.T) {
	require.NoError(t, newapii18n.Init())
	setupHeaderNavAuthTestDB(t, model.User{Id: 1, Username: "tester", Status: common.UserStatusEnabled})
	require.NoError(t, model.DB.Migrator().DropTable(&model.User{}))

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("password-test"))))
	router.GET("/test", func(c *gin.Context) {
		if requireUserPassword(c, 1) {
			t.Fatal("database failure allowed access")
		}
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	require.Equal(t, http.StatusInternalServerError, w.Code)
}
