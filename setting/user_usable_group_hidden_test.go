package setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func saveHiddenUserGroups(t *testing.T) {
	t.Helper()
	savedHidden := GetUserGroupHiddenCopy()
	t.Cleanup(func() {
		hiddenUserGroupsMutex.Lock()
		hiddenUserGroups = make(map[string]bool)
		for name := range savedHidden {
			hiddenUserGroups[name] = true
		}
		hiddenUserGroupsMutex.Unlock()
	})
}

func TestHiddenUserGroupsRoundTrip(t *testing.T) {
	saveHiddenUserGroups(t)

	require.NoError(t, UpdateHiddenUserGroupsByJSONString(`["vip","svip"]`))
	require.True(t, IsUserGroupHidden("vip"))
	require.True(t, IsUserGroupHidden("svip"))
	require.False(t, IsUserGroupHidden("default"))

	require.JSONEq(t, `["vip","svip"]`, HiddenUserGroups2JSONString())
}

func TestUpdateHiddenUserGroupsReplacesPrevious(t *testing.T) {
	saveHiddenUserGroups(t)

	require.NoError(t, UpdateHiddenUserGroupsByJSONString(`["vip"]`))
	require.NoError(t, UpdateHiddenUserGroupsByJSONString(`["svip"]`))
	require.False(t, IsUserGroupHidden("vip"))
	require.True(t, IsUserGroupHidden("svip"))
}

func TestCheckHiddenUserGroups(t *testing.T) {
	require.NoError(t, CheckHiddenUserGroups(`[]`))
	require.NoError(t, CheckHiddenUserGroups(`["vip"]`))
	require.Error(t, CheckHiddenUserGroups(`[""]`))
	require.Error(t, CheckHiddenUserGroups(`{"vip":true}`))
	require.Error(t, CheckHiddenUserGroups(`not json`))
}
