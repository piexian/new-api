package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// 词元贷 AI 业务员的模型直调：复用渠道测试的 in-process 链路
// （gin.CreateTestContext + relay adaptor 直调，参照 testChannel），
// 不走 HTTP 回环、不计费、不写用户请求日志、不消耗限流。
// service 包被 relay 引用无法反向 import，故实现放在这里并通过 init 接线。

func init() {
	service.RegisterLoanOfficerModelCaller(callLoanOfficerUpstream)
}

// callLoanOfficerUpstream 直调上游模型并返回 assistant 文本（非 stream）
func callLoanOfficerUpstream(modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	channel, err := selectLoanOfficerChannel(c, modelName)
	if err != nil {
		return "", err
	}
	if apiErr := middleware.SetupContextForSelectedChannel(c, channel, modelName); apiErr != nil {
		return "", apiErr
	}

	request := &dto.GeneralOpenAIRequest{
		Model:    modelName,
		Messages: messages,
	}
	if maxOutputTokens > 0 {
		request.MaxTokens = lo.ToPtr(uint(maxOutputTokens))
	}

	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, request, nil)
	if err != nil {
		return "", err
	}
	info.IsChannelTest = true
	info.InitChannelMeta(c)

	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return "", err
	}
	if info.UpstreamModelName != "" {
		request.SetModelName(info.UpstreamModelName)
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		return "", fmt.Errorf("invalid api type: %d, adaptor is nil", apiType)
	}
	adaptor.Init(info)

	convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, request)
	if err != nil {
		return "", err
	}
	var requestBody io.Reader
	if reader, ok := convertedRequest.(io.Reader); ok {
		requestBody = reader
	} else {
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return "", err
		}
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return "", err
			}
		}
		requestBody = bytes.NewBuffer(jsonData)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
	}

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", errors.New("loan officer upstream returned nil response")
	}
	httpResp := resp.(*http.Response)
	if httpResp.StatusCode != http.StatusOK {
		err := service.RelayErrorHandler(c.Request.Context(), httpResp, true)
		common.SysError(fmt.Sprintf("loan officer upstream bad response: channel_id=%d model=%s status=%d err=%v",
			channel.Id, modelName, httpResp.StatusCode, err))
		return "", err
	}
	if _, respErr := adaptor.DoResponse(c, httpResp, info); respErr != nil {
		return "", respErr
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

// selectLoanOfficerChannel 在启用了该模型的所有分组里随机选一个可用渠道
// （ability 分组逐个尝试，组内由 CacheGetRandomSatisfiedChannel 按权重随机）
func selectLoanOfficerChannel(c *gin.Context, modelName string) (*model.Channel, error) {
	groups := make([]string, 0, 4)
	seen := make(map[string]bool)
	for _, ability := range model.GetAllEnableAbilities() {
		if ability.Model == modelName && !seen[ability.Group] {
			seen[ability.Group] = true
			groups = append(groups, ability.Group)
		}
	}
	for _, group := range groups {
		channel, _, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
			Ctx:         c,
			TokenGroup:  group,
			ModelName:   modelName,
			RequestPath: "/v1/chat/completions",
		})
		if err == nil && channel != nil {
			return channel, nil
		}
	}
	return nil, fmt.Errorf("no available channel for loan officer model %s", modelName)
}
