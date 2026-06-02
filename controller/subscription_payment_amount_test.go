package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetSubscriptionPayMoneyAppliesPaymentRatio(t *testing.T) {
	originalTopupGroupRatio := common.TopupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(originalTopupGroupRatio))
	})

	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1,"vip":1.2,"zero":0}`))

	testCases := []struct {
		name      string
		planPrice float64
		group     string
		unitPrice float64
		expected  float64
	}{
		{
			name:      "50 to 1 balance price",
			planPrice: 1,
			group:     "default",
			unitPrice: 50,
			expected:  50,
		},
		{
			name:      "applies topup group ratio",
			planPrice: 1,
			group:     "vip",
			unitPrice: 50,
			expected:  60,
		},
		{
			name:      "zero group ratio falls back to one",
			planPrice: 2,
			group:     "zero",
			unitPrice: 25,
			expected:  50,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getSubscriptionPayMoney(tc.planPrice, tc.group, tc.unitPrice)
			require.InDelta(t, tc.expected, actual, 0.000001)
		})
	}
}
