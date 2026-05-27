package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUserMarshalJSONUsesUsernameAsDisplayName(t *testing.T) {
	user := User{
		Username:    "alice",
		DisplayName: "Alice Display",
	}

	data, err := common.Marshal(user)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := common.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}

	if payload["display_name"] != user.Username {
		t.Fatalf("display_name = %v, want %q", payload["display_name"], user.Username)
	}
}

func insertSearchUserForTest(t *testing.T, user *User) {
	t.Helper()
	if user.AffCode == "" {
		user.AffCode = user.Username + "-aff"
	}
	require.NoError(t, DB.Create(user).Error)
}

func collectSearchUsernames(users []*User) []string {
	names := make([]string, 0, len(users))
	for _, user := range users {
		names = append(names, user.Username)
	}
	return names
}

func TestSearchUsersFiltersByStatusAndRole(t *testing.T) {
	truncateTables(t)
	initCol()

	insertSearchUserForTest(t, &User{
		Username:    "enabled-user",
		DisplayName: "Enabled User",
		Email:       "enabled@example.com",
		Group:       "default",
		Status:      common.UserStatusEnabled,
		Role:        common.RoleCommonUser,
	})
	insertSearchUserForTest(t, &User{
		Username:    "disabled-admin",
		DisplayName: "Disabled Admin",
		Email:       "disabled@example.com",
		Group:       "default",
		Status:      common.UserStatusDisabled,
		Role:        common.RoleAdminUser,
	})
	insertSearchUserForTest(t, &User{
		Username:    "disabled-root",
		DisplayName: "Disabled Root",
		Email:       "root@example.com",
		Group:       "ops",
		Status:      common.UserStatusDisabled,
		Role:        common.RoleRootUser,
	})

	users, total, err := SearchUsers("", "", common.UserStatusDisabled, 0, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.ElementsMatch(t, []string{"disabled-admin", "disabled-root"}, collectSearchUsernames(users))

	users, total, err = SearchUsers("", "", 0, common.RoleAdminUser, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, []string{"disabled-admin"}, collectSearchUsernames(users))
}

func TestSearchUsersCombinesKeywordGroupStatusAndRoleFilters(t *testing.T) {
	truncateTables(t)
	initCol()

	insertSearchUserForTest(t, &User{
		Username:    "alice-admin",
		DisplayName: "Alice Billing",
		Email:       "alice@example.com",
		Group:       "vip",
		Status:      common.UserStatusEnabled,
		Role:        common.RoleAdminUser,
	})
	insertSearchUserForTest(t, &User{
		Username:    "alice-user",
		DisplayName: "Alice Default",
		Email:       "alice-user@example.com",
		Group:       "default",
		Status:      common.UserStatusEnabled,
		Role:        common.RoleCommonUser,
	})
	insertSearchUserForTest(t, &User{
		Username:    "bob-admin",
		DisplayName: "Bob Billing",
		Email:       "bob@example.com",
		Group:       "vip",
		Status:      common.UserStatusDisabled,
		Role:        common.RoleAdminUser,
	})

	users, total, err := SearchUsers("alice", "vip", common.UserStatusEnabled, common.RoleAdminUser, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, []string{"alice-admin"}, collectSearchUsernames(users))
}
