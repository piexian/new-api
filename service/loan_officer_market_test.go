package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLoanOfficerMarketTest 迁移市场表并创建测试借款人/放贷人（service 测试不删行，
// sqlite 自增 id 在测试进程内单调，互不干扰）
func setupLoanOfficerMarketTest(t *testing.T) (*model.User, *model.User) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(
		&model.TokenLoanAccount{}, &model.TokenLoanRecord{},
		&model.TokenLoanApplication{}, &model.TokenLoanApplicationMessage{},
		&model.Checkin{}, &model.TokenLoanOffer{}, &model.TokenLoanFunding{},
	))
	username := fmt.Sprintf("officer-market-%d", time.Now().UnixNano())
	borrower := &model.User{
		Username: username,
		Password: "officer-market-password",
		Status:   common.UserStatusEnabled,
		AffCode:  username + "-aff",
	}
	require.NoError(t, model.DB.Create(borrower).Error)
	lender := &model.User{
		Username: username + "-lender",
		Password: "officer-market-password",
		Status:   common.UserStatusEnabled,
		AffCode:  username + "-lender-aff",
	}
	require.NoError(t, model.DB.Create(lender).Error)
	return borrower, lender
}

// createOfficerFunding 直接建一条 funding（service 测试）：active 的 DueDay 推后 10 天、
// overdue 的 DueDay 置 5 天前（LastSettledDay=今天，结算不再计息，债务数值确定）
func createOfficerFunding(t *testing.T, userId, lenderId int, sourceType, plan, status string) *model.TokenLoanFunding {
	t.Helper()
	now := time.Now()
	day := model.LoanDayOf(now)
	dueDay := day + 10
	if status == model.LoanFundingOverdue {
		dueDay = day - 5
	}
	f := &model.TokenLoanFunding{
		LoanUserId:         userId,
		SourceType:         sourceType,
		OfferId:            0,
		LenderId:           lenderId,
		Amount:             1_000_000,
		PrincipalRemaining: 1_000_000,
		DebtQuota:          1_050_000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          plan,
		Status:             status,
		DueDay:             dueDay,
		PenaltyStartedDay:  dueDay,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, model.DB.Create(f).Error)
	return f
}

// aiOffer 构造一个候选 ai 模式 offer（内存态，无需落库）
func aiOffer(id, lenderId int, available int64, rateMin, rateMax float64) model.TokenLoanOffer {
	return model.TokenLoanOffer{
		Id:              id,
		LenderId:        lenderId,
		Mode:            model.LoanOfferModeAi,
		Status:          model.LoanOfferStatusActive,
		AmountAvailable: available,
		RateMin:         rateMin,
		RateMax:         rateMax,
		PerLoanCap:      available,
	}
}

// ===== 区间单定价 =====

func TestPriceAiSpaceFundingsSkipsWhenDisabledOrNoCandidates(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) { s.AiEnabled = false })
	called := false
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		called = true
		return "", nil
	})
	plans, err := PriceAiSpaceFundings(1, 5, []model.TokenLoanOffer{aiOffer(1, 9, 1_000_000, 0.0005, 0.002)})
	require.NoError(t, err)
	assert.Nil(t, plans)
	assert.False(t, called, "AI 未启用时不发模型")

	// 无候选：不发模型、返回空
	plans, err = PriceAiSpaceFundings(1, 5, nil)
	require.NoError(t, err)
	assert.Nil(t, plans)
	assert.False(t, called)
}

func TestPriceAiSpaceFundingsParsesAndMaps(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		require.NotEmpty(t, messages)
		sys := messages[0].Content
		// 匿名化 offer 列表 + 借款人档案 + 利率区间文案
		assert.Contains(t, sys, "offer_index=0")
		assert.Contains(t, sys, "当前用户档案")
		assert.Contains(t, sys, "0.05%")
		assert.Contains(t, sys, `{"fundings":[{"offer_index":0,"amount_usd":0.0,"daily_rate":0.0}]}`)
		return "```json\n{\"fundings\":[{\"offer_index\":0,\"amount_usd\":2.0,\"daily_rate\":0.001}]}\n```", nil
	})
	borrower, lender := setupLoanOfficerMarketTest(t)
	candidates := []model.TokenLoanOffer{
		aiOffer(11, lender.Id, 5_000_000, 0.0005, 0.002),
		aiOffer(12, lender.Id, 3_000_000, 0.0005, 0.001),
	}
	plans, err := PriceAiSpaceFundings(borrower.Id, 5.0, candidates)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	assert.Equal(t, 11, plans[0].OfferId)
	assert.Equal(t, lender.Id, plans[0].LenderId)
	assert.Equal(t, model.LoanFundingAi, plans[0].SourceType)
	assert.Equal(t, int64(1_000_000), plans[0].Amount) // 2 USD × QuotaPerUnit(500000)
	assert.Equal(t, 0.001, plans[0].Rate)
}

