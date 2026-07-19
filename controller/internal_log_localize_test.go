package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
)

func TestLocalizeInternalLogResponses(t *testing.T) {
	taskDTOs := []*dto.TaskDto{{FailReason: "任务超时（30分钟）"}}
	localizeTaskDTOs(taskDTOs, model.LogLanguageEN)
	assert.Equal(t, "Task timed out (30 minutes)", taskDTOs[0].FailReason)

	midjourneyLogs := []*model.Midjourney{{FailReason: "获取渠道信息失败，请联系管理员，渠道ID：8"}}
	localizeMidjourneyLogs(midjourneyLogs, model.LogLanguageEN)
	assert.Equal(t, "Failed to get channel information; contact an administrator. Channel ID: 8", midjourneyLogs[0].FailReason)

	emailLogs := []*model.EmailLog{
		{ErrorMessage: "SMTP server not configured"},
		{ErrorMessage: "Cloudflare API error: provider detail"},
	}
	localizeEmailLogs(emailLogs, model.LogLanguageZH)
	assert.Equal(t, "未配置 SMTP 服务器", emailLogs[0].ErrorMessage)
	assert.Equal(t, "Cloudflare API error: provider detail", emailLogs[1].ErrorMessage)

	systemTask := &model.SystemTask{Error: "task lease expired"}
	response := localizedSystemTaskResponse(systemTask, model.LogLanguageZH)
	assert.Equal(t, "系统任务租约已过期", response.Error)
}
