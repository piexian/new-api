package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withLoanOfficerSetting 临时启用词元贷 + AI 业务员并写入测试配置，测试结束后恢复
func withLoanOfficerSetting(t *testing.T, mutate func(s *operation_setting.LoanSetting)) {
	t.Helper()
	setting := operation_setting.GetLoanSetting()
	old := *setting
	setting.Enabled = true
	setting.AiEnabled = true
	setting.MaxTotal = 2500000
	setting.DailyRate = 0.001
	setting.AiModels = []operation_setting.AiModelConfig{{Model: "officer-a", ContextWindow: 128000}}
	setting.AiMaxLimit = 10000000 // 20 USD
	setting.AiMinRate = 0.0005
	setting.AiMaxGraceDays = 30
	setting.AiMaxActiveApplications = 0
	setting.AiDailyLimit = 0
	setting.AiMaxRounds = 10
	setting.AiMaxOutput = 2048
	setting.AiPrompt = "硬边界：额度上限 {{ai_max_limit}}，最低日利率 {{ai_min_rate}}，最长免息 {{ai_max_grace_days}} 天。"
	setting.CreditTierLimits = nil // 默认不启用档位钳制（回退 AiMaxLimit），档位用例自行配置
	if mutate != nil {
		mutate(setting)
	}
	t.Cleanup(func() { *setting = old })
}

// withFakeOfficerModel 注入假的模型调用实现，测试结束后恢复
func withFakeOfficerModel(t *testing.T, f func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error)) {
	t.Helper()
	old := callOfficerModel
	callOfficerModel = f
	t.Cleanup(func() { callOfficerModel = old })
}

// setupLoanOfficerApp 迁移工单/贷款表并创建测试用户与工单
func setupLoanOfficerApp(t *testing.T) (*model.User, *model.TokenLoanApplication) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(
		&model.TokenLoanAccount{}, &model.TokenLoanRecord{},
		&model.TokenLoanApplication{}, &model.TokenLoanApplicationMessage{},
		&model.Checkin{},
	))
	username := fmt.Sprintf("officer-test-%d", time.Now().UnixNano())
	user := &model.User{
		Username: username,
		Password: "officer-test-password",
		Status:   common.UserStatusEnabled,
		AffCode:  username + "-aff",
	}
	require.NoError(t, model.DB.Create(user).Error)
	app, err := model.CreateLoanApplication(user.Id, "测试提额", "officer-a")
	require.NoError(t, err)
	return user, app
}

func TestRunLoanOfficerRoundNormalClose(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		assert.Equal(t, "officer-a", modelName)
		require.NotEmpty(t, messages)
		// system prompt 注入 USD 文案而非 quota 原值
		assert.Contains(t, messages[0].Content, "$20.00")
		assert.Contains(t, messages[0].Content, "0.05%")
		assert.Contains(t, messages[0].Content, "当前用户档案")
		// 用户输入被包裹为引用块
		last := messages[len(messages)-1]
		assert.Equal(t, "user", last.Role)
		assert.Contains(t, last.Content, "> 我要提额")
		return "评估完毕。\n```json\n{\"action\":\"close\",\"reply\":\"已批准提额\",\"decision\":{\"credit_limit\":10,\"daily_rate\":0.0008,\"interest_free_days\":7}}\n```", nil
	})
	user, app := setupLoanOfficerApp(t)

	reply, closed, err := RunLoanOfficerRound(user.Id, app, "我要提额")
	require.NoError(t, err)
	assert.True(t, closed)
	assert.Equal(t, "评估完毕。", reply)

	var updated model.TokenLoanApplication
	require.NoError(t, model.DB.First(&updated, app.Id).Error)
	assert.Equal(t, model.LoanAppStatusClosed, updated.Status)
	assert.Contains(t, updated.Decision, `"credit_limit":10`)

	var acc model.TokenLoanAccount
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&acc).Error)
	assert.Equal(t, int64(5000000), acc.CustomMaxTotal) // 10 USD × 500000
	assert.Equal(t, 0.0008, acc.CustomDailyRate)
	assert.Equal(t, model.LoanDayOf(time.Now())+7, acc.InterestFreeUntil)

	msgs, err := model.GetLoanApplicationMessages(app.Id)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "评估完毕。", msgs[1].Content)
}

func TestRunLoanOfficerRoundParseFailKeepsOpen(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "还需要补充一些材料", nil
	})
	user, app := setupLoanOfficerApp(t)

	reply, closed, err := RunLoanOfficerRound(user.Id, app, "我要提额")
	require.NoError(t, err)
	assert.False(t, closed)
	assert.Equal(t, "还需要补充一些材料", reply)

	var updated model.TokenLoanApplication
	require.NoError(t, model.DB.First(&updated, app.Id).Error)
	assert.Equal(t, model.LoanAppStatusOpen, updated.Status)
	assert.Equal(t, "", updated.Decision)
}

