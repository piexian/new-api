package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

func newOAuthOwnershipTransferTestContext(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder, sessions.Session) {
	t.Helper()
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("initialize i18n: %v", err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/oauth/ownership/confirm", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	store := cookie.NewStore([]byte("oauth-ownership-transfer-test"))
	sessions.Sessions("oauth-ownership-transfer-test", store)(ctx)
	return ctx, recorder, sessions.Default(ctx)
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

func TestExistingGitHubOAuthUserEmailConflictReturnsTransferCandidate(t *testing.T) {
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
	if got == nil || got.Id != target.Id {
		t.Fatalf("user = %#v, want existing user %d", got, target.Id)
	}
	var conflict *OAuthEmailAlreadyTakenError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T %v, want OAuthEmailAlreadyTakenError", err, err)
	}
	if conflict.Provider != "GitHub" || conflict.Email != "taken@example.com" {
		t.Fatalf("conflict = %#v", conflict)
	}

	var stored model.User
	if err := db.First(&stored, target.Id).Error; err != nil {
		t.Fatalf("failed to reload target: %v", err)
	}
	if stored.Email != "" {
		t.Fatalf("conflicting email was persisted: %q", stored.Email)
	}
}

func TestNewGitHubOAuthUserEmailConflictCreatesTransferTarget(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	owner := seedSelfUser(t, db, "new-github-email-owner", "")
	owner.Email = "taken-new@example.com"
	if err := db.Model(owner).Update("email", owner.Email).Error; err != nil {
		t.Fatalf("failed to seed owner email: %v", err)
	}

	oldRegisterEnabled := common.RegisterEnabled
	oldOAuthRegisterEnabled := common.OAuthRegisterEnabled
	oldMinimumAge := common.GitHubMinimumAccountAge
	common.RegisterEnabled = true
	common.OAuthRegisterEnabled = true
	common.GitHubMinimumAccountAge = 0
	t.Cleanup(func() {
		common.RegisterEnabled = oldRegisterEnabled
		common.OAuthRegisterEnabled = oldOAuthRegisterEnabled
		common.GitHubMinimumAccountAge = oldMinimumAge
	})

	ctx, session := newGitHubOAuthTestContext(t)
	got, err := findOrCreateOAuthUser(ctx, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: "1122334455",
		Username:       "new-github-conflict",
		Email:          "TAKEN-NEW@example.com",
		CreatedAt:      time.Now().AddDate(-1, 0, 0),
	}, session)
	var emailConflict *OAuthEmailAlreadyTakenError
	if !errors.As(err, &emailConflict) {
		t.Fatalf("error = %T %v, want OAuthEmailAlreadyTakenError", err, err)
	}
	if got == nil || got.Id == 0 {
		t.Fatalf("user = %#v, want provisional OAuth target", got)
	}
	if got.Email != "" || got.GitHubId != "1122334455" {
		t.Fatalf("provisional target = %#v", got)
	}
	if emailConflict.Provider != "GitHub" || emailConflict.Email != "taken-new@example.com" {
		t.Fatalf("conflict = %#v", emailConflict)
	}
}

