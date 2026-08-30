package service

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

func saveGroupSettings(t *testing.T) {
	t.Helper()

	savedUsable := setting.GetUserUsableGroupsCopy()
	savedHidden := hiddenNames(setting.GetUserGroupHiddenCopy())
	special := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	savedSpecial := special.ReadAll()

	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(mustJSON(t, savedUsable)))
		require.NoError(t, setting.UpdateHiddenUserGroupsByJSONString(mustJSON(t, savedHidden)))
		special.Clear()
		special.AddAll(savedSpecial)
	})
}

func hiddenNames(hidden map[string]bool) []string {
	names := make([]string, 0, len(hidden))
	for name, ok := range hidden {
		if ok {
			names = append(names, name)
		}
	}
	return names
}

func TestFilterHiddenGroupsForDisplay(t *testing.T) {
	saveGroupSettings(t)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"vip分组"}`))
	require.NoError(t, setting.UpdateHiddenUserGroupsByJSONString(`["vip"]`))

	groups := map[string]string{"default": "默认分组", "vip": "vip分组"}

	// 非成员不可见隐藏分组
	visible := FilterHiddenGroupsForDisplay(groups, "default")
	require.Equal(t, map[string]string{"default": "默认分组"}, visible)

	// 分组本人可见
	visible = FilterHiddenGroupsForDisplay(groups, "vip")
	require.Equal(t, groups, visible)

	// 未配置隐藏时不受影响
	require.NoError(t, setting.UpdateHiddenUserGroupsByJSONString(`[]`))
	visible = FilterHiddenGroupsForDisplay(groups, "")
	require.Equal(t, groups, visible)
}

func TestFilterHiddenGroupsKeepsSpecialGrantedGroups(t *testing.T) {
	saveGroupSettings(t)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"vip分组"}`))
	require.NoError(t, setting.UpdateHiddenUserGroupsByJSONString(`["vip"]`))

	special := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	special.Set("default", map[string]string{"+:vip": "vip分组"})

	visible := FilterHiddenGroupsForDisplay(
		map[string]string{"default": "默认分组", "vip": "vip分组"},
		"default",
	)
	require.Equal(t, map[string]string{"default": "默认分组", "vip": "vip分组"}, visible)
}
