package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 需求 A：管理/充值日志必须记录 User-Agent。RecordOperationAuditLog 与 RecordTopupLog
// 新增的 userAgent 参数应裁剪空白后落库到 Log.UserAgent；后台流程/服务端回调传空串时
// 落库为空。IP 照旧记录。

func TestRecordOperationAuditLogWritesUserAgent(t *testing.T) {
	user := createLoanTestUser(t)

	RecordOperationAuditLog(user.Id, "audit content", "1.2.3.4", "loan.test_action",
		map[string]interface{}{"a": 1}, nil, nil, "  Mozilla/5.0 loan-test-agent  ")
	var log Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", user.Id, LogTypeManage).Order("id DESC").First(&log).Error)
	require.Equal(t, "Mozilla/5.0 loan-test-agent", log.UserAgent, "User-Agent 应裁剪空白后落库")
	require.Equal(t, "1.2.3.4", log.Ip)

	// 后台流程无请求上下文：传空串，落库为空
	RecordOperationAuditLog(user.Id, "audit content 2", "1.2.3.5", "loan.test_action2", nil, nil, nil, "")
	var log2 Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ? AND ip = ?", user.Id, LogTypeManage, "1.2.3.5").First(&log2).Error)
	require.Empty(t, log2.UserAgent)
}

func TestRecordTopupLogWritesUserAgent(t *testing.T) {
	user := createLoanTestUser(t)

	RecordTopupLog(user.Id, "topup content", "9.9.9.9", "test", "test", "  topup-agent/2.0  ")
	var log Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", user.Id, LogTypeTopup).Order("id DESC").First(&log).Error)
	require.Equal(t, "topup-agent/2.0", log.UserAgent, "User-Agent 应裁剪空白后落库")
	require.Equal(t, "9.9.9.9", log.Ip)

	// 服务端支付回调无请求上下文：传空串，落库为空
	RecordTopupLog(user.Id, "topup content 2", "9.9.9.10", "test", "test", "")
	var log2 Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ? AND ip = ?", user.Id, LogTypeTopup, "9.9.9.10").First(&log2).Error)
	require.Empty(t, log2.UserAgent)
}