func TestNewOAuthUserEmailConflictUsesProviderSpecificTransferCandidate(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	if err := db.AutoMigrate(&model.UserOAuthBinding{}, &model.CustomOAuthProvider{}); err != nil {
		t.Fatalf("migrate OAuth tables: %v", err)
	}
	owner := seedSelfUser(t, db, "mock-email-owner", "")
	owner.Email = "mock@example.com"
	if err := db.Model(owner).Update("email", owner.Email).Error; err != nil {
		t.Fatalf("seed owner email: %v", err)
	}

	oldRegisterEnabled := common.RegisterEnabled
	oldOAuthRegisterEnabled := common.OAuthRegisterEnabled
	common.RegisterEnabled = true
	common.OAuthRegisterEnabled = true
	t.Cleanup(func() {
		common.RegisterEnabled = oldRegisterEnabled
		common.OAuthRegisterEnabled = oldOAuthRegisterEnabled
	})

	ctx, session := newGitHubOAuthTestContext(t)
	got, err := findOrCreateOAuthUser(ctx, mockOAuthProvider{}, &oauth.OAuthUser{
		ProviderUserID: "mock-user-id",
		Username:       "mock-conflict",
		Email:          "MOCK@example.com",
	}, session)
	var conflict *OAuthEmailAlreadyTakenError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T %v, want OAuthEmailAlreadyTakenError", err, err)
	}
	if got == nil || got.Id == 0 || got.Email != "" {
		t.Fatalf("user = %#v, want provisional target without email", got)
	}
	if conflict.Provider != "Mock" || conflict.Email != "mock@example.com" {
		t.Fatalf("conflict = %#v", conflict)
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

func prepareControllerOAuthOwnershipTransfer(t *testing.T, db *gorm.DB, codeSentAt int64) (*model.OAuthOwnershipTransfer, *model.User, *model.User) {
	t.Helper()
	if err := db.AutoMigrate(&model.RiskBanLog{}, &model.OAuthOwnershipTransfer{}); err != nil {
		t.Fatalf("migrate ownership transfer tables: %v", err)
	}
	previous := seedSelfUser(t, db, "controller-transfer-owner", "")
	previous.Email = "controller-transfer@example.com"
	if err := db.Model(previous).Update("email", previous.Email).Error; err != nil {
		t.Fatalf("seed previous email: %v", err)
	}
	target := seedSelfUser(t, db, "controller-transfer-target", "")
	target.GitHubId = "controller-transfer-github"
	if err := db.Model(target).Update("github_id", target.GitHubId).Error; err != nil {
		t.Fatalf("seed target GitHub ID: %v", err)
	}
	challenge, err := model.PrepareOAuthOwnershipTransfer(
		"github",
		"GitHub",
		target.GitHubId,
		previous.Email,
		previous.Id,
		target.Id,
		model.OAuthOwnershipTransferModeLogin,
		"github_id",
		0,
		codeSentAt,
	)
	if err != nil {
		t.Fatalf("prepare ownership transfer: %v", err)
	}
	if _, err := model.ClaimOAuthOwnershipTransferSend(challenge.Id, codeSentAt); err != nil {
		t.Fatalf("claim ownership code send: %v", err)
	}
	if err := model.MarkOAuthOwnershipTransferCodeSent(challenge.Id, codeSentAt); err != nil {
		t.Fatalf("mark ownership code sent: %v", err)
	}
	challenge, err = model.GetOAuthOwnershipTransferById(challenge.Id)
	if err != nil {
		t.Fatalf("reload ownership transfer: %v", err)
	}
	return challenge, previous, target
}

func TestConfirmOAuthOwnershipTransferMovesEmailAndBansPreviousAccount(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	challenge, previous, target := prepareControllerOAuthOwnershipTransfer(t, db, common.GetTimestamp())
	common.RegisterVerificationCodeWithKey(challenge.PairKey, "abc123", common.OAuthOwnershipTransferPurpose)
	t.Cleanup(func() {
		common.DeleteKey(challenge.PairKey, common.OAuthOwnershipTransferPurpose)
	})
	ctx, recorder, session := newOAuthOwnershipTransferTestContext(t, `{"code":"abc123"}`)
	session.Set(oauthOwnershipTransferSessionKey, challenge.Id)
	if err := session.Save(); err != nil {
		t.Fatalf("save ownership transfer session: %v", err)
	}

	ConfirmOAuthOwnershipTransfer(ctx)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Id int `json:"id"`
		} `json:"data"`
	}
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	if !response.Success || response.Data.Id != target.Id {
		t.Fatalf("response = %s", recorder.Body.String())
	}
	var storedPrevious, storedTarget model.User
	if err := db.First(&storedPrevious, previous.Id).Error; err != nil {
		t.Fatalf("reload previous user: %v", err)
	}
	if err := db.First(&storedTarget, target.Id).Error; err != nil {
		t.Fatalf("reload target user: %v", err)
	}
	if storedPrevious.Status != common.UserStatusDisabled || storedPrevious.Email != "" {
		t.Fatalf("previous user = %#v", storedPrevious)
	}
	if storedTarget.Email != "controller-transfer@example.com" {
		t.Fatalf("target email = %q", storedTarget.Email)
	}
}

