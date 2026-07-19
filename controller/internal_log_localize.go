package controller

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func localizeTaskDTOs(tasks []*dto.TaskDto, language string) {
	for _, task := range tasks {
		if task != nil {
			task.FailReason = model.LocalizeInternalLogText(task.FailReason, language)
		}
	}
}

func localizeMidjourneyLogs(tasks []*model.Midjourney, language string) {
	for _, task := range tasks {
		if task != nil {
			task.FailReason = model.LocalizeInternalLogText(task.FailReason, language)
		}
	}
}

func localizeEmailLogs(logs []*model.EmailLog, language string) {
	for _, log := range logs {
		if log != nil {
			log.ErrorMessage = model.LocalizeInternalLogText(log.ErrorMessage, language)
		}
	}
}

func localizedSystemTaskResponse(task *model.SystemTask, language string) model.SystemTaskResponse {
	response := task.ToResponse()
	response.Error = model.LocalizeInternalLogText(response.Error, language)
	return response
}
