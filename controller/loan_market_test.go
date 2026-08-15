package controller

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// setLoanUserQuota 直接写库设置用户余额（quota）
func setLoanUserQuota(t *testing.T, db *gorm.DB, userId int, quota int64) {
	t.Helper()
	if err := db.Model(&model.User{}).Where("id = ?", userId).Update("quota", quota).Error; err != nil {
		t.Fatalf("failed to set user quota: %v", err)
	}
}

// marketOfferBody 构造创建挂单请求体
func marketOfferBody(mode, amountUsd, rateFixed string, rateMin, rateMax float64, perLoanCap int64, minCreditScore int) map[string]any {
	return map[string]any{
		"mode":             mode,
		"amount_usd":       amountUsd,
		"rate_fixed":       rateFixed,
		"rate_min":         rateMin,
		"rate_max":         rateMax,
		"per_loan_cap":     perLoanCap,
		"min_credit_score": minCreditScore,
	}
}

// TestLoanMarketDisclaimerGate 免责声明门槛：未同意禁止挂单，同意后幂等放行
func TestLoanMarketDisclaimerGate(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.MarketEnabled = true
	})
	user := seedLoanUser(t, db)
	setLoanUserQuota(t, db, user.Id, 2000000)

	// 未同意免责声明挂单 → 拒绝
	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers",
		marketOfferBody("order", "1.00", "0.001", 0, 0, 0, -50), user.Id, nil)
	CreateLoanMarketOffer(ctx)
	resp := decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "请先阅读并同意放贷免责声明" {
		t.Fatalf("expected disclaimer required rejection, got: %+v", resp)
	}

	// 同意免责声明（幂等）
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/disclaimer", nil, user.Id, nil)
	AgreeLenderDisclaimer(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("agree disclaimer failed: %s", resp.Message)
	}
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/disclaimer", nil, user.Id, nil)
	AgreeLenderDisclaimer(ctx)
	if resp = decodeLoanResponse(t, recorder); !resp.Success {
		t.Fatalf("second agree disclaimer should be idempotent, got: %s", resp.Message)
	}

	// 同意后挂单成功
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers",
		marketOfferBody("order", "1.00", "0.001", 0, 0, 0, -50), user.Id, nil)
	CreateLoanMarketOffer(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("create offer failed: %s", resp.Message)
	}
	if got := resp.Data["amount_available"]; got != float64(500000) {
		t.Fatalf("expected amount_available 500000, got %v", got)
	}
	if got := resp.Data["status"]; got != "active" {
		t.Fatalf("expected status active, got %v", got)
	}
}

// TestLoanMarketDisabled 市场未启用时挂单被拒绝
func TestLoanMarketDisabled(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.MarketEnabled = false
	})
	user := seedLoanUser(t, db)
	setLoanUserQuota(t, db, user.Id, 2000000)

	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers",
		marketOfferBody("order", "1.00", "0.001", 0, 0, 0, -50), user.Id, nil)
	CreateLoanMarketOffer(ctx)
	resp := decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "借贷市场未启用" {
		t.Fatalf("expected market disabled rejection, got: %+v", resp)
	}
}

