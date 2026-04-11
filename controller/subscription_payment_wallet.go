package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type SubscriptionWalletPayRequest struct {
	PlanId int `json:"plan_id"`
}

func SubscriptionRequestWalletPay(c *gin.Context) {
	var req SubscriptionWalletPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	userId := c.GetInt("id")
	order, err := model.WalletPurchaseSubscription(userId, req.PlanId)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrSubscriptionWalletQuotaNotEnough):
			common.ApiErrorMsg(c, "钱包余额不足")
		default:
			common.ApiError(c, err)
		}
		return
	}

	common.ApiSuccess(c, gin.H{
		"order": order,
	})
}
