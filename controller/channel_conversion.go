package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func ConvertChannelToMultiKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	channel, err := model.ConvertChannelToMultiKey(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	recordManageAudit(c, "channel.update", map[string]interface{}{
		"id": id, "name": channel.Name, "changed_fields": []string{"channel_info.is_multi_key"},
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"id": id, "channel_info": channel.ChannelInfo},
	})
}
