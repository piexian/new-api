package controller

import "testing"

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
