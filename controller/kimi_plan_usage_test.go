package controller

import (
	"reflect"
	"testing"
)

func TestKimiCodingPlanAPIBase(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		baseURL string
		want    string
		ok      bool
	}{
		{name: "special base", baseURL: "kimi-coding-plan", want: "https://api.kimi.com/coding/v1", ok: true},
		{name: "custom coding base", baseURL: "https://example.com/coding", want: "https://example.com/coding/v1", ok: true},
		{name: "custom coding base with slash", baseURL: "https://example.com/coding/", want: "https://example.com/coding/v1", ok: true},
		{name: "custom coding v1 base", baseURL: "https://example.com/coding/v1", want: "https://example.com/coding/v1", ok: true},
		{name: "regular moonshot base", baseURL: "https://api.moonshot.cn", ok: false},
		{name: "empty base", baseURL: "", ok: false},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, ok := kimiCodingPlanAPIBase(testCase.baseURL)
			if ok != testCase.ok || got != testCase.want {
				t.Fatalf("kimiCodingPlanAPIBase(%q) = (%q, %v), want (%q, %v)", testCase.baseURL, got, ok, testCase.want, testCase.ok)
			}
		})
	}
}

func TestParseKimiCodingPlanExtraUsage(t *testing.T) {
	t.Parallel()

	body := []byte(`{
  "usage": {"used": 100, "limit": 100},
  "boosterWallet": {
    "balance": {
      "type": "BOOSTER",
      "amount": "2500000000",
      "amountLeft": 0
    },
    "monthlyChargeLimitEnabled": false,
    "monthlyChargeLimit": {
      "priceInCents": "5000",
      "currency": "CNY"
    },
    "monthlyUsed": {
      "priceInCents": 2500,
      "currency": "CNY"
    }
  }
}`)

	want := &kimiCodingPlanExtraUsage{
		BalanceCents:              0,
		TotalCents:                2500,
		MonthlyChargeLimitEnabled: false,
		MonthlyChargeLimitCents:   5000,
		MonthlyUsedCents:          2500,
		Currency:                  "CNY",
	}
	if got := parseKimiCodingPlanExtraUsage(body); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseKimiCodingPlanExtraUsage() = %#v, want %#v", got, want)
	}
}

func TestParseKimiCodingPlanExtraUsageRequiresBoosterBalance(t *testing.T) {
	t.Parallel()

	testCases := [][]byte{
		[]byte(`{}`),
		[]byte(`{"boosterWallet":{"balance":{"type":"OTHER","amount":1000000}}}`),
		[]byte(`{"boosterWallet":{"balance":{"type":"BOOSTER","amount":0}}}`),
	}
	for _, body := range testCases {
		if got := parseKimiCodingPlanExtraUsage(body); got != nil {
			t.Fatalf("parseKimiCodingPlanExtraUsage(%s) = %#v, want nil", body, got)
		}
	}
}

func TestKimiCodingPlanFixedPointToCentsRoundsLikeCLI(t *testing.T) {
	t.Parallel()

	testCases := map[int64]int64{
		0:       0,
		1:       1,
		500000:  1,
		1000000: 1,
		1499999: 1,
		1500000: 2,
	}
	for input, want := range testCases {
		if got := kimiCodingPlanFixedPointToCents(input); got != want {
			t.Fatalf("kimiCodingPlanFixedPointToCents(%d) = %d, want %d", input, got, want)
		}
	}
}
