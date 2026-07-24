package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func newGitHubOAuthTestContext(t *testing.T) (*gin.Context, sessions.Session) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/github", nil)
	store := cookie.NewStore([]byte("github-email-test"))
	sessions.Sessions("github-email-test", store)(ctx)
	return ctx, sessions.Default(ctx)
}

func TestExistingGitHubOAuthUserFillsOnlyEmptyEmail(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	user := seedSelfUser(t, db, "existing-github-user", "")
	user.GitHubId = "123456"
	if err := db.Model(user).Update("github_id", user.GitHubId).Error; err != nil {
		t.Fatalf("failed to seed GitHub ID: %v", err)
	}
	ctx, session := newGitHubOAuthTestContext(t)

	got, err := findOrCreateOAuthUser(ctx, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: user.GitHubId,
		Username:       user.Username,
		Email:          " GitHub@Example.COM ",
	}, session)
	if err != nil {
		t.Fatalf("findOrCreateOAuthUser returned error: %v", err)
	}
	if got.Email != "github@example.com" {
		t.Fatalf("email = %q, want github@example.com", got.Email)
	}

	got, err = findOrCreateOAuthUser(ctx, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: user.GitHubId,
		Username:       user.Username,
		Email:          "replacement@example.com",
	}, session)
	if err != nil {
		t.Fatalf("second findOrCreateOAuthUser returned error: %v", err)
	}
	if got.Email != "github@example.com" {
		t.Fatalf("email was overwritten with %q", got.Email)
	}
}

func TestExistingGitHubOAuthUserEmailConflictBlocksLogin(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	owner := seedSelfUser(t, db, "github-email-owner", "")
	owner.Email = "taken@example.com"
	if err := db.Model(owner).Update("email", owner.Email).Error; err != nil {
		t.Fatalf("failed to seed owner email: %v", err)
	}
	target := seedSelfUser(t, db, "github-email-conflict", "")
	target.GitHubId = "654321"
	if err := db.Model(target).Update("github_id", target.GitHubId).Error; err != nil {
		t.Fatalf("failed to seed target GitHub ID: %v", err)
	}
	ctx, session := newGitHubOAuthTestContext(t)

	got, err := findOrCreateOAuthUser(ctx, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: target.GitHubId,
		Username:       target.Username,
		Email:          "TAKEN@example.com",
	}, session)
	if _, ok := err.(*OAuthEmailAlreadyTakenError); !ok {
		t.Fatalf("error = %T %v, want OAuthEmailAlreadyTakenError", err, err)
	}
	if got != nil {
		t.Fatalf("user = %#v, want nil", got)
	}

	var stored model.User
	if err := db.First(&stored, target.Id).Error; err != nil {
		t.Fatalf("failed to reload target: %v", err)
	}
	if stored.Email != "" {
		t.Fatalf("conflicting email was persisted: %q", stored.Email)
	}
}

func TestExistingGitHubOAuthUserWithoutProviderEmailBlocksLogin(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	user := seedSelfUser(t, db, "github-missing-provider-email", "")
	user.GitHubId = "987654"
	if err := db.Model(user).Update("github_id", user.GitHubId).Error; err != nil {
		t.Fatalf("failed to seed GitHub ID: %v", err)
	}
	ctx, session := newGitHubOAuthTestContext(t)

	got, err := findOrCreateOAuthUser(ctx, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: user.GitHubId,
		Username:       user.Username,
	}, session)
	if _, ok := err.(*oauth.OAuthError); !ok {
		t.Fatalf("error = %T %v, want OAuthError", err, err)
	}
	if got != nil {
		t.Fatalf("user = %#v, want nil", got)
	}
}
