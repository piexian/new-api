package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestLoanMarketModels 市场数据模型冒烟：建表、offer/funding CRUD 写读回、
// 既有表新列经 AutoMigrate 落库且可读写（credit_score 默认 0，回填属 Task 5 迁移）
func TestLoanMarketModels(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(
		&TokenLoanAccount{}, &TokenLoanRecord{},
		&TokenLoanOffer{}, &TokenLoanFunding{},
	))

	// 新列经 AutoMigrate 自动补默认值
	for _, col := range []string{"credit_score", "blacklisted_until_day", "lender_disclaimer_agreed_at"} {
		require.True(t, DB.Migrator().HasColumn(&TokenLoanAccount{}, col), col)
	}
	for _, col := range []string{"funding_id", "lender_id"} {
		require.True(t, DB.Migrator().HasColumn(&TokenLoanRecord{}, col), col)
	}

	// SQLite 复用被删用户 id，先清掉名下残留的市场行再建行
	lender := createLoanTestUser(t)
	borrower := createLoanTestUser(t)
	require.NoError(t, DB.Where("lender_id = ?", lender.Id).Delete(&TokenLoanOffer{}).Error)
	require.NoError(t, DB.Where("lender_id = ? OR loan_user_id = ?", lender.Id, borrower.Id).
		Delete(&TokenLoanFunding{}).Error)

	now := time.Now()
	offer := &TokenLoanOffer{
		LenderId:        lender.Id,
		Mode:            LoanOfferModePool,
		Status:          LoanOfferStatusActive,
		AmountTotal:     1_000_000,
		AmountAvailable: 1_000_000,
		RateFixed:       0.001,
		PerLoanCap:      200_000,
		MinCreditScore:  60,
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	require.NoError(t, DB.Create(offer).Error)
	require.NotZero(t, offer.Id)

	funding := &TokenLoanFunding{
		LoanUserId:         borrower.Id,
		BorrowEventId:      42,
		SourceType:         LoanFundingPool,
		OfferId:            offer.Id,
		LenderId:           lender.Id,
		Amount:             200_000,
		PrincipalRemaining: 200_000,
		DebtQuota:          200_000,
		LastSettledDay:     loanDay(now),
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             loanDay(now) + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(funding).Error)
	require.NotZero(t, funding.Id)

	// 读回断言
	var gotOffer TokenLoanOffer
	require.NoError(t, DB.First(&gotOffer, offer.Id).Error)
	require.Equal(t, lender.Id, gotOffer.LenderId)
	require.Equal(t, LoanOfferModePool, gotOffer.Mode)
	require.Equal(t, LoanOfferStatusActive, gotOffer.Status)
	require.Equal(t, int64(1_000_000), gotOffer.AmountTotal)
	require.Equal(t, int64(1_000_000), gotOffer.AmountAvailable)
	require.Equal(t, 0.001, gotOffer.RateFixed)
	require.Equal(t, int64(200_000), gotOffer.PerLoanCap)
	require.Equal(t, 60, gotOffer.MinCreditScore)

	var gotFunding TokenLoanFunding
	require.NoError(t, DB.First(&gotFunding, funding.Id).Error)
	require.Equal(t, borrower.Id, gotFunding.LoanUserId)
	require.Equal(t, int64(42), gotFunding.BorrowEventId)
	require.Equal(t, LoanFundingPool, gotFunding.SourceType)
	require.Equal(t, offer.Id, gotFunding.OfferId)
	require.Equal(t, lender.Id, gotFunding.LenderId)
	require.Equal(t, int64(200_000), gotFunding.Amount)
	require.Equal(t, int64(200_000), gotFunding.PrincipalRemaining)
	require.Equal(t, LoanRepayFull, gotFunding.RepayPlan)
	require.Equal(t, LoanFundingActive, gotFunding.Status)

	// 状态流转可更新
	overdueDay := loanDay(now) + 30
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("id = ?", funding.Id).
		Updates(map[string]interface{}{"status": LoanFundingOverdue, "penalty_started_day": overdueDay}).Error)
	var updated TokenLoanFunding
	require.NoError(t, DB.First(&updated, funding.Id).Error)
	require.Equal(t, LoanFundingOverdue, updated.Status)
	require.Equal(t, overdueDay, updated.PenaltyStartedDay)

	// 账户新列：credit_score 默认 0（不回填），免责声明时间戳可写入读回
	require.NoError(t, DB.Where("user_id = ?", borrower.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Create(&TokenLoanAccount{
		UserId:                   borrower.Id,
		LastSettledDay:           loanDay(now),
		LenderDisclaimerAgreedAt: now.Unix(),
		CreatedAt:                now.Unix(),
		UpdatedAt:                now.Unix(),
	}).Error)
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", borrower.Id).First(&acc).Error)
	require.Zero(t, acc.CreditScore)
	require.Equal(t, now.Unix(), acc.LenderDisclaimerAgreedAt)
}
