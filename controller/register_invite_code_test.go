package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func withRequiredRegisterInviteCode(t *testing.T) {
	t.Helper()

	previousRegisterEnabled := common.RegisterEnabled
	previousOAuthRegisterEnabled := common.OAuthRegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	previousRequired := common.RegisterInviteCodeRequired
	previousQuotaForNewUser := common.QuotaForNewUser
	previousQuotaForInvitee := common.QuotaForInvitee
	previousQuotaForInviter := common.QuotaForInviter

	common.RegisterEnabled = true
	common.OAuthRegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.RegisterInviteCodeRequired = true
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0

	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.OAuthRegisterEnabled = previousOAuthRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
		common.RegisterInviteCodeRequired = previousRequired
		common.QuotaForNewUser = previousQuotaForNewUser
		common.QuotaForInvitee = previousQuotaForInvitee
		common.QuotaForInviter = previousQuotaForInviter
	})
}

func TestOAuthRegistrationCanBeDisabledSeparately(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	setupUserSelfControllerTestDB(t)

	previousRegisterEnabled := common.RegisterEnabled
	previousOAuthRegisterEnabled := common.OAuthRegisterEnabled
	common.RegisterEnabled = true
	common.OAuthRegisterEnabled = false
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.OAuthRegisterEnabled = previousOAuthRegisterEnabled
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/mock", nil)
	ctx.Request.Header.Set("Accept-Language", "en")
	store := cookie.NewStore([]byte("oauth-registration-test"))
	sessions.Sessions("new-api-session", store)(ctx)

	user, err := findOrCreateOAuthUser(ctx, mockOAuthProvider{}, &oauth.OAuthUser{
		ProviderUserID: "provider-new-user",
		Username:       "provider-new-user",
	}, sessions.Default(ctx))
	if err == nil {
		t.Fatalf("expected OAuth registration disabled error, got user: %+v", user)
	}
	if _, ok := err.(*OAuthRegistrationDisabledError); !ok {
		t.Fatalf("expected OAuthRegistrationDisabledError, got %T: %v", err, err)
	}
}

func TestOAuthRegistrationPersistsInviterId(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	withRequiredRegisterInviteCode(t)
	inviter := seedSelfUser(t, db, "oauth-invite-owner", "")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/mock", nil)
	ctx.Request.Header.Set("Accept-Language", "en")
	store := cookie.NewStore([]byte("oauth-invite-test"))
	sessions.Sessions("new-api-session", store)(ctx)
	session := sessions.Default(ctx)
	session.Set("aff", inviter.AffCode)
	if err := session.Save(); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	user, err := findOrCreateOAuthUser(ctx, mockOAuthProvider{}, &oauth.OAuthUser{
		ProviderUserID: "provider-invite-user",
		Username:       "provider-invite-user",
	}, session)
	if err != nil {
		t.Fatalf("expected OAuth registration success, got error: %v", err)
	}

	var registered model.User
	if err := db.First(&registered, user.Id).Error; err != nil {
		t.Fatalf("failed to load registered OAuth user: %v", err)
	}
	if registered.InviterId != inviter.Id {
		t.Fatalf("expected inviter id %d, got %d", inviter.Id, registered.InviterId)
	}
}

func TestGitHubOAuthRegistrationRequiresMinimumAccountAge(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	setupUserSelfControllerTestDB(t)

	previousRegisterEnabled := common.RegisterEnabled
	previousOAuthRegisterEnabled := common.OAuthRegisterEnabled
	previousRequired := common.RegisterInviteCodeRequired
	previousMinimumAge := common.GitHubMinimumAccountAge
	previousMinimumAgeUnit := common.GitHubMinimumAccountAgeUnit
	common.RegisterEnabled = true
	common.OAuthRegisterEnabled = true
	common.RegisterInviteCodeRequired = false
	common.GitHubMinimumAccountAge = 30
	common.GitHubMinimumAccountAgeUnit = common.GitHubAccountAgeUnitDay
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.OAuthRegisterEnabled = previousOAuthRegisterEnabled
		common.RegisterInviteCodeRequired = previousRequired
		common.GitHubMinimumAccountAge = previousMinimumAge
		common.GitHubMinimumAccountAgeUnit = previousMinimumAgeUnit
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/github", nil)
	ctx.Request.Header.Set("Accept-Language", "en")
	store := cookie.NewStore([]byte("github-age-test"))
	sessions.Sessions("new-api-session", store)(ctx)

	user, err := findOrCreateOAuthUser(ctx, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: "github-young-user",
		Username:       "github-young-user",
		CreatedAt:      time.Now().UTC().AddDate(0, 0, -7),
	}, sessions.Default(ctx))
	if err == nil {
		t.Fatalf("expected GitHub account age error, got user: %+v", user)
	}
	if _, ok := err.(*GitHubAccountAgeTooYoungError); !ok {
		t.Fatalf("expected GitHubAccountAgeTooYoungError, got %T: %v", err, err)
	}
}

type mockOAuthProvider struct{}

func (mockOAuthProvider) GetName() string           { return "Mock" }
func (mockOAuthProvider) GetProviderPrefix() string { return "mock_" }
func (mockOAuthProvider) IsEnabled() bool           { return true }
func (mockOAuthProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return nil, nil
}
func (mockOAuthProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return nil, nil
}
func (mockOAuthProvider) IsUserIDTaken(string) bool { return false }
func (mockOAuthProvider) FillUserByProviderID(*model.User, string) error {
	return nil
}
func (mockOAuthProvider) SetProviderUserID(*model.User, string) {}

func TestRegisterRequiresInviteCodeWhenEnabled(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	setupUserSelfControllerTestDB(t)
	withRequiredRegisterInviteCode(t)

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/user/register",
		map[string]any{
			"username": "invite-required-user",
			"password": "password123",
		},
		0,
		0,
	)
	ctx.Request.Header.Set("Accept-Language", "en")
	Register(ctx)

	response := decodeSelfResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected register failure without invite code, got success: %s", recorder.Body.String())
	}
	if response.Message != "Invitation code is required!" {
		t.Fatalf("expected invite code required message, got %q", response.Message)
	}
}

func TestRegisterRejectsInvalidInviteCodeWhenRequired(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	setupUserSelfControllerTestDB(t)
	withRequiredRegisterInviteCode(t)

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/user/register",
		map[string]any{
			"username": "invalid-invite-user",
			"password": "password123",
			"aff_code": "missing",
		},
		0,
		0,
	)
	ctx.Request.Header.Set("Accept-Language", "en")
	Register(ctx)

	response := decodeSelfResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected register failure with invalid invite code, got success: %s", recorder.Body.String())
	}
	if response.Message != "Invitation code is invalid!" {
		t.Fatalf("expected invalid invite code message, got %q", response.Message)
	}
}

func TestRegisterAllowsValidInviteCodeWhenRequired(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	withRequiredRegisterInviteCode(t)
	inviter := seedSelfUser(t, db, "invite-owner", "")

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/user/register",
		map[string]any{
			"username": "valid-invite-user",
			"password": "password123",
			"aff_code": inviter.AffCode,
		},
		0,
		0,
	)
	ctx.Request.Header.Set("Accept-Language", "en")
	Register(ctx)

	response := decodeSelfResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected register success with valid invite code, got message: %s", response.Message)
	}

	var registered model.User
	if err := db.Where("username = ?", "valid-invite-user").First(&registered).Error; err != nil {
		t.Fatalf("failed to load registered user: %v", err)
	}
	if registered.InviterId != inviter.Id {
		t.Fatalf("expected inviter id %d, got %d", inviter.Id, registered.InviterId)
	}
}
