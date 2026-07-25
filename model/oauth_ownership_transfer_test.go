package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOAuthOwnershipTransferTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := DB
	oldLogDB := LOG_DB
	oldMainDatabaseType := common.MainDatabaseType()
	oldLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	DB = db
	LOG_DB = db
	if err := db.AutoMigrate(&User{}, &RiskBanLog{}, &OAuthOwnershipTransfer{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
		common.SetDatabaseTypes(oldMainDatabaseType, oldLogDatabaseType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func seedOAuthTransferUser(t *testing.T, db *gorm.DB, username, email, githubID string) *User {
	t.Helper()
	user := &User{
		Username:  username,
		Email:     email,
		GitHubId:  githubID,
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   username + "-aff",
		CreatedAt: common.GetTimestamp(),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user %s: %v", username, err)
	}
	return user
}

func TestOAuthOwnershipTransferCompletesOnceAndBansPreviousAccount(t *testing.T) {
	db := setupOAuthOwnershipTransferTestDB(t)
	previous := seedOAuthTransferUser(t, db, "previous-owner", "Owner@Example.com", "")
	target := seedOAuthTransferUser(t, db, "oauth-target", "", "github-123")
	now := common.GetTimestamp()

	challenge, err := PrepareOAuthOwnershipTransfer(
		"github",
		"GitHub",
		"github-123",
		"owner@example.com",
		previous.Id,
		target.Id,
		OAuthOwnershipTransferModeLogin,
		"github_id",
		0,
		now,
	)
	if err != nil {
		t.Fatalf("prepare transfer: %v", err)
	}
	repeated, err := PrepareOAuthOwnershipTransfer(
		"github",
		"GitHub",
		"github-123",
		"OWNER@example.com",
		previous.Id,
		target.Id,
		OAuthOwnershipTransferModeLogin,
		"github_id",
		0,
		now+1,
	)
	if err != nil {
		t.Fatalf("repeat pending transfer: %v", err)
	}
	if repeated.Id != challenge.Id {
		t.Fatalf("repeat challenge id = %d, want %d", repeated.Id, challenge.Id)
	}

	if _, err := ClaimOAuthOwnershipTransferSend(challenge.Id, now+2); err != nil {
		t.Fatalf("claim send: %v", err)
	}
	if err := MarkOAuthOwnershipTransferCodeSent(challenge.Id, now+3); err != nil {
		t.Fatalf("mark code sent: %v", err)
	}
	if _, err := ClaimOAuthOwnershipTransferAttempt(challenge.Id, now+4); err != nil {
		t.Fatalf("claim attempt: %v", err)
	}

	result, err := CompleteOAuthOwnershipTransfer(challenge.Id, now+5)
	if err != nil {
		t.Fatalf("complete transfer: %v", err)
	}
	if result.PreviousUser.Status != common.UserStatusDisabled || result.PreviousUser.Email != "" {
		t.Fatalf("previous user after transfer = %#v", result.PreviousUser)
	}
	if result.TargetUser.Email != "owner@example.com" {
		t.Fatalf("target email = %q", result.TargetUser.Email)
	}

	var banLog RiskBanLog
	if err := db.Where("user_id = ? AND source = ?", previous.Id, RiskBanSourceOAuthTransfer).First(&banLog).Error; err != nil {
		t.Fatalf("find OAuth transfer ban log: %v", err)
	}
	if banLog.OperatorId != target.Id || !banLog.IsPermanent {
		t.Fatalf("unexpected ban log: %#v", banLog)
	}

	_, err = PrepareOAuthOwnershipTransfer(
		"github",
		"GitHub",
		"github-123",
		"owner@example.com",
		previous.Id,
		target.Id,
		OAuthOwnershipTransferModeLogin,
		"github_id",
		0,
		now+6,
	)
	if !errors.Is(err, ErrOAuthOwnershipTransferUnavailable) {
		t.Fatalf("repeat completed transfer error = %v, want unavailable", err)
	}
}

func TestOAuthOwnershipTransferFifthFailedAttemptConsumesPair(t *testing.T) {
	db := setupOAuthOwnershipTransferTestDB(t)
	previous := seedOAuthTransferUser(t, db, "failed-owner", "failed@example.com", "")
	target := seedOAuthTransferUser(t, db, "failed-target", "", "github-failed")
	now := common.GetTimestamp()

	challenge, err := PrepareOAuthOwnershipTransfer(
		"github",
		"GitHub",
		"github-failed",
		"failed@example.com",
		previous.Id,
		target.Id,
		OAuthOwnershipTransferModeLogin,
		"github_id",
		0,
		now,
	)
	if err != nil {
		t.Fatalf("prepare transfer: %v", err)
	}
	if _, err := ClaimOAuthOwnershipTransferSend(challenge.Id, now+1); err != nil {
		t.Fatalf("claim send: %v", err)
	}
	if err := MarkOAuthOwnershipTransferCodeSent(challenge.Id, now+2); err != nil {
		t.Fatalf("mark code sent: %v", err)
	}
	for attempt := 1; attempt <= OAuthOwnershipTransferMaxAttempts; attempt++ {
		if _, err := ClaimOAuthOwnershipTransferAttempt(challenge.Id, now+int64(attempt*2+1)); err != nil {
			t.Fatalf("claim attempt %d: %v", attempt, err)
		}
		rejected, err := RejectOAuthOwnershipTransferAttempt(challenge.Id, now+int64(attempt*2+2))
		if err != nil {
			t.Fatalf("reject attempt %d: %v", attempt, err)
		}
		if rejected.FailedAttempts != attempt {
			t.Fatalf("failed attempts = %d, want %d", rejected.FailedAttempts, attempt)
		}
		if attempt < OAuthOwnershipTransferMaxAttempts && rejected.Status != OAuthOwnershipTransferStatusCodeSent {
			t.Fatalf("status after attempt %d = %q, want code_sent", attempt, rejected.Status)
		}
		if attempt == OAuthOwnershipTransferMaxAttempts && rejected.Status != OAuthOwnershipTransferStatusFailed {
			t.Fatalf("status after final attempt = %q, want failed", rejected.Status)
		}
	}

	_, err = PrepareOAuthOwnershipTransfer(
		"github",
		"GitHub",
		"github-failed",
		"failed@example.com",
		previous.Id,
		target.Id,
		OAuthOwnershipTransferModeLogin,
		"github_id",
		0,
		now+20,
	)
	if !errors.Is(err, ErrOAuthOwnershipTransferUnavailable) {
		t.Fatalf("repeat failed transfer error = %v, want unavailable", err)
	}
}