// TestLoanMarketOfferCRUD 挂单生命周期：参数校验 → 建单 → 列表 → 暂停/恢复 → 撤回 → 关闭，
// 以及非本人操作按 not found 处理
func TestLoanMarketOfferCRUD(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.MarketEnabled = true
	})
	user := seedLoanUser(t, db)
	setLoanUserQuota(t, db, user.Id, 2000000)
	if err := model.AgreeLenderDisclaimer(user.Id); err != nil {
		t.Fatalf("failed to agree disclaimer: %v", err)
	}

	// 非法模式 → 参数无效
	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers",
		marketOfferBody("hack", "1.00", "0.001", 0, 0, 0, -50), user.Id, nil)
	CreateLoanMarketOffer(ctx)
	resp := decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "放贷挂单参数无效" {
		t.Fatalf("expected invalid params rejection, got: %+v", resp)
	}

	// 建单
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers",
		marketOfferBody("order", "1.00", "0.001", 0, 0, 0, -50), user.Id, nil)
	CreateLoanMarketOffer(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("create offer failed: %s", resp.Message)
	}
	offerId := int(resp.Data["id"].(float64))

	// 自己的挂单列表
	ctx, recorder = newLoanContext(t, http.MethodGet, "/api/user/loan/market/offers", nil, user.Id, nil)
	GetLoanMarketOffers(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("offers list failed: %s", resp.Message)
	}
	offers, ok := resp.Data["offers"].([]any)
	if !ok || len(offers) != 1 {
		t.Fatalf("expected 1 own offer, got %v", resp.Data["offers"])
	}

	// 暂停 → 恢复
	pause := func(userId int) loanAPIResponse {
		ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers/:id/pause", nil, userId,
			gin.Params{{Key: "id", Value: strconv.Itoa(offerId)}})
		PauseLoanMarketOffer(ctx)
		return decodeLoanResponse(t, recorder)
	}
	if resp := pause(user.Id); !resp.Success {
		t.Fatalf("pause failed: %s", resp.Message)
	}
	// 非本人暂停 → not found
	if resp := pause(otherLoanUser(t, db).Id); resp.Success || resp.Message != "放贷挂单不存在" {
		t.Fatalf("expected not found for foreign pause, got: %+v", resp)
	}
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers/:id/resume", nil, user.Id,
		gin.Params{{Key: "id", Value: strconv.Itoa(offerId)}})
	ResumeLoanMarketOffer(ctx)
	if resp = decodeLoanResponse(t, recorder); !resp.Success {
		t.Fatalf("resume failed: %s", resp.Message)
	}

	// 撤回闲置额度
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers/:id/withdraw", nil, user.Id,
		gin.Params{{Key: "id", Value: strconv.Itoa(offerId)}})
	WithdrawLoanMarketOffer(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("withdraw failed: %s", resp.Message)
	}
	if got := resp.Data["refunded"]; got != float64(500000) {
		t.Fatalf("expected refunded 500000, got %v", got)
	}

	// 关闭
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers/:id/close", nil, user.Id,
		gin.Params{{Key: "id", Value: strconv.Itoa(offerId)}})
	CloseLoanMarketOffer(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("close failed: %s", resp.Message)
	}

	// 已关闭再撤回 → 不可操作
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers/:id/withdraw", nil, user.Id,
		gin.Params{{Key: "id", Value: strconv.Itoa(offerId)}})
	WithdrawLoanMarketOffer(ctx)
	resp = decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "该放贷挂单当前不可操作" {
		t.Fatalf("expected not active rejection, got: %+v", resp)
	}
}

// otherLoanUser 额外种子一个用户（非本人场景）
func otherLoanUser(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	return seedLoanUser(t, db)
}

// TestLoanMarketListAnonymized 市场浏览：只透出 active order 挂单，且不含放贷人身份
func TestLoanMarketListAnonymized(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.MarketEnabled = true
	})
	lender := seedLoanUser(t, db)
	setLoanUserQuota(t, db, lender.Id, 3000000)
	if err := model.AgreeLenderDisclaimer(lender.Id); err != nil {
		t.Fatalf("failed to agree disclaimer: %v", err)
	}
	// 给放贷人一个明确的信用分（账户已由 AgreeLenderDisclaimer 创建，分值 0，直接写库）
	if err := db.Model(&model.TokenLoanAccount{}).Where("user_id = ?", lender.Id).
		Update("credit_score", 66).Error; err != nil {
		t.Fatalf("failed to set lender credit score: %v", err)
	}

	create := func(body map[string]any) int {
		ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers", body, lender.Id, nil)
		CreateLoanMarketOffer(ctx)
		resp := decodeLoanResponse(t, recorder)
		if !resp.Success {
			t.Fatalf("create offer failed: %s", resp.Message)
		}
		return int(resp.Data["id"].(float64))
	}
	create(marketOfferBody("order", "1.00", "0.001", 0, 0, 0, -50))
	create(marketOfferBody("pool", "1.00", "0.002", 0, 0, 0, -50))
	create(marketOfferBody("ai", "1.00", "", 0.001, 0.002, 500000, -50))

	// 其他用户浏览市场
	viewer := otherLoanUser(t, db)
	ctx, recorder := newLoanContext(t, http.MethodGet, "/api/user/loan/market/list", nil, viewer.Id, nil)
	GetLoanMarketList(ctx)
	resp := decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("market list failed: %s", resp.Message)
	}
	offers, ok := resp.Data["offers"].([]any)
	if !ok {
		t.Fatalf("missing offers in response: %v", resp.Data)
	}
	if len(offers) != 1 {
		t.Fatalf("expected only the order offer in market list, got %d items", len(offers))
	}
	item := offers[0].(map[string]any)
	// 匿名化：不得暴露放贷人原始 user id 与用户名
	for _, forbidden := range []string{"lender_id", "username", "lender_username"} {
		if _, exists := item[forbidden]; exists {
			t.Fatalf("field %s must not be exposed in market list: %v", forbidden, item)
		}
	}
	if got := item["amount_available"]; got != float64(500000) {
		t.Fatalf("expected amount_available 500000, got %v", got)
	}
	if got := item["rate_fixed"]; got != 0.001 {
		t.Fatalf("expected rate_fixed 0.001, got %v", got)
	}
	if got := item["min_credit_score"]; got != float64(-50) {
		t.Fatalf("expected min_credit_score -50, got %v", got)
	}
	if got := item["lender_credit_score"]; got != float64(66) {
		t.Fatalf("expected lender_credit_score 66, got %v", got)
	}
}

