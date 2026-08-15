package service

import (
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// LoanDecision AI 业务员的结案决定，金额字段单位为 USD（落库前由编排层换算为 quota）。
// FundingId/RepayPlan 为减免申诉专用字段（Task 15）：funding_id > 0 时走改档路径
// （SetFundingRepayPlanByOfficer），经典字段不参与；funding_id == 0 走经典路径。
type LoanDecision struct {
	CreditLimit      float64 `json:"credit_limit"`
	DailyRate        float64 `json:"daily_rate"`
	InterestFreeDays int     `json:"interest_free_days"`
	FundingId        int64   `json:"funding_id"`
	RepayPlan        string  `json:"repay_plan"`
}

// loanDecisionEnvelope 结案 json 块的整体结构；action 白名单只认 "close"
type loanDecisionEnvelope struct {
	Action   string        `json:"action"`
	Reply    string        `json:"reply"`
	Decision *LoanDecision `json:"decision"`
}

// 大小写不敏感匹配 fenced ```json 块（懒惰匹配到最近的闭合反引号）
var loanDecisionBlockRe = regexp.MustCompile("(?s)```[ \\t]*(?i:json)[ \\t]*\\r?\\n(.*?)```")

// 推理模型可能把 <think> 思考块混进正文（未闭合的截断块也算），
// 一律剥离，避免思考内容泄漏给用户
var loanThinkBlockRe = regexp.MustCompile("(?s)<think>.*?(</think>|$)")

// StripLoanThinkContent 剥离 AI 回复中的 <think> 思考块及孤立闭合标签
func StripLoanThinkContent(s string) string {
	cleaned := loanThinkBlockRe.ReplaceAllString(s, "")
	cleaned = strings.ReplaceAll(cleaned, "</think>", "")
	return strings.TrimSpace(cleaned)
}

// ExtractLoanDecision 从 AI 回复中提取结案决定（spec 5.3）：
// 恰好一个 fenced json 块且 action == "close" 且 JSON 合法时才 ok=true；
// 多块 / 裸 JSON / 非法 JSON / action 非白名单一律 ok=false。
// displayText 为剥离 json 块后的展示文本；块外为空时回退到块内 reply 字段。
func ExtractLoanDecision(reply string) (displayText string, decision *LoanDecision, ok bool) {
	matches := loanDecisionBlockRe.FindAllStringSubmatchIndex(reply, -1)
	if len(matches) != 1 {
		return strings.TrimSpace(reply), nil, false
	}
	stripped := strings.TrimSpace(reply[:matches[0][0]] + reply[matches[0][1]:])
	content := strings.TrimSpace(reply[matches[0][2]:matches[0][3]])
	var envelope loanDecisionEnvelope
	if err := common.UnmarshalJsonStr(content, &envelope); err != nil {
		return stripped, nil, false
	}
	if envelope.Action != "close" {
		return stripped, nil, false
	}
	if envelope.Decision == nil {
		envelope.Decision = &LoanDecision{}
	}
	if stripped == "" {
		stripped = strings.TrimSpace(envelope.Reply)
	}
	return stripped, envelope.Decision, true
}

// ClampLoanDecision 按配置钳制决定数值：三字段 <0 一律置 0；
// credit_limit 截断到 ai_max_limit（quota 换算为 USD）；
// daily_rate 先夹 ai_min_rate 下限再夹全局 daily_rate 上限（先下限后上限，误配时落在全局上限），
// 0 表示不调整、不参与钳制；interest_free_days 截断到 ai_max_grace_days；
// funding_id < 0 置 0；repay_plan 非法值钳制为空（不调整），合法四档保留。
func ClampLoanDecision(d *LoanDecision, s *operation_setting.LoanSetting) *LoanDecision {
	if d == nil {
		return &LoanDecision{}
	}
	out := *d
	if out.CreditLimit < 0 {
		out.CreditLimit = 0
	}
	if out.DailyRate < 0 {
		out.DailyRate = 0
	}
	if out.InterestFreeDays < 0 {
		out.InterestFreeDays = 0
	}
	if out.FundingId < 0 {
		out.FundingId = 0
	}
	switch out.RepayPlan {
	case model.LoanRepayFull, model.LoanRepayNoPenalty, model.LoanRepayInterestFreeze, model.LoanRepayPrincipalOnly:
		// 合法还款计划保留
	default:
		out.RepayPlan = "" // 非法计划钳制为空 = 不调整（同其余字段的钳制语义）
	}
	maxLimitUsd := float64(s.AiMaxLimit) / common.QuotaPerUnit
	if out.CreditLimit > maxLimitUsd {
		out.CreditLimit = maxLimitUsd
	}
	if out.DailyRate > 0 {
		if out.DailyRate < s.AiMinRate {
			out.DailyRate = s.AiMinRate
		}
		if out.DailyRate > s.DailyRate {
			out.DailyRate = s.DailyRate
		}
	}
	if out.InterestFreeDays > s.AiMaxGraceDays {
		out.InterestFreeDays = s.AiMaxGraceDays
	}
	return &out
}

// TrimLoanMessages 上下文裁剪：从最早的消息开始丢弃，直到估算 token 总量塞进预算。
// 裁剪到只剩最新一条仍超预算时返回 ErrLoanContentTooLong（调用方不得发给模型）。
func TrimLoanMessages(msgs []model.TokenLoanApplicationMessage, budgetTokens int) ([]model.TokenLoanApplicationMessage, error) {
	if len(msgs) == 0 {
		return msgs, nil
	}
	total := 0
	for _, m := range msgs {
		total += estimateLoanMessageTokens(m)
	}
	start := 0
	for start < len(msgs)-1 && total > budgetTokens {
		total -= estimateLoanMessageTokens(msgs[start])
		start++
	}
	if total > budgetTokens {
		return nil, ErrLoanContentTooLong
	}
	return msgs[start:], nil
}

// estimateLoanMessageTokens 单条消息的估算 token 数（正文 + 角色/分隔固定开销）
func estimateLoanMessageTokens(m model.TokenLoanApplicationMessage) int {
	return CountTextToken(m.Content, "") + 4
}
