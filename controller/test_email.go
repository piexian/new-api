package controller

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type testEmailRequest struct {
	Receiver string `json:"receiver"`
}

func TestEmailDelivery(c *gin.Context) {
	var request testEmailRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	receiver := strings.TrimSpace(request.Receiver)
	if err := common.Validate.Var(receiver, "required,email"); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	sentAt := time.Now()
	if err := service.SendTemplatedEmail(
		service.EmailTemplateEventSystemTest,
		i18n.GetLangFromContext(c),
		receiver,
		map[string]string{
			"provider": strings.ToUpper(common.EmailProvider),
			"sent_at":  sentAt.Format(time.RFC3339Nano),
		},
	); err != nil {
		common.ApiError(c, err)
		return
	}

	recordManageAudit(c, "email.test", map[string]interface{}{
		"provider": common.EmailProvider,
	})
	common.ApiSuccess(c, nil)
}
