package controller

import (
	"errors"
	"fmt"
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

// 工单诉求类型白名单（spec 3.3：提额/降息/宽限/其他；Task 15 增加减免申诉）
var loanApplicationTopics = map[string]bool{
	"credit": true,
	"rate":   true,
	"grace":  true,
	"other":  true,
	"appeal": true,
}

// loanOfferParamMsgKeys 挂单参数明细错误（model.LoanOfferParamError.Reason）→ i18n key
var loanOfferParamMsgKeys = map[string]string{
	"mode":            i18n.MsgLoanOfferInvalidMode,
	"amount_min":      i18n.MsgLoanOfferAmountBelowMin,
	"penalty":         i18n.MsgLoanOfferInvalidPenalty,
	"penalty_exceeds": i18n.MsgLoanOfferPenaltyExceeds,
	"window":          i18n.MsgLoanOfferInvalidWindow,
	"rate":            i18n.MsgLoanOfferInvalidRate,
	"rate_range":      i18n.MsgLoanOfferInvalidRateRange,
	"per_loan_cap":    i18n.MsgLoanOfferInvalidPerLoanCap,
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
	case errors.Is(err, model.ErrLoanLenderQuotaOverflow):
		common.ApiErrorI18n(c, i18n.MsgLoanLenderOverflow)
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
	case errors.Is(err, model.ErrLoanBlacklisted):
		common.ApiErrorI18n(c, i18n.MsgLoanBlacklisted)
	case errors.Is(err, model.ErrLoanHasOverdue):
		common.ApiErrorI18n(c, i18n.MsgLoanHasOverdue)
	case errors.Is(err, model.ErrLoanMarketDisabled):
		common.ApiErrorI18n(c, i18n.MsgLoanMarketDisabled)
	case errors.Is(err, model.ErrLoanDisclaimerRequired):
		common.ApiErrorI18n(c, i18n.MsgLoanDisclaimerRequired)
	case errors.Is(err, model.ErrLoanOfferNotFound):
		common.ApiErrorI18n(c, i18n.MsgLoanOfferNotFound)
	case errors.Is(err, model.ErrLoanOfferInvalidParams):
		// 明细参数错误：按 Reason 映射到具体 i18n key 并透传合法范围模板参数，
		// 无明细时回退泛用"参数无效"
		var pe *model.LoanOfferParamError
		if errors.As(err, &pe) {
			key := loanOfferParamMsgKeys[pe.Reason]
			if key == "" {
				key = i18n.MsgLoanOfferInvalidParams
			}
			common.ApiErrorI18n(c, key, pe.Params)
		} else {
			common.ApiErrorI18n(c, i18n.MsgLoanOfferInvalidParams)
		}
	case errors.Is(err, model.ErrLoanOfferNotActive):
		common.ApiErrorI18n(c, i18n.MsgLoanOfferNotActive)
	case errors.Is(err, model.ErrLoanNothingToWithdraw):
		common.ApiErrorI18n(c, i18n.MsgLoanNothingToWithdraw)
	case errors.Is(err, model.ErrLoanFundingNotOverdue):
		common.ApiErrorI18n(c, i18n.MsgLoanFundingNotOverdue)
	case errors.Is(err, model.ErrLoanInvalidDefaultAction):
		common.ApiErrorI18n(c, i18n.MsgLoanInvalidDefaultAction)
	case errors.Is(err, model.ErrLoanNotFundingOwner):
		common.ApiErrorI18n(c, i18n.MsgLoanNotFundingOwner)
	case errors.Is(err, model.ErrLoanInvalidRepayPlan):
		common.ApiErrorI18n(c, i18n.MsgLoanInvalidRepayPlan)
	case errors.Is(err, model.ErrLoanLendBorrowedNotAllowed):
		common.ApiErrorI18n(c, i18n.MsgLoanLendBorrowedNotAllowed)
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

// buildLoanStatusData 组装 status 响应字段；acc 为 nil 时按无贷用户返回零值
// （信用分取初始值，镜像 GetCreditScore）。金额字段一律为整数 quota，USD 展示由前端
// 换算。只读投影，不落盘；has_overdue 为 best-effort 只读判定，失败按 false 处理。
func buildLoanStatusData(setting *operation_setting.LoanSetting, acc *model.TokenLoanAccount, userId int, now time.Time) gin.H {
	var principal, debt, interest int64
	var interestFreeUntil int
	var totalBorrowed, totalRepaid int64
	termsAgreed := false
	creditScore := setting.CreditInitial
	blacklistedUntilDay := 0
	lenderDisclaimerAgreed := false
	effectiveMax := setting.MaxTotal
	dailyRate := setting.DailyRate
	if acc != nil {
		debt, interest = model.ProjectLoanStatus(acc, now)
		principal = acc.PrincipalQuota
		interestFreeUntil = acc.InterestFreeUntil
		totalBorrowed = acc.TotalBorrowed
		totalRepaid = acc.TotalRepaid
		termsAgreed = acc.TermsAgreedAt != 0
		creditScore = acc.CreditScore
		blacklistedUntilDay = acc.BlacklistedUntilDay
		lenderDisclaimerAgreed = acc.LenderDisclaimerAgreedAt != 0
		// 个人覆盖只降不升：上限直接覆盖，利率取较小者（与 model.effectiveRate 一致）；
		// 个人 AI 授予上限同时受信用分档位封顶（与 BorrowLoan 借款侧兜底口径一致）
		if acc.CustomMaxTotal > 0 {
			effectiveMax = acc.CustomMaxTotal
			if tierMax := operation_setting.GetCreditTierMaxTotal(setting, acc.CreditScore); tierMax < effectiveMax {
				effectiveMax = tierMax
			}
		}
		if acc.CustomDailyRate > 0 && acc.CustomDailyRate < setting.DailyRate {
			dailyRate = acc.CustomDailyRate
		}
	}
	hasOverdue, _ := model.HasOverdueFundings(model.DB, userId)
	// 秒结清惩罚预估（best-effort 只读）：手动全额结清将触发的惩罚总额，失败按 0
	fastRepayPenaltyEstimate, _ := model.ProjectFastRepayPenalty(userId, now)
	available := effectiveMax - debt
	if available < 0 {
		available = 0
	}
	return gin.H{
		"enabled":                     setting.Enabled,
		"principal":                   principal,
		"interest":                    interest,
		"debt":                        debt,
		"available":                   available,
		"effective_max":               effectiveMax,
		"daily_rate":                  dailyRate,
		"interest_free_until":         interestFreeUntil,
		"total_borrowed":              totalBorrowed,
		"total_repaid":                totalRepaid,
		"ai_enabled":                  setting.AiEnabled,
		"terms_enabled":               setting.TermsEnabled,
		"terms_agreed":                termsAgreed,
		"terms_text":                  setting.TermsText,
		"repay_fee_rate":              setting.RepayFeeRate,
		"credit_score":                creditScore,
		"market_enabled":              setting.MarketEnabled,
		"blacklisted_until_day":       blacklistedUntilDay,
		"has_overdue":                 hasOverdue,
		"lender_disclaimer_agreed":    lenderDisclaimerAgreed,
		"fast_repay_penalty_estimate": fastRepayPenaltyEstimate,
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
	common.ApiSuccess(c, buildLoanStatusData(setting, acc, userId, time.Now()))
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
	AmountUsd    string `json:"amount_usd"`
	OrderId      int    `json:"order_id"`      // 定向挂单 offer id，0 = 不挑单
	PlatformOnly bool   `json:"platform_only"` // 只接官方资金：跳过市场撮合，整笔平台放款
}

// BorrowLoan 借款，成功返回最新状态（同 GET status 字段）+ 本次放款的 funding 明细
// （含秒结清惩罚条款，供前端借款后即时提示）。
// 市场开启时：先收集 ai 模式候选挂单并调用服务层 AI 定价（best-effort，
// 定价失败不阻断借款，见 service.PriceAiSpaceFundings），连同 order_id 传入撮合引擎。
// platform_only=true 时跳过 AI 定价与撮合，整笔由平台资金池放款。
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
	orderId := req.OrderId
	if orderId < 0 {
		orderId = 0
	}
	// AI 出资方案：仅市场开启且未勾选"只接官方"时收集候选并定价；任何失败（候选查询/
	// 定价）都跳过，不阻断借款（撮合引擎对 aiPriced 为空、定向挂单无效均按缺额平台兜底）
	var aiPriced []model.FundingPlan
	if operation_setting.GetLoanSetting().MarketEnabled && !req.PlatformOnly {
		if candidates, err := model.ListActiveAiOffersForBorrow(userId, 20); err == nil && len(candidates) > 0 {
			amountUsdFloat, _ := strconv.ParseFloat(amountUsd, 64)
			aiPriced, _ = service.PriceAiSpaceFundings(userId, amountUsdFloat, candidates)
		}
	}
	acc, fundings, err := model.BorrowLoanWithOptions(userId, amountUsd, orderId, aiPriced, req.PlatformOnly)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	// 用户自助操作审计：归属用户本人，无 admin_info（同 recordUserSecurityAudit 语义）
	recordUserSecurityAudit(c, userId, "loan.borrow", map[string]interface{}{
		"amount_usd": amountUsd,
		"debt_after": logger.LogQuota(int(acc.DebtQuota)),
	})
	// 撮合审计：非 platform 部分即市场撮合命中（含 pool/order/ai），仅命中时记录，
	// 纯平台兜底借款不产生噪音
	matchedCount := 0
	var matchedTotal int64
	for i := range fundings {
		if fundings[i].SourceType == model.LoanFundingPlatform {
			continue
		}
		matchedCount++
		matchedTotal += fundings[i].Amount
	}
	if matchedCount > 0 {
		recordUserSecurityAudit(c, userId, "loan.funding_matched", map[string]interface{}{
			"count":      matchedCount,
			"amount_usd": fmt.Sprintf("%.2f", float64(matchedTotal)/common.QuotaPerUnit),
		})
	}
	// 借款入账计入充值日志：借得额度 = Σ fundings.Amount（模型层已换算为整数 quota）
	var borrowedQuota int64
	for i := range fundings {
		borrowedQuota += fundings[i].Amount
	}
	model.RecordTopupLog(userId, fmt.Sprintf("词元贷借款入账，额度: %v", logger.LogQuota(int(borrowedQuota))), c.ClientIP(), "loan", "loan", c.GetHeader("User-Agent"))
	data := buildLoanStatusData(operation_setting.GetLoanSetting(), acc, userId, time.Now())
	// 本次放款明细透出（含秒结清惩罚条款）：借款人借款后第一时间知晓哪些资金
	// 带"窗口期内手动全额提前结清收取惩罚"条款，并提示签到自动还款不触发该惩罚
	fundingList := make([]gin.H, 0, len(fundings))
	for i := range fundings {
		fundingList = append(fundingList, gin.H{
			"source_type":              fundings[i].SourceType,
			"amount":                   fundings[i].Amount,
			"rate":                     fundings[i].Rate,
			"fast_repay_penalty_quota": fundings[i].FastRepayPenaltyQuota,
			"fast_repay_window_days":   fundings[i].FastRepayWindowDays,
		})
	}
	data["fundings"] = fundingList
	common.ApiSuccess(c, data)
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
	acc, info, credits, err := model.RepayLoan(userId, amountUsd)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	recordUserSecurityAudit(c, userId, "loan.repay", map[string]interface{}{
		"amount":         logger.LogQuota(int(info.Amount)),
		"interest_part":  logger.LogQuota(int(info.InterestPart)),
		"principal_part": logger.LogQuota(int(info.PrincipalPart)),
		"fee_part":       logger.LogQuota(int(info.FeePart)),
		"penalty_part":   logger.LogQuota(int(info.PenaltyPart)),
		"debt_after":     logger.LogQuota(int(info.DebtAfter)),
	})
	// 放贷收益入账计入充值日志；此处 IP/User-Agent 为还款方（借款人）的请求上下文，
	// 即触发本次放贷人入账的请求方
	for _, credit := range credits {
		model.RecordTopupLog(credit.UserId, fmt.Sprintf("词元贷放贷收益入账，额度: %v（借款人还款）", logger.LogQuota(int(credit.Amount))), c.ClientIP(), "loan", "loan", c.GetHeader("User-Agent"))
	}
	data := buildLoanStatusData(operation_setting.GetLoanSetting(), acc, userId, time.Now())
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
