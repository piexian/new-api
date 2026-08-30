package setting

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserGroupConcurrencyLimitRoundTrip(t *testing.T) {
	UserGroupConcurrencyLimitMutex.Lock()
	saved := UserGroupConcurrencyLimit
	UserGroupConcurrencyLimitMutex.Unlock()
	defer func() {
		UserGroupConcurrencyLimitMutex.Lock()
		UserGroupConcurrencyLimit = saved
		UserGroupConcurrencyLimitMutex.Unlock()
	}()

	err := UpdateUserGroupConcurrencyLimitByJSONString(`{"vip":3,"default":0}`)
	require.NoError(t, err)
	require.Equal(t, 3, GetUserGroupConcurrencyLimit("vip"))
	require.Equal(t, 0, GetUserGroupConcurrencyLimit("default"))
	require.Equal(t, 0, GetUserGroupConcurrencyLimit("missing"))

	var parsed map[string]int
	require.NoError(t, json.Unmarshal([]byte(UserGroupConcurrencyLimit2JSONString()), &parsed))
	require.Equal(t, map[string]int{"vip": 3, "default": 0}, parsed)
}

func TestCheckUserGroupConcurrencyLimit(t *testing.T) {
	require.NoError(t, CheckUserGroupConcurrencyLimit(`{"vip":1,"default":0}`))
	require.Error(t, CheckUserGroupConcurrencyLimit(`{"vip":-1}`))
	require.Error(t, CheckUserGroupConcurrencyLimit(`{"vip":2147483648}`))
	require.Error(t, CheckUserGroupConcurrencyLimit(`not json`))
}
