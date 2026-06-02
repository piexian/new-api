package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
)

func getSubscriptionPayMoney(planPrice float64, group string, unitPrice float64) float64 {
	if planPrice <= 0 || unitPrice <= 0 {
		return 0
	}
	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	return decimal.NewFromFloat(planPrice).
		Mul(decimal.NewFromFloat(unitPrice)).
		Mul(decimal.NewFromFloat(topupGroupRatio)).
		InexactFloat64()
}
