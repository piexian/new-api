package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestDoQwenTokenPlanUsageRequest(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer cli-access-token", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload qwenTokenPlanGatewayRequest
		require.NoError(t, common.Unmarshal(body, &payload))
		require.Equal(t, "BssOpenAPI-V3", payload.Product)
		require.Equal(t, "DescribeFrInstances", payload.Action)
		require.Equal(t, qwenTokenPlanCommodityCode, payload.Params["CommodityCode"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"200","data":{"Data":[]}}`))
	}))
	defer server.Close()

	statusCode, _, err := doQwenTokenPlanUsageRequest(context.Background(), server.Client(), server.URL, "cli-access-token")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, statusCode)
}

func TestParseQwenTokenPlanUsageResponse(t *testing.T) {
	t.Parallel()
	body := []byte(`{
		"code":"200",
		"data":{"Data":[
			{"TemplateName":"Expired","Status":"expired","InitCapacityBaseValue":"100","CurrCapacityBaseValue":"0"},
			{"TemplateName":"Pro","Status":{"Code":"valid"},"InitCapacityBaseValue":"1000","CurrCapacityBaseValue":"800","periodCapacityBaseValue":"750","CapacityTypeCode":"periodMonthlyShift","EndTime":1767225600000}
		]}
	}`)

	usage, success, message := parseQwenTokenPlanUsageResponse(http.StatusOK, body)
	require.True(t, success)
	require.Empty(t, message)
	require.True(t, usage.Subscribed)
	require.Equal(t, "Pro", usage.PlanName)
	require.Equal(t, float64(1000), usage.TotalCredits)
	require.Equal(t, float64(750), usage.RemainingCredits)
	require.Equal(t, float64(250), usage.UsedCredits)
	require.InDelta(t, 25, usage.UsedPercent, 0.001)
	require.Equal(t, int64(1767225600000), usage.ResetAt)
}

func TestParseQwenTokenPlanUsageResponseHandlesZeroTotal(t *testing.T) {
	t.Parallel()
	body := []byte(`{"code":"200","data":{"Data":[{"Status":"valid","InitCapacityBaseValue":"0","CurrCapacityBaseValue":"0"}]}}`)

	usage, success, message := parseQwenTokenPlanUsageResponse(http.StatusOK, body)
	require.True(t, success)
	require.Empty(t, message)
	require.Zero(t, usage.UsedPercent)
}

func TestParseQwenTokenPlanUsageResponseParsesDateResetTime(t *testing.T) {
	t.Parallel()
	body := []byte(`{"code":"200","data":{"Data":[{"Status":"valid","InitCapacityBaseValue":"100","CurrCapacityBaseValue":"80","EndTime":"2026-08-01T00:00:00Z"}]}}`)

	usage, success, message := parseQwenTokenPlanUsageResponse(http.StatusOK, body)
	require.True(t, success)
	require.Empty(t, message)
	require.Equal(t, int64(1785542400000), usage.ResetAt)
}
