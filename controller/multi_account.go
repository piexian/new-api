package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type multiAccountBanRequest struct {
	Reason          string `json:"reason"`
	DurationMinutes int    `json:"duration_minutes"`
}

type multiAccountReviewPassRequest struct {
	UserIds []int  `json:"user_ids"`
	Note    string `json:"note"`
	// LastSeenAt 为管理员审查时聚类的最新证据时间
	LastSeenAt int64 `json:"last_seen_at"`
}

// PassMultiAccountReview 将整个聚类标记为「本次审查通过」，
// 不封禁账号、不加入白名单；之后出现更新证据时聚类会重新出现在列表中。
func PassMultiAccountReview(c *gin.Context) {
	var request multiAccountReviewPassRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	err := service.PassMultiAccountReview(request.UserIds, c.GetInt("id"), request.Note, request.LastSeenAt)
	switch {
	case err == nil:
		common.ApiSuccess(c, nil)
	case errors.Is(err, model.ErrMultiAccountReviewEmptyUsers):
		common.ApiErrorI18n(c, i18n.MsgRiskMultiAccountReviewEmptyUsers)
	case errors.Is(err, model.ErrMultiAccountReviewNoteTooLong):
		common.ApiErrorI18n(c, i18n.MsgRiskMultiAccountReviewNoteTooLong, map[string]any{"Max": model.MultiAccountReviewNoteMaxLength})
	case errors.Is(err, model.ErrMultiAccountReviewUnknownUsers):
		common.ApiErrorI18n(c, i18n.MsgRiskMultiAccountReviewUnknownUsers)
	default:
		common.ApiError(c, err)
	}
}

func ListMultiAccountClusters(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	result, err := service.ListMultiAccountClusters(pageInfo.GetPage(), pageInfo.GetPageSize(), c.Query("keyword"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func BanMultiAccountUser(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	var request multiAccountBanRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "无效的请求参数")
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" {
		common.ApiErrorMsg(c, "封禁原因不能为空")
		return
	}
	user, err := service.BanMultiAccountUser(userId, c.GetInt("id"), request.DurationMinutes, request.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, user)
}
