package dify

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service/responsescompat"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	BotTypeChatFlow   = 1 // chatflow default
	BotTypeAgent      = 2
	BotTypeWorkFlow   = 3
	BotTypeCompletion = 4
)

type Adaptor struct {
	BotType int
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	//if strings.HasPrefix(info.UpstreamModelName, "agent") {
	//	a.BotType = BotTypeAgent
	//} else if strings.HasPrefix(info.UpstreamModelName, "workflow") {
	//	a.BotType = BotTypeWorkFlow
	//} else if strings.HasPrefix(info.UpstreamModelName, "chat") {
	//	a.BotType = BotTypeCompletion
	//} else {
	//}
	a.BotType = BotTypeChatFlow

}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	switch a.BotType {
	case BotTypeWorkFlow:
		return fmt.Sprintf("%s/v1/workflows/run", info.ChannelBaseUrl), nil
	case BotTypeCompletion:
		return fmt.Sprintf("%s/v1/completion-messages", info.ChannelBaseUrl), nil
	case BotTypeAgent:
		fallthrough
	default:
		return fmt.Sprintf("%s/v1/chat-messages", info.ChannelBaseUrl), nil
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return requestOpenAI2Dify(c, info, *request), nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// 渠道无原生 Responses 端点, 自行降级为 Chat Completions
	chatRequest, err := responsescompat.ConvertToOpenAIChatRequest(request)
	if err != nil {
		return nil, err
	}
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatOpenAI
	}
	return requestOpenAI2Dify(c, info, *chatRequest), nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	// Responses 自降级后上游回的是 dify 格式, 转回 Responses 协议输出
	if info.RelayMode == relayconstant.RelayModeResponses && info.GetFinalRequestRelayFormat() == types.RelayFormatOpenAI {
		if info.IsStream {
			return openai.ChatCompletionResponsesStreamHandlerWithDataTransformer(c, info, resp, responsesStreamData2Chat)
		}
		return openai.ChatCompletionResponsesHandlerWithBodyTransformer(c, info, resp, responsesBody2Chat)
	}
	if info.IsStream {
		return difyStreamHandler(c, info, resp)
	} else {
		return difyHandler(c, info, resp)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
