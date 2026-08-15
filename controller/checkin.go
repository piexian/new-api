package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// GetCheckinStatus 获取用户签到状态和历史记录
func GetCheckinStatus(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "签到功能未启用")
		return
	}
	userId := c.GetInt("id")
	// 获取月份参数，默认为当前月份
	month := c.DefaultQuery("month", time.Now().Format("2006-01"))

	stats, err := model.GetUserCheckinStats(userId, month)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":   setting.Enabled,
			"min_quota": setting.MinQuota,
			"max_quota": setting.MaxQuota,
			"stats":     stats,
		},
	})
}

// DoCheckin 执行用户签到
func DoCheckin(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "签到功能未启用")
		return
	}

	userId := c.GetInt("id")

	checkin, loanRepay, lenderCredits, err := model.UserCheckin(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("用户签到，获得额度 %s", logger.LogQuota(checkin.QuotaAwarded)))
	data := gin.H{
		"quota_awarded": checkin.QuotaAwarded,
		"checkin_date":  checkin.CheckinDate,
	}
	// 签到自动还款结果，无还款时不输出该 key；有还款时记入操作日志
	if loanRepay != nil {
		data["loan_repay"] = loanRepay
		recordUserSecurityAudit(c, userId, "loan.checkin_repay", map[string]interface{}{
			"amount":         logger.LogQuota(int(loanRepay.Amount)),
			"interest_part":  logger.LogQuota(int(loanRepay.InterestPart)),
			"principal_part": logger.LogQuota(int(loanRepay.PrincipalPart)),
			"debt_after":     logger.LogQuota(int(loanRepay.DebtAfter)),
		})
	}
	// 签到自动还款触发的放贷收益入账计入充值日志；此处 IP/User-Agent 为签到者
	// （借款人）的请求上下文，即触发本次入账的请求方
	for _, credit := range lenderCredits {
		model.RecordTopupLog(credit.UserId, fmt.Sprintf("词元贷放贷收益入账，额度: %v（借款人还款）", logger.LogQuota(int(credit.Amount))), c.ClientIP(), "loan", "loan", c.GetHeader("User-Agent"))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "签到成功",
		"data":    data,
	})
}
