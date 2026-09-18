package stepfun

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// 域名白名单：只有 StepFun 官方域名（含 Step Plan 前缀）才启用原生三端点透传。
// 与 moonshot 的 supportsNativeKimiResponses / zhipu 的 coding plan base 白名单同构：
// 非白名单 base（自建代理、中转、本地网关）不假设上游一定存在 /v1/messages 与
// /v1/responses，回落到通用协议转换，避免把请求打到不存在的路径。
var stepFunBaseAliases = map[string]struct {
	root string
	plan bool
}{
	constant.StepFunBaseURLAlias:             {root: constant.StepFunRootCNBaseURL},
	constant.StepFunIntlBaseURLAlias:         {root: constant.StepFunRootIntlBaseURL},
	constant.StepFunStepPlanBaseURLAlias:     {root: constant.StepFunRootCNBaseURL, plan: true},
	constant.StepFunIntlStepPlanBaseURLAlias: {root: constant.StepFunRootIntlBaseURL, plan: true},
}

type stepFunBase struct {
	root string // https://api.stepfun.com 或 https://api.stepfun.ai
	plan bool   // true 表示 Step Plan 通道，上游路径带 /step_plan 前缀
	raw  string // 归一化后的原始 base，用于回落与日志
}

// rootURL 返回拼接上游路径用的前缀（Step Plan 通道带 /step_plan）。
func (b stepFunBase) rootURL() string {
	if b.plan {
		return b.root + constant.StepFunStepPlanPath
	}
	return b.root
}

func (b stepFunBase) path(suffix string) string {
	return b.rootURL() + suffix
}

// resolveBase 归一化渠道 base_url：
//   - 支持别名（stepfun / stepfun-intl / stepfun-step-plan / stepfun-intl-step-plan）
//   - 支持官方域名 URL，容忍尾部斜杠与 /v1、/step_plan/v1 之类的版本后缀
//   - 其它形态（自有代理、带额外路径段、非官方域名）返回 false，交由通用实现处理
func resolveBase(raw string) (stepFunBase, bool) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		normalized = constant.StepFunRootCNBaseURL
	}
	if alias, ok := stepFunBaseAliases[strings.ToLower(normalized)]; ok {
		return stepFunBase{root: alias.root, plan: alias.plan, raw: normalized}, true
	}

	trimmed := strings.TrimRight(normalized, "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return stepFunBase{}, false
	}
	host := strings.ToLower(parsed.Host)
	if host != "api.stepfun.com" && host != "api.stepfun.ai" {
		return stepFunBase{}, false
	}
	root := "https://" + host
	if strings.EqualFold(parsed.Scheme, "http") {
		root = "http://" + host
	}

	// 仅接受根路径、/v1 与 /step_plan(/v1) 四种形态；出现额外路径段说明是代理，不做假设。
	path := strings.TrimRight(parsed.Path, "/")
	plan := false
	switch {
	case path == "" || path == "/v1":
	case path == constant.StepFunStepPlanPath:
		plan = true
	case path == constant.StepFunStepPlanPath+"/v1":
		plan = true
	case strings.HasPrefix(path, constant.StepFunStepPlanPath+"/"):
		// /step_plan/<其它> 视为代理前缀，保守回落到非白名单
		return stepFunBase{}, false
	default:
		return stepFunBase{}, false
	}
	return stepFunBase{root: root, plan: plan, raw: trimmed}, true
}

// isStepFunBase 判断 base 是否命中 StepFun 官方域名白名单。
func isStepFunBase(raw string) bool {
	_, ok := resolveBase(raw)
	return ok
}

// OpenAIBaseURL 归一化渠道 base 为 OpenAI 兼容根地址（Step Plan 通道带 /step_plan 前缀），
// 供模型拉取等按「base + /v1/...」拼 URL 的调用方使用；非白名单 base 原样返回。
func OpenAIBaseURL(raw string) string {
	if base, ok := resolveBase(raw); ok {
		return base.rootURL()
	}
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}
