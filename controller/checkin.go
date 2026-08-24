package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// rollCheckinRewardForRequest 计算一次签到（或补签）的最终奖励：
// 自适应计划（衰减/加成/风控锁底）→ 客户端环境+行为评分压制。
// 评分压制只压低不拦截，低分结果与一次"运气不好"的正常签到无法区分。
func rollCheckinRewardForRequest(c *gin.Context, userId int, isMakeup bool) int {
	setting := operation_setting.GetCheckinSetting()
	reward, _ := service.RollCheckinReward(userId, time.Now(), isMakeup)
	clientScore := service.CheckinClientScore(c, userId)
	reward = setting.ApplyClientScore(reward, clientScore)
	if clientScore < 100 {
		// 只进系统日志，绝不在响应中暴露评分存在
		common.SysLog(fmt.Sprintf("checkin reward suppressed by client score: user %d score %d", userId, clientScore))
	}
	return reward
}

// finishCheckinResponse 签到/补签共用的响应收尾：系统日志、还款信息、放贷人入账日志
func finishCheckinResponse(c *gin.Context, userId int, checkin *model.Checkin, loanRepay *model.LoanRepayInfo, lenderCredits []model.LenderCredit, action string) {
	model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("%s，获得额度 %s", action, logger.LogQuota(checkin.QuotaAwarded)))
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
		"message": action + "成功",
		"data":    data,
	})
}

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

	// 自适应信息：前端只展示「当前上限」（衰减后的有效上限）和连续天数，
	// 不暴露衰减/加成/风控规则本身
	plan := service.ComputeCheckinRewardPlan(userId, time.Now())
	streak, _ := model.CurrentCheckinStreak(userId)

	data := gin.H{
		"enabled":             setting.Enabled,
		"min_quota":           setting.MinQuota,
		"max_quota":           setting.MaxQuota,
		"effective_max_quota": plan.EffectiveMax,
		"streak_days":         streak,
		"stats":               stats,
	}
	if setting.MakeUpEnabled {
		eligible, err := model.MakeupEligibleDates(userId, setting.MakeUpMaxDays)
		if err == nil {
			data["makeup_eligible_dates"] = eligible
		}
		data["makeup_enabled"] = true
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
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
	reward := rollCheckinRewardForRequest(c, userId, false)

	checkin, loanRepay, lenderCredits, err := model.UserCheckin(userId, reward)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 风控观察评估依赖当天签到已落库，事务提交后异步执行
	go service.EvaluateCheckinRiskWatch(userId)
	finishCheckinResponse(c, userId, checkin, loanRepay, lenderCredits, "签到")
}

// DoMakeupCheckin 补签：为断签的历史日期补录签到。
// 奖励走同一套自适应逻辑，但补签日期命中特殊星期也不发固定大额。
func DoMakeupCheckin(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled || !setting.MakeUpEnabled {
		common.ApiErrorMsg(c, "补签功能未启用")
		return
	}

	var req struct {
		Date string `json:"date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Date == "" {
		common.ApiErrorMsg(c, "请提供补签日期")
		return
	}

	userId := c.GetInt("id")

	// 日期必须在可补签清单内（最近 MakeUpMaxDays 天、无既有记录、今天之前）
	eligible, err := model.MakeupEligibleDates(userId, setting.MakeUpMaxDays)
	if err != nil {
		common.ApiErrorMsg(c, "补签失败，请稍后重试")
		return
	}
	allowed := false
	for _, d := range eligible {
		if d == req.Date {
			allowed = true
			break
		}
	}
	if !allowed {
		common.ApiErrorMsg(c, "该日期不可补签")
		return
	}

	reward := rollCheckinRewardForRequest(c, userId, true)
	checkin, loanRepay, lenderCredits, err := model.UserMakeupCheckin(userId, req.Date, reward)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	finishCheckinResponse(c, userId, checkin, loanRepay, lenderCredits, "补签")
}
