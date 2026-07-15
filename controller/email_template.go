package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type emailTemplateUpdateRequest struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type emailTemplatePreviewRequest struct {
	Event   string `json:"event"`
	Locale  string `json:"locale"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

func GetEmailTemplateCatalog(c *gin.Context) {
	common.ApiSuccess(c, service.GetEmailTemplateCatalog())
}

func GetEmailTemplate(c *gin.Context) {
	template, err := service.GetEmailTemplate(c.Param("event"), c.Param("locale"))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, template)
}

func UpdateEmailTemplate(c *gin.Context) {
	var request emailTemplateUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	event := strings.TrimSpace(c.Param("event"))
	locale := strings.TrimSpace(c.Param("locale"))
	template, err := service.SaveEmailTemplate(event, locale, request.Subject, request.HTML)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAudit(c, "email.template_update", map[string]interface{}{
		"event":  event,
		"locale": locale,
	})
	common.ApiSuccess(c, template)
}

func RestoreEmailTemplate(c *gin.Context) {
	event := strings.TrimSpace(c.Param("event"))
	locale := strings.TrimSpace(c.Param("locale"))
	template, err := service.RestoreDefaultEmailTemplate(event, locale)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAudit(c, "email.template_restore", map[string]interface{}{
		"event":  event,
		"locale": locale,
	})
	common.ApiSuccess(c, template)
}

func PreviewEmailTemplate(c *gin.Context) {
	var request emailTemplatePreviewRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	preview, err := service.PreviewEmailTemplate(request.Event, request.Locale, request.Subject, request.HTML)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, preview)
}
