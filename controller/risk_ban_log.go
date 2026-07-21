package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// parseDryRunFilter 将查询参数 dry_run 解析为三态过滤器（nil 表示不过滤）。
func parseDryRunFilter(c *gin.Context) *bool {
	switch c.Query("dry_run") {
	case "true", "1":
		v := true
		return &v
	case "false", "0":
		v := false
		return &v
	default:
		return nil
	}
}

// ListRiskBanLogs 分页查询封禁日志，支持维度、来源、关键字、时间与演练状态过滤。
func ListRiskBanLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	filter := model.RiskBanLogFilter{
		Dimension: c.Query("dimension"),
		Source:    c.Query("source"),
		Keyword:   c.Query("keyword"),
		DryRun:    parseDryRunFilter(c),
	}
	if v, err := strconv.ParseInt(c.Query("start_at"), 10, 64); err == nil {
		filter.StartAt = v
	}
	if v, err := strconv.ParseInt(c.Query("end_at"), 10, 64); err == nil {
		filter.EndAt = v
	}
	logs, total, err := model.GetRiskBanLogs(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

// GetRiskBanLog 返回单条封禁日志详情。
func GetRiskBanLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的日志 ID")
		return
	}
	log, err := model.GetRiskBanLogById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, log)
}

// RiskBanLogStats 返回封禁日志统计数据。
func RiskBanLogStats(c *gin.Context) {
	stats, err := model.GetRiskBanLogStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}
