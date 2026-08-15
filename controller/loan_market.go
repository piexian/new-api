package controller

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// parseLoanOfferId 解析路径中的 offer id，失败时输出 i18n 错误并返回 false
func parseLoanOfferId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return 0, false
	}
	return id, true
}

// parseLoanFundingId 解析路径中的 funding id（int64），失败时输出 i18n 错误并返回 false
func parseLoanFundingId(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return 0, false
	}
	return id, true
}

// AgreeLenderDisclaimer 同意放贷免责声明（幂等，镜像 AgreeLoanTerms）
func AgreeLenderDisclaimer(c *gin.Context) {
	userId := c.GetInt("id")
	if err := model.AgreeLenderDisclaimer(userId); err != nil {
		respondLoanError(c, err)
		return
	}
	recordUserSecurityAudit(c, userId, "loan.disclaimer_agreed", nil)
	common.ApiSuccess(c, gin.H{"disclaimer_agreed": true})
}

// GetLoanMarketOffers 返回当前用户自己的放贷挂单（id 倒序，最新在前）
func GetLoanMarketOffers(c *gin.Context) {
	userId := c.GetInt("id")
	offers, err := model.GetUserLoanOffers(userId)
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"offers": offers})
}

type createLoanOfferRequest struct {
	Mode                string  `json:"mode"`
	AmountUsd           string  `json:"amount_usd"`
	RateFixed           string  `json:"rate_fixed"`
	RateMin             float64 `json:"rate_min"`
	RateMax             float64 `json:"rate_max"`
	PerLoanCap          int64   `json:"per_loan_cap"`
	MinCreditScore      int     `json:"min_credit_score"`
	FastRepayPenaltyUsd string  `json:"fast_repay_penalty_usd"` // 秒结清固定惩罚额度（USD，空 = 0 不收），decimal 解析与 amount 同模式
	FastRepayWindowDays int     `json:"fast_repay_window_days"` // 秒结清窗口天数，0 = 仅当天；∈ [0, 365]
}

// CreateLoanMarketOffer 挂出放贷供给单（rate_fixed 用字符串传递保留精度，
// 由 model 层按模式校验区间/上限，见 model.CreateLoanOffer；秒结清惩罚条款
// fast_repay_penalty_usd / fast_repay_window_days 亦由 model 层校验）
func CreateLoanMarketOffer(c *gin.Context) {
	var req createLoanOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	offer, err := model.CreateLoanOffer(userId, strings.TrimSpace(req.Mode),
		strings.TrimSpace(req.AmountUsd), strings.TrimSpace(req.RateFixed),
		req.RateMin, req.RateMax, req.PerLoanCap, req.MinCreditScore,
		strings.TrimSpace(req.FastRepayPenaltyUsd), req.FastRepayWindowDays)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	recordUserSecurityAudit(c, userId, "loan.offer_create", map[string]interface{}{
		"mode":                   offer.Mode,
		"amount_usd":             fmt.Sprintf("%.2f", float64(offer.AmountTotal)/common.QuotaPerUnit),
		"rate_fixed":             fmt.Sprintf("%v", offer.RateFixed),
		"fast_repay_penalty_usd": fmt.Sprintf("%.2f", float64(offer.FastRepayPenaltyQuota)/common.QuotaPerUnit),
		"fast_repay_window_days": offer.FastRepayWindowDays,
	})
	common.ApiSuccess(c, offer)
}

// setLoanMarketOfferStatus 暂停/恢复共用实现：active ⇄ paused（终态流转走 close）
func setLoanMarketOfferStatus(c *gin.Context, status string) {
	offerId, ok := parseLoanOfferId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if err := model.SetLoanOfferStatus(userId, offerId, status); err != nil {
		respondLoanError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"status": status})
}

// PauseLoanMarketOffer 暂停挂单，不再参与新撮合
func PauseLoanMarketOffer(c *gin.Context) {
	setLoanMarketOfferStatus(c, model.LoanOfferStatusPaused)
}

// ResumeLoanMarketOffer 恢复挂单上架
func ResumeLoanMarketOffer(c *gin.Context) {
	setLoanMarketOfferStatus(c, model.LoanOfferStatusActive)
}

// CloseLoanMarketOffer 关闭挂单（终态），闲置额度退回余额
func CloseLoanMarketOffer(c *gin.Context) {
	offerId, ok := parseLoanOfferId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	refunded, err := model.CloseLoanOffer(userId, offerId)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	recordUserSecurityAudit(c, userId, "loan.offer_close", map[string]interface{}{
		"offer_id": offerId,
	})
	// 关闭退回的闲置额度计入充值日志（无闲置额度时不产生资金移动，不记日志）
	if refunded > 0 {
		model.RecordTopupLog(userId, fmt.Sprintf("词元贷放贷资金退回，额度: %v", logger.LogQuota(int(refunded))), c.ClientIP(), "loan", "loan", c.GetHeader("User-Agent"))
	}
	common.ApiSuccess(c, nil)
}

