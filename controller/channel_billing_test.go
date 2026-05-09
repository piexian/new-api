package controller

import (
	"math"
	"strings"
	"testing"
)

func TestParsePoeCurrentBalance(t *testing.T) {
	t.Parallel()

	balance, err := parsePoeCurrentBalance([]byte(`{"current_point_balance":295932027}`))
	if err != nil {
		t.Fatalf("parsePoeCurrentBalance returned error: %v", err)
	}
	if balance != 295932027 {
		t.Fatalf("balance = %v, want %v", balance, float64(295932027))
	}
}

func TestParseDeepSeekBalanceUSDResponse(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"is_available": true,
		"balance_infos": [
			{
				"currency": "USD",
				"total_balance": "26.42",
				"granted_balance": "0.00",
				"topped_up_balance": "26.42"
			}
		]
	}`)

	balance, err := parseDeepSeekBalance(body)
	if err != nil {
		t.Fatalf("parseDeepSeekBalance returned error: %v", err)
	}
	assertFloatEqual(t, balance, 26.42)
}

func TestParseDeepSeekBalanceUnavailable(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"is_available": false,
		"balance_infos": [
			{
				"currency": "USD",
				"total_balance": "26.42"
			}
		]
	}`)

	balance, err := parseDeepSeekBalance(body)
	if err != nil {
		t.Fatalf("parseDeepSeekBalance returned error: %v", err)
	}
	assertFloatEqual(t, balance, 0)
}

func TestResolveDeepSeekBalanceLegacyCNYResponse(t *testing.T) {
	t.Parallel()

	balance, err := resolveDeepSeekBalance(DeepSeekUsageResponse{
		BalanceInfos: []DeepSeekBalanceInfo{
			{
				Currency:     "CNY",
				TotalBalance: "73.00",
			},
		},
	}, 7.3)
	if err != nil {
		t.Fatalf("resolveDeepSeekBalance returned error: %v", err)
	}
	assertFloatEqual(t, balance, 10)
}

func TestParseDeepSeekBalanceMissingCurrency(t *testing.T) {
	t.Parallel()

	_, err := parseDeepSeekBalance([]byte(`{
		"is_available": true,
		"balance_infos": [
			{
				"currency": "EUR",
				"total_balance": "10.00"
			}
		]
	}`))
	if err == nil {
		t.Fatal("parseDeepSeekBalance returned nil error")
	}
	if !strings.Contains(err.Error(), "currency USD or CNY not found") {
		t.Fatalf("error = %q, want missing currency error", err.Error())
	}
}

func assertFloatEqual(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("value = %v, want %v", got, want)
	}
}
