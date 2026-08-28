package controller

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// AdminListCheckinRiskWatches 分页列出签到风控观察名单，可按 status 过滤
func AdminListCheckinRiskWatches(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := c.Query("status")
	items, total, err := model.ListCheckinRiskWatches(pageInfo.GetPage(), pageInfo.GetPageSize(), status)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// AdminGetCheckinRiskContrast 返回某用户最近 N 天的签到/调用逐日对比（风控判读用）
func AdminGetCheckinRiskContrast(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	rows, err := model.GetCheckinDailyContrast(userId, days)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// AdminReleaseCheckinRiskWatch 管理员手动解除用户的签到风控锁底
func AdminReleaseCheckinRiskWatch(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req) // 备注可选
	adminId := c.GetInt("id")
	if err := model.ReleaseCheckinRiskWatch(userId, adminId, req.Note); err != nil {
		common.ApiError(c, err)
		return
	}
	// 解除动作落审计：系统日志（含备注）+ 目标用户系统日志（不含备注）
	common.SysLog(fmt.Sprintf("checkin risk watch released: user %d, operator %d, note: %s", userId, adminId, req.Note))
	model.RecordLog(userId, model.LogTypeSystem, "您的签到风控锁已被管理员解除，签到奖励恢复正常")
	common.ApiSuccess(c, nil)
}
