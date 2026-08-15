package controller

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 工单诉求类型白名单（spec 3.3：提额/降息/宽限/其他）
var loanApplicationTopics = map[string]bool{
	"credit": true,
	"rate":   true,
	"grace":  true,
	"other":  true,
}

// respondLoanError 将 model/service 层哨兵错误映射为 i18n 响应；未知错误走通用 ApiError
func respondLoanError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrLoanDisabled):
		common.ApiErrorI18n(c, i18n.MsgLoanDisabled)
	case errors.Is(err, model.ErrLoanTermsNotAgreed):
		common.ApiErrorI18n(c, i18n.MsgLoanTermsRequired)
	case errors.Is(err, model.ErrLoanLimitExceeded):
		common.ApiErrorI18n(c, i18n.MsgLoanLimitExceeded)
	case errors.Is(err, model.ErrLoanInvalidAmount):
		common.ApiErrorI18n(c, i18n.MsgLoanInvalidAmount)
	case errors.Is(err, model.ErrLoanRegisterTooNew):
		common.ApiErrorI18n(c, i18n.MsgLoanRegisterTooNew)
	case errors.Is(err, model.ErrLoanQuotaOverflow):
		common.ApiErrorI18n(c, i18n.MsgLoanQuotaOverflow)
	case errors.Is(err, model.ErrLoanUserDisabled):
		common.ApiErrorI18n(c, i18n.MsgLoanUserDisabled)
	case errors.Is(err, model.ErrLoanNoDebt):
		common.ApiErrorI18n(c, i18n.MsgLoanNoDebt)
	case errors.Is(err, model.ErrLoanInsufficientBalance):
		common.ApiErrorI18n(c, i18n.MsgLoanInsufficientBalance)
	case errors.Is(err, model.ErrLoanApplicationLimit):
		common.ApiErrorI18n(c, i18n.MsgLoanApplicationLimit)
	case errors.Is(err, model.ErrLoanAlreadyRated):
		common.ApiErrorI18n(c, i18n.MsgLoanAlreadyRated)
	case errors.Is(err, model.ErrLoanInvalidRating):
		common.ApiErrorI18n(c, i18n.MsgLoanInvalidRating)
	case errors.Is(err, model.ErrLoanApplicationNotOpen):
		common.ApiErrorI18n(c, i18n.MsgLoanApplicationNotOpen)
	case errors.Is(err, service.ErrLoanOfficerBusy):
		common.ApiErrorI18n(c, i18n.MsgLoanReplyInProgress)
	case errors.Is(err, service.ErrLoanOfficerNoModel):
		common.ApiErrorI18n(c, i18n.MsgLoanOfficerNoModel)
	case errors.Is(err, service.ErrLoanContentTooLong):
		common.ApiErrorI18n(c, i18n.MsgLoanContentTooLong)
	case errors.Is(err, service.ErrLoanOfficerUnavailable):
		common.ApiErrorI18n(c, i18n.MsgLoanOfficerUnavailable)
	case errors.Is(err, gorm.ErrRecordNotFound):
		common.ApiErrorI18n(c, i18n.MsgLoanNotFound)
	default:
		respondLoanInternalError(c, err)
	}
}

// respondLoanInternalError 未知内部错误统一响应 i18n 兜底文案，原始错误只进服务端日志，
// 避免把 gorm/数据库英文原文透出给用户
func respondLoanInternalError(c *gin.Context, err error) {
	common.SysError("loan api internal error: " + err.Error())
	common.ApiErrorI18n(c, i18n.MsgLoanInternalError)
}

