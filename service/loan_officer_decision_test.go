package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractLoanDecisionNormal(t *testing.T) {
	reply := "好的，我来评估一下。\n```json\n{\"action\":\"close\",\"reply\":\"已批准\",\"decision\":{\"credit_limit\":10,\"daily_rate\":0.0008,\"interest_free_days\":7}}\n```"
	display, decision, ok := ExtractLoanDecision(reply)
	require.True(t, ok)
	require.NotNil(t, decision)
	assert.Equal(t, 10.0, decision.CreditLimit)
	assert.Equal(t, 0.0008, decision.DailyRate)
	assert.Equal(t, 7, decision.InterestFreeDays)
	// json 块被剥离，块外文本作为展示文本
	assert.Equal(t, "好的，我来评估一下。", display)
}

func TestExtractLoanDecisionUppercaseLangTag(t *testing.T) {
	reply := "```JSON\n{\"action\":\"close\",\"decision\":{\"credit_limit\":1,\"daily_rate\":0,\"interest_free_days\":0}}\n```"
	display, decision, ok := ExtractLoanDecision(reply)
	require.True(t, ok)
	require.NotNil(t, decision)
	assert.Equal(t, 1.0, decision.CreditLimit)
	// 块外无文本时回退到 json 内的 reply 字段；此处 reply 也为空则展示文本为空
	assert.Equal(t, "", display)
}

func TestExtractLoanDecisionFallsBackToInnerReply(t *testing.T) {
	reply := "```json\n{\"action\":\"close\",\"reply\":\"给你提额了\",\"decision\":{\"credit_limit\":5,\"daily_rate\":0,\"interest_free_days\":0}}\n```"
	display, _, ok := ExtractLoanDecision(reply)
	require.True(t, ok)
	assert.Equal(t, "给你提额了", display)
}

func TestExtractLoanDecisionMultipleBlocksFail(t *testing.T) {
	reply := "```json\n{\"action\":\"close\",\"decision\":{\"credit_limit\":1,\"daily_rate\":0,\"interest_free_days\":0}}\n```\n中间\n```json\n{\"action\":\"close\",\"decision\":{\"credit_limit\":2,\"daily_rate\":0,\"interest_free_days\":0}}\n```"
	_, decision, ok := ExtractLoanDecision(reply)
	assert.False(t, ok)
	assert.Nil(t, decision)
}

func TestExtractLoanDecisionBareJsonFail(t *testing.T) {
	reply := "{\"action\":\"close\",\"decision\":{\"credit_limit\":1,\"daily_rate\":0,\"interest_free_days\":0}}"
	display, decision, ok := ExtractLoanDecision(reply)
	assert.False(t, ok)
	assert.Nil(t, decision)
	// 无 fenced 块时原样展示
	assert.Equal(t, reply, display)
}

func TestExtractLoanDecisionInvalidJsonFail(t *testing.T) {
	reply := "前言\n```json\n{not a json}\n```"
	display, decision, ok := ExtractLoanDecision(reply)
	assert.False(t, ok)
	assert.Nil(t, decision)
	// 剥离坏块后展示剩余文本
	assert.Equal(t, "前言", display)
}

func TestExtractLoanDecisionActionNotCloseFail(t *testing.T) {
	reply := "```json\n{\"action\":\"adjust\",\"decision\":{\"credit_limit\":1,\"daily_rate\":0,\"interest_free_days\":0}}\n```"
	_, decision, ok := ExtractLoanDecision(reply)
	assert.False(t, ok)
	assert.Nil(t, decision)
}

func TestExtractLoanDecisionPlainText(t *testing.T) {
	display, decision, ok := ExtractLoanDecision("还在评估中，请补充材料")
	assert.False(t, ok)
	assert.Nil(t, decision)
	assert.Equal(t, "还在评估中，请补充材料", display)
}

func TestClampLoanDecisionNegativeFieldsZeroed(t *testing.T) {
	s := &operation_setting.LoanSetting{AiMaxLimit: 10000000, AiMinRate: 0.0005, AiMaxGraceDays: 30, DailyRate: 0.001}
	out := ClampLoanDecision(&LoanDecision{CreditLimit: -1, DailyRate: -0.5, InterestFreeDays: -3}, s)
	assert.Equal(t, 0.0, out.CreditLimit)
	assert.Equal(t, 0.0, out.DailyRate)
	assert.Equal(t, 0, out.InterestFreeDays)
}

