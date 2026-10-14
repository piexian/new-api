package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func sessionRiskLog(user, token int, id, cache string, when int64) model.Log {
	return model.Log{UserId: user, TokenId: token, Username: fmt.Sprintf("user-%d", user), TokenName: fmt.Sprintf("token-%d", token), CreatedAt: when, Type: model.LogTypeConsume, Quota: 10,
		SessionId: id, SessionHash: common.SessionIdentifierHash(id), SessionSource: "header:session-id", CacheKey: cache, CacheHash: common.SessionIdentifierHash(cache), CacheSource: "body:prompt_cache_key", SessionRecorded: true}
}

func cleanupSessionRiskTestData(t *testing.T, userIds ...int) {
	t.Helper()
	t.Cleanup(func() {
		model.DB.Where("user_id IN ?", userIds).Delete(&model.RiskBanLog{})
		model.LOG_DB.Where("user_id IN ?", userIds).Delete(&model.Log{})
		model.DB.Unscoped().Where("id IN ?", userIds).Delete(&model.User{})
	})
}

func TestSharedSessionsWindowIdentityAndSearch(t *testing.T) {
	setupRiskModels(t)
	userIds := []int{98801, 98802, 98803, 98804, 98805, 98806, 98807}
	cleanupSessionRiskTestData(t, userIds...)
	now := common.GetTimestamp()
	logs := []model.Log{
		sessionRiskLog(98801, 11, "shared", "cache", now-60), sessionRiskLog(98802, 22, "shared", "cache", now),
		sessionRiskLog(98801, 12, "shared", "cache", now-10),
		sessionRiskLog(98803, 31, "single-user", "other", now), sessionRiskLog(98803, 32, "single-user", "other", now),
		sessionRiskLog(98801, 11, "SHARED", "", now), sessionRiskLog(98804, 41, "shared", "", now-61),
		sessionRiskLog(98805, 51, "shared", "", now+60),
		// Identical text in different identifier kinds must never form a group.
		sessionRiskLog(98806, 61, "split", "", now), sessionRiskLog(98807, 71, "", "split", now),
	}
	logs[0].Ip = "203.0.113.1"
	logs[1].Ip = "203.0.113.2"
	logs[1].Type, logs[1].Quota = model.LogTypeError, 0
	require.NoError(t, model.LOG_DB.Create(&logs).Error)
	for _, keyword := range []string{"", "user-98802", "token-22", "203.0.113."} {
		page, err := ListSharedSessions(SessionRiskKindSession, 1, 10, keyword, now-60, now, "", "")
		require.NoError(t, err)
		require.Equal(t, 1, page.Total)
		require.Len(t, page.Items, 1)
		item := page.Items[0]
		require.Equal(t, "shared", item.Identifier)
		require.EqualValues(t, 2, item.UserCount)
		require.EqualValues(t, 3, item.TokenCount)
		require.EqualValues(t, 3, item.RequestCount)
		require.EqualValues(t, 1, item.ErrorCount)
		require.EqualValues(t, 20, item.Quota)
		require.Len(t, item.Users, 2)
		require.Len(t, item.IPs, 2)
		require.Equal(t, now-60, item.FirstSeenAt)
	}
	page, err := ListSharedSessions(SessionRiskKindCache, 1, 10, "", now-60, now, "", "")
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Equal(t, "cache", page.Items[0].Identifier)
	_, err = ListSharedSessions("session_id; DROP TABLE logs", 1, 10, "", 0, 0, "", "")
	require.Error(t, err)
}

func TestSharedSessionBanValidatesUsers(t *testing.T) {
	setupRiskModels(t)
	userIds := []int{98811, 98812, 98813}
	cleanupSessionRiskTestData(t, userIds...)
	now := common.GetTimestamp()
	for i, id := range userIds {
		require.NoError(t, model.DB.Create(&model.User{
			Id: id, Username: fmt.Sprintf("sess_risk_%d", i), AffCode: fmt.Sprintf("SESSR%d", id),
			Role: common.RoleCommonUser, Status: common.UserStatusEnabled, CreatedAt: now - 120,
		}).Error)
	}
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		sessionRiskLog(userIds[0], 1, "shared", "", now),
		sessionRiskLog(userIds[1], 2, "shared", "", now),
	}).Error)

	// 选定窗口外未关联的用户必须整体拒绝，且不做任何封禁。
	req := SessionRiskBanRequest{Kind: SessionRiskKindSession, Identifier: "shared", Start: now - 60, End: now,
		UserIds: []int{userIds[0], userIds[2]}, Reason: "session review", DurationMinutes: 5}
	_, err := BanSharedSessionUsers(req, 1)
	require.ErrorContains(t, err, "not associated")
	var unchanged model.User
	require.NoError(t, model.DB.First(&unchanged, userIds[0]).Error)
	require.Equal(t, common.UserStatusEnabled, unchanged.Status)

	// 全部关联用户正常封禁，并按会话风控规则写入封禁日志。
	req.UserIds = []int{userIds[0], userIds[1]}
	result, err := BanSharedSessionUsers(req, 1)
	require.NoError(t, err)
	require.Equal(t, 2, result["success"])
	require.Empty(t, result["failures"])
	var banned model.User
	require.NoError(t, model.DB.First(&banned, userIds[1]).Error)
	require.Equal(t, common.UserStatusDisabled, banned.Status)
	require.GreaterOrEqual(t, banned.DisabledUntil, now+300)
	var banLog model.RiskBanLog
	require.NoError(t, model.DB.Where("rule_id = ? AND user_id = ?", model.SessionRiskRuleID, userIds[1]).First(&banLog).Error)
	require.Equal(t, "session review", banLog.Reason)
}
