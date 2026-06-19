package model

import (
	"testing"

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