func TestPriceAiSpaceFundingsModelFailureReturnsEmpty(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "", errors.New("upstream down")
	})
	borrower, lender := setupLoanOfficerMarketTest(t)
	plans, err := PriceAiSpaceFundings(borrower.Id, 5, []model.TokenLoanOffer{aiOffer(11, lender.Id, 5_000_000, 0.0005, 0.002)})
	require.NoError(t, err)
	assert.Nil(t, plans, "模型失败返回空，不阻断借款")
}

func TestPriceAiSpaceFundingsStripsThinkContent(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "<think>分析一下借款人资质</think>```json\n{\"fundings\":[{\"offer_index\":0,\"amount_usd\":1.0,\"daily_rate\":0.001}]}\n```", nil
	})
	borrower, lender := setupLoanOfficerMarketTest(t)
	plans, err := PriceAiSpaceFundings(borrower.Id, 5, []model.TokenLoanOffer{aiOffer(11, lender.Id, 5_000_000, 0.0005, 0.002)})
	require.NoError(t, err)
	require.Len(t, plans, 1, "think 块剥离后仍能解析")
}

func TestParseAiPricingOutput(t *testing.T) {
	offers := []model.TokenLoanOffer{
		aiOffer(1, 9, 1_000_000, 0.0005, 0.002),
		aiOffer(2, 9, 1_000_000, 0.001, 0.0015),
		aiOffer(3, 9, 1_000_000, 0.0005, 0.002),
	}
	cases := []struct {
		name string
		raw  string
		want int // 期望保留的计划条数
	}{
		{"合法单条", "```json\n{\"fundings\":[{\"offer_index\":1,\"amount_usd\":1.0,\"daily_rate\":0.0012}]}\n```", 1},
		{"多个 fenced 块失败", "```json\n{\"fundings\":[{\"offer_index\":1,\"amount_usd\":1.0,\"daily_rate\":0.0012}]}\n```\n```json\n{\"fundings\":[]}\n```", 0},
		{"裸 JSON 失败", "{\"fundings\":[{\"offer_index\":1,\"amount_usd\":1.0,\"daily_rate\":0.0012}]}", 0},
		{"非法 JSON 失败", "```json\n{not a json}\n```", 0},
		{"无块失败", "很抱歉无法分配资金", 0},
		{"空列表合法", "```json\n{\"fundings\":[]}\n```", 0},
		{"利率高于区间剔除", "```json\n{\"fundings\":[{\"offer_index\":1,\"amount_usd\":1.0,\"daily_rate\":0.002}]}\n```", 0},
		{"利率低于区间剔除", "```json\n{\"fundings\":[{\"offer_index\":1,\"amount_usd\":1.0,\"daily_rate\":0.0004}]}\n```", 0},
		{"金额为 0 剔除", "```json\n{\"fundings\":[{\"offer_index\":0,\"amount_usd\":0,\"daily_rate\":0.001}]}\n```", 0},
		{"金额为负剔除", "```json\n{\"fundings\":[{\"offer_index\":0,\"amount_usd\":-1,\"daily_rate\":0.001}]}\n```", 0},
		{"索引越界剔除", "```json\n{\"fundings\":[{\"offer_index\":5,\"amount_usd\":1.0,\"daily_rate\":0.001}]}\n```", 0},
		{"负索引剔除", "```json\n{\"fundings\":[{\"offer_index\":-1,\"amount_usd\":1.0,\"daily_rate\":0.001}]}\n```", 0},
		{"混合合法与非法只保留合法", "```json\n{\"fundings\":[{\"offer_index\":0,\"amount_usd\":1.0,\"daily_rate\":0.001},{\"offer_index\":1,\"amount_usd\":1.0,\"daily_rate\":0.002},{\"offer_index\":9,\"amount_usd\":1.0,\"daily_rate\":0.001}]}\n```", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plans := parseAiPricingOutput(tc.raw, offers)
			require.Len(t, plans, tc.want)
		})
	}
}

// ===== 减免申诉工单 =====