// WithdrawLoanMarketOffer 撤回挂单全部闲置额度到余额（offer 保留原状态）
func WithdrawLoanMarketOffer(c *gin.Context) {
	offerId, ok := parseLoanOfferId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	refunded, err := model.WithdrawLoanOffer(userId, offerId)
	if err != nil {
		respondLoanError(c, err)
		return
	}
	recordUserSecurityAudit(c, userId, "loan.offer_withdraw", map[string]interface{}{
		"offer_id": offerId,
		"refunded": logger.LogQuota(int(refunded)),
	})
	// 撤回的闲置额度计入充值日志（成功撤回时 refunded 恒 > 0）
	model.RecordTopupLog(userId, fmt.Sprintf("词元贷放贷资金退回，额度: %v", logger.LogQuota(int(refunded))), c.ClientIP(), "loan", "loan", c.GetHeader("User-Agent"))
	common.ApiSuccess(c, gin.H{"refunded": refunded})
}

// GetLoanMarketList 市场浏览：全部 active order 挂单，匿名化输出——
// 不暴露放贷人用户名与原始 user id，每单只透出 offer id、可撮合额度、
// 固定日利率、信用分门槛与放贷人当前信用分。
func GetLoanMarketList(c *gin.Context) {
	offers, err := model.ListActiveOrderOffers()
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	items := make([]gin.H, 0, len(offers))
	for i := range offers {
		o := &offers[i]
		score, err := model.GetCreditScore(o.LenderId)
		if err != nil {
			respondLoanInternalError(c, err)
			return
		}
		items = append(items, gin.H{
			"id":                       o.Id,
			"amount_available":         o.AmountAvailable,
			"rate_fixed":               o.RateFixed,
			"min_credit_score":         o.MinCreditScore,
			"lender_credit_score":      score,
			"fast_repay_penalty_quota": o.FastRepayPenaltyQuota,
			"fast_repay_window_days":   o.FastRepayWindowDays,
		})
	}
	common.ApiSuccess(c, gin.H{"offers": items})
}

// GetLoanMarketFundings 放贷人收益台账：本人名下 funding 分页（id 倒序），
// 每笔附投影债务（active/overdue 按 funding 自身利率惰性投影，终态取账面值）、
// 已回本金与借款人信用分。
func GetLoanMarketFundings(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	fundings, total, err := model.GetLenderFundings(userId, pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	now := time.Now()
	items := make([]gin.H, 0, len(fundings))
	for i := range fundings {
		f := &fundings[i]
		debt := f.DebtQuota
		if f.Status == model.LoanFundingActive || f.Status == model.LoanFundingOverdue {
			// P2P funding 用自身利率结算，借款人账户仅 platform 分支消费，传 nil 安全
			debt = model.ProjectFundingDebt(f, nil, now)
		}
		score, err := model.GetCreditScore(f.LoanUserId)
		if err != nil {
			respondLoanInternalError(c, err)
			return
		}
		items = append(items, gin.H{
			"id":                       f.Id,
			"loan_user_id":             f.LoanUserId,
			"source_type":              f.SourceType,
			"offer_id":                 f.OfferId,
			"amount":                   f.Amount,
			"principal_remaining":      f.PrincipalRemaining,
			"repaid_principal":         f.Amount - f.PrincipalRemaining,
			"debt":                     debt,
			"rate":                     f.Rate,
			"repay_plan":               f.RepayPlan,
			"status":                   f.Status,
			"due_day":                  f.DueDay,
			"created_at":               f.CreatedAt,
			"borrower_credit_score":    score,
			"fast_repay_penalty_quota": f.FastRepayPenaltyQuota,
			"fast_repay_window_days":   f.FastRepayWindowDays,
		})
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

type resolveLoanFundingRequest struct {
	Action     string `json:"action"`
	ExtendDays int    `json:"extend_days"`
}

// ResolveLoanMarketFunding 放贷人处置本人逾期债权（extend / writeoff / perpetual）
func ResolveLoanMarketFunding(c *gin.Context) {
	fundingId, ok := parseLoanFundingId(c)
	if !ok {
		return
	}
	var req resolveLoanFundingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	action := strings.TrimSpace(req.Action)
	userId := c.GetInt("id")
	if err := model.ResolveOverdueFunding(userId, fundingId, action, req.ExtendDays); err != nil {
		respondLoanError(c, err)
		return
	}
	extendDays := ""
	if action == model.LoanDefaultActionExtend {
		extendDays = fmt.Sprintf("，延长 %d 天", req.ExtendDays)
	}
	recordUserSecurityAudit(c, userId, "loan.default_decision", map[string]interface{}{
		"funding_id":  fundingId,
		"action":      action,
		"extend_days": extendDays,
	})
	common.ApiSuccess(c, nil)
}

type setFundingRepayPlanRequest struct {
	Plan string `json:"plan"`
}

// SetLoanMarketFundingRepayPlan 放贷人调整本人 P2P funding 的还款计划
func SetLoanMarketFundingRepayPlan(c *gin.Context) {
	fundingId, ok := parseLoanFundingId(c)
	if !ok {
		return
	}
	var req setFundingRepayPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	plan := strings.TrimSpace(req.Plan)
	userId := c.GetInt("id")
	if err := model.SetFundingRepayPlan(userId, fundingId, plan); err != nil {
		respondLoanError(c, err)
		return
	}
	recordUserSecurityAudit(c, userId, "loan.repay_plan_change", map[string]interface{}{
		"funding_id": fundingId,
		"plan":       plan,
	})
	common.ApiSuccess(c, nil)
}