func TestClampLoanDecisionCaps(t *testing.T) {
	// AiMaxLimit 10000000 quota，QuotaPerUnit 500000 → 上限 20 USD
	s := &operation_setting.LoanSetting{AiMaxLimit: 10000000, AiMinRate: 0.0005, AiMaxGraceDays: 30, DailyRate: 0.001}
	out := ClampLoanDecision(&LoanDecision{CreditLimit: 100, DailyRate: 0.01, InterestFreeDays: 365}, s)
	assert.Equal(t, 20.0, out.CreditLimit)
	assert.Equal(t, 0.001, out.DailyRate)
	assert.Equal(t, 30, out.InterestFreeDays)
}

func TestClampLoanDecisionRateFloor(t *testing.T) {
	s := &operation_setting.LoanSetting{AiMaxLimit: 10000000, AiMinRate: 0.0005, AiMaxGraceDays: 30, DailyRate: 0.001}
	out := ClampLoanDecision(&LoanDecision{CreditLimit: 1, DailyRate: 0.0001, InterestFreeDays: 0}, s)
	assert.Equal(t, 0.0005, out.DailyRate)
}

func TestClampLoanDecisionRateFloorThenCeilingOnMisconfig(t *testing.T) {
	// 误配 ai_min_rate > 全局 daily_rate：先夹下限再夹上限，最终落在全局上限
	s := &operation_setting.LoanSetting{AiMaxLimit: 10000000, AiMinRate: 0.002, AiMaxGraceDays: 30, DailyRate: 0.001}
	out := ClampLoanDecision(&LoanDecision{CreditLimit: 1, DailyRate: 0.0001, InterestFreeDays: 0}, s)
	assert.Equal(t, 0.001, out.DailyRate)
}

func TestClampLoanDecisionZeroRateMeansNoAdjust(t *testing.T) {
	// daily_rate 为 0 表示不调整，不应被下限抬起
	s := &operation_setting.LoanSetting{AiMaxLimit: 10000000, AiMinRate: 0.0005, AiMaxGraceDays: 30, DailyRate: 0.001}
	out := ClampLoanDecision(&LoanDecision{CreditLimit: 0, DailyRate: 0, InterestFreeDays: 0}, s)
	assert.Equal(t, 0.0, out.DailyRate)
}

func TestClampLoanDecisionNil(t *testing.T) {
	s := &operation_setting.LoanSetting{AiMaxLimit: 10000000, AiMinRate: 0.0005, AiMaxGraceDays: 30, DailyRate: 0.001}
	out := ClampLoanDecision(nil, s)
	require.NotNil(t, out)
	assert.Equal(t, LoanDecision{}, *out)
}

func loanOfficerMsg(id int, role, content string) model.TokenLoanApplicationMessage {
	return model.TokenLoanApplicationMessage{Id: id, ApplicationId: 1, Role: role, Content: content}
}

func TestTrimLoanMessagesSlidingWindow(t *testing.T) {
	var msgs []model.TokenLoanApplicationMessage
	// 每条 300 个 CJK 字符，估算约 255 tokens/条
	for i := 1; i <= 20; i++ {
		msgs = append(msgs, loanOfficerMsg(i, "user", strings.Repeat("字", 300)))
	}
	full, err := TrimLoanMessages(msgs, 100000)
	require.NoError(t, err)
	assert.Len(t, full, 20)

	trimmed, err := TrimLoanMessages(msgs, 600)
	require.NoError(t, err)
	require.NotEmpty(t, trimmed)
	// 从最早开始丢弃：保留的是尾部连续段
	assert.Equal(t, msgs[len(msgs)-len(trimmed)].Id, trimmed[0].Id)
	assert.Equal(t, 20, trimmed[len(trimmed)-1].Id)
	assert.Less(t, len(trimmed), 20)
}

func TestTrimLoanMessagesSingleOverBudget(t *testing.T) {
	msgs := []model.TokenLoanApplicationMessage{
		loanOfficerMsg(1, "user", strings.Repeat("a", 100)),
		loanOfficerMsg(2, "assistant", strings.Repeat("字", 300)),
	}
	// 裁剪到只剩最新一条仍超预算：报哨兵错误，调用方不得发给模型
	_, err := TrimLoanMessages(msgs, 10)
	require.ErrorIs(t, err, ErrLoanContentTooLong)
}
