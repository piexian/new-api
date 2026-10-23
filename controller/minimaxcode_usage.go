package controller

// MiniMax Code 渠道用量聚合与账户操作：套餐余量、积分、重置卡、签到。

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type miniMaxCodeChannelContext struct {
	channel  *model.Channel
	oauthKey *service.MiniMaxCodeOAuthKey
	client   *service.MiniMaxCodeAccountClient
	ctx      context.Context
	cancel   context.CancelFunc
}

func loadMiniMaxCodeChannelContext(c *gin.Context, timeout time.Duration) (*miniMaxCodeChannelContext, bool) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgChannelIdFormatError)
		return nil, false
	}
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": i18n.T(c, i18n.MsgChannelNotExists)})
		return nil, false
	}
	if ch.Type != constant.ChannelTypeMiniMaxCode {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": i18n.T(c, i18n.MsgChannelTypeNotMatched)})
		return nil, false
	}
	if ch.ChannelInfo.IsMultiKey {
		common.ApiErrorI18n(c, i18n.MsgChannelMultiKeyUnsupported)
		return nil, false
	}
	oauthKey, err := service.ParseMiniMaxCodeOAuthKey(ch.Key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道密钥不是有效的 MiniMax OAuth 凭证，请重新授权"})
		return nil, false
	}
	client, err := service.NewMiniMaxCodeAccountClient(ch.GetSetting().Proxy, oauthKey.Region, oauthKey.AccessToken, oauthKey.UserID)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	return &miniMaxCodeChannelContext{
		channel:  ch,
		oauthKey: oauthKey,
		client:   client,
		ctx:      ctx,
		cancel:   cancel,
	}, true
}

// GetMiniMaxCodeChannelUsage 聚合套餐余量、积分、重置卡与签到状态。
// 上游账户接口的原始响应可能夹带凭据，不透传给前端，只返回白名单字段（summary 与余量窗口）。
func GetMiniMaxCodeChannelUsage(c *gin.Context) {
	mctx, ok := loadMiniMaxCodeChannelContext(c, 20*time.Second)
	if !ok {
		return
	}
	defer mctx.cancel()

	// 身份接口同时提供积分余额；分组缺失时也从这里补齐。不回写渠道密钥：
	// 这里读到的密钥可能已被后台刷新轮换，整体写回会覆盖新的 refresh_token。
	groupID := mctx.oauthKey.GroupID
	creditBalance := ""
	if identity := fetchMiniMaxCodeIdentityQuiet(mctx.ctx, mctx.channel.GetSetting().Proxy, mctx.oauthKey.Region, mctx.oauthKey.AccessToken, mctx.oauthKey.UserID); identity != nil {
		groupID = firstNonEmptyController(groupID, identity.GroupID)
		creditBalance = identity.CreditBalance
	}

	account := gin.H{
		"region":     mctx.oauthKey.Region,
		"user_name":  mctx.oauthKey.UserName,
		"email":      mctx.oauthKey.Email,
		"group_id":   groupID,
		"expires_at": mctx.oauthKey.Expired,
	}

	var remains, credit, resetCards, signin, creditDetails any
	var remainsOK, creditOK, resetCardsOK, signinOK, creditDetailsOK bool
	var remainsMsg, creditMsg, resetCardsMsg, signinMsg, creditDetailsMsg string

	if groupID == "" {
		remainsMsg = "缺少套餐分组（op_group_id），无法查询余量"
	} else {
		err := mctx.client.FetchMiniMaxCodeRemains(mctx.ctx, groupID, &remains)
		remainsOK, remainsMsg = miniMaxCodeSectionResult(err)
	}

	err := mctx.client.CreditPool(mctx.ctx, &credit)
	creditOK, creditMsg = miniMaxCodeSectionResult(err)

	err = mctx.client.ResetCardList(mctx.ctx, &resetCards)
	resetCardsOK, resetCardsMsg = miniMaxCodeSectionResult(err)

	err = mctx.client.SigninStatus(mctx.ctx, &signin)
	signinOK, signinMsg = miniMaxCodeSectionResult(err)

	err = mctx.client.CreditDetails(mctx.ctx, &creditDetails)
	creditDetailsOK, creditDetailsMsg = miniMaxCodeSectionResult(err)

	success := remainsOK || creditOK || resetCardsOK
	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"message": "",
		"data": gin.H{
			"account":        account,
			"summary":        buildMiniMaxCodeUsageSummary(creditBalance, credit, creditDetails, resetCards, signin),
			"remains":        gin.H{"success": remainsOK, "message": remainsMsg, "data": miniMaxCodeRemainsView(remains)},
			"credit":         gin.H{"success": creditOK, "message": creditMsg},
			"reset_cards":    gin.H{"success": resetCardsOK, "message": resetCardsMsg},
			"signin":         gin.H{"success": signinOK, "message": signinMsg},
			"credit_details": gin.H{"success": creditDetailsOK, "message": creditDetailsMsg},
		},
	})
}