func TestRunLoanOfficerRoundForceCloseAutoClose(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) {
		s.AiMaxRounds = 1 // 第一轮即强制结案轮
	})
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		// 末轮 system prompt 必须包含强制结案指令
		require.NotEmpty(t, messages)
		assert.Contains(t, messages[0].Content, "你必须在本轮结案")
		return "还在考虑中", nil // 解析失败
	})
	user, app := setupLoanOfficerApp(t)

	reply, closed, err := RunLoanOfficerRound(user.Id, app, "我要提额")
	require.NoError(t, err)
	assert.True(t, closed)
	assert.Equal(t, "还在考虑中", reply)

	var updated model.TokenLoanApplication
	require.NoError(t, model.DB.First(&updated, app.Id).Error)
	assert.Equal(t, model.LoanAppStatusClosed, updated.Status)
	assert.Equal(t, "", updated.Decision)

	msgs, err := model.GetLoanApplicationMessages(app.Id)
	require.NoError(t, err)
	require.Len(t, msgs, 3)
	assert.Equal(t, "system", msgs[2].Role)
	assert.Equal(t, "本次协商未达成任何调整", msgs[2].Content)

	// 不执行任何决定：账户不得被创建
	var acc model.TokenLoanAccount
	err = model.DB.Where("user_id = ?", user.Id).First(&acc).Error
	assert.Error(t, err)
}

func TestRunLoanOfficerRoundDecisionRollbackKeepsOpen(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "```json\n{\"action\":\"close\",\"reply\":\"已批准\",\"decision\":{\"credit_limit\":10,\"daily_rate\":0.0008,\"interest_free_days\":7}}\n```", nil
	})
	// 注入失败的决定落库实现，覆盖事务回滚路径
	oldApply := applyLoanOfficerDecision
	applyLoanOfficerDecision = func(appId int, decisionJSON string, customMaxTotal int64, customDailyRate float64, interestFreeDays int) error {
		return errors.New("simulated tx failure")
	}
	t.Cleanup(func() { applyLoanOfficerDecision = oldApply })

	user, app := setupLoanOfficerApp(t)

	reply, closed, err := RunLoanOfficerRound(user.Id, app, "我要提额")
	require.NoError(t, err)
	assert.False(t, closed)       // 执行失败：按普通回复展示
	assert.Equal(t, "已批准", reply) // 块外无文本时回退到 json 内 reply

	var updated model.TokenLoanApplication
	require.NoError(t, model.DB.First(&updated, app.Id).Error)
	assert.Equal(t, model.LoanAppStatusOpen, updated.Status)
	assert.Equal(t, "", updated.Decision)

	// 回复仍作为普通 assistant 消息入库
	msgs, err := model.GetLoanApplicationMessages(app.Id)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "assistant", msgs[1].Role)
}

func TestRunLoanOfficerRoundModelRedrawAfter3Failures(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) {
		s.AiModels = []operation_setting.AiModelConfig{
			{Model: "officer-a", ContextWindow: 128000},
			{Model: "officer-b", ContextWindow: 128000},
		}
	})
	callCount := 0
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		callCount++
		return "", errors.New("upstream down")
	})
	user, app := setupLoanOfficerApp(t)
	t.Cleanup(func() { clearLoanModelFailure(app.Id) })

	for i := 0; i < 3; i++ {
		_, _, err := RunLoanOfficerRound(user.Id, app, "你好")
		// 上游错误细节不透出，调用方只见通用哨兵错误
		require.ErrorIs(t, err, ErrLoanOfficerUnavailable)
	}
	assert.Equal(t, 3, callCount)
	// 连续失败 3 次后重抽模型（候选排除当前 officer-a → 必为 officer-b）
	assert.Equal(t, "officer-b", app.ModelUsed)

	var updated model.TokenLoanApplication
	require.NoError(t, model.DB.First(&updated, app.Id).Error)
	assert.Equal(t, "officer-b", updated.ModelUsed)
	assert.Equal(t, model.LoanAppStatusOpen, updated.Status)

	// 失败轮不产生任何消息（用户消息也未入库）
	msgs, err := model.GetLoanApplicationMessages(app.Id)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestRunLoanOfficerRoundContentTooLong(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) {
		// 极小窗口：预算触底 256 tokens
		s.AiModels = []operation_setting.AiModelConfig{{Model: "officer-a", ContextWindow: 100}}
		s.AiMaxOutput = 50
	})
	called := false
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		called = true
		return "ok", nil
	})
	user, app := setupLoanOfficerApp(t)

	// 400 个 CJK 字符估算约 340 tokens，超出触底预算 256
	_, _, err := RunLoanOfficerRound(user.Id, app, strings.Repeat("字", 400))
	require.ErrorIs(t, err, ErrLoanContentTooLong)
	assert.False(t, called) // 不发模型

	// 不入库、不关单
	msgs, qerr := model.GetLoanApplicationMessages(app.Id)
	require.NoError(t, qerr)
	assert.Empty(t, msgs)
	var updated model.TokenLoanApplication
	require.NoError(t, model.DB.First(&updated, app.Id).Error)
	assert.Equal(t, model.LoanAppStatusOpen, updated.Status)
}

