package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MultiAccountReviewNoteMaxLength = 1000
	// multiAccountReviewBatchSize 聚类规模不设上限，按批写入以避开单条 SQL 的参数个数限制
	multiAccountReviewBatchSize = 500
)

var (
	ErrMultiAccountReviewNoteTooLong  = errors.New("multi-account review note is too long")
	ErrMultiAccountReviewEmptyUsers   = errors.New("multi-account review user list is empty")
	ErrMultiAccountReviewUnknownUsers = errors.New("multi-account review contains unknown users")
)

// MultiAccountReview 记录管理员对多开账号聚类做出的「本次审查通过」结论。
// 只表示截至 ReviewedAt 的已有证据已被人工确认，不等于白名单：
// 之后一旦出现更晚的证据（last_seen_at 晚于 ReviewedAt），聚类会重新出现在待审列表。
type MultiAccountReview struct {
	Id         int    `json:"id"`
	UserId     int    `json:"user_id" gorm:"uniqueIndex"`
	ReviewedAt int64  `json:"reviewed_at" gorm:"index"`
	OperatorId int    `json:"operator_id"`
	Note       string `json:"note" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

// UpsertMultiAccountReviews 为一批用户写入/刷新「本次审查通过」记录。
func UpsertMultiAccountReviews(userIds []int, operatorId int, note string, reviewedAt int64) error {
	note = strings.TrimSpace(note)
	if len([]rune(note)) > MultiAccountReviewNoteMaxLength {
		return ErrMultiAccountReviewNoteTooLong
	}
	unique := make(map[int]struct{}, len(userIds))
	ids := make([]int, 0, len(userIds))
	for _, id := range userIds {
		if id <= 0 {
			continue
		}
		if _, ok := unique[id]; ok {
			continue
		}
		unique[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return ErrMultiAccountReviewEmptyUsers
	}
	if reviewedAt <= 0 {
		reviewedAt = common.GetTimestamp()
	}
	users, err := GetUsersByIDsUnscoped(ids)
	if err != nil {
		return err
	}
	if len(users) != len(ids) {
		return ErrMultiAccountReviewUnknownUsers
	}

	now := common.GetTimestamp()
	reviews := make([]MultiAccountReview, 0, len(ids))
	for _, id := range ids {
		reviews = append(reviews, MultiAccountReview{
			UserId:     id,
			ReviewedAt: reviewedAt,
			OperatorId: operatorId,
			Note:       note,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"reviewed_at": reviewedAt,
				"operator_id": operatorId,
				"note":        note,
				"updated_at":  now,
			}),
		}).CreateInBatches(&reviews, multiAccountReviewBatchSize).Error
	})
}