func TestConfirmOAuthOwnershipTransferReportsRemainingAttempts(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	challenge, _, _ := prepareControllerOAuthOwnershipTransfer(t, db, common.GetTimestamp())
	common.RegisterVerificationCodeWithKey(challenge.PairKey, "abc123", common.OAuthOwnershipTransferPurpose)
	t.Cleanup(func() {
		common.DeleteKey(challenge.PairKey, common.OAuthOwnershipTransferPurpose)
	})
	ctx, recorder, session := newOAuthOwnershipTransferTestContext(t, `{"code":"wrong1"}`)
	session.Set(oauthOwnershipTransferSessionKey, challenge.Id)
	if err := session.Save(); err != nil {
		t.Fatalf("save ownership transfer session: %v", err)
	}

	ConfirmOAuthOwnershipTransfer(ctx)

	var response struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
		Data    struct {
			FailedAttempts    int `json:"failed_attempts"`
			AttemptsRemaining int `json:"attempts_remaining"`
		} `json:"data"`
	}
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	if response.Success || response.Code != "oauth_ownership_code_invalid" || response.Data.FailedAttempts != 1 || response.Data.AttemptsRemaining != 4 {
		t.Fatalf("response = %s", recorder.Body.String())
	}
	stored, err := model.GetOAuthOwnershipTransferById(challenge.Id)
	if err != nil {
		t.Fatalf("reload challenge: %v", err)
	}
	if stored.Status != model.OAuthOwnershipTransferStatusCodeSent || stored.FailedAttempts != 1 {
		t.Fatalf("challenge after invalid code = %#v", stored)
	}
}

func TestConfirmOAuthOwnershipTransferFifthErrorClosesPermanently(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	challenge, _, _ := prepareControllerOAuthOwnershipTransfer(t, db, common.GetTimestamp())
	common.RegisterVerificationCodeWithKey(challenge.PairKey, "abc123", common.OAuthOwnershipTransferPurpose)
	t.Cleanup(func() {
		common.DeleteKey(challenge.PairKey, common.OAuthOwnershipTransferPurpose)
	})

	for attempt := 1; attempt <= model.OAuthOwnershipTransferMaxAttempts; attempt++ {
		ctx, recorder, session := newOAuthOwnershipTransferTestContext(t, `{"code":"wrong1"}`)
		session.Set(oauthOwnershipTransferSessionKey, challenge.Id)
		if err := session.Save(); err != nil {
			t.Fatalf("save ownership transfer session for attempt %d: %v", attempt, err)
		}

		ConfirmOAuthOwnershipTransfer(ctx)

		var response struct {
			Success bool   `json:"success"`
			Code    string `json:"code"`
			Data    struct {
				Closed            bool `json:"closed"`
				FailedAttempts    int  `json:"failed_attempts"`
				AttemptsRemaining int  `json:"attempts_remaining"`
			} `json:"data"`
		}
		if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response for attempt %d %q: %v", attempt, recorder.Body.String(), err)
		}
		if response.Success || response.Data.FailedAttempts != attempt || response.Data.AttemptsRemaining != model.OAuthOwnershipTransferMaxAttempts-attempt {
			t.Fatalf("response for attempt %d = %s", attempt, recorder.Body.String())
		}
		if attempt < model.OAuthOwnershipTransferMaxAttempts && response.Code != "oauth_ownership_code_invalid" {
			t.Fatalf("response for attempt %d = %s", attempt, recorder.Body.String())
		}
		if attempt == model.OAuthOwnershipTransferMaxAttempts && (response.Code != oauthOwnershipTransferClosedCode || !response.Data.Closed) {
			t.Fatalf("final response = %s", recorder.Body.String())
		}
	}

	stored, err := model.GetOAuthOwnershipTransferById(challenge.Id)
	if err != nil {
		t.Fatalf("reload challenge: %v", err)
	}
	if stored.Status != model.OAuthOwnershipTransferStatusFailed || stored.FailureReason != "too_many_invalid_codes" {
		t.Fatalf("closed challenge = %#v", stored)
	}
}

func TestConfirmOAuthOwnershipTransferExpiryClosesPermanently(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	codeSentAt := common.GetTimestamp() - int64(common.VerificationValidMinutes*60) - 1
	challenge, _, _ := prepareControllerOAuthOwnershipTransfer(t, db, codeSentAt)
	ctx, recorder, session := newOAuthOwnershipTransferTestContext(t, `{"code":"abc123"}`)
	session.Set(oauthOwnershipTransferSessionKey, challenge.Id)
	if err := session.Save(); err != nil {
		t.Fatalf("save ownership transfer session: %v", err)
	}

	ConfirmOAuthOwnershipTransfer(ctx)

	var response struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
	}
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	if response.Success || response.Code != oauthOwnershipTransferClosedCode {
		t.Fatalf("response = %s", recorder.Body.String())
	}
	stored, err := model.GetOAuthOwnershipTransferById(challenge.Id)
	if err != nil {
		t.Fatalf("reload challenge: %v", err)
	}
	if stored.Status != model.OAuthOwnershipTransferStatusFailed || stored.FailureReason != "code_expired" {
		t.Fatalf("expired challenge = %#v", stored)
	}
}
