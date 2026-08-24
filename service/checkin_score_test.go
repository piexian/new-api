package service

import (
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== 反脚本客户端评分（P1/P2）测试 =====

func newCheckinTestContext(headers map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/user/checkin", nil)
	for k, v := range headers {
		c.Request.Header.Set(k, v)
	}
	return c
}

// browserLikeHeaders 凑齐真实浏览器的全部信号
func browserLikeHeaders() map[string]string {
	return map[string]string{
		"User-Agent":      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
		"Sec-Fetch-Mode":  "cors",
		"Sec-Fetch-Site":  "same-origin",
		"Sec-Fetch-Dest":  "empty",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Accept-Encoding": "gzip, deflate, br, zstd",
		"Sec-Ch-Ua":       `"Chromium";v="126", "Google Chrome";v="126"`,
		"Origin":          "https://example.com",
		"Accept":          "application/json, text/plain, */*",
	}
}

func withClientCheckEnabled(t *testing.T, enabled bool) {
	t.Helper()
	setting := operation_setting.GetCheckinSetting()
	old := *setting
	setting.ClientCheckEnabled = enabled
	t.Cleanup(func() { *setting = old })
}

func TestClientEnvironmentScoreFullBrowser(t *testing.T) {
	c := newCheckinTestContext(browserLikeHeaders())
	assert.Equal(t, checkinHeaderMaxScore, clientEnvironmentScore(c), "完整浏览器请求头应拿满环境分")
}

func TestClientEnvironmentScorePythonRequests(t *testing.T) {
	c := newCheckinTestContext(map[string]string{
		"User-Agent": "python-requests/2.31.0",
	})
	assert.Equal(t, 0, clientEnvironmentScore(c), "裸 python-requests 应得 0 分")
}

func TestClientEnvironmentScoreForgedUAAlone(t *testing.T) {
	// 只伪造 UA、其余浏览器头全缺：只能拿到 UA 一项的分，
	// 不存在"装一个头就满分"的单一决定性信号
	c := newCheckinTestContext(map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36",
	})
	score := clientEnvironmentScore(c)
	assert.LessOrEqual(t, score, 6, "只伪造 UA 最多拿到 UA 一项的分")
	assert.Greater(t, score, 0)
}

func TestCheckinClientScoreDisabledAlways100(t *testing.T) {
	withClientCheckEnabled(t, false)
	c := newCheckinTestContext(map[string]string{"User-Agent": "curl/8.0.0"})
	assert.Equal(t, 100, CheckinClientScore(c, 999999), "功能未启用时恒定 100")
}

func TestCheckinClientScoreScriptVsBrowser(t *testing.T) {
	withClientCheckEnabled(t, true)
	// 无签到历史的用户行为分为满分（不误伤新用户），差异全部来自环境分
	script := newCheckinTestContext(map[string]string{"User-Agent": "python-requests/2.31.0"})
	browser := newCheckinTestContext(browserLikeHeaders())

	scriptScore := CheckinClientScore(script, 999998)
	browserScore := CheckinClientScore(browser, 999997)

	assert.Equal(t, checkinBehaviorMaxScore, scriptScore, "脚本环境：只剩行为分")
	assert.Equal(t, 100, browserScore, "完整浏览器 + 无历史：满分")
	assert.Less(t, scriptScore, browserScore)
}

func TestGradedScore(t *testing.T) {
	assert.Equal(t, 10, gradedScore(999, 60, 600, 10), "高于人类上限满分")
	assert.Equal(t, 10, gradedScore(600, 60, 600, 10))
	assert.Equal(t, 0, gradedScore(60, 60, 600, 10), "低于脚本线 0 分")
	assert.Equal(t, 0, gradedScore(0, 60, 600, 10))
	assert.Equal(t, 5, gradedScore(330, 60, 600, 10), "中点线性插值")
	// +Inf（无足够样本）按满分
	assert.Equal(t, 10, gradedScore(math.Inf(1), 60, 600, 10))
}

func TestCircularStdDevSeconds(t *testing.T) {
	// 完全相同的签到时刻（cron 定时任务）→ 离散度 0
	same := []float64{3600, 3600, 3600, 3600}
	assert.InDelta(t, 0, circularStdDevSeconds(same), 1e-6)

	// 跨零点聚集：23:59 与 00:01 实际只差 2 分钟，圆周统计必须识别为小离散度
	midnightCluster := []float64{86340, 60, 86340, 60}
	assert.Less(t, circularStdDevSeconds(midnightCluster), 300.0,
		"跨零点的聚集样本不能用线性方差误判")

	// 全天分散（人类作息）→ 离散度大
	spread := []float64{3600, 18000, 36000, 54000, 72000}
	assert.Greater(t, circularStdDevSeconds(spread), 3600.0)
}

func TestCheckinBehaviorScoreCronUser(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.Checkin{}))
	userId := 990001
	require.NoError(t, model.DB.Where("user_id = ?", userId).Delete(&model.Checkin{}).Error)

	// 5 次签到全部掐在 00:00:05（cron 特征）
	for i := 1; i <= 5; i++ {
		day := time.Now().AddDate(0, 0, -i)
		ts := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 5, 0, time.Local).Unix()
		require.NoError(t, model.DB.Create(&model.Checkin{
			UserId:       userId,
			CheckinDate:  day.Format("2006-01-02"),
			QuotaAwarded: 1000,
			CreatedAt:    ts,
		}).Error)
	}

	// 中午触发：时刻离散度 0 分 + 非零点 10 分 + 新账号（<7天）真实使用 10 分
	noon := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 12, 0, 0, 0, time.Local)
	score := checkinBehaviorScore(userId, noon)
	assert.Equal(t, behaviorWeightMidnightRush+behaviorWeightRealUsage, score,
		"cron 用户只应拿到非零点和宽限分")
}

func TestCheckinBehaviorScoreHumanUser(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.Checkin{}))
	userId := 990002
	require.NoError(t, model.DB.Where("user_id = ?", userId).Delete(&model.Checkin{}).Error)

	// 签到时刻分散在不同时段 + 很早就开始签到 + 最近 7 天有真实消费
	hours := []int{8, 13, 19, 22, 10}
	for i, h := range hours {
		day := time.Now().AddDate(0, 0, -(i + 10))
		ts := time.Date(day.Year(), day.Month(), day.Day(), h, 17, 31, 0, time.Local).Unix()
		require.NoError(t, model.DB.Create(&model.Checkin{
			UserId:       userId,
			CheckinDate:  day.Format("2006-01-02"),
			QuotaAwarded: 1000,
			CreatedAt:    ts,
		}).Error)
	}
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId:    userId,
		Type:      model.LogTypeConsume,
		Quota:     500,
		CreatedAt: common.GetTimestamp(),
		Content:   "real usage",
	}).Error)

	noon := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 12, 0, 0, 0, time.Local)
	score := checkinBehaviorScore(userId, noon)
	assert.Equal(t, checkinBehaviorMaxScore, score, "作息分散且有真实消费的用户应拿满分")
}
