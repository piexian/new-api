package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func decodeMiniMaxCodeFixture(t *testing.T, raw string) any {
	t.Helper()
	var value any
	require.NoError(t, common.UnmarshalJsonStr(raw, &value))
	return value
}

func TestBuildMiniMaxCodeUsageSummaryExtractsClientFields(t *testing.T) {
	credit := decodeMiniMaxCodeFixture(t, `{"token_plan_credit_fallback_enabled":true,"base_resp":{"status_code":0}}`)
	details := decodeMiniMaxCodeFixture(t, `{"details":[{"credit_type":2,"granted_amount":"30","remaining_amount":"12.5","expire_at_ms":1793000000000,"unused":1}],"total_count":1}`)
	cards := decodeMiniMaxCodeFixture(t, `{"cards":[{"card_id":"c1","source_type":3,"out_biz_no":"biz-1","expire_time":1793000000000}],"total":1}`)
	signin := decodeMiniMaxCodeFixture(t, `{"scene":2,"days":[{"day_no":1,"points":10,"is_today":true,"status":2}]}`)

	summary := buildMiniMaxCodeUsageSummary("42.5", credit, details, cards, signin)

	require.Equal(t, "42.5", summary["credit_balance"])
	require.Equal(t, true, summary["credit_fallback_enabled"])
	detailList := summary["credit_details"].([]gin.H)
	require.Len(t, detailList, 1)
	require.Equal(t, "12.5", detailList[0]["remaining_amount"])
	require.NotContains(t, detailList[0], "unused")
	cardList := summary["reset_cards"].([]gin.H)
	require.Len(t, cardList, 1)
	// source_type 必须保留数字类型，使用时原样回传给上游。
	require.Equal(t, float64(3), cardList[0]["source_type"])
	require.Equal(t, "biz-1", cardList[0]["out_biz_no"])
	days := summary["signin_days"].([]gin.H)
	require.Len(t, days, 1)
	require.Equal(t, true, days[0]["is_today"])
}

func TestBuildMiniMaxCodeUsageSummaryToleratesFailedSections(t *testing.T) {
	summary := buildMiniMaxCodeUsageSummary("", nil, nil, nil, nil)

	require.Nil(t, summary["credit_balance"])
	require.Nil(t, summary["credit_fallback_enabled"])
	require.Empty(t, summary["reset_cards"])
	require.Empty(t, summary["signin_days"])
}

func TestMiniMaxCodeRemainsViewDropsUnlistedFields(t *testing.T) {
	remains := decodeMiniMaxCodeFixture(t, `{"model_remains":[{"model_name":"general"}],"base_resp":{"status_code":0},"api_key":"secret"}`)

	view := miniMaxCodeRemainsView(remains)

	require.Contains(t, view, "model_remains")
	require.Contains(t, view, "base_resp")
	require.NotContains(t, view, "api_key")
	require.Nil(t, miniMaxCodeRemainsView(nil))
}
