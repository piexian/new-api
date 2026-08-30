package setting

import (
	"errors"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var userUsableGroups = map[string]string{
	"default": "默认分组",
	"vip":     "vip分组",
}
var userUsableGroupsMutex sync.RWMutex

func GetUserUsableGroupsCopy() map[string]string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	copyUserUsableGroups := make(map[string]string)
	for k, v := range userUsableGroups {
		copyUserUsableGroups[k] = v
	}
	return copyUserUsableGroups
}

func UserUsableGroups2JSONString() string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	jsonBytes, err := common.Marshal(userUsableGroups)
	if err != nil {
		common.SysLog("error marshalling user groups: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateUserUsableGroupsByJSONString(jsonStr string) error {
	userUsableGroupsMutex.Lock()
	defer userUsableGroupsMutex.Unlock()

	userUsableGroups = make(map[string]string)
	return common.Unmarshal([]byte(jsonStr), &userUsableGroups)
}

func GetUsableGroupDescription(groupName string) string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	if desc, ok := userUsableGroups[groupName]; ok {
		return desc
	}
	return groupName
}

// hiddenUserGroups 隐藏分组：仅在展示层（可选分组列表、价格页等）对非该分组用户隐藏，
// 不影响中转鉴权与计费；分组用户和管理员仍可见
var hiddenUserGroups = make(map[string]bool)
var hiddenUserGroupsMutex sync.RWMutex

func GetUserGroupHiddenCopy() map[string]bool {
	hiddenUserGroupsMutex.RLock()
	defer hiddenUserGroupsMutex.RUnlock()

	copyHidden := make(map[string]bool, len(hiddenUserGroups))
	for name, hidden := range hiddenUserGroups {
		copyHidden[name] = hidden
	}
	return copyHidden
}

func IsUserGroupHidden(groupName string) bool {
	hiddenUserGroupsMutex.RLock()
	defer hiddenUserGroupsMutex.RUnlock()
	return hiddenUserGroups[groupName]
}

func HiddenUserGroups2JSONString() string {
	hiddenUserGroupsMutex.RLock()
	defer hiddenUserGroupsMutex.RUnlock()

	names := make([]string, 0, len(hiddenUserGroups))
	for name, hidden := range hiddenUserGroups {
		if hidden {
			names = append(names, name)
		}
	}
	jsonBytes, err := common.Marshal(names)
	if err != nil {
		common.SysLog("error marshalling hidden user groups: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateHiddenUserGroupsByJSONString(jsonStr string) error {
	hiddenUserGroupsMutex.Lock()
	defer hiddenUserGroupsMutex.Unlock()

	var names []string
	if err := common.Unmarshal([]byte(jsonStr), &names); err != nil {
		return err
	}
	hiddenUserGroups = make(map[string]bool)
	for _, name := range names {
		hiddenUserGroups[name] = true
	}
	return nil
}

func CheckHiddenUserGroups(jsonStr string) error {
	var names []string
	if err := common.Unmarshal([]byte(jsonStr), &names); err != nil {
		return err
	}
	for _, name := range names {
		if name == "" {
			return errors.New("hidden user group name cannot be empty")
		}
	}
	return nil
}