// TestLoanMarketFundingsLedger 放贷人收益台账：只含本人投放、附投影债务/已回本金/借款人信用分；
// 同时验证借款 order_id 定向挂单透传
func TestLoanMarketFundingsLedger(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.MarketEnabled = true
		s.TermsEnabled = true
		s.MaxTotal = 2500000
		s.MinRegisterDays = 0
		s.MaxPerBorrow = 0
	})
	lender := seedLoanUser(t, db)
	setLoanUserQuota(t, db, lender.Id, 2000000)
	if err := model.AgreeLenderDisclaimer(lender.Id); err != nil {
		t.Fatalf("failed to agree disclaimer: %v", err)
	}
	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers",
		marketOfferBody("order", "1.00", "0.001", 0, 0, 0, -50), lender.Id, nil)
	CreateLoanMarketOffer(ctx)
	resp := decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("create offer failed: %s", resp.Message)
	}
	offerId := int(resp.Data["id"].(float64))

	borrower := otherLoanUser(t, db)
	if err := model.AgreeLoanTerms(borrower.Id); err != nil {
		t.Fatalf("failed to agree terms: %v", err)
	}
	// 定向挂单借款：order_id 透传到撮合引擎
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/borrow",
		map[string]any{"amount_usd": "1.00", "order_id": offerId}, borrower.Id, nil)
	BorrowLoan(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("borrow with order_id failed: %s", resp.Message)
	}

	var funding model.TokenLoanFunding
	if err := db.Where("loan_user_id = ? AND source_type = ?", borrower.Id, model.LoanFundingOrder).
		First(&funding).Error; err != nil {
		t.Fatalf("expected order funding to be created: %v", err)
	}
	if funding.OfferId != offerId {
		t.Fatalf("expected funding offer_id %d, got %d", offerId, funding.OfferId)
	}
	if funding.LenderId != lender.Id {
		t.Fatalf("expected funding lender_id %d, got %d", lender.Id, funding.LenderId)
	}
	if funding.Amount != 500000 {
		t.Fatalf("expected funding amount 500000, got %d", funding.Amount)
	}

	// 放贷人台账
	ctx, recorder = newLoanContext(t, http.MethodGet, "/api/user/loan/market/fundings", nil, lender.Id, nil)
	GetLoanMarketFundings(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("fundings ledger failed: %s", resp.Message)
	}
	if got := resp.Data["total"]; got != float64(1) {
		t.Fatalf("expected total 1, got %v", got)
	}
	items, ok := resp.Data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 funding item, got %v", resp.Data["items"])
	}
	item := items[0].(map[string]any)
	if got := item["loan_user_id"]; got != float64(borrower.Id) {
		t.Fatalf("expected loan_user_id %d, got %v", borrower.Id, got)
	}
	if got := item["amount"]; got != float64(500000) {
		t.Fatalf("expected amount 500000, got %v", got)
	}
	if got := item["repaid_principal"]; got != float64(0) {
		t.Fatalf("expected repaid_principal 0, got %v", got)
	}
	if got := item["debt"]; got != float64(500000) {
		t.Fatalf("expected debt 500000, got %v", got)
	}
	if _, exists := item["borrower_credit_score"]; !exists {
		t.Fatalf("missing borrower_credit_score: %v", item)
	}

	// 借款人视角：自己不是放贷人 → 台账为空
	ctx, recorder = newLoanContext(t, http.MethodGet, "/api/user/loan/market/fundings", nil, borrower.Id, nil)
	GetLoanMarketFundings(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("fundings ledger failed: %s", resp.Message)
	}
	if got := resp.Data["total"]; got != float64(0) {
		t.Fatalf("expected 0 fundings for borrower, got %v", got)
	}
}