// buildMiniMaxCodeUsageSummary 按官方客户端读取的字段提取展示数据，缺失字段保持为空。
func buildMiniMaxCodeUsageSummary(creditBalance string, credit, creditDetails, resetCards, signin any) gin.H {
	summary := gin.H{
		"credit_balance":          nil,
		"credit_fallback_enabled": nil,
		"credit_details":          []gin.H{},
		"reset_cards":             []gin.H{},
		"signin_days":             []gin.H{},
	}
	if creditBalance != "" {
		summary["credit_balance"] = creditBalance
	}
	if pool, ok := credit.(map[string]any); ok {
		if enabled, ok := pool["token_plan_credit_fallback_enabled"].(bool); ok {
			summary["credit_fallback_enabled"] = enabled
		}
	}
	if body, ok := creditDetails.(map[string]any); ok {
		details := make([]gin.H, 0)
		for _, item := range miniMaxCodeRecordList(body["details"]) {
			details = append(details, miniMaxCodePick(item, "credit_type", "granted_amount", "remaining_amount", "consumed_amount", "granted_at_ms", "expire_at_ms"))
		}
		summary["credit_details"] = details
	}
	if body, ok := resetCards.(map[string]any); ok {
		cards := make([]gin.H, 0)
		for _, item := range miniMaxCodeRecordList(body["cards"]) {
			cards = append(cards, miniMaxCodePick(item, "card_id", "card_name", "name", "source_type", "out_biz_no", "expire_time", "create_time"))
		}
		summary["reset_cards"] = cards
	}
	if body, ok := signin.(map[string]any); ok {
		days := make([]gin.H, 0)
		for _, item := range miniMaxCodeRecordList(body["days"]) {
			days = append(days, miniMaxCodePick(item, "day_no", "points", "bonus_points", "is_today", "status"))
		}
		summary["signin_days"] = days
	}
	return summary
}

// miniMaxCodeRemainsView 只保留余量窗口与业务状态，供前端复用 Token Plan 的窗口解析。
func miniMaxCodeRemainsView(remains any) gin.H {
	body, ok := remains.(map[string]any)
	if !ok {
		return nil
	}
	view := gin.H{}
	if items, ok := body["model_remains"]; ok {
		view["model_remains"] = items
	}
	if baseResp, ok := body["base_resp"]; ok {
		view["base_resp"] = baseResp
	}
	return view
}

func miniMaxCodeRecordList(value any) []map[string]any {
	items, _ := value.([]any)
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if record, ok := item.(map[string]any); ok {
			records = append(records, record)
		}
	}
	return records
}

func miniMaxCodePick(record map[string]any, keys ...string) gin.H {
	picked := gin.H{}
	for _, key := range keys {
		if value, ok := record[key]; ok && value != nil {
			picked[key] = value
		}
	}
	return picked
}

func miniMaxCodeSectionResult(err error) (bool, string) {
	if err != nil {
		return false, err.Error()
	}
	return true, ""
}

func firstNonEmptyController(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// UseMiniMaxCodeResetCard 使用重置卡。
func UseMiniMaxCodeResetCard(c *gin.Context) {
	var request struct {
		SourceType any    `json:"source_type"`
		OutBizNo   string `json:"out_biz_no"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.SourceType == nil || request.SourceType == "" || request.OutBizNo == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数错误，需要 source_type 与 out_biz_no"})
		return
	}
	mctx, ok := loadMiniMaxCodeChannelContext(c, 20*time.Second)
	if !ok {
		return
	}
	defer mctx.cancel()

	var result any
	err := mctx.client.ResetCardUse(mctx.ctx, request.SourceType, request.OutBizNo, &result)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "重置卡使用失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

// UpdateMiniMaxCodeCreditFallback 开关积分兜底。
func UpdateMiniMaxCodeCreditFallback(c *gin.Context) {
	var request struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Enabled == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数错误，需要 enabled 布尔值"})
		return
	}
	mctx, ok := loadMiniMaxCodeChannelContext(c, 20*time.Second)
	if !ok {
		return
	}
	defer mctx.cancel()

	var result any
	err := mctx.client.CreditUpdateFallback(mctx.ctx, *request.Enabled, &result)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "更新积分兜底失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"enabled": *request.Enabled}})
}

// ClaimMiniMaxCodeSignin 手动签到（不做自动签到）。
func ClaimMiniMaxCodeSignin(c *gin.Context) {
	mctx, ok := loadMiniMaxCodeChannelContext(c, 20*time.Second)
	if !ok {
		return
	}
	defer mctx.cancel()

	var result any
	err := mctx.client.SigninClaim(mctx.ctx, &result)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "签到失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

// resolveMiniMaxCodeTokenPlanCooldownUntil 拉取 MiniMax Code 套餐余量并计算冷却截止。
// 积分兜底开启时不进入冷却。
func resolveMiniMaxCodeTokenPlanCooldownUntil(ctx context.Context, ch *model.Channel, modelName string, now time.Time) (until int64, detail string, ok bool, err error) {
	oauthKey, err := service.ParseMiniMaxCodeOAuthKey(ch.Key)
	if err != nil {
		return 0, "", false, err
	}
	client, err := service.NewMiniMaxCodeAccountClient(ch.GetSetting().Proxy, oauthKey.Region, oauthKey.AccessToken, oauthKey.UserID)
	if err != nil {
		return 0, "", false, err
	}

	if service.MiniMaxCodeCreditFallbackEnabled(ctx, client) {
		return 0, "", false, nil
	}

	groupID := oauthKey.GroupID
	if groupID == "" {
		identity := fetchMiniMaxCodeIdentityQuiet(ctx, ch.GetSetting().Proxy, oauthKey.Region, oauthKey.AccessToken, oauthKey.UserID)
		if identity == nil || identity.GroupID == "" {
			return 0, "", false, fmt.Errorf("minimax code group id is unavailable")
		}
		groupID = identity.GroupID
	}

	var remains any
	if err := client.FetchMiniMaxCodeRemains(ctx, groupID, &remains); err != nil {
		return 0, "", false, err
	}
	body, err := common.Marshal(remains)
	if err != nil {
		return 0, "", false, err
	}
	return miniMaxTokenPlanCooldownUntil(body, now, modelName)
}
