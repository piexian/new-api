package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// createLoanAppTestUser 创建工单测试用户并确保两张工单表已迁移
func createLoanAppTestUser(t *testing.T) *User {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenLoanApplication{}, &TokenLoanApplicationMessage{}))
	return createLoanTestUser(t)
}

// closeLoanApp 直接把工单置为 closed（审批流由后续任务实现，此处仅构造状态）
func closeLoanApp(t *testing.T, appId int) {
	t.Helper()
	require.NoError(t, DB.Model(&TokenLoanApplication{}).
		Where("id = ?", appId).Update("status", LoanAppStatusClosed).Error)
}

func countLoanApps(t *testing.T, userId int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(&TokenLoanApplication{}).Where("user_id = ?", userId).Count(&count).Error)
	return count
}

func TestCreateLoanApplicationActiveLimit(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.AiMaxActiveApplications = 1
		s.AiDailyLimit = 100 // 放开每日上限，只测并行工单上限
	})
	user := createLoanAppTestUser(t)

	app, err := CreateLoanApplication(user.Id, "提额", "gpt-4o")
	require.NoError(t, err)
	require.Equal(t, LoanAppStatusOpen, app.Status)
	require.Equal(t, user.Id, app.UserId)
	require.NotZero(t, app.CreatedAt)

	// 已有 1 个 open 工单，再建触发上限并整体回滚
	_, err = CreateLoanApplication(user.Id, "降息", "gpt-4o")
	require.ErrorIs(t, err, ErrLoanApplicationLimit)
	require.Equal(t, int64(1), countLoanApps(t, user.Id))

	// 关掉旧工单后放行
	closeLoanApp(t, app.Id)
	_, err = CreateLoanApplication(user.Id, "降息", "gpt-4o")
	require.NoError(t, err)
	require.Equal(t, int64(2), countLoanApps(t, user.Id))
}

func TestCreateLoanApplicationDailyLimit(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.AiMaxActiveApplications = 100 // 放开并行上限，只测每日上限
		s.AiDailyLimit = 2
	})
	user := createLoanAppTestUser(t)

	app1, err := CreateLoanApplication(user.Id, "t1", "gpt-4o")
	require.NoError(t, err)
	closeLoanApp(t, app1.Id) // 已关闭的工单仍计入当日新建数
	app2, err := CreateLoanApplication(user.Id, "t2", "gpt-4o")
	require.NoError(t, err)
	closeLoanApp(t, app2.Id)

	// 当日第 3 单被拒（此时已无 open 工单，证明当日数与状态无关）
	_, err = CreateLoanApplication(user.Id, "t3", "gpt-4o")
	require.ErrorIs(t, err, ErrLoanApplicationLimit)
	// 回滚后不落行
	require.Equal(t, int64(2), countLoanApps(t, user.Id))
}

func TestRateLoanApplicationOnce(t *testing.T) {
	user := createLoanAppTestUser(t)
	app, err := CreateLoanApplication(user.Id, "提额", "gpt-4o")
	require.NoError(t, err)
	closeLoanApp(t, app.Id)

	require.NoError(t, RateLoanApplication(user.Id, app.Id, 5, "很好"))

	var got TokenLoanApplication
	require.NoError(t, DB.First(&got, app.Id).Error)
	require.Equal(t, 5, got.Rating)
	require.Equal(t, "很好", got.RatingComment)

	// 重复评分报错，且不覆盖首次评分
	require.ErrorIs(t, RateLoanApplication(user.Id, app.Id, 1, "差"), ErrLoanAlreadyRated)
	require.NoError(t, DB.First(&got, app.Id).Error)
	require.Equal(t, 5, got.Rating)
	require.Equal(t, "很好", got.RatingComment)
}

func TestRateLoanApplicationRequiresClosed(t *testing.T) {
	user := createLoanAppTestUser(t)
	app, err := CreateLoanApplication(user.Id, "提额", "gpt-4o")
	require.NoError(t, err)

	// open 状态不可评
	require.ErrorIs(t, RateLoanApplication(user.Id, app.Id, 4, ""), ErrLoanAlreadyRated)
	var got TokenLoanApplication
	require.NoError(t, DB.First(&got, app.Id).Error)
	require.Equal(t, 0, got.Rating)
}

