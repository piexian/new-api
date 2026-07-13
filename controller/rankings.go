package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetRankings(c *gin.Context) {
	period := c.DefaultQuery("period", "week")
	result, err := service.GetRankingsSnapshot(period)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if userID := c.GetInt("id"); userID > 0 {
		result = service.AttachRankingsViewer(result, userID, c.GetString("username"))
	} else {
		result = service.PublicRankingsSnapshot(result)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
