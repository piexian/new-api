package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// 词元贷 AI 业务员的模型调用：操练场同款——构造不落库的临时系统令牌
// （Name=loan-officer，模型限制内嵌），经 /pg 路径走完整 Relay 管道。
// 零倍率由代码级上下文标记强制（HandleGroupRatio 短路），quota 恒为 0，
// 但每次调用都在消费日志留档（用户/模型/token 数/渠道归属可查，盗刷可追责）。
// service 包被 relay 引用无法反向 import，故实现放在这里并通过 init 接线。

func init() {
	service.RegisterLoanOfficerModelCaller(callLoanOfficerUpstream)
}

// callLoanOfficerUpstream 经 Relay 管道调用上游模型并返回 assistant 文本（非 stream）
func callLoanOfficerUpstream(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
	request := &dto.GeneralOpenAIRequest{
		Model:    modelName,
		Messages: messages,
	}
	if maxOutputTokens > 0 {
		request.MaxTokens = lo.ToPtr(uint(maxOutputTokens))
	}
	body, err := common.Marshal(request)
	if err != nil {
		return "", err
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// /pg 前缀触发 IsPlayground：临时令牌无 DB 行，跳过令牌额度扣减（消费日志照留）
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// 用户上下文（acceptUnsetRatio 等），与操练场一致
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		return "", err
	}
	userCache.WriteContext(c)

	userGroup, _ := model.GetUserGroup(userId, false)
	// 临时系统令牌：不落库、无 key 泄露面；模型限制写死在令牌里
	tempToken := &model.Token{
		UserId:             userId,
		Name:               "loan-officer",
		Group:              userGroup,
		ModelLimitsEnabled: true,
		ModelLimits:        modelName,
	}
	if err := middleware.SetupContextForToken(c, tempToken); err != nil {
		return "", err
	}
	// 代码级零倍率：AI 业务员调用永不计费
	common.SetContextKey(c, constant.ContextKeyForceZeroGroupRatio, true)

	Relay(c, types.RelayFormatOpenAI)

	if w.Code != http.StatusOK {
		// 上游错误细节（含可能的响应体）只进服务端日志，对外返回通用错误
		common.SysError(fmt.Sprintf("loan officer relay failed: user_id=%d model=%s status=%d body=%s",
			userId, modelName, w.Code, common.LocalLogPreview(w.Body.String())))
		return "", errors.New("loan officer upstream request failed")
	}

	var textResp dto.OpenAITextResponse
	if err := common.Unmarshal(w.Body.Bytes(), &textResp); err != nil {
		return "", err
	}
	if len(textResp.Choices) == 0 {
		return "", errors.New("loan officer upstream returned no choices")
	}
	content := textResp.Choices[0].Message.StringContent()
	if strings.TrimSpace(content) == "" {
		return "", errors.New("loan officer upstream returned empty content")
	}
	return content, nil
}
