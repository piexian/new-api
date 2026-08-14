package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// createLoanTestApplication 迁移工单表并创建测试工单（关闭数量限制）
func createLoanTestApplication(t *testing.T, userId int) *TokenLoanApplication {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenLoanApplication{}, &TokenLoanApplicationMessage{}))
	setting := operation_setting.GetLoanSetting()
	old := *setting
	setting.AiMaxActiveApplications = 0
	setting.AiDailyLimit = 0
	t.Cleanup(func() { *setting = old })
	app, err := CreateLoanApplication(userId, "测试提额", "test-model")
	require.NoError(t, err)
	return app
}

func TestApplyLoanOfficerDecisionSuccess(t *testing.T) {
	user := createLoanTestUser(t)
	app := createLoanTestApplication(t, user.Id)

	err := ApplyLoanOfficerDecision(app.Id, `{"action":"close"}`, 5000000, 0.0008, 7)
	require.NoError(t, err)

	var updated TokenLoanApplication
	require.NoError(t, DB.First(&updated, app.Id).Error)
	require.Equal(t, LoanAppStatusClosed, updated.Status)
	require.Equal(t, `{"action":"close"}`, updated.Decision)

	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(5000000), acc.CustomMaxTotal)
	require.Equal(t, 0.0008, acc.CustomDailyRate)
	require.Equal(t, loanDay(time.Now())+7, acc.InterestFreeUntil)
}

func TestApplyLoanOfficerDecisionPartialFields(t *testing.T) {
	user := createLoanTestUser(t)
	app := createLoanTestApplication(t, user.Id)

	// 仅授予免息：0 值字段不写入
	err := ApplyLoanOfficerDecision(app.Id, `{"action":"close"}`, 0, 0, 3)
	require.NoError(t, err)

	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(0), acc.CustomMaxTotal)
	require.Equal(t, 0.0, acc.CustomDailyRate)
	require.Equal(t, loanDay(time.Now())+3, acc.InterestFreeUntil)
}

func TestApplyLoanOfficerDecisionNotOpenRollback(t *testing.T) {
	user := createLoanTestUser(t)
	app := createLoanTestApplication(t, user.Id)
	require.NoError(t, CloseLoanApplication(app.Id))

	// 工单已关闭：决定执行失败，账户不得被创建/修改
	err := ApplyLoanOfficerDecision(app.Id, `{"action":"close"}`, 5000000, 0.0008, 7)
	require.ErrorIs(t, err, ErrLoanApplicationNotOpen)

	var acc TokenLoanAccount
	err = DB.Where("user_id = ?", user.Id).First(&acc).Error
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	var updated TokenLoanApplication
	require.NoError(t, DB.First(&updated, app.Id).Error)
	require.Equal(t, LoanAppStatusClosed, updated.Status)
	require.Equal(t, "", updated.Decision)
}

func TestCloseLoanApplicationIdempotent(t *testing.T) {
	user := createLoanTestUser(t)
	app := createLoanTestApplication(t, user.Id)
	require.NoError(t, CloseLoanApplication(app.Id))
	require.ErrorIs(t, CloseLoanApplication(app.Id), ErrLoanApplicationNotOpen)
}
