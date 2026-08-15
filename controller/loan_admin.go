package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// AdminGetLoanAccounts 分页返回全部贷款账户（附用户名与实时投影债务），keyword 模糊匹配用户名/用户 ID
func AdminGetLoanAccounts(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	items, total, err := model.AdminGetLoanAccounts(pageInfo.GetPage(), pageInfo.GetPageSize(), keyword)
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// AdminGetLoanRecords 分页返回台账（可按 user_id 过滤）
func AdminGetLoanRecords(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId, _ := strconv.Atoi(c.Query("user_id"))
	items, total, err := model.AdminGetLoanRecords(userId, pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// AdminGetLoanApplications 分页返回 AI 业务员工单（可按 user_id / status 过滤）
func AdminGetLoanApplications(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId, _ := strconv.Atoi(c.Query("user_id"))
	status := strings.TrimSpace(c.Query("status"))
	items, total, err := model.AdminGetLoanApplications(userId, status, pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}
