package stepfun

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestResolveBaseWhitelist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		wantRoot string
		wantPlan bool
		wantOK   bool
	}{
		{"empty defaults to cn root", "", constant.StepFunRootCNBaseURL, false, true},
		{"cn alias", "stepfun", constant.StepFunRootCNBaseURL, false, true},
		{"intl alias", "stepfun-intl", constant.StepFunRootIntlBaseURL, false, true},
		{"cn step plan alias", "stepfun-step-plan", constant.StepFunRootCNBaseURL, true, true},
		{"intl step plan alias", "stepfun-intl-step-plan", constant.StepFunRootIntlBaseURL, true, true},
		{"cn url", "https://api.stepfun.com", constant.StepFunRootCNBaseURL, false, true},
		{"cn url with v1", "https://api.stepfun.com/v1", constant.StepFunRootCNBaseURL, false, true},
		{"cn url with trailing slash", "https://api.stepfun.com/", constant.StepFunRootCNBaseURL, false, true},
		{"cn step plan url", "https://api.stepfun.com/step_plan", constant.StepFunRootCNBaseURL, true, true},
		{"cn step plan url with v1", "https://api.stepfun.com/step_plan/v1", constant.StepFunRootCNBaseURL, true, true},
		{"intl url upper case host", "https://API.StepFun.AI", constant.StepFunRootIntlBaseURL, false, true},
		{"third party domain", "https://api.example.com", "", false, false},
		{"stepfun sister domain", "https://api.stepfun.org", "", false, false},
		{"proxy path", "https://api.stepfun.com/proxy", "", false, false},
		{"step plan extra path", "https://api.stepfun.com/step_plan/proxy", "", false, false},
		{"platform domain", "https://platform.stepfun.com", "", false, false},
		{"plain text", "not-a-url", "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := resolveBase(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("resolveBase(%q) ok = %v, want %v", tt.raw, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.root != tt.wantRoot || got.plan != tt.wantPlan {
				t.Fatalf("resolveBase(%q) = {root:%q plan:%v}, want {root:%q plan:%v}",
					tt.raw, got.root, got.plan, tt.wantRoot, tt.wantPlan)
			}
		})
	}
}

func TestStepFunBaseRootURLPrefix(t *testing.T) {
	t.Parallel()

	open, ok := resolveBase("https://api.stepfun.com")
	if !ok {
		t.Fatal("cn root should be whitelisted")
	}
	if got := open.path("/v1/messages"); got != "https://api.stepfun.com/v1/messages" {
		t.Fatalf("open platform path = %q", got)
	}

	plan, ok := resolveBase("stepfun-step-plan")
	if !ok {
		t.Fatal("step plan alias should be whitelisted")
	}
	if got := plan.path("/v1/messages"); got != "https://api.stepfun.com/step_plan/v1/messages" {
		t.Fatalf("step plan path = %q", got)
	}
}