// runAppealRound 建一张 appeal 工单并执行一轮对话（fake 模型返回给定结案块）
func runAppealRound(t *testing.T, user *model.User, rawReply string) (string, bool) {
	t.Helper()
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		require.NotEmpty(t, messages)
		// 申诉上下文注入：借款明细 + 申诉 schema
		assert.Contains(t, messages[0].Content, "用户当前借款明细")
		assert.Contains(t, messages[0].Content, "funding_id")
		return rawReply, nil
	})
	app, err := model.CreateLoanApplication(user.Id, loanAppTopicAppeal, "officer-a")
	require.NoError(t, err)
	reply, closed, err := RunLoanOfficerRound(user.Id, app, "帮我减免利息")
	require.NoError(t, err)
	return reply, closed
}

func TestRunLoanOfficerRoundAppealRejectsForeignFunding(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	other, _ := setupLoanOfficerMarketTest(t)
	foreign := createOfficerFunding(t, other.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingActive)
	user, _ := setupLoanOfficerMarketTest(t)

	reply, closed := runAppealRound(t, user, fmt.Sprintf(
		"```json\n{\"action\":\"close\",\"reply\":\"已处理\",\"decision\":{\"funding_id\":%d,\"repay_plan\":\"no_penalty\"}}\n```", foreign.Id))
	assert.True(t, closed)
	assert.Contains(t, reply, "不属于你") // 拒绝提示并入 assistant 回复

	// funding 未被改动
	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, foreign.Id).Error)
	assert.Equal(t, model.LoanRepayFull, got.RepayPlan)
	// 工单已关闭并记录决定
	var app model.TokenLoanApplication
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).Order("id DESC").First(&app).Error)
	assert.Equal(t, model.LoanAppStatusClosed, app.Status)
	assert.Contains(t, app.Decision, `"repay_plan":"no_penalty"`)
}

func TestRunLoanOfficerRoundAppealRejectsP2PPrincipalOnly(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 777, model.LoanFundingPool, model.LoanRepayFull, model.LoanFundingActive)

	// P2P 边界（Task 14）：principal_only 永远拒绝
	reply, closed := runAppealRound(t, user, fmt.Sprintf(
		"```json\n{\"action\":\"close\",\"reply\":\"已处理\",\"decision\":{\"funding_id\":%d,\"repay_plan\":\"principal_only\"}}\n```", f.Id))
	assert.True(t, closed)
	assert.Contains(t, reply, "不支持")

	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanRepayFull, got.RepayPlan)
}

func TestRunLoanOfficerRoundAppealRejectsUpgrade(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	user, _ := setupLoanOfficerMarketTest(t)
	// 当前已是 no_penalty，尝试升回 full（P2P 单向降档拒绝）
	f := createOfficerFunding(t, user.Id, 777, model.LoanFundingPool, model.LoanRepayNoPenalty, model.LoanFundingActive)

	reply, closed := runAppealRound(t, user, fmt.Sprintf(
		"```json\n{\"action\":\"close\",\"reply\":\"已处理\",\"decision\":{\"funding_id\":%d,\"repay_plan\":\"full\"}}\n```", f.Id))
	assert.True(t, closed)
	assert.Contains(t, reply, "不支持")

	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanRepayNoPenalty, got.RepayPlan)
}

func TestRunLoanOfficerRoundAppealPlatformPlanApplied(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingActive)

	// platform 借款允许改档（四档全允许）
	reply, closed := runAppealRound(t, user, fmt.Sprintf(
		"```json\n{\"action\":\"close\",\"reply\":\"已为你减免\",\"decision\":{\"funding_id\":%d,\"repay_plan\":\"no_penalty\"}}\n```", f.Id))
	assert.True(t, closed)
	assert.NotContains(t, reply, "未生效")

	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanRepayNoPenalty, got.RepayPlan)

	var app model.TokenLoanApplication
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).Order("id DESC").First(&app).Error)
	assert.Equal(t, model.LoanAppStatusClosed, app.Status)
	assert.Contains(t, app.Decision, `"funding_id":`)
}

func TestRunLoanOfficerRoundAppealInvalidPlanRejected(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingActive)

	// 非法 plan 被 ClampLoanDecision 钳制为空 → 拒绝提示，工单照常关闭
	reply, closed := runAppealRound(t, user, fmt.Sprintf(
		"```json\n{\"action\":\"close\",\"reply\":\"已处理\",\"decision\":{\"funding_id\":%d,\"repay_plan\":\"garbage\"}}\n```", f.Id))
	assert.True(t, closed)
	assert.Contains(t, reply, "未提供有效")

	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanRepayFull, got.RepayPlan)
}