func TestRateLoanApplicationInvalidRating(t *testing.T) {
	user := createLoanAppTestUser(t)
	app, err := CreateLoanApplication(user.Id, "提额", "gpt-4o")
	require.NoError(t, err)
	closeLoanApp(t, app.Id)

	for _, r := range []int{0, -1, 6, 100} {
		require.ErrorIs(t, RateLoanApplication(user.Id, app.Id, r, ""), ErrLoanInvalidRating,
			"rating %d should be rejected", r)
	}
}

func TestRateLoanApplicationNotOwned(t *testing.T) {
	user := createLoanAppTestUser(t)
	other := createLoanAppTestUser(t)
	app, err := CreateLoanApplication(user.Id, "提额", "gpt-4o")
	require.NoError(t, err)
	closeLoanApp(t, app.Id)

	// 他人工单与不存在的 id 统一透出 gorm.ErrRecordNotFound
	require.ErrorIs(t, RateLoanApplication(other.Id, app.Id, 5, ""), gorm.ErrRecordNotFound)
	require.ErrorIs(t, RateLoanApplication(user.Id, app.Id+100000, 5, ""), gorm.ErrRecordNotFound)
}

func TestLoanApplicationMessagesOrdered(t *testing.T) {
	user := createLoanAppTestUser(t)
	app, err := CreateLoanApplication(user.Id, "提额", "gpt-4o")
	require.NoError(t, err)

	require.NoError(t, AddLoanApplicationMessage(app.Id, "user", "我想提额"))
	require.NoError(t, AddLoanApplicationMessage(app.Id, "assistant", "请说明用途"))
	require.NoError(t, AddLoanApplicationMessage(app.Id, "user", "周转"))

	msgs, err := GetLoanApplicationMessages(app.Id)
	require.NoError(t, err)
	require.Len(t, msgs, 3)
	// 按 id 升序还原对话顺序
	require.Equal(t, "user", msgs[0].Role)
	require.Equal(t, "我想提额", msgs[0].Content)
	require.Equal(t, "assistant", msgs[1].Role)
	require.Equal(t, "周转", msgs[2].Content)
	for _, m := range msgs {
		require.Equal(t, app.Id, m.ApplicationId)
		require.NotZero(t, m.CreatedAt)
	}
}

func TestGetUserLoanApplicationsPagination(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.AiMaxActiveApplications = 100
		s.AiDailyLimit = 100
	})
	user := createLoanAppTestUser(t)
	for _, topic := range []string{"a", "b", "c"} {
		_, err := CreateLoanApplication(user.Id, topic, "gpt-4o")
		require.NoError(t, err)
	}

	// id 倒序：最新在前
	page1, err := GetUserLoanApplications(user.Id, 1, 2)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.Equal(t, "c", page1[0].Topic)
	require.Equal(t, "b", page1[1].Topic)

	page2, err := GetUserLoanApplications(user.Id, 2, 2)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, "a", page2[0].Topic)

	// 越界页返回空
	page3, err := GetUserLoanApplications(user.Id, 3, 2)
	require.NoError(t, err)
	require.Empty(t, page3)
}

// ===== 并发用例（spec §10）=====

func TestRateLoanApplicationConcurrent(t *testing.T) {
	user := createLoanAppTestUser(t)
	app, err := CreateLoanApplication(user.Id, "提额", "gpt-4o")
	require.NoError(t, err)
	closeLoanApp(t, app.Id)

	// 两个 goroutine 同时评分（5 和 1），条件更新保证恰好一个成功
	var wg sync.WaitGroup
	errs := make([]error, 2)
	ratings := []int{5, 1}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = RateLoanApplication(user.Id, app.Id, ratings[idx], "")
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, ErrLoanAlreadyRated)
		}
	}
	require.Equal(t, 1, successes)

	// rating 只写一次：最终值恰好是胜者的评分
	var got TokenLoanApplication
	require.NoError(t, DB.First(&got, app.Id).Error)
	require.Contains(t, []int{5, 1}, got.Rating)
}

func TestCreateLoanApplicationConcurrentRespectsActiveLimit(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.AiMaxActiveApplications = 1
		s.AiDailyLimit = 100 // 放开每日上限，只压并行上限
	})
	user := createLoanAppTestUser(t)

	// SQLite 下一方失败或串行均可接受，关键是 open 工单数不得超过上限
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = CreateLoanApplication(user.Id, "提额", "gpt-4o")
		}()
	}
	wg.Wait()

	var openCount int64
	require.NoError(t, DB.Model(&TokenLoanApplication{}).
		Where("user_id = ? AND status = ?", user.Id, LoanAppStatusOpen).
		Count(&openCount).Error)
	require.LessOrEqual(t, openCount, int64(1))
}