// buildLoanStatusData 组装 status 响应字段；acc 为 nil 时按无贷用户返回零值。
// 金额字段一律为整数 quota，USD 展示由前端换算。只读投影，不落盘。
func buildLoanStatusData(setting *operation_setting.LoanSetting, acc *model.TokenLoanAccount, now time.Time) gin.H {
	var principal, debt, interest int64
	var interestFreeUntil int
	var totalBorrowed, totalRepaid int64
	termsAgreed := false
	effectiveMax := setting.MaxTotal
	dailyRate := setting.DailyRate
	if acc != nil {
		debt, interest = model.ProjectLoanStatus(acc, now)
		principal = acc.PrincipalQuota
		interestFreeUntil = acc.InterestFreeUntil
		totalBorrowed = acc.TotalBorrowed
		totalRepaid = acc.TotalRepaid
		termsAgreed = acc.TermsAgreedAt != 0
		// 个人覆盖只降不升：上限直接覆盖，利率取较小者（与 model.effectiveRate 一致）
		if acc.CustomMaxTotal > 0 {
			effectiveMax = acc.CustomMaxTotal
		}
		if acc.CustomDailyRate > 0 && acc.CustomDailyRate < setting.DailyRate {
			dailyRate = acc.CustomDailyRate
		}
	}
	available := effectiveMax - debt
	if available < 0 {
		available = 0
	}
	return gin.H{
		"enabled":             setting.Enabled,
		"principal":           principal,
		"interest":            interest,
		"debt":                debt,
		"available":           available,
		"effective_max":       effectiveMax,
		"daily_rate":          dailyRate,
		"interest_free_until": interestFreeUntil,
		"total_borrowed":      totalBorrowed,
		"total_repaid":        totalRepaid,
		"ai_enabled":          setting.AiEnabled,
		"terms_enabled":       setting.TermsEnabled,
		"terms_agreed":        termsAgreed,
		"terms_text":          setting.TermsText,
		"repay_fee_rate":      setting.RepayFeeRate,
	}
}

// parseLoanAppId 解析路径中的工单 id，失败时输出 i18n 错误并返回 false
func parseLoanAppId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return 0, false
	}
	return id, true
}

// GetLoanStatus 查询当前用户词元贷状态（只读投影，不落盘结算）
func GetLoanStatus(c *gin.Context) {
	userId := c.GetInt("id")
	setting := operation_setting.GetLoanSetting()
	acc, err := model.GetLoanAccountReadOnly(userId)
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	common.ApiSuccess(c, buildLoanStatusData(setting, acc, time.Now()))
}

// AgreeLoanTerms 同意词元贷声明（幂等）
func AgreeLoanTerms(c *gin.Context) {
	userId := c.GetInt("id")
	if err := model.AgreeLoanTerms(userId); err != nil {
		respondLoanError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"terms_agreed": true})
}

type borrowLoanRequest struct {
	AmountUsd string `json:"amount_usd"`
}

// BorrowLoan 借款，成功返回最新状态（同 GET status 字段）
func BorrowLoan(c *gin.Context) {
	var req borrowLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	amountUsd := strings.TrimSpace(req.AmountUsd)
	if amountUsd == "" {
		common.ApiErrorI18n(c, i18n.MsgLoanInvalidAmount)
		return
	}
	userId := c.GetInt("id")
	acc, err := model.BorrowLoan(userId, amountUsd)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	// 用户自助操作审计：归属用户本人，无 admin_info（同 recordUserSecurityAudit 语义）
	recordUserSecurityAudit(c, userId, "loan.borrow", map[string]interface{}{
		"amount_usd": amountUsd,
		"debt_after": logger.LogQuota(int(acc.DebtQuota)),
	})
	common.ApiSuccess(c, buildLoanStatusData(operation_setting.GetLoanSetting(), acc, time.Now()))
}

type repayLoanRequest struct {
	AmountUsd string `json:"amount_usd"`
}

// RepayLoan 提前还款（amount_usd 为美元金额或 "all" 全部还清），成功返回最新状态与本次还款拆分
func RepayLoan(c *gin.Context) {
	var req repayLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	amountUsd := strings.TrimSpace(req.AmountUsd)
	if amountUsd == "" {
		common.ApiErrorI18n(c, i18n.MsgLoanInvalidAmount)
		return
	}
	userId := c.GetInt("id")
	acc, info, err := model.RepayLoan(userId, amountUsd)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	recordUserSecurityAudit(c, userId, "loan.repay", map[string]interface{}{
		"amount":         logger.LogQuota(int(info.Amount)),
		"interest_part":  logger.LogQuota(int(info.InterestPart)),
		"principal_part": logger.LogQuota(int(info.PrincipalPart)),
		"fee_part":       logger.LogQuota(int(info.FeePart)),
		"debt_after":     logger.LogQuota(int(info.DebtAfter)),
	})
	data := buildLoanStatusData(operation_setting.GetLoanSetting(), acc, time.Now())
	data["repay"] = info
	common.ApiSuccess(c, data)
}