func TestRunLoanOfficerRoundAppealTopicClassicDecisionStillApplies(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	user, _ := setupLoanOfficerMarketTest(t)
	// 无 funding_id：走经典路径（credit_limit 生效）
	reply, closed := runAppealRound(t, user,
		"```json\n{\"action\":\"close\",\"reply\":\"已批准\",\"decision\":{\"credit_limit\":10,\"daily_rate\":0,\"interest_free_days\":0}}\n```")
	assert.True(t, closed)
	assert.Equal(t, "已批准", reply)

	var acc model.TokenLoanAccount
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&acc).Error)
	assert.Equal(t, int64(5_000_000), acc.CustomMaxTotal) // 10 USD × 500000
}

// 经典话题工单不得被 funding_id 改档（P2-4）：模型即使带出申诉字段也走经典路径
// （申诉字段清零），appeal 改档路径只对 appeal 话题开放
func TestRunLoanOfficerRoundClassicTopicIgnoresAppealFields(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 777, model.LoanFundingPool, model.LoanRepayFull, model.LoanFundingActive)
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		// 经典话题不注入申诉上下文（buildAppealContext 只在 appeal 话题触发）
		assert.NotContains(t, messages[0].Content, "用户当前借款明细")
		return fmt.Sprintf("```json\n{\"action\":\"close\",\"reply\":\"已批准\",\"decision\":{\"credit_limit\":10,\"daily_rate\":0,\"interest_free_days\":0,\"funding_id\":%d,\"repay_plan\":\"no_penalty\"}}\n```", f.Id), nil
	})
	// 经典话题（credit）工单：即使模型带出 funding_id/repay_plan，也必须走经典路径
	app, err := model.CreateLoanApplication(user.Id, "credit", "officer-a")
	require.NoError(t, err)
	reply, closed, err := RunLoanOfficerRound(user.Id, app, "我要提额")
	require.NoError(t, err)
	assert.True(t, closed)
	assert.Equal(t, "已批准", reply)

	// appeal 路径未被触发：funding 的还款计划保持 full（未发生改档）
	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanRepayFull, got.RepayPlan)

	// 经典路径照常生效：credit_limit 被应用为 CustomMaxTotal
	var acc model.TokenLoanAccount
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&acc).Error)
	assert.Equal(t, int64(5_000_000), acc.CustomMaxTotal) // 10 USD × 500000

	// 落库决定中申诉字段被清零（经典决定，非改档决定）
	var appRow model.TokenLoanApplication
	require.NoError(t, model.DB.Where("id = ?", app.Id).First(&appRow).Error)
	assert.Equal(t, model.LoanAppStatusClosed, appRow.Status)
	assert.Contains(t, appRow.Decision, `"funding_id":0`)
	assert.Contains(t, appRow.Decision, `"repay_plan":""`)
}

// ===== 官方逾期处置 =====

func TestParseDisposalOutput(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		ok         bool
		wantAction string
		wantDays   int
	}{
		{"extend", "```json\n{\"action\":\"extend\",\"extend_days\":15}\n```", true, model.LoanDefaultActionExtend, 15},
		{"writeoff", "```json\n{\"action\":\"writeoff\"}\n```", true, model.LoanDefaultActionWriteoff, 0},
		{"perpetual", "```json\n{\"action\":\"perpetual\"}\n```", true, model.LoanDefaultActionPerpetual, 0},
		{"未知动作", "```json\n{\"action\":\"delete\"}\n```", false, "", 0},
		{"extend 无天数", "```json\n{\"action\":\"extend\"}\n```", false, "", 0},
		{"extend 天数为 0", "```json\n{\"action\":\"extend\",\"extend_days\":0}\n```", false, "", 0},
		{"extend 天数为负", "```json\n{\"action\":\"extend\",\"extend_days\":-3}\n```", false, "", 0},
		{"多个 fenced 块失败", "```json\n{\"action\":\"extend\",\"extend_days\":5}\n```\n```json\n{\"action\":\"writeoff\"}\n```", false, "", 0},
		{"裸 JSON 失败", "{\"action\":\"writeoff\"}", false, "", 0},
		{"非法 JSON 失败", "```json\n{not json}\n```", false, "", 0},
		{"think 块剥离后合法", "<think>核销处理</think>```json\n{\"action\":\"writeoff\"}\n```", true, model.LoanDefaultActionWriteoff, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			action, days, ok := parseDisposalOutput(tc.raw)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.Equal(t, tc.wantAction, action)
				require.Equal(t, tc.wantDays, days)
			}
		})
	}
}

func TestDisposePlatformOverdueFundingExtend(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) { s.LoanTermDays = 30 })
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		assert.Contains(t, messages[0].Content, "处置选项")
		assert.Contains(t, messages[0].Content, "当前用户档案")
		return "```json\n{\"action\":\"extend\",\"extend_days\":10}\n```", nil
	})
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingOverdue)

	DisposePlatformOverdueFunding(f.Id)

	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanFundingActive, got.Status)
	assert.Equal(t, model.LoanDayOf(time.Now())+10, got.DueDay)
}