// seedOverdueFundingForLender 直接构造一笔放贷人名下 overdue funding（供处置/改档测试）
func seedOverdueFundingForLender(t *testing.T, db *gorm.DB, lenderId, borrowerId int) *model.TokenLoanFunding {
	t.Helper()
	now := time.Now()
	day := model.LoanDayOf(now)
	funding := &model.TokenLoanFunding{
		LoanUserId:         borrowerId,
		SourceType:         model.LoanFundingOrder,
		OfferId:            1,
		LenderId:           lenderId,
		Amount:             500000,
		PrincipalRemaining: 500000,
		DebtQuota:          500000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          model.LoanRepayFull,
		Status:             model.LoanFundingOverdue,
		DueDay:             day - 5,
		PenaltyStartedDay:  day - 3,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	if err := db.Create(funding).Error; err != nil {
		t.Fatalf("failed to seed overdue funding: %v", err)
	}
	return funding
}

// TestLoanMarketResolveFunding 逾期债权处置接线：extend 成功、非逾期拒绝、非法动作拒绝、非本人拒绝
func TestLoanMarketResolveFunding(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.MarketEnabled = true
	})
	lender := seedLoanUser(t, db)
	borrower := otherLoanUser(t, db)

	// extend 成功
	funding := seedOverdueFundingForLender(t, db, lender.Id, borrower.Id)
	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/market/fundings/:id/resolve",
		map[string]any{"action": "extend", "extend_days": 5}, lender.Id,
		gin.Params{{Key: "id", Value: strconv.FormatInt(funding.Id, 10)}})
	ResolveLoanMarketFunding(ctx)
	resp := decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("resolve extend failed: %s", resp.Message)
	}
	var reloaded model.TokenLoanFunding
	if err := db.First(&reloaded, funding.Id).Error; err != nil {
		t.Fatalf("failed to reload funding: %v", err)
	}
	if reloaded.Status != model.LoanFundingActive {
		t.Fatalf("expected funding active after extend, got %s", reloaded.Status)
	}

	// 已恢复 → 再处置拒绝
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/fundings/:id/resolve",
		map[string]any{"action": "perpetual"}, lender.Id,
		gin.Params{{Key: "id", Value: strconv.FormatInt(funding.Id, 10)}})
	ResolveLoanMarketFunding(ctx)
	resp = decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "该借款未逾期，无法处置" {
		t.Fatalf("expected not overdue rejection, got: %+v", resp)
	}

	// 非法动作拒绝
	funding2 := seedOverdueFundingForLender(t, db, lender.Id, borrower.Id)
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/fundings/:id/resolve",
		map[string]any{"action": "hack"}, lender.Id,
		gin.Params{{Key: "id", Value: strconv.FormatInt(funding2.Id, 10)}})
	ResolveLoanMarketFunding(ctx)
	resp = decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "无效的逾期处置动作" {
		t.Fatalf("expected invalid action rejection, got: %+v", resp)
	}

	// 非本人（放贷人以外用户）处置 → 不属于你
	other := otherLoanUser(t, db)
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/fundings/:id/resolve",
		map[string]any{"action": "writeoff"}, other.Id,
		gin.Params{{Key: "id", Value: strconv.FormatInt(funding2.Id, 10)}})
	ResolveLoanMarketFunding(ctx)
	resp = decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "该借款不属于你，无法操作" {
		t.Fatalf("expected not owner rejection, got: %+v", resp)
	}

	// 不存在的 funding
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/market/fundings/:id/resolve",
		map[string]any{"action": "extend", "extend_days": 1}, lender.Id,
		gin.Params{{Key: "id", Value: "999999"}})
	ResolveLoanMarketFunding(ctx)
	if resp = decodeLoanResponse(t, recorder); resp.Success {
		t.Fatalf("expected resolve of missing funding to fail")
	}
}

