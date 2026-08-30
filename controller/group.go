package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// isPrivilegedViewer 管理员不受隐藏分组的展示过滤
func isPrivilegedViewer(c *gin.Context) bool {
	return c.GetInt("role") >= common.RoleAdminUser
}

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	userId := c.GetInt("id")
	userGroup, _ := model.GetUserGroup(userId, false)
	userUsableGroups := buildUserGroupsResponse(userGroup, isPrivilegedViewer(c))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    userUsableGroups,
	})
}

func AdminGetUserGroups(c *gin.Context) {
	user, ok := getAdminTokenTargetUser(c)
	if !ok {
		return
	}
	userUsableGroups := buildUserGroupsResponse(user.Group, true)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    userUsableGroups,
	})
}

func buildUserGroupsResponse(userGroup string, includeHidden bool) map[string]map[string]interface{} {
	usableGroups := make(map[string]map[string]interface{})
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	if !includeHidden {
		userUsableGroups = service.FilterHiddenGroupsForDisplay(userUsableGroups, userGroup)
	}
	for groupName, _ := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]interface{}{
				"ratio": service.GetUserGroupRatio(userGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	return usableGroups
}
