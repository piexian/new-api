package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func insertLogSearchTestLog(t *testing.T, log *Log) {
	t.Helper()
	require.NoError(t, DB.Create(log).Error)
}

func TestGetAllLogsUsesExactIpAndChannelFilters(t *testing.T) {
	truncateTables(t)

	exact := &Log{UserId: 1, CreatedAt: 100, ChannelId: 12, Ip: "10.0.0.1"}
	samePrefix := &Log{UserId: 1, CreatedAt: 101, ChannelId: 12, Ip: "10.0.0.10"}
	otherChannel := &Log{UserId: 1, CreatedAt: 102, ChannelId: 13, Ip: "10.0.0.1"}
	insertLogSearchTestLog(t, exact)
	insertLogSearchTestLog(t, samePrefix)
	insertLogSearchTestLog(t, otherChannel)

	logs, total, err := GetAllLogs(
		LogTypeUnknown,
		0,
		0,
		"",
		"",
		"",
		0,
		10,
		12,
		"",
		"",
		"",
		"",
		"10.0.0.1",
	)

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, exact.Id, logs[0].Id)
}

func TestGetUserLogsUsesExactIpFilter(t *testing.T) {
	truncateTables(t)

	exact := &Log{UserId: 7, CreatedAt: 100, Ip: "10.0.0.1"}
	samePrefix := &Log{UserId: 7, CreatedAt: 101, Ip: "10.0.0.10"}
	otherUser := &Log{UserId: 8, CreatedAt: 102, Ip: "10.0.0.1"}
	insertLogSearchTestLog(t, exact)
	insertLogSearchTestLog(t, samePrefix)
	insertLogSearchTestLog(t, otherUser)

	logs, total, err := GetUserLogs(
		7,
		LogTypeUnknown,
		0,
		0,
		"",
		"",
		0,
		10,
		"",
		"",
		"",
		"",
		"10.0.0.1",
	)

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, exact.Id, logs[0].Id)
}

func TestRecordChannelManageLog(t *testing.T) {
	truncateTables(t)

	RecordChannelManageLog(42, "通道进入套餐限额冷却", map[string]interface{}{
		"event":          "channel_plan_quota_cooldown",
		"disabled_until": int64(1781865600),
	})

	var log Log
	require.NoError(t, LOG_DB.First(&log).Error)
	require.Equal(t, 0, log.UserId)
	require.Equal(t, "system", log.Username)
	require.Equal(t, LogTypeManage, log.Type)
	require.Equal(t, 42, log.ChannelId)
	require.Equal(t, "通道进入套餐限额冷却", log.Content)

	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "channel_plan_quota_cooldown", adminInfo["event"])
	require.Equal(t, float64(1781865600), adminInfo["disabled_until"])
}
