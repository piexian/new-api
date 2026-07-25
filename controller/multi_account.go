package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type multiAccountBanRequest struct {
	Reason          string `json:"reason"`
	DurationMinutes int    `json:"duration_minutes"`
}

func ListMultiAccountClusters(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	result, err := service.ListMultiAccountClusters(pageInfo.GetPage(), pageInfo.GetPageSize(), c.Query("keyword"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func BanMultiAccountUser(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	var request multiAccountBanRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "无效的请求参数")
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" {
		common.ApiErrorMsg(c, "封禁原因不能为空")
		return
	}
	user, err := service.BanMultiAccountUser(userId, c.GetInt("id"), request.DurationMinutes, request.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, user)
}