func TestDisposePlatformOverdueFundingWriteoff(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) { s.LoanTermDays = 30 })
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		// think 块剥离后 writeoff 仍生效
		return "<think>确定坏账，核销</think>```json\n{\"action\":\"writeoff\"}\n```", nil
	})
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingOverdue)

	DisposePlatformOverdueFunding(f.Id)

	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanFundingWrittenOff, got.Status)
	// 借款人拉黑 + 投影销毁
	var acc model.TokenLoanAccount
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&acc).Error)
	assert.Equal(t, model.LoanDayOf(time.Now())+30, acc.BlacklistedUntilDay)
	assert.Zero(t, acc.DebtQuota)
}

func TestDisposePlatformOverdueFundingPerpetual(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) { s.LoanTermDays = 30 })
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "```json\n{\"action\":\"perpetual\"}\n```", nil
	})
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingOverdue)

	DisposePlatformOverdueFunding(f.Id)

	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanFundingOverdue, got.Status, "永续保持逾期继续计息")
}

func TestDisposePlatformOverdueFundingFallbackOnAiFailure(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) { s.LoanTermDays = 30 })
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "", errors.New("upstream down")
	})
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingOverdue)

	DisposePlatformOverdueFunding(f.Id)

	// 兜底自动延长一个 LoanTermDays（spec §9）+ SysError 告警（进服务端日志，不在断言范围）
	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanFundingActive, got.Status)
	assert.Equal(t, model.LoanDayOf(time.Now())+30, got.DueDay)
}

func TestDisposePlatformOverdueFundingFallbackWhenAiDisabled(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) {
		s.AiEnabled = false
		s.LoanTermDays = 30
	})
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		t.Fatal("AI 未启用不得调用模型")
		return "", nil
	})
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingOverdue)

	DisposePlatformOverdueFunding(f.Id)

	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanFundingActive, got.Status)
	assert.Equal(t, model.LoanDayOf(time.Now())+30, got.DueDay)
}

func TestDisposePlatformOverdueFundingInvalidOutputFallback(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) { s.LoanTermDays = 30 })
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "还在考虑中", nil // 无 fenced 块
	})
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingOverdue)

	DisposePlatformOverdueFunding(f.Id)

	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanFundingActive, got.Status)
	assert.Equal(t, model.LoanDayOf(time.Now())+30, got.DueDay)
}

func TestDisposePlatformOverdueFundingIdempotent(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) { s.LoanTermDays = 30 })
	callCount := 0
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		callCount++
		return "```json\n{\"action\":\"extend\",\"extend_days\":10}\n```", nil
	})
	user, _ := setupLoanOfficerMarketTest(t)
	f := createOfficerFunding(t, user.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingOverdue)

	// 重复派发（如翻转路径在事务提交后多次触发）：第二次状态已变，no-op 不发模型
	DisposePlatformOverdueFunding(f.Id)
	DisposePlatformOverdueFunding(f.Id)
	assert.Equal(t, 1, callCount)

	var got model.TokenLoanFunding
	require.NoError(t, model.DB.First(&got, f.Id).Error)
	assert.Equal(t, model.LoanFundingActive, got.Status)
}

func TestDisposePlatformOverdueFundingSkipsNonPlatformAndNonOverdue(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) { s.LoanTermDays = 30 })
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		t.Fatal("非平台/非逾期不得处置")
		return "", nil
	})
	user, _ := setupLoanOfficerMarketTest(t)
	p2p := createOfficerFunding(t, user.Id, 777, model.LoanFundingPool, model.LoanRepayFull, model.LoanFundingOverdue)
	active := createOfficerFunding(t, user.Id, 0, model.LoanFundingPlatform, model.LoanRepayFull, model.LoanFundingActive)
	missing := int64(99999999)

	DisposePlatformOverdueFunding(p2p.Id)    // P2P：跳过
	DisposePlatformOverdueFunding(active.Id) // active：跳过
	DisposePlatformOverdueFunding(missing)   // 不存在：告警后返回

	var gotP2p model.TokenLoanFunding
	require.NoError(t, model.DB.First(&gotP2p, p2p.Id).Error)
	assert.Equal(t, model.LoanFundingOverdue, gotP2p.Status)
	var gotActive model.TokenLoanFunding
	require.NoError(t, model.DB.First(&gotActive, active.Id).Error)
	assert.Equal(t, model.LoanFundingActive, gotActive.Status)
}
