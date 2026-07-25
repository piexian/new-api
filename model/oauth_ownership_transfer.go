package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	OAuthOwnershipTransferModeLogin = "login"
	OAuthOwnershipTransferModeBind  = "bind"

	OAuthOwnershipTransferStatusReady     = "ready"
	OAuthOwnershipTransferStatusSending   = "sending"
	OAuthOwnershipTransferStatusCodeSent  = "code_sent"
	OAuthOwnershipTransferStatusVerifying = "verifying"
	OAuthOwnershipTransferStatusCompleted = "completed"
	OAuthOwnershipTransferStatusFailed    = "failed"

	OAuthOwnershipTransferMaxAttempts = 5
)

var (
	ErrOAuthOwnershipTransferUnavailable = errors.New("OAuth ownership transfer opportunity is unavailable")
	ErrOAuthOwnershipTransferState       = errors.New("OAuth ownership transfer state is invalid")
)

// OAuthOwnershipTransfer is a permanent, single-use record for one provider
// identity and one normalized email. Completed and failed rows are retained so
// the same pair can never receive another verification opportunity.
type OAuthOwnershipTransfer struct {
	Id               int    `json:"id"`
	PairKey          string `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	ProviderKey      string `json:"provider_key" gorm:"type:varchar(96);index"`
	ProviderName     string `json:"provider_name" gorm:"type:varchar(96)"`
	ProviderUserId   string `json:"provider_user_id" gorm:"type:varchar(256);index"`
	BindingColumn    string `json:"-" gorm:"type:varchar(32)"`
	CustomProviderId int    `json:"-" gorm:"index"`
	Email            string `json:"email" gorm:"type:varchar(255);index"`
	PreviousUserId   int    `json:"previous_user_id" gorm:"index"`
	TargetUserId     int    `json:"target_user_id" gorm:"index"`
	Mode             string `json:"mode" gorm:"type:varchar(16)"`
	Status           string `json:"status" gorm:"type:varchar(24);index"`
	FailureReason    string `json:"failure_reason,omitempty" gorm:"type:varchar(64)"`
	CreatedAt        int64  `json:"created_at" gorm:"index"`
	CodeSentAt       int64  `json:"code_sent_at"`
	AttemptedAt      int64  `json:"attempted_at"`
	FailedAttempts   int    `json:"failed_attempts"`
	CompletedAt      int64  `json:"completed_at"`
}

type OAuthOwnershipTransferResult struct {
	Challenge    OAuthOwnershipTransfer
	PreviousUser User
	TargetUser   User
}

func OAuthOwnershipTransferPairKey(providerKey, providerUserId, email string) string {
	payload := strings.TrimSpace(providerKey) + "\x00" + strings.TrimSpace(providerUserId) + "\x00" + NormalizeEmail(email)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))
}

func PrepareOAuthOwnershipTransfer(
	providerKey string,
	providerName string,
	providerUserId string,
	email string,
	previousUserId int,
	targetUserId int,
	mode string,
	bindingColumn string,
	customProviderId int,
	createdAt int64,
) (*OAuthOwnershipTransfer, error) {
	providerKey = strings.TrimSpace(providerKey)
	providerName = strings.TrimSpace(providerName)
	providerUserId = strings.TrimSpace(providerUserId)
	bindingColumn = strings.TrimSpace(bindingColumn)
	email = NormalizeEmail(email)
	if providerKey == "" || providerName == "" || providerUserId == "" || email == "" ||
		previousUserId <= 0 || targetUserId <= 0 || previousUserId == targetUserId {
		return nil, ErrOAuthOwnershipTransferState
	}
	if (bindingColumn == "") == (customProviderId <= 0) {
		return nil, ErrOAuthOwnershipTransferState
	}
	if bindingColumn != "" && !isOAuthOwnershipBindingColumn(bindingColumn) {
		return nil, ErrOAuthOwnershipTransferState
	}
	if mode != OAuthOwnershipTransferModeLogin && mode != OAuthOwnershipTransferModeBind {
		return nil, ErrOAuthOwnershipTransferState
	}
	if createdAt <= 0 {
		createdAt = common.GetTimestamp()
	}

	pairKey := OAuthOwnershipTransferPairKey(providerKey, providerUserId, email)
	var challenge OAuthOwnershipTransfer
	err := DB.Transaction(func(tx *gorm.DB) error {
		var previous User
		if err := lockForUpdate(tx).First(&previous, previousUserId).Error; err != nil {
			return err
		}
		if previous.Role != common.RoleCommonUser || previous.Status != common.UserStatusEnabled || NormalizeEmail(previous.Email) != email {
			return ErrOAuthOwnershipTransferUnavailable
		}

		var target User
		if err := lockForUpdate(tx).First(&target, targetUserId).Error; err != nil {
			return err
		}
		if target.Status != common.UserStatusEnabled || NormalizeEmail(target.Email) != "" {
			return ErrOAuthOwnershipTransferUnavailable
		}
		if !oauthOwnershipBindingMatches(tx, &target, bindingColumn, customProviderId, providerUserId) {
			return ErrOAuthOwnershipTransferUnavailable
		}

		candidate := OAuthOwnershipTransfer{
			PairKey:          pairKey,
			ProviderKey:      providerKey,
			ProviderName:     providerName,
			ProviderUserId:   providerUserId,
			BindingColumn:    bindingColumn,
			CustomProviderId: customProviderId,
			Email:            email,
			PreviousUserId:   previousUserId,
			TargetUserId:     targetUserId,
			Mode:             mode,
			Status:           OAuthOwnershipTransferStatusReady,
			CreatedAt:        createdAt,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "pair_key"}},
			DoNothing: true,
		}).Create(&candidate).Error; err != nil {
			return err
		}
		if err := lockForUpdate(tx).Where("pair_key = ?", pairKey).First(&challenge).Error; err != nil {
			return err
		}
		if challenge.ProviderKey != providerKey || challenge.ProviderUserId != providerUserId ||
			NormalizeEmail(challenge.Email) != email || challenge.PreviousUserId != previousUserId ||
			challenge.TargetUserId != targetUserId || challenge.Mode != mode ||
			challenge.BindingColumn != bindingColumn || challenge.CustomProviderId != customProviderId {
			return ErrOAuthOwnershipTransferUnavailable
		}
		if challenge.Status != OAuthOwnershipTransferStatusReady && challenge.Status != OAuthOwnershipTransferStatusCodeSent {
			return ErrOAuthOwnershipTransferUnavailable
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &challenge, nil
}

func isOAuthOwnershipBindingColumn(column string) bool {
	switch column {
	case "github_id", "discord_id", "oidc_id", "linux_do_id", "wechat_id", "telegram_id", "qq_id", "steam_id":
		return true
	default:
		return false
	}
}

func oauthOwnershipBindingMatches(tx *gorm.DB, target *User, bindingColumn string, customProviderId int, providerUserId string) bool {
	if target == nil || target.Id <= 0 {
		return false
	}
	if customProviderId > 0 {
		var binding UserOAuthBinding
		if err := lockForUpdate(tx).
			Where("user_id = ? AND provider_id = ? AND provider_user_id = ?", target.Id, customProviderId, providerUserId).
			First(&binding).Error; err != nil {
			return false
		}
		return true
	}
	switch bindingColumn {
	case "github_id":
		return target.GitHubId == providerUserId
	case "discord_id":
		return target.DiscordId == providerUserId
	case "oidc_id":
		return target.OidcId == providerUserId
	case "linux_do_id":
		return target.LinuxDOId == providerUserId
	case "wechat_id":
		return target.WeChatId == providerUserId
	case "telegram_id":
		return target.TelegramId == providerUserId
	case "qq_id":
		return target.QQId == providerUserId
	case "steam_id":
		return target.SteamId == providerUserId
	default:
		return false
	}
}

func GetOAuthOwnershipTransferById(id int) (*OAuthOwnershipTransfer, error) {
	if id <= 0 {
		return nil, ErrOAuthOwnershipTransferState
	}
	var challenge OAuthOwnershipTransfer
	if err := DB.First(&challenge, id).Error; err != nil {
		return nil, err
	}
	return &challenge, nil
}

func claimOAuthOwnershipTransferStatus(id int, fromStatus, toStatus string, updates map[string]interface{}) (*OAuthOwnershipTransfer, error) {
	if id <= 0 {
		return nil, ErrOAuthOwnershipTransferState
	}
	if updates == nil {
		updates = make(map[string]interface{})
	}
	updates["status"] = toStatus
	result := DB.Model(&OAuthOwnershipTransfer{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrOAuthOwnershipTransferUnavailable
	}
	return GetOAuthOwnershipTransferById(id)
}

func ClaimOAuthOwnershipTransferSend(id int, sentAt int64) (*OAuthOwnershipTransfer, error) {
	if sentAt <= 0 {
		sentAt = common.GetTimestamp()
	}
	return claimOAuthOwnershipTransferStatus(
		id,
		OAuthOwnershipTransferStatusReady,
		OAuthOwnershipTransferStatusSending,
		map[string]interface{}{"code_sent_at": sentAt},
	)
}

func ResetOAuthOwnershipTransferSend(id int) error {
	result := DB.Model(&OAuthOwnershipTransfer{}).
		Where("id = ? AND status = ?", id, OAuthOwnershipTransferStatusSending).
		Updates(map[string]interface{}{
			"status":       OAuthOwnershipTransferStatusReady,
			"code_sent_at": 0,
		})
	return result.Error
}

func MarkOAuthOwnershipTransferCodeSent(id int, sentAt int64) error {
	if sentAt <= 0 {
		sentAt = common.GetTimestamp()
	}
	result := DB.Model(&OAuthOwnershipTransfer{}).
		Where("id = ? AND status = ?", id, OAuthOwnershipTransferStatusSending).
		Updates(map[string]interface{}{
			"status":       OAuthOwnershipTransferStatusCodeSent,
			"code_sent_at": sentAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrOAuthOwnershipTransferUnavailable
	}
	return nil
}

func ClaimOAuthOwnershipTransferAttempt(id int, attemptedAt int64) (*OAuthOwnershipTransfer, error) {
	if attemptedAt <= 0 {
		attemptedAt = common.GetTimestamp()
	}
	return claimOAuthOwnershipTransferStatus(
		id,
		OAuthOwnershipTransferStatusCodeSent,
		OAuthOwnershipTransferStatusVerifying,
		map[string]interface{}{"attempted_at": attemptedAt},
	)
}

func RejectOAuthOwnershipTransferAttempt(id int, failedAt int64) (*OAuthOwnershipTransfer, error) {
	if failedAt <= 0 {
		failedAt = common.GetTimestamp()
	}
	var challenge OAuthOwnershipTransfer
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&challenge, id).Error; err != nil {
			return err
		}
		if challenge.Status != OAuthOwnershipTransferStatusVerifying {
			return ErrOAuthOwnershipTransferUnavailable
		}
		challenge.FailedAttempts++
		updates := map[string]interface{}{
			"failed_attempts": challenge.FailedAttempts,
			"attempted_at":    failedAt,
		}
		if challenge.FailedAttempts >= OAuthOwnershipTransferMaxAttempts {
			challenge.Status = OAuthOwnershipTransferStatusFailed
			challenge.FailureReason = "too_many_invalid_codes"
			challenge.CompletedAt = failedAt
			updates["status"] = challenge.Status
			updates["failure_reason"] = challenge.FailureReason
			updates["completed_at"] = failedAt
		} else {
			challenge.Status = OAuthOwnershipTransferStatusCodeSent
			updates["status"] = challenge.Status
		}
		return tx.Model(&challenge).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}
	return &challenge, nil
}

func ExpireOAuthOwnershipTransfer(id int, expiredAt int64) error {
	return CloseOAuthOwnershipTransfer(id, "code_expired", expiredAt)
}

func CloseOAuthOwnershipTransfer(id int, reason string, closedAt int64) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "closed"
	}
	if closedAt <= 0 {
		closedAt = common.GetTimestamp()
	}
	result := DB.Model(&OAuthOwnershipTransfer{}).
		Where("id = ? AND status IN ?", id, []string{
			OAuthOwnershipTransferStatusReady,
			OAuthOwnershipTransferStatusSending,
			OAuthOwnershipTransferStatusCodeSent,
			OAuthOwnershipTransferStatusVerifying,
		}).
		Updates(map[string]interface{}{
			"status":         OAuthOwnershipTransferStatusFailed,
			"failure_reason": reason,
			"completed_at":   closedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrOAuthOwnershipTransferUnavailable
	}
	return nil
}

func CompleteOAuthOwnershipTransfer(id int, completedAt int64) (*OAuthOwnershipTransferResult, error) {
	if id <= 0 {
		return nil, ErrOAuthOwnershipTransferState
	}
	if completedAt <= 0 {
		completedAt = common.GetTimestamp()
	}
	result := &OAuthOwnershipTransferResult{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&result.Challenge, id).Error; err != nil {
			return err
		}
		challenge := &result.Challenge
		if challenge.Status != OAuthOwnershipTransferStatusVerifying {
			return ErrOAuthOwnershipTransferUnavailable
		}
		return withNormalizedEmailLock(tx, challenge.Email, func(tx *gorm.DB) error {
			if err := lockForUpdate(tx).First(&result.PreviousUser, challenge.PreviousUserId).Error; err != nil {
				return err
			}
			if err := lockForUpdate(tx).First(&result.TargetUser, challenge.TargetUserId).Error; err != nil {
				return err
			}
			if result.PreviousUser.Role != common.RoleCommonUser || result.PreviousUser.Status != common.UserStatusEnabled ||
				NormalizeEmail(result.PreviousUser.Email) != challenge.Email ||
				result.TargetUser.Status != common.UserStatusEnabled || NormalizeEmail(result.TargetUser.Email) != "" ||
				!oauthOwnershipBindingMatches(tx, &result.TargetUser, challenge.BindingColumn, challenge.CustomProviderId, challenge.ProviderUserId) {
				return ErrOAuthOwnershipTransferUnavailable
			}

			reason := fmt.Sprintf(
				"OAuth ownership transfer: %s email %s moved to user #%d",
				challenge.ProviderName,
				challenge.Email,
				result.TargetUser.Id,
			)
			if err := tx.Model(&result.PreviousUser).Updates(map[string]interface{}{
				"email":          "",
				"status":         common.UserStatusDisabled,
				"disable_reason": reason,
				"disabled_until": 0,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&result.TargetUser).Update("email", challenge.Email).Error; err != nil {
				return err
			}
			if err := tx.Create(&RiskBanLog{
				Dimension:       RiskBanDimensionUser,
				UserId:          result.PreviousUser.Id,
				Username:        result.PreviousUser.Username,
				Source:          RiskBanSourceOAuthTransfer,
				RuleId:          OAuthOwnershipTransferRuleID,
				RuleName:        OAuthOwnershipTransferRuleName,
				Action:          "disable_user",
				DurationMinutes: 0,
				IsPermanent:     true,
				Reason:          reason,
				OperatorId:      result.TargetUser.Id,
				DryRun:          false,
				CreatedAt:       completedAt,
			}).Error; err != nil {
				return err
			}
			updateResult := tx.Model(challenge).
				Where("status = ?", OAuthOwnershipTransferStatusVerifying).
				Updates(map[string]interface{}{
					"status":         OAuthOwnershipTransferStatusCompleted,
					"failure_reason": "",
					"completed_at":   completedAt,
				})
			if updateResult.Error != nil {
				return updateResult.Error
			}
			if updateResult.RowsAffected != 1 {
				return ErrOAuthOwnershipTransferUnavailable
			}

			result.PreviousUser.Email = ""
			result.PreviousUser.Status = common.UserStatusDisabled
			result.PreviousUser.DisableReason = reason
			result.PreviousUser.DisabledUntil = 0
			result.TargetUser.Email = challenge.Email
			result.Challenge.Status = OAuthOwnershipTransferStatusCompleted
			result.Challenge.CompletedAt = completedAt
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