// TestLoanMarketRepayPlan 还款计划调整接线：合法改档成功、非法 plan 拒绝、非本人拒绝
func TestLoanMarketRepayPlan(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.MarketEnabled = true
	})
	lender := seedLoanUser(t, db)
	borrower := otherLoanUser(t, db)
	funding := seedOverdueFundingForLender(t, db, lender.Id, borrower.Id)

	setPlan := func(userId int, plan string) loanAPIResponse {
		ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/market/fundings/:id/repay_plan",
			map[string]any{"plan": plan}, userId,
			gin.Params{{Key: "id", Value: strconv.FormatInt(funding.Id, 10)}})
		SetLoanMarketFundingRepayPlan(ctx)
		return decodeLoanResponse(t, recorder)
	}

	// 合法改档
	if resp := setPlan(lender.Id, "no_penalty"); !resp.Success {
		t.Fatalf("set repay plan failed: %s", resp.Message)
	}
	var reloaded model.TokenLoanFunding
	if err := db.First(&reloaded, funding.Id).Error; err != nil {
		t.Fatalf("failed to reload funding: %v", err)
	}
	if reloaded.RepayPlan != model.LoanRepayNoPenalty {
		t.Fatalf("expected repay_plan no_penalty, got %s", reloaded.RepayPlan)
	}

	// 非法 plan
	if resp := setPlan(lender.Id, "bogus"); resp.Success || resp.Message != "还款计划无效或当前不可调整" {
		t.Fatalf("expected invalid plan rejection, got: %+v", resp)
	}

	// 非本人
	other := otherLoanUser(t, db)
	if resp := setPlan(other.Id, "interest_freeze"); resp.Success || resp.Message != "该借款不属于你，无法操作" {
		t.Fatalf("expected not owner rejection, got: %+v", resp)
	}
}

// TestLoanMarketStatusDataFields status 投影新增字段：credit_score / market_enabled /
// blacklisted_until_day / has_overdue / lender_disclaimer_agreed
func TestLoanMarketStatusDataFields(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.MarketEnabled = true
	})
	user := seedLoanUser(t, db)

	// 无账户：信用分取初始值，其余零值
	ctx, recorder := newLoanContext(t, http.MethodGet, "/api/user/loan/status", nil, user.Id, nil)
	GetLoanStatus(ctx)
	resp := decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("status failed: %s", resp.Message)
	}
	if got := resp.Data["credit_score"]; got != float64(50) {
		t.Fatalf("expected credit_score 50 (initial), got %v", got)
	}
	if got := resp.Data["market_enabled"]; got != true {
		t.Fatalf("expected market_enabled true, got %v", got)
	}
	if got := resp.Data["blacklisted_until_day"]; got != float64(0) {
		t.Fatalf("expected blacklisted_until_day 0, got %v", got)
	}
	if got := resp.Data["has_overdue"]; got != false {
		t.Fatalf("expected has_overdue false, got %v", got)
	}
	if got := resp.Data["lender_disclaimer_agreed"]; got != false {
		t.Fatalf("expected lender_disclaimer_agreed false, got %v", got)
	}

	// 同意免责声明 → 标记翻转
	if err := model.AgreeLenderDisclaimer(user.Id); err != nil {
		t.Fatalf("failed to agree disclaimer: %v", err)
	}
	ctx, recorder = newLoanContext(t, http.MethodGet, "/api/user/loan/status", nil, user.Id, nil)
	GetLoanStatus(ctx)
	resp = decodeLoanResponse(t, recorder)
	if got := resp.Data["lender_disclaimer_agreed"]; got != true {
		t.Fatalf("expected lender_disclaimer_agreed true, got %v", got)
	}

	// 写库设置信用分/黑名单，并造一笔 overdue funding
	if err := db.Model(&model.TokenLoanAccount{}).Where("user_id = ?", user.Id).
		Update("credit_score", 77).Error; err != nil {
		t.Fatalf("failed to set credit score: %v", err)
	}
	if err := db.Model(&model.TokenLoanAccount{}).Where("user_id = ?", user.Id).
		Update("blacklisted_until_day", 1000).Error; err != nil {
		t.Fatalf("failed to set blacklist: %v", err)
	}
	now := time.Now()
	day := model.LoanDayOf(now)
	if err := db.Create(&model.TokenLoanFunding{
		LoanUserId:         user.Id,
		SourceType:         model.LoanFundingPlatform,
		Amount:             100000,
		PrincipalRemaining: 100000,
		DebtQuota:          100000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          model.LoanRepayFull,
		Status:             model.LoanFundingOverdue,
		DueDay:             day - 1,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}).Error; err != nil {
		t.Fatalf("failed to seed overdue funding: %v", err)
	}

	ctx, recorder = newLoanContext(t, http.MethodGet, "/api/user/loan/status", nil, user.Id, nil)
	GetLoanStatus(ctx)
	resp = decodeLoanResponse(t, recorder)
	if got := resp.Data["credit_score"]; got != float64(77) {
		t.Fatalf("expected credit_score 77, got %v", got)
	}
	if got := resp.Data["blacklisted_until_day"]; got != float64(1000) {
		t.Fatalf("expected blacklisted_until_day 1000, got %v", got)
	}
	if got := resp.Data["has_overdue"]; got != true {
		t.Fatalf("expected has_overdue true, got %v", got)
	}
}

