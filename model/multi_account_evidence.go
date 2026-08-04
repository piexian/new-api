package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MultiAccountEvidenceGitHubEmailConflict = "github_email_conflict"
	MultiAccountRuleID                      = "multi_account_review"
	MultiAccountRuleName                    = "Multi-account review"
)

// MultiAccountEvidence stores explicit account-link evidence that cannot be
// reconstructed reliably from ordinary login logs.
type MultiAccountEvidence struct {
	Id            int    `json:"id"`
	EvidenceKey   string `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	EvidenceType  string `json:"evidence_type" gorm:"type:varchar(32);index"`
	PrimaryUserId int    `json:"primary_user_id" gorm:"index"`
	RelatedUserId int    `json:"related_user_id" gorm:"index"`
	Email         string `json:"email" gorm:"type:varchar(255);index"`
	HitCount      int    `json:"hit_count" gorm:"default:1"`
	FirstSeenAt   int64  `json:"first_seen_at" gorm:"index"`
	LastSeenAt    int64  `json:"last_seen_at" gorm:"index"`
}

type LoginEnvironmentSignal struct {
	UserId    int
	Username  string
	CreatedAt int64
	IP        string
	UserAgent string
}

func multiAccountEvidenceKey(evidenceType string, primaryUserId, relatedUserId int, email string) string {
	payload := fmt.Sprintf("%s:%d:%d:%s", evidenceType, primaryUserId, relatedUserId, NormalizeEmail(email))
	return fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))
}

func UpsertGitHubEmailConflictEvidence(primaryUserId, relatedUserId int, email string, seenAt int64) error {
	email = NormalizeEmail(email)
	if primaryUserId <= 0 || relatedUserId <= 0 || primaryUserId == relatedUserId || email == "" {
		return errors.New("invalid GitHub email conflict evidence")
	}
	if seenAt <= 0 {
		seenAt = common.GetTimestamp()
	}
	evidence := MultiAccountEvidence{
		EvidenceKey:   multiAccountEvidenceKey(MultiAccountEvidenceGitHubEmailConflict, primaryUserId, relatedUserId, email),
		EvidenceType:  MultiAccountEvidenceGitHubEmailConflict,
		PrimaryUserId: primaryUserId,
		RelatedUserId: relatedUserId,
		Email:         email,
		HitCount:      1,
		FirstSeenAt:   seenAt,
		LastSeenAt:    seenAt,
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "evidence_key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"email":        email,
			"hit_count":    gorm.Expr("hit_count + ?", 1),
			"last_seen_at": seenAt,
		}),
	}).Create(&evidence).Error
}

func ListMultiAccountEvidence(limit int) ([]MultiAccountEvidence, error) {
	if limit <= 0 || limit > 20000 {
		limit = 20000
	}
	var evidence []MultiAccountEvidence
	err := DB.Order("last_seen_at DESC").Limit(limit).Find(&evidence).Error
	return evidence, err
}

func ListMultiAccountReviewTimes() (map[int]int64, error) {
	var rows []struct {
		UserId     int
		ReviewedAt int64
	}
	err := DB.Model(&RiskBanLog{}).
		Select("user_id, MAX(created_at) AS reviewed_at").
		Where("rule_id = ? AND user_id > 0 AND dry_run = ?", MultiAccountRuleID, false).
		Group("user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	reviewTimes := make(map[int]int64, len(rows))
	for _, row := range rows {
		reviewTimes[row.UserId] = row.ReviewedAt
	}
	return reviewTimes, nil
}

func GetUsersByNormalizedEmailUnscoped(email string, excludeUserId int) ([]User, error) {
	email = NormalizeEmail(email)
	if email == "" {
		return []User{}, nil
	}
	query := DB.Unscoped().Omit("password", "access_token").Where("LOWER(email) = ?", email)
	if excludeUserId > 0 {
		query = query.Where("id <> ?", excludeUserId)
	}
	var users []User
	err := query.Order("id ASC").Find(&users).Error
	return users, err
}

func GetUsersByIDsUnscoped(ids []int) ([]User, error) {
	if len(ids) == 0 {
		return []User{}, nil
	}
	unique := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			unique[id] = struct{}{}
		}
	}
	ids = ids[:0]
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var users []User
	err := DB.Unscoped().Omit("password", "access_token").Where("id IN ?", ids).Find(&users).Error
	return users, err
}

func ListRecentLoginEnvironmentSignals(since int64, limit int) ([]LoginEnvironmentSignal, error) {
	if LOG_DB == nil {
		return []LoginEnvironmentSignal{}, nil
	}
	if limit <= 0 || limit > 50000 {
		limit = 50000
	}
	var logs []Log
	query := LOG_DB.Model(&Log{}).
		Select("user_id", "username", "created_at", "ip", "user_agent", "other").
		Where("type = ? AND user_id > 0", LogTypeLogin)
	if since > 0 {
		query = query.Where("created_at >= ?", since)
	}
	if err := query.Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}

	signals := make([]LoginEnvironmentSignal, 0, len(logs))
	for _, log := range logs {
		ip := strings.TrimSpace(log.Ip)
		userAgent := strings.TrimSpace(log.UserAgent)
		if userAgent == "" && log.Other != "" {
			var extra map[string]interface{}
			if err := common.UnmarshalJsonStr(log.Other, &extra); err == nil {
				if value, ok := extra["user_agent"].(string); ok {
					userAgent = strings.TrimSpace(value)
				}
			}
		}
		if ip == "" || userAgent == "" {
			continue
		}
		signals = append(signals, LoginEnvironmentSignal{
			UserId:    log.UserId,
			Username:  log.Username,
			CreatedAt: log.CreatedAt,
			IP:        ip,
			UserAgent: userAgent,
		})
	}
	return signals, nil
}

func DisableUserByMultiAccountReview(id int, reason string, durationMinutes int, operatorId int, bannedAt int64) (*User, error) {
	reason = strings.TrimSpace(reason)
	if id <= 0 {
		return nil, errors.New("无效的用户 ID")
	}
	if reason == "" {
		return nil, errors.New("封禁原因不能为空")
	}
	if len(reason) > DisableReasonMaxLength {
		return nil, fmt.Errorf("封禁原因不能超过 %d 个字符", DisableReasonMaxLength)
	}
	if durationMinutes < 0 || durationMinutes > UserBanMaxMinutes {
		return nil, fmt.Errorf("封禁时长必须在 0 到 %d 分钟之间", UserBanMaxMinutes)
	}
	if bannedAt <= 0 {
		bannedAt = common.GetTimestamp()
	}
	disabledUntil := int64(0)
	if durationMinutes > 0 {
		disabledUntil = bannedAt + int64(durationMinutes)*60
	}

	var user User
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&user, id).Error; err != nil {
			return err
		}
		if user.Role != common.RoleCommonUser {
			return errors.New("只能封禁普通用户")
		}
		if user.Status != common.UserStatusEnabled {
			return errors.New("该用户当前不可封禁")
		}
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"status":         common.UserStatusDisabled,
			"disable_reason": reason,
			"disabled_until": disabledUntil,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&RiskBanLog{
			Dimension:       RiskBanDimensionUser,
			UserId:          user.Id,
			Username:        user.Username,
			Source:          RiskBanSourceManual,
			RuleId:          MultiAccountRuleID,
			RuleName:        MultiAccountRuleName,
			Action:          "disable_user",
			DurationMinutes: durationMinutes,
			IsPermanent:     durationMinutes == 0,
			UnbanAt:         disabledUntil,
			Reason:          reason,
			OperatorId:      operatorId,
			DryRun:          false,
			CreatedAt:       bannedAt,
		}).Error; err != nil {
			return err
		}
		user.Status = common.UserStatusDisabled
		user.DisableReason = reason
		user.DisabledUntil = disabledUntil
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}
