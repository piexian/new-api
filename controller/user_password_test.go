package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 第三方注册（无密码）用户的强制设置密码全流程。
func TestLoginPasswordSetupFlow(t *testing.T) {
	require.NoError(t, newapii18n.Init())
	db := setupChannelStatusControllerTestDB(t)
	accessToken := "setup-flow-management-token"
	user := model.User{Username: "third-party", GitHubId: "123", Email: "third-party@example.com", DisplayName: "original", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AccessToken: &accessToken, Group: "default"}
	require.NoError(t, db.Create(&user).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("password-setup-flow-secret"))))
	router.GET("/login", func(c *gin.Context) {
		loaded, err := model.GetUserById(user.Id, false)
		require.NoError(t, err)
		setupLogin(loaded, c)
	})
	router.GET("/api/user/self", middleware.UserAuth(), GetSelf)
	router.PUT("/api/user/self/password", middleware.UserAuth(), UpdateSelfPassword)
	router.PUT("/api/user/self", middleware.UserAuth(), UpdateSelf)

	var cookieHeader string
	request := func(method, path string, payload gin.H, session bool) (*httptest.ResponseRecorder, map[string]any) {
		var body []byte
		if payload != nil {
			var err error
			body, err = common.Marshal(payload)
			require.NoError(t, err)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("New-Api-User", strconv.Itoa(user.Id))
		if session {
			req.Header.Set("Cookie", cookieHeader)
		} else {
			req.Header.Set("Authorization", accessToken)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var response map[string]any
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
		return w, response
	}

	// 登录响应直接带 has_password=false
	w, result := request("GET", "/login", nil, true)
	cookieHeader = w.Result().Cookies()[0].String()
	require.Equal(t, false, result["data"].(map[string]any)["has_password"])

	// 门禁只放行 GetSelf
	_, result = request("GET", "/api/user/self", nil, true)
	data := result["data"].(map[string]any)
	require.Equal(t, false, data["has_password"])
	require.NotContains(t, data, "password")
	// AccessToken 不能走设置密码接口
	w, _ = request("PUT", "/api/user/self/password", gin.H{"password": "Password123!"}, false)
	require.Equal(t, http.StatusForbidden, w.Code)
	// 弱密码被拒
	_, result = request("PUT", "/api/user/self/password", gin.H{"password": "password123"}, true)
	require.Equal(t, false, result["success"])
	// 旧接口不再接受密码字段
	_, result = request("PUT", "/api/user/self", gin.H{"password": "Password123!"}, true)
	require.Equal(t, false, result["success"])
	// 携带注入字段的首次设置：多余字段被忽略
	_, result = request("PUT", "/api/user/self/password", gin.H{"password": "Password123!", "role": common.RoleRootUser, "display_name": "injected"}, true)
	require.Equal(t, true, result["success"])

	_, result = request("GET", "/api/user/self", nil, true)
	data = result["data"].(map[string]any)
	require.Equal(t, true, data["has_password"])
	// 注入的 display_name 未生效（响应中 display_name 取用户名，另查库验证）
	storedAfterSetup, err := model.GetUserById(user.Id, true)
	require.NoError(t, err)
	require.Equal(t, "original", storedAfterSetup.DisplayName)
	require.EqualValues(t, common.RoleCommonUser, data["role"])
	_, result = request("GET", "/login", nil, true)
	require.Equal(t, true, result["data"].(map[string]any)["has_password"])

	// 修改密码：原密码错误 / 未变 均被拒
	_, result = request("PUT", "/api/user/self/password", gin.H{"password": "Another123!", "original_password": "wrong"}, true)
	require.Equal(t, false, result["success"])
	_, result = request("PUT", "/api/user/self/password", gin.H{"password": "Password123!", "original_password": "Password123!"}, true)
	require.Equal(t, false, result["success"])
	_, result = request("PUT", "/api/user/self/password", gin.H{"password": "Another123!", "original_password": "Password123!"}, true)
	require.Equal(t, true, result["success"])
	stored, err := model.GetUserById(user.Id, true)
	require.NoError(t, err)
	require.True(t, common.ValidatePasswordAndHash("Another123!", stored.Password))

	// 资料修改（不含密码字段）不受影响
	_, result = request("PUT", "/api/user/self", gin.H{"display_name": "changed"}, true)
	require.Equal(t, true, result["success"])
}
