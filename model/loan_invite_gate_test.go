package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// setupInviteGateInviter 创建带 aff_code 的邀请人；debt > 0 时建行贷款账户（debt_quota = debt）
func setupInviteGateInviter(t *testing.T, debt int64) *User {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenLoanAccount{}, &OneTimeInviteCode{}))
	username := fmt.Sprintf("invite-gate-%d", time.Now().UnixNano())
	inviter := &User{Username: username, Password: "x", AffCode: username + "-aff"}
	require.NoError(t, DB.Create(inviter).Error)
	if debt > 0 {
		require.NoError(t, DB.Create(&TokenLoanAccount{
			UserId:         inviter.Id,
			PrincipalQuota: debt,
			DebtQuota:      debt,
		}).Error)
	}
	return inviter
}

func TestHasOutstandingLoanDebt(t *testing.T) {
	// 无账户
	plain := setupInviteGateInviter(t, 0)
	has, err := HasOutstandingLoanDebt(DB, plain.Id)
	require.NoError(t, err)
	require.False(t, has)

	// 有债
	inDebt := setupInviteGateInviter(t, 1000)
	has, err = HasOutstandingLoanDebt(DB, inDebt.Id)
	require.NoError(t, err)
	require.True(t, has)

	// 账户存在但已结清（debt_quota = 0）
	require.NoError(t, DB.Model(&TokenLoanAccount{}).
		Where("user_id = ?", inDebt.Id).
		Updates(map[string]interface{}{"debt_quota": 0, "principal_quota": 0}).Error)
	has, err = HasOutstandingLoanDebt(DB, inDebt.Id)
	require.NoError(t, err)
	require.False(t, has)
}

func TestResolveCredentialReferralInviterInDebtBlocked(t *testing.T) {
	inviter := setupInviteGateInviter(t, 1000)

	// 强制邀请码注册：有债邀请人的 aff_code 视为无效
	_, err := resolveRegistrationCredentialWithTx(DB, inviter.AffCode, true)
	require.ErrorIs(t, err, ErrRegistrationCredentialInvalid)

	// 非强制注册：被静默忽略，不携带 inviter
	cred, err := resolveRegistrationCredentialWithTx(DB, inviter.AffCode, false)
	require.NoError(t, err)
	require.Equal(t, 0, cred.InviterId)
}

func TestResolveCredentialReferralNoDebtAllowed(t *testing.T) {
	inviter := setupInviteGateInviter(t, 0)
	cred, err := resolveRegistrationCredentialWithTx(DB, inviter.AffCode, false)
	require.NoError(t, err)
	require.Equal(t, inviter.Id, cred.InviterId)

	// 债务结清（debt_quota = 0）后恢复
	require.NoError(t, DB.Create(&TokenLoanAccount{UserId: inviter.Id}).Error)
	cred, err = resolveRegistrationCredentialWithTx(DB, inviter.AffCode, true)
	require.NoError(t, err)
	require.Equal(t, inviter.Id, cred.InviterId)
}

func TestResolveCredentialOneTimeInviteInviterInDebtBlocked(t *testing.T) {
	inviter := setupInviteGateInviter(t, 1000)
	code := "ot_" + strings.Repeat("a", 24)
	require.NoError(t, DB.Create(&OneTimeInviteCode{Code: code, InviterId: inviter.Id}).Error)

	_, err := resolveRegistrationCredentialWithTx(DB, code, true)
	require.ErrorIs(t, err, ErrRegistrationCredentialInvalid)
}

func TestGenerateOneTimeInviteCodeBlockedByDebt(t *testing.T) {
	inviter := setupInviteGateInviter(t, 1000)
	_, err := GenerateOneTimeInviteCode(inviter.Id)
	require.ErrorIs(t, err, ErrLoanDebtInviteBlocked)
}

func TestGenerateOneTimeInviteCodeAllowedWithoutDebt(t *testing.T) {
	inviter := setupInviteGateInviter(t, 0)
	code, err := GenerateOneTimeInviteCode(inviter.Id)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(code, oneTimeInviteCodePrefix))
}
