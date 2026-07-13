package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func withHeaderNavModules(t *testing.T, raw string) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	previous, hadPrevious := common.OptionMap["HeaderNavModules"]
	common.OptionMap["HeaderNavModules"] = raw
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if hadPrevious {
			common.OptionMap["HeaderNavModules"] = previous
			return
		}
		delete(common.OptionMap, "HeaderNavModules")
	})
}

func setupHeaderNavAuthTestDB(t *testing.T, user model.User) {
	t.Helper()

	originalDB := model.DB
	originalRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&user).Error)
	model.DB = db

	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
		model.DB = originalDB
		common.RedisEnabled = originalRedisEnabled
	})
}

func performHeaderNavRequest(t *testing.T, handler gin.HandlerFunc, authenticated bool, dbUsers ...model.User) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	if authenticated {
		user := model.User{
			Id:       1,
			Username: "tester",
			Password: "password123",
			Role:     common.RoleCommonUser,
			Status:   common.UserStatusEnabled,
			Group:    "default",
		}
		if len(dbUsers) > 0 {
			user = dbUsers[0]
		}
		setupHeaderNavAuthTestDB(t, user)
	}

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("header-nav-test"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "tester")
		session.Set("role", common.RoleCommonUser)
		session.Set("id", 1)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		if err := session.Save(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false})
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/test", handler, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"username":   c.GetString("username"),
			"role":       c.GetInt("role"),
			"group":      c.GetString("group"),
			"user_group": c.GetString("user_group"),
		})
	})

	var cookies []*http.Cookie
	if authenticated {
		loginRecorder := httptest.NewRecorder()
		loginRequest := httptest.NewRequest(http.MethodGet, "/login", nil)
		router.ServeHTTP(loginRecorder, loginRequest)
		require.Equal(t, http.StatusNoContent, loginRecorder.Code)
		cookies = loginRecorder.Result().Cookies()
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	if authenticated {
		request.Header.Set("New-Api-User", "1")
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestHeaderNavModuleAuthAllowsDefaultPublicAccess(t *testing.T) {
	withHeaderNavModules(t, "")

	recorder := performHeaderNavRequest(t, HeaderNavModuleAuth("pricing"), false)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestHeaderNavModuleAuthRejectsDisabledPricing(t *testing.T) {
	raw := `{"pricing":{"enabled":false,"requireAuth":false}}`
	withHeaderNavModules(t, raw)

	recorder := performHeaderNavRequest(t, HeaderNavModuleAuth("pricing"), false)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestHeaderNavModuleAuthRequiresLoginForPricing(t *testing.T) {
	raw := `{"pricing":{"enabled":true,"requireAuth":true}}`
	withHeaderNavModules(t, raw)

	recorder := performHeaderNavRequest(t, HeaderNavModuleAuth("pricing"), false)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestHeaderNavModuleAuthRequiresLoginForRankings(t *testing.T) {
	raw := `{"rankings":{"enabled":true,"requireAuth":true}}`
	withHeaderNavModules(t, raw)

	recorder := performHeaderNavRequest(t, HeaderNavModuleAuth("rankings"), false)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestHeaderNavModuleAuthRejectsLegacyDisabledModule(t *testing.T) {
	raw := `{"rankings":false}`
	withHeaderNavModules(t, raw)

	recorder := performHeaderNavRequest(t, HeaderNavModuleAuth("rankings"), false)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestHeaderNavModulePublicOrUserAuthAllowsDefaultPublicAccess(t *testing.T) {
	withHeaderNavModules(t, "")

	recorder := performHeaderNavRequest(t, HeaderNavModulePublicOrUserAuth("pricing"), false)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestHeaderNavModulePublicOrUserAuthRequiresLoginWhenDisabled(t *testing.T) {
	raw := `{"pricing":{"enabled":false,"requireAuth":false}}`
	withHeaderNavModules(t, raw)

	recorder := performHeaderNavRequest(t, HeaderNavModulePublicOrUserAuth("pricing"), false)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestHeaderNavModulePublicOrUserAuthAllowsLoggedInWhenDisabled(t *testing.T) {
	raw := `{"pricing":{"enabled":false,"requireAuth":false}}`
	withHeaderNavModules(t, raw)

	recorder := performHeaderNavRequest(t, HeaderNavModulePublicOrUserAuth("pricing"), true)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestAdminAuthRefreshesSessionRoleAndGroupFromUserCache(t *testing.T) {
	recorder := performHeaderNavRequest(t, AdminAuth(), true, model.User{
		Id:       1,
		Username: "tester",
		Password: "password123",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		Group:    "91vip",
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":true,"username":"tester","role":10,"group":"91vip","user_group":"91vip"}`, recorder.Body.String())
}

func TestTryUserAuthRefreshesUsernameFromUserCache(t *testing.T) {
	recorder := performHeaderNavRequest(t, TryUserAuth(), true, model.User{
		Id:       1,
		Username: "current-name",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":true,"username":"current-name","role":1,"group":"default","user_group":"default"}`, recorder.Body.String())
}

func TestTryUserAuthIgnoresDisabledUser(t *testing.T) {
	recorder := performHeaderNavRequest(t, TryUserAuth(), true, model.User{
		Id:       1,
		Username: "tester",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusDisabled,
		Group:    "default",
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":true,"username":"","role":0,"group":"","user_group":""}`, recorder.Body.String())
}

func TestHeaderNavModulePublicOrUserAuthRequiresLoginWhenRequireAuth(t *testing.T) {
	raw := `{"pricing":{"enabled":true,"requireAuth":true}}`
	withHeaderNavModules(t, raw)

	recorder := performHeaderNavRequest(t, HeaderNavModulePublicOrUserAuth("pricing"), false)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestHeaderNavModulePublicOrUserAuthRequiresLoginForLegacyDisabledModule(t *testing.T) {
	raw := `{"pricing":false}`
	withHeaderNavModules(t, raw)

	recorder := performHeaderNavRequest(t, HeaderNavModulePublicOrUserAuth("pricing"), false)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
