package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllEmailLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	logs, total, err := model.GetAllEmailLogs(
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
		model.EmailLogQueryParams{
			StartTimestamp: startTimestamp,
			EndTimestamp:   endTimestamp,
			Receiver:       c.Query("receiver"),
			Subject:        c.Query("subject"),
			Status:         c.Query("status"),
			Provider:       c.Query("provider"),
		},
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	localizeEmailLogs(logs, resolveRequestLogLanguage(c))
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func GetEmailLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid email log id")
		return
	}
	log, err := model.GetEmailLogById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	localizeEmailLogs([]*model.EmailLog{log}, resolveRequestLogLanguage(c))
	common.ApiSuccess(c, log)
}
