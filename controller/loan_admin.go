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

// AdminGetLoanOffers 分页返回全部放贷挂单（附放贷人用户名），keyword 过滤放贷人
// （纯数字按用户 ID，否则用户名模糊匹配）
func AdminGetLoanOffers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	items, total, err := model.AdminGetLoanOffers(pageInfo.GetPage(), pageInfo.GetPageSize(), keyword)
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// AdminGetLoanFundings 分页返回全部投放记录（附放贷人/借款人用户名），
// 可按 lender_id / loan_user_id / status 过滤
func AdminGetLoanFundings(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	lenderId, _ := strconv.Atoi(c.Query("lender_id"))
	loanUserId, _ := strconv.Atoi(c.Query("loan_user_id"))
	status := strings.TrimSpace(c.Query("status"))
	items, total, err := model.AdminGetLoanFundings(lenderId, loanUserId, status, pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// AdminGetLoanMarketOverview 借贷市场总览：挂单按状态计数、冻结闲置/在贷本金/累计利息、
// 逾期笔数与在售挂单数
func AdminGetLoanMarketOverview(c *gin.Context) {
	overview, err := model.AdminLoanMarketOverview()
	if err != nil {
		respondLoanInternalError(c, err)
		return
	}
	common.ApiSuccess(c, overview)
}