func TestRunLoanOfficerRoundBusy(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "ok", nil
	})
	user, app := setupLoanOfficerApp(t)

	// 手动占住该工单的轮次锁，模拟上一轮处理中
	v, _ := loanRoundLocks.LoadOrStore(app.Id, &sync.Mutex{})
	lock := v.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	_, _, err := RunLoanOfficerRound(user.Id, app, "你好")
	require.ErrorIs(t, err, ErrLoanOfficerBusy)
}

func TestRunLoanOfficerRoundClosedApplicationRejected(t *testing.T) {
	withLoanOfficerSetting(t, nil)
	withFakeOfficerModel(t, func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "ok", nil
	})
	user, app := setupLoanOfficerApp(t)
	require.NoError(t, model.CloseLoanApplication(app.Id))
	closedApp := *app
	closedApp.Status = model.LoanAppStatusClosed

	_, _, err := RunLoanOfficerRound(user.Id, &closedApp, "你好")
	require.ErrorIs(t, err, model.ErrLoanApplicationNotOpen)
}

// withLoanCreditAccount 为用户创建带指定信用分的贷款账户（executeLoanDecision 经
// GetCreditScore 读取；账户不存在时信用分回退 CreditInitial）
func withLoanCreditAccount(t *testing.T, userId int, creditScore int) {
	t.Helper()
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.TokenLoanAccount{
		UserId:      userId,
		CreditScore: creditScore,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error)
}

// defaultCreditTiers 默认四档：[-50,$2] [0,$5] [60,$10] [80,$20]（QuotaPerUnit=500000）
var defaultCreditTiers = []operation_setting.CreditTierLimit{
	{MinScore: -50, MaxTotal: 1000000},
	{MinScore: 0, MaxTotal: 2500000},
	{MinScore: 60, MaxTotal: 5000000},
	{MinScore: 80, MaxTotal: 10000000},
}

// TestExecuteLoanDecisionClampedByCreditTier spec 测试③：信用分 50 的用户申请 $20，
// ai_max_limit 为 $20 不受限，但档位只允许 $5 → 落库总额上限为 2500000 quota
func TestExecuteLoanDecisionClampedByCreditTier(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) {
		s.CreditTierLimits = defaultCreditTiers
	})
	var decisionJSON string
	var capturedMaxTotal int64
	oldApply := applyLoanOfficerDecision
	applyLoanOfficerDecision = func(appId int, json string, customMaxTotal int64, customDailyRate float64, interestFreeDays int) error {
		decisionJSON = json
		capturedMaxTotal = customMaxTotal
		return nil
	}
	t.Cleanup(func() { applyLoanOfficerDecision = oldApply })

	user, app := setupLoanOfficerApp(t)
	withLoanCreditAccount(t, user.Id, 50) // 50 分命中 [0,$5] 档

	closed, notice := executeLoanDecision(app, operation_setting.GetLoanSetting(), "已批准", &LoanDecision{CreditLimit: 20, DailyRate: 0.0008, InterestFreeDays: 7})
	require.True(t, closed)
	assert.Equal(t, "", notice)
	assert.Equal(t, int64(2500000), capturedMaxTotal)    // AI 想给 $20，档位只允许 $5
	assert.Contains(t, decisionJSON, `"credit_limit":5`) // 决定 JSON 记录钳制后的值
}