// GetLoanRecords 分页查询当前用户台账（id 倒序）
func GetLoanRecords(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.GetUserLoanRecords(userId, pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}

type createLoanApplicationRequest struct {
	Topic   string `json:"topic"`
	Content string `json:"content"`
}

// CreateLoanApplication 新建 AI 业务员工单并执行首轮对话，返回工单与首轮回复
func CreateLoanApplication(c *gin.Context) {
	var req createLoanApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	topic := strings.TrimSpace(req.Topic)
	content := strings.TrimSpace(req.Content)
	if !loanApplicationTopics[topic] {
		common.ApiErrorI18n(c, i18n.MsgLoanInvalidTopic)
		return
	}
	if content == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	setting := operation_setting.GetLoanSetting()
	if !setting.Enabled {
		common.ApiErrorI18n(c, i18n.MsgLoanDisabled)
		return
	}
	if !setting.AiEnabled {
		common.ApiErrorI18n(c, i18n.MsgLoanOfficerDisabled)
		return
	}
	userId := c.GetInt("id")
	if setting.TermsEnabled {
		acc, err := model.GetLoanAccountReadOnly(userId)
		if err != nil {
			respondLoanInternalError(c, err)
			return
		}
		if acc == nil || acc.TermsAgreedAt == 0 {
			common.ApiErrorI18n(c, i18n.MsgLoanTermsRequired)
			return
		}
	}
	modelCfg, ok := service.PickLoanOfficerModel(setting)
	if !ok {
		common.ApiErrorI18n(c, i18n.MsgLoanOfficerNoModel)
		return
	}
	app, err := model.CreateLoanApplication(userId, topic, modelCfg.Model)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	// 首轮对话失败时工单已存在，用户可在详情页继续回复，这里只回报错误
	reply, closed, err := service.RunLoanOfficerRound(userId, app, content)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"application": app,
		"reply":       reply,
		"closed":      closed,
	})
}

// GetLoanApplications 分页返回当前用户的工单列表（id 倒序）
func GetLoanApplications(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	apps, err := model.GetUserLoanApplications(userId, pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	var total int64
	if err := model.DB.Model(&model.TokenLoanApplication{}).
		Where("user_id = ?", userId).Count(&total).Error; err != nil {
		respondLoanInternalError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(apps)
	common.ApiSuccess(c, pageInfo)
}

// GetLoanApplicationDetail 返回工单详情与全部对话消息（仅本人）
func GetLoanApplicationDetail(c *gin.Context) {
	appId, ok := parseLoanAppId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	app, err := model.GetLoanApplicationById(userId, appId)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	msgs, err := model.GetLoanApplicationMessages(app.Id)
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	// 历史数据可能含未剥离的 <think> 思考块，透出前统一清洗
	for i := range msgs {
		if msgs[i].Role == "assistant" {
			msgs[i].Content = service.StripLoanThinkContent(msgs[i].Content)
		}
	}
	common.ApiSuccess(c, gin.H{
		"application": app,
		"messages":    msgs,
	})
}

type replyLoanApplicationRequest struct {
	Content string `json:"content"`
}

// ReplyLoanApplication 在 open 工单下追加一轮 AI 对话（进行中轮次互斥由 service 层保证）
func ReplyLoanApplication(c *gin.Context) {
	appId, ok := parseLoanAppId(c)
	if !ok {
		return
	}
	var req replyLoanApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	app, err := model.GetLoanApplicationById(userId, appId)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	if app.Status != model.LoanAppStatusOpen {
		common.ApiErrorI18n(c, i18n.MsgLoanApplicationNotOpen)
		return
	}
	reply, closed, err := service.RunLoanOfficerRound(userId, app, content)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"reply":  reply,
		"closed": closed,
	})
}

type rateLoanApplicationRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

// RateLoanApplication 对已结案工单评分（1-5，条件更新保证仅一次）
func RateLoanApplication(c *gin.Context) {
	appId, ok := parseLoanAppId(c)
	if !ok {
		return
	}
	var req rateLoanApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	if err := model.RateLoanApplication(userId, appId, req.Rating, req.Comment); err != nil {
		respondLoanError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
