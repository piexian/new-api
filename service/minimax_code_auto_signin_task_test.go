package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// 签到响应字段为数字，提取用于日志时需容错字符串/数字两种形态。
func TestMiniMaxCodeSigninClaimSummary(t *testing.T) {
	raw := `{"claim_id":"2113308165482234393","claim_result":2,"day_no":1,"points":400}`
	var result map[string]any
	if err := common.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	dayNo, points := miniMaxCodeSigninClaimSummary(result)
	if dayNo != "1" || points != "400" {
		t.Fatalf("unexpected summary: day_no=%s points=%s", dayNo, points)
	}

	dayNo, points = miniMaxCodeSigninClaimSummary(map[string]any{"day_no": "3", "points": "1000"})
	if dayNo != "3" || points != "1000" {
		t.Fatalf("unexpected string summary: day_no=%s points=%s", dayNo, points)
	}

	dayNo, points = miniMaxCodeSigninClaimSummary(nil)
	if dayNo != "" || points != "" {
		t.Fatalf("expected empty summary, got: day_no=%s points=%s", dayNo, points)
	}
}
