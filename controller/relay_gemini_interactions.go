package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// RelayGeminiInteractionState 处理 Interactions API 的 get/cancel/delete。
// 这些端点无模型信息,不能走标准 Distribute 流程,经创建时记录的映射表路由到原渠道原 key
// (interaction 上游状态按 API key 隔离)。
func RelayGeminiInteractionState(c *gin.Context) {
	interactionID := c.Param("id")
	if interactionID == "" {
		abortWithGeminiInteractionError(c, http.StatusNotFound, "interaction not found")
		return
	}

	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	state, found := service.GetGeminiInteractionState(interactionID)
	if !found || state.UserID != userID {
		// 不区分"不存在"与"越权",避免 id 探测
		abortWithGeminiInteractionError(c, http.StatusNotFound, "interaction not found")
		return
	}

	channel, err := model.GetChannelById(state.ChannelID, true)
	if err != nil || channel == nil || channel.Status != common.ChannelStatusEnabled ||
		(channel.Type != constant.ChannelTypeGemini && channel.Type != constant.ChannelTypeGeminiInteractions && channel.Type != constant.ChannelTypeCLIProxyAPI) || !service.IsChannelKeyUsable(channel, state.Key) {
		logger.LogWarn(c, fmt.Sprintf("gemini interaction %s channel %d unavailable: %v", interactionID, state.ChannelID, err))
		abortWithGeminiInteractionError(c, http.StatusNotFound, "interaction not found")
		return
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)
	info := &relaycommon.RelayInfo{
		UserId:          state.UserID,
		TokenId:         state.TokenID,
		UsingGroup:      common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
		UserGroup:       common.GetContextKeyString(c, constant.ContextKeyUserGroup),
		TokenGroup:      common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
		OriginModelName: state.Model,
		RequestURLPath:  c.Request.URL.String(),
		RelayMode:       relayconstant.RelayModeGeminiInteractions,
		RelayFormat:     types.RelayFormatGemini,
		StartTime:       time.Now(),
	}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:       channel.Type,
		ChannelId:         channel.Id,
		ChannelBaseUrl:    channel.GetBaseURL(),
		ChannelIsMultiKey: channel.ChannelInfo.IsMultiKey,
		ApiType:           apiType,
		ApiKey:            state.Key,
	}

	adaptor := &gemini.Adaptor{}
	adaptor.Init(info)

	respAny, err := adaptor.DoRequest(c, info, http.NoBody)
	if err != nil {
		logger.LogError(c, "Do gemini interactions state request failed: "+err.Error())
		abortWithGeminiInteractionError(c, http.StatusBadGateway, "upstream request failed")
		return
	}
	resp, _ := respAny.(*http.Response)
	if resp == nil {
		abortWithGeminiInteractionError(c, http.StatusBadGateway, "upstream request failed")
		return
	}

	if resp.StatusCode != http.StatusOK {
		newAPIError := service.RelayErrorHandler(c.Request.Context(), resp, false)
		service.ResetStatusCode(newAPIError, c.GetString("status_code_mapping"))
		writeGeminiInteractionUpstreamError(c, newAPIError)
		return
	}

	isStream := c.Query("stream") == "true" || strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream")
	settle := func(interaction *dto.GeminiInteraction) {
		settleGeminiInteractionIfNeeded(c, info, state, interaction)
	}

	if isStream {
		info.IsStream = true
		gemini.GeminiInteractionsStateStreamPassthrough(c, info, resp, settle)
		return
	}

	if c.Request.Method == http.MethodDelete {
		// 删除成功,清理本地映射
		_ = gemini.GeminiInteractionsStatePassthrough(c, resp, nil)
		service.DeleteGeminiInteractionState(interactionID)
		return
	}
	_ = gemini.GeminiInteractionsStatePassthrough(c, resp, settle)
}

// settleGeminiInteractionIfNeeded background 创建的 interaction 在首个终态读取时结算一次
func settleGeminiInteractionIfNeeded(c *gin.Context, info *relaycommon.RelayInfo, state *service.GeminiInteractionState, interaction *dto.GeminiInteraction) {
	if state == nil || state.Billed || interaction == nil || interaction.ID == "" {
		return
	}
	if interaction.Usage == nil || !interaction.Usage.HasTokens() {
		return
	}
	if !dto.IsGeminiInteractionTerminal(interaction.Status) {
		return
	}
	if !service.ClaimGeminiInteractionBilling(interaction.ID) {
		return
	}
	usage := interaction.Usage.ToUsage(0)
	if usage == nil || usage.TotalTokens == 0 {
		return
	}
	if _, err := helper.ModelPriceHelper(c, info, 0, &types.TokenCountMeta{}); err != nil {
		logger.LogError(c, fmt.Sprintf("gemini interaction %s settle price failed: %s", interaction.ID, err.Error()))
		return
	}
	service.PostTextConsumeQuota(c, info, usage, nil)
	logger.LogInfo(c, fmt.Sprintf("gemini interaction %s settled on terminal read, tokens: %d", interaction.ID, usage.TotalTokens))
}

// abortWithGeminiInteractionError 按 Google API 错误格式返回
func abortWithGeminiInteractionError(c *gin.Context, code int, message string) {
	status := http.StatusText(code)
	status = strings.ToUpper(strings.ReplaceAll(status, " ", "_"))
	c.JSON(code, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
			"status":  status,
		},
	})
	c.Abort()
}

// writeGeminiInteractionUpstreamError 上游错误按原状态码与 Gemini 错误格式回写
func writeGeminiInteractionUpstreamError(c *gin.Context, newAPIError *types.NewAPIError) {
	c.JSON(newAPIError.StatusCode, gin.H{
		"error": gin.H{
			"code":    newAPIError.StatusCode,
			"message": newAPIError.Error(),
			"status":  strings.ToUpper(strings.ReplaceAll(http.StatusText(newAPIError.StatusCode), " ", "_")),
		},
	})
}
