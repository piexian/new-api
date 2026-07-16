package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const oneTimeInviteCodePrefix = "ot_"

type OneTimeInviteCode struct {
	Id         int    `json:"id"`
	Code       string `json:"code" gorm:"type:varchar(32);uniqueIndex"`
	InviterId  int    `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`
	UsedUserId int    `json:"used_user_id" gorm:"type:int;column:used_user_id;index"`
	CreatedAt  int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UsedAt     int64  `json:"used_at" gorm:"type:bigint;column:used_at"`
}

type registrationCredentialKind int

const (
	registrationCredentialNone registrationCredentialKind = iota
	registrationCredentialReferral
	registrationCredentialOneTimeInvite
	registrationCredentialRedemption
)

type RegistrationCredential struct {
	Kind                registrationCredentialKind
	InviterId           int
	OneTimeInviteCodeId int
	RedemptionId        int
	RedemptionCount     int
	RedemptionLimit     int
	RedemptionExpiry    int64
	SourceName          string
}

func GenerateOneTimeInviteCode(inviterId int) (string, error) {
	if inviterId <= 0 {
		return "", ErrUserNotFound
	}
	var inviteCode OneTimeInviteCode
	err := DB.Transaction(func(tx *gorm.DB) error {
		var inviter User
		if err := lockForUpdate(tx).Select("id").First(&inviter, "id = ?", inviterId).Error; err != nil {
			return err
		}
		for i := 0; i < 10; i++ {
			candidate := oneTimeInviteCodePrefix + common.GetRandomString(24)
			var count int64
			if err := tx.Model(&OneTimeInviteCode{}).Where("code = ?", candidate).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			inviteCode = OneTimeInviteCode{
				Code:      candidate,
				InviterId: inviterId,
			}
			return tx.Create(&inviteCode).Error
		}
		return errors.New("failed to generate unique one-time invite code")
	})
	return inviteCode.Code, err
}

func resolveRegistrationCredentialWithTx(tx *gorm.DB, code string, required bool) (RegistrationCredential, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		if required {
			return RegistrationCredential{}, ErrRegistrationCredentialRequired
		}
		return RegistrationCredential{}, nil
	}

	credential, err := findRegistrationCredentialWithTx(tx, code)
	if err == nil {
		return credential, nil
	}
	if errors.Is(err, ErrRegistrationCredentialInvalid) && !required {
		return RegistrationCredential{}, nil
	}
	return RegistrationCredential{}, err
}

func findRegistrationCredentialWithTx(tx *gorm.DB, code string) (RegistrationCredential, error) {
	var inviter User
	lookup := tx.Select("id").Where("aff_code = ?", code).Limit(1).Find(&inviter)
	if lookup.Error != nil {
		return RegistrationCredential{}, lookup.Error
	}
	if lookup.RowsAffected == 1 {
		return RegistrationCredential{
			Kind:      registrationCredentialReferral,
			InviterId: inviter.Id,
		}, nil
	}

	var oneTimeInvite OneTimeInviteCode
	lookup = lockForUpdate(tx).Where("code = ?", code).Limit(1).Find(&oneTimeInvite)
	if lookup.Error != nil {
		return RegistrationCredential{}, lookup.Error
	}
	if lookup.RowsAffected == 1 {
		if oneTimeInvite.UsedUserId != 0 {
			return RegistrationCredential{}, ErrRegistrationCredentialInvalid
		}
		return RegistrationCredential{
			Kind:                registrationCredentialOneTimeInvite,
			InviterId:           oneTimeInvite.InviterId,
			OneTimeInviteCodeId: oneTimeInvite.Id,
		}, nil
	}

	var redemption Redemption
	lookup = lockForUpdate(tx).
		Where(commonKeyCol+" = ? AND type = ?", code, RedemptionTypeRegistration).
		Limit(1).
		Find(&redemption)
	if lookup.Error != nil {
		return RegistrationCredential{}, lookup.Error
	}
	if lookup.RowsAffected == 0 {
		return RegistrationCredential{}, ErrRegistrationCredentialInvalid
	}
	if redemption.Status != common.RedemptionCodeStatusEnabled ||
		redemption.MaxRedemptions <= 0 ||
		redemption.RedeemedCount >= redemption.MaxRedemptions ||
		(redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp()) {
		return RegistrationCredential{}, ErrRegistrationCredentialInvalid
	}
	return RegistrationCredential{
		Kind:             registrationCredentialRedemption,
		RedemptionId:     redemption.Id,
		RedemptionCount:  redemption.RedeemedCount,
		RedemptionLimit:  redemption.MaxRedemptions,
		RedemptionExpiry: redemption.ExpiredTime,
		SourceName:       redemption.Name,
	}, nil
}

func (credential RegistrationCredential) consumeWithTx(tx *gorm.DB, userId int) error {
	now := common.GetTimestamp()
	switch credential.Kind {
	case registrationCredentialNone, registrationCredentialReferral:
		return nil
	case registrationCredentialOneTimeInvite:
		result := tx.Model(&OneTimeInviteCode{}).
			Where("id = ? AND used_user_id = ?", credential.OneTimeInviteCodeId, 0).
			Updates(map[string]interface{}{
				"used_user_id": userId,
				"used_at":      now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRegistrationCredentialInvalid
		}
		return nil
	case registrationCredentialRedemption:
		newCount := credential.RedemptionCount + 1
		nextStatus := common.RedemptionCodeStatusEnabled
		if newCount >= credential.RedemptionLimit {
			nextStatus = common.RedemptionCodeStatusUsed
		}
		result := tx.Model(&Redemption{}).
			Where(
				"id = ? AND type = ? AND status = ? AND redeemed_count = ? AND max_redemptions = ? AND expired_time = ?",
				credential.RedemptionId,
				RedemptionTypeRegistration,
				common.RedemptionCodeStatusEnabled,
				credential.RedemptionCount,
				credential.RedemptionLimit,
				credential.RedemptionExpiry,
			).
			Updates(map[string]interface{}{
				"redeemed_time":  now,
				"status":         nextStatus,
				"used_user_id":   userId,
				"redeemed_count": newCount,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRegistrationCredentialInvalid
		}
		return nil
	default:
		return ErrRegistrationCredentialInvalid
	}
}
