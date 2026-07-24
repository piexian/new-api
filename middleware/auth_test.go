package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResolveTokenUsingGroup(t *testing.T) {
	tests := []struct {
		name             string
		userGroup        string
		tokenGroup       string
		userUsableGroups map[string]string
		expectedGroup    string
		expectedOK       bool
	}{
		{
			name:             "empty token group uses user group",
			userGroup:        "user",
			tokenGroup:       "",
			userUsableGroups: map[string]string{"user": "用户分组"},
			expectedGroup:    "user",
			expectedOK:       true,
		},
		{
			name:             "token group in usable groups becomes effective group",
			userGroup:        "vip",
			tokenGroup:       "model",
			userUsableGroups: map[string]string{"vip": "用户分组", "model": "模型分组"},
			expectedGroup:    "model",
			expectedOK:       true,
		},
		{
			name:             "token group matching user group is allowed",
			userGroup:        "91vip",
			tokenGroup:       "91vip",
			userUsableGroups: map[string]string{"91vip": "用户分组"},
			expectedGroup:    "91vip",
			expectedOK:       true,
		},
		{
			name:             "auto token group is allowed for downstream auto selection",
			userGroup:        "user",
			tokenGroup:       "auto",
			userUsableGroups: map[string]string{"user": "用户分组"},
			expectedGroup:    "auto",
			expectedOK:       true,
		},
		{
			name:             "token group outside usable groups is rejected",
			userGroup:        "user",
			tokenGroup:       "model",
			userUsableGroups: map[string]string{"user": "用户分组"},
			expectedGroup:    "",
			expectedOK:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, ok := resolveTokenUsingGroup(tt.userGroup, tt.tokenGroup, tt.userUsableGroups)
			require.Equal(t, tt.expectedOK, ok)
			require.Equal(t, tt.expectedGroup, group)
		})
	}
}

func TestFormatAvailableGroups(t *testing.T) {
	require.Equal(t, "无", formatAvailableGroups(nil))
	require.Equal(t, "model、user", formatAvailableGroups(map[string]string{
		"user":  "用户分组",
		"model": "模型分组",
	}))
}

func TestRequiresGitHubEmailRelogin(t *testing.T) {
	tests := []struct {
		name     string
		user     *model.UserBase
		expected bool
	}{
		{
			name:     "legacy GitHub session with empty email",
			user:     &model.UserBase{GitHubId: "123"},
			expected: true,
		},
		{
			name: "GitHub user with email",
			user: &model.UserBase{GitHubId: "123", Email: "user@example.com"},
		},
		{
			name: "non GitHub user with empty email",
			user: &model.UserBase{},
		},
		{
			name: "missing user",
			user: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, requiresGitHubEmailRelogin(tt.user))
		})
	}
}

func TestClearGitHubEmailReloginSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	sessions.Sessions("session", cookie.NewStore([]byte("github-email-relogin-test")))(ctx)
	session := sessions.Default(ctx)
	session.Set("id", 123)
	session.Set("username", "github-user")

	clearGitHubEmailReloginSession(session)

	require.Nil(t, session.Get("id"))
	require.Nil(t, session.Get("username"))
	require.NotEmpty(t, recorder.Header().Values("Set-Cookie"))
}

func TestUserAuthClearsLegacyGitHubSessionWithoutEmail(t *testing.T) {
	recorder := performHeaderNavRequest(t, UserAuth(), true, model.User{
		Id:       1,
		Username: "tester",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		GitHubId: "123456",
	})

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.NotEmpty(t, recorder.Header().Values("Set-Cookie"))
}

func TestUserAuthAllowsGitHubSessionWithEmail(t *testing.T) {
	recorder := performHeaderNavRequest(t, UserAuth(), true, model.User{
		Id:       1,
		Username: "tester",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Email:    "existing@example.com",
		GitHubId: "123456",
	})

	require.Equal(t, http.StatusOK, recorder.Code)
}

// Ensure model types are used to satisfy import
var _ = &model.Token{}
var _ = &model.UserBase{}