// TestExecuteLoanDecisionHighScoreGetsHigherCap spec 测试④：信用分 90 的用户申请 $20，
// 命中 [80,$20] 档，档位不产生额外收窄（仍受 ai_max_limit 约束）
func TestExecuteLoanDecisionHighScoreGetsHigherCap(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) {
		s.CreditTierLimits = defaultCreditTiers
	})
	var capturedMaxTotal int64
	oldApply := applyLoanOfficerDecision
	applyLoanOfficerDecision = func(appId int, json string, customMaxTotal int64, customDailyRate float64, interestFreeDays int) error {
		capturedMaxTotal = customMaxTotal
		return nil
	}
	t.Cleanup(func() { applyLoanOfficerDecision = oldApply })

	user, app := setupLoanOfficerApp(t)
	withLoanCreditAccount(t, user.Id, 90) // 90 分命中 [80,$20] 档

	closed, notice := executeLoanDecision(app, operation_setting.GetLoanSetting(), "已批准", &LoanDecision{CreditLimit: 20, DailyRate: 0.0008, InterestFreeDays: 7})
	require.True(t, closed)
	assert.Equal(t, "", notice)
	assert.Equal(t, int64(10000000), capturedMaxTotal) // 高信用分用户拿到 $20
}

// TestExecuteLoanDecisionNoGrantWhenAtTierCap spec 点 1：用户个人上限已达档位上限时，
// 再次申请即使 AI 给出更高额度也钳制为 0（不再授予），结案照常执行、账户上限不变。
// （CreditLimit 是绝对值：钳 0 表示不修改 CustomMaxTotal，而非扣减。）
func TestExecuteLoanDecisionNoGrantWhenAtTierCap(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) {
		s.CreditTierLimits = defaultCreditTiers
	})
	var decisionJSON string
	var capturedMaxTotal int64
	oldApply := applyLoanOfficerDecision
	applyLoanOfficerDecision = func(appId int, json string, customMaxTotal int64, customDailyRate float64, interestFreeDays int) error {
		decisionJSON = json
		capturedMaxTotal = customMaxTotal
		return nil
	}
	t.Cleanup(func() { applyLoanOfficerDecision = oldApply })

	user, app := setupLoanOfficerApp(t)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.TokenLoanAccount{
		UserId:         user.Id,
		CustomMaxTotal: 2500000, // 已到 50 分档位上限 $5
		CreditScore:    50,
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error)

	closed, notice := executeLoanDecision(app, operation_setting.GetLoanSetting(), "已到上限", &LoanDecision{CreditLimit: 20, DailyRate: 0.0008, InterestFreeDays: 7})
	require.True(t, closed)
	assert.Equal(t, "", notice)
	assert.Equal(t, int64(0), capturedMaxTotal)          // 不再授予：0 = 不修改 CustomMaxTotal
	assert.Contains(t, decisionJSON, `"credit_limit":0`) // 决定 JSON 记录钳制后的值

	var acc model.TokenLoanAccount
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&acc).Error)
	assert.Equal(t, int64(2500000), acc.CustomMaxTotal) // 账户上限未被修改
}

// TestLoanOfficerProfileIncludesCreditTier spec 点 4：system prompt 的用户档案需包含
// 用户信用分与分级提额上限，让 AI 知道真实边界
func TestLoanOfficerProfileIncludesCreditTier(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) {
		s.CreditTierLimits = []operation_setting.CreditTierLimit{{MinScore: 0, MaxTotal: 2500000}}
	})
	user, _ := setupLoanOfficerApp(t)
	withLoanCreditAccount(t, user.Id, 50)

	profile := buildLoanOfficerProfile(user.Id)
	assert.Contains(t, profile, "信用分：50")
	assert.Contains(t, profile, "分级提额上限：$5.00")
	// CustomMaxTotal==0：个人上限展示"默认/0"，余量 = 档位上限 - 全局上限（此处 0）
	assert.Contains(t, profile, "当前个人上限：默认/0")
	assert.Contains(t, profile, "剩余可提空间：$0.00")
}

// TestLoanOfficerProfileIncludesHeadroom spec 点 4：档案需包含当前个人上限与剩余可提空间，
// 让 AI 知道该用户距离档位上限还有多少余量
func TestLoanOfficerProfileIncludesHeadroom(t *testing.T) {
	withLoanOfficerSetting(t, func(s *operation_setting.LoanSetting) {
		s.CreditTierLimits = defaultCreditTiers
		s.MaxTotal = 2500000
	})
	user, _ := setupLoanOfficerApp(t)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.TokenLoanAccount{
		UserId:         user.Id,
		CustomMaxTotal: 1000000, // 已授予 $2
		CreditScore:    50,      // 50 分命中 [$0,$5] 档
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error)

	profile := buildLoanOfficerProfile(user.Id)
	assert.Contains(t, profile, "信用分：50")
	assert.Contains(t, profile, "分级提额上限：$5.00")
	assert.Contains(t, profile, "当前个人上限：$2.00")
	assert.Contains(t, profile, "剩余可提空间：$3.00")
}