// TestLoanMarketBorrowWithAiPricing 市场开启时借款走 AI 定价：注入假模型输出，
// 验证 ai 候选收集、定价结果透传与 funding 落库（ai 来源、放贷人、利率）
func TestLoanMarketBorrowWithAiPricing(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.MarketEnabled = true
		s.AiEnabled = true
		s.TermsEnabled = true
		s.MaxTotal = 2500000
		s.MinRegisterDays = 0
		s.MaxPerBorrow = 0
		s.AiModels = []operation_setting.AiModelConfig{{Model: "loan-test-model", ContextWindow: 8192}}
		s.AiMaxOutput = 256
	})
	lender := seedLoanUser(t, db)
	setLoanUserQuota(t, db, lender.Id, 2000000)
	if err := model.AgreeLenderDisclaimer(lender.Id); err != nil {
		t.Fatalf("failed to agree disclaimer: %v", err)
	}
	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/market/offers",
		marketOfferBody("ai", "1.00", "", 0.001, 0.002, 500000, -50), lender.Id, nil)
	CreateLoanMarketOffer(ctx)
	resp := decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("create ai offer failed: %s", resp.Message)
	}
	offerId := int(resp.Data["id"].(float64))

	// 注入假 AI 定价输出（恰好一个 fenced json 块）
	service.RegisterLoanOfficerModelCaller(func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "```json\n{\"fundings\":[{\"offer_index\":0,\"amount_usd\":1.00,\"daily_rate\":0.0015}]}\n```", nil
	})
	t.Cleanup(func() {
		service.RegisterLoanOfficerModelCaller(callLoanOfficerUpstream)
	})

	borrower := otherLoanUser(t, db)
	if err := model.AgreeLoanTerms(borrower.Id); err != nil {
		t.Fatalf("failed to agree terms: %v", err)
	}
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/borrow",
		map[string]any{"amount_usd": "1.00"}, borrower.Id, nil)
	BorrowLoan(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("borrow with ai pricing failed: %s", resp.Message)
	}

	var funding model.TokenLoanFunding
	if err := db.Where("loan_user_id = ? AND source_type = ?", borrower.Id, model.LoanFundingAi).
		First(&funding).Error; err != nil {
		t.Fatalf("expected ai funding to be created: %v", err)
	}
	if funding.OfferId != offerId {
		t.Fatalf("expected funding offer_id %d, got %d", offerId, funding.OfferId)
	}
	if funding.LenderId != lender.Id {
		t.Fatalf("expected funding lender_id %d, got %d", lender.Id, funding.LenderId)
	}
	if funding.Amount != 500000 {
		t.Fatalf("expected funding amount 500000, got %d", funding.Amount)
	}
	if funding.Rate != 0.0015 {
		t.Fatalf("expected funding rate 0.0015, got %v", funding.Rate)
	}
}
