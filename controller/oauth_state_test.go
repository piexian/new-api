package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOAuthStateTestContext(t *testing.T, target string) (*gin.Context, sessions.Session, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	sessions.Sessions("new-api-session", cookie.NewStore([]byte("oauth-state-test")))(ctx)

	return ctx, sessions.Default(ctx), recorder
}

func TestGenerateOAuthCodeScopesRegistrationCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		target   string
		existing string
		expected interface{}
	}{
		{
			name:     "login clears stale credential",
			target:   "/api/oauth/state",
			existing: "stale-code",
			expected: nil,
		},
		{
			name:     "blank credential clears stale credential",
			target:   "/api/oauth/state?aff=%20%20",
			existing: "stale-code",
			expected: nil,
		},
		{
			name:     "registration stores normalized credential",
			target:   "/api/oauth/state?aff=%20fresh-code%20",
			existing: "stale-code",
			expected: "fresh-code",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, session, recorder := newOAuthStateTestContext(t, test.target)
			session.Set("aff", test.existing)

			GenerateOAuthCode(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, test.expected, session.Get("aff"))
			require.NotEmpty(t, session.Get("oauth_state"))
		})
	}
}
