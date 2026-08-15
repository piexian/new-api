package controller

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// 需求 B：词元贷资金移动必须出现在充值日志（LogTypeTopup）。借款入账、放贷收益入账、
// 挂单退回分别由 BorrowLoan / RepayLoan（含签到）/ offer 关闭与撤回 的 controller 写入，
// 且记录请求的 User-Agent 与 IP（需求 A）。以下用例在 controller 层验证写入行为。

// TestLoanBorrowWritesTopupLog 借款成功为借款人写入一条 LogTypeTopup 日志（词元贷借款入账），
// 额度 = 借得 quota（1 USD = 500000 quota），payment_method/callback_payment_method = loan，
// 并记录请求的 User-Agent 与 IP。
func TestLoanBorrowWritesTopupLog(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 2500000
		s.MinRegisterDays = 0
		s.MaxPerBorrow = 0
	})
	user := seedLoanUser(t, db)

	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/borrow",
		map[string]any{"amount_usd": "1.00"}, user.Id, nil)
	ctx.Request.Header.Set("User-Agent", "loan-test-agent/1.0")
	BorrowLoan(ctx)
	resp := decodeLoanResponse(t, recorder)
	require.True(t, resp.Success, "borrow failed: %s", resp.Message)

	var log model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", user.Id, model.LogTypeTopup).Order("id DESC").First(&log).Error)
	require.Contains(t, log.Content, "词元贷借款入账")
	require.Contains(t, log.Content, logger.LogQuota(500000))
	require.Equal(t, "loan-test-agent/1.0", log.UserAgent, "充值日志必须记录请求 User-Agent")
	require.Equal(t, ctx.ClientIP(), log.Ip, "充值日志必须记录请求 IP")
	var other map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(log.Other), &other))
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok, "充值日志应携带 admin_info")
	require.Equal(t, "loan", adminInfo["payment_method"])
	require.Equal(t, "loan", adminInfo["callback_payment_method"])
}

// TestLoanRepayWritesLenderTopupLog 手工还款命中 P2P funding 时，为放贷人写入一条
// LogTypeTopup 日志（词元贷放贷收益入账），额度 = 利息入账额；日志的 IP/User-Agent 是
// 还款方（借款人）请求上下文（触发入账的请求方）。
func TestLoanRepayWritesLenderTopupLog(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MarketEnabled = true
		s.RepayFeeRate = 0
		s.LenderRateMin = 0.0005
		s.LenderRateMax = 0.002
	})
	lender := seedLoanUser(t, db)
	borrower := seedLoanUser(t, db)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", lender.Id).Update("quota", 2000000).Error)
	require.NoError(t, model.AgreeLenderDisclaimer(lender.Id))
	offer, err := model.CreateLoanOffer(lender.Id, model.LoanOfferModePool, "2.00", "0.001", 0, 0, 0, -50, "", 0)
	require.NoError(t, err)

	// P2P funding：本金 300000 + 利息 10000，LastSettledDay=今天（当天不计息，债务确定）
	now := time.Now()
	day := model.LoanDayOf(now)
	require.NoError(t, db.Create(&model.TokenLoanFunding{
		LoanUserId:         borrower.Id,
		SourceType:         model.LoanFundingPool,
		OfferId:            offer.Id,
		LenderId:           lender.Id,
		Amount:             300000,
		PrincipalRemaining: 300000,
		DebtQuota:          310000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          model.LoanRepayFull,
		Status:             model.LoanFundingActive,
		DueDay:             day + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}).Error)
	require.NoError(t, db.Create(&model.TokenLoanAccount{
		UserId:         borrower.Id,
		PrincipalQuota: 300000,
		DebtQuota:      310000,
		LastSettledDay: day,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}).Error)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", borrower.Id).Update("quota", 500000).Error)

	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/repay",
		map[string]any{"amount_usd": "0.08"}, borrower.Id, nil)
	ctx.Request.Header.Set("User-Agent", "borrower-agent/1.0")
	RepayLoan(ctx)
	resp := decodeLoanResponse(t, recorder)
	require.True(t, resp.Success, "repay failed: %s", resp.Message)

	// 放贷人收到一条收益入账充值日志，额度 = 利息 10000（offer 未关闭，本金回补不进余额）
	var log model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", lender.Id, model.LogTypeTopup).Order("id DESC").First(&log).Error)
	require.Contains(t, log.Content, "词元贷放贷收益入账")
	require.Contains(t, log.Content, "（借款人还款）")
	require.Contains(t, log.Content, logger.LogQuota(10000))
	// 日志记录的是触发入账的还款请求（借款人上下文）
	require.Equal(t, "borrower-agent/1.0", log.UserAgent)
	require.Equal(t, ctx.ClientIP(), log.Ip)
}
