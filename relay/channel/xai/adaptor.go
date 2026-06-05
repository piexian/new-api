package xai

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type Adaptor struct {
	ResponseFormat string
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	//TODO implement me
	//panic("implement me")
	return nil, errors.New("not available")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return a.convertAudioRequest(c, info, request)
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return convertImageRequest(c, info, request)
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if apiErr := ValidateEndpointForModel(info); apiErr != nil {
		return "", apiErr
	}
	requestPath := info.RequestURLPath
	switch info.RelayMode {
	case constant.RelayModeRealtime:
		info.ChannelBaseUrl = xAIWebsocketBaseURL(info.ChannelBaseUrl)
	case constant.RelayModeXAINative:
		if info.RelayFormat == types.RelayFormatXAIRealtime {
			info.ChannelBaseUrl = xAIWebsocketBaseURL(info.ChannelBaseUrl)
		}
	case constant.RelayModeAudioSpeech:
		if strings.HasPrefix(requestPath, "/v1/audio/speech") {
			requestPath = "/v1/tts"
		}
	case constant.RelayModeAudioTranscription, constant.RelayModeAudioTranslation:
		if strings.HasPrefix(requestPath, "/v1/audio/transcriptions") || strings.HasPrefix(requestPath, "/v1/audio/translations") {
			requestPath = "/v1/stt"
		}
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, requestPath, info.ChannelType), nil
}

func xAIWebsocketBaseURL(baseURL string) string {
	switch {
	case strings.HasPrefix(baseURL, "https://"):
		return "wss://" + strings.TrimPrefix(baseURL, "https://")
	case strings.HasPrefix(baseURL, "http://"):
		return "ws://" + strings.TrimPrefix(baseURL, "http://")
	default:
		return baseURL
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	if IsCodexCompatibilityRequest(c, info) {
		if sessionID := strings.TrimSpace(c.GetHeader("Session_id")); sessionID != "" && req.Get(xaiGrokConversationID) == "" {
			req.Set(xaiGrokConversationID, sessionID)
		}
	}
	if info.RelayMode == constant.RelayModeImagesGenerations || info.RelayMode == constant.RelayModeImagesEdits {
		req.Set("Content-Type", "application/json")
	}
	if info.RelayMode == constant.RelayModeRealtime {
		req.Del("openai-beta")
	}
	if info.RelayMode == constant.RelayModeXAINative {
		if c.Request.Header.Get("Content-Type") == "" {
			req.Del("Content-Type")
		}
		if c.Request.Header.Get("Accept") == "" {
			req.Del("Accept")
		}
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	var original map[string]json.RawMessage
	if raw, ok := originalJSONFields(c); ok {
		original = raw
	}
	if strings.HasSuffix(info.UpstreamModelName, "-search") {
		info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-search")
		request.Model = info.UpstreamModelName
		toMap := request.ToMap()
		mergeOriginalJSONFields(toMap, original)
		toMap["search_parameters"] = map[string]any{
			"mode": "on",
		}
		return toMap, nil
	}
	if strings.HasPrefix(request.Model, "grok-3-mini") {
		if lo.FromPtrOr(request.MaxCompletionTokens, uint(0)) == 0 && lo.FromPtrOr(request.MaxTokens, uint(0)) != 0 {
			request.MaxCompletionTokens = request.MaxTokens
			request.MaxTokens = nil
		}
		if strings.HasSuffix(request.Model, "-high") {
			request.ReasoningEffort = "high"
			request.Model = strings.TrimSuffix(request.Model, "-high")
		} else if strings.HasSuffix(request.Model, "-low") {
			request.ReasoningEffort = "low"
			request.Model = strings.TrimSuffix(request.Model, "-low")
		}
		info.ReasoningEffort = request.ReasoningEffort
		info.UpstreamModelName = request.Model
	}
	if len(original) > 0 {
		toMap := request.ToMap()
		mergeOriginalJSONFields(toMap, original)
		toMap["model"] = request.Model
		return toMap, nil
	}
	return request, nil
}

func originalJSONFields(c *gin.Context) (map[string]json.RawMessage, bool) {
	if c == nil || c.Request == nil || !strings.Contains(c.Request.Header.Get("Content-Type"), "application/json") {
		return nil, false
	}
	if c.Request.Body == nil {
		return nil, false
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, false
	}
	data, err := storage.Bytes()
	if err != nil || len(data) == 0 {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(data, &fields); err != nil || len(fields) == 0 {
		return nil, false
	}
	return fields, true
}

func mergeOriginalJSONFields(target map[string]any, original map[string]json.RawMessage) {
	if len(target) == 0 || len(original) == 0 {
		return
	}
	for key, raw := range original {
		if _, exists := target[key]; exists {
			continue
		}
		var value any
		if err := common.Unmarshal(raw, &value); err == nil {
			target[key] = value
		}
	}
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	//not available
	return nil, errors.New("not available")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if request.Model == "" && info != nil {
		request.Model = info.UpstreamModelName
	}
	if info != nil {
		if info.RelayMode == constant.RelayModeResponsesCompact {
			info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponsesCompaction
		} else {
			info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponses
		}
	}
	if IsCodexCompatibilityRequest(c, info) {
		return convertCodexResponsesRequestForXAI(request, info.RelayMode == constant.RelayModeResponsesCompact), nil
	}
	if original, ok := originalJSONFields(c); ok {
		payload := make(map[string]any, len(original))
		for key, raw := range original {
			var value any
			if err := common.Unmarshal(raw, &value); err == nil {
				payload[key] = value
			}
		}
		payload["model"] = request.Model
		return payload, nil
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info.RelayMode == constant.RelayModeRealtime || info.RelayFormat == types.RelayFormatXAIRealtime {
		return channel.DoWssRequest(a, c, info, requestBody)
	}
	if info.RelayMode == constant.RelayModeAudioTranscription || info.RelayMode == constant.RelayModeAudioTranslation {
		return channel.DoFormRequest(a, c, info, requestBody)
	}
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case constant.RelayModeRealtime:
		err, usage = openai.OpenaiRealtimeHandler(c, info)
	case constant.RelayModeAudioSpeech:
		usage = openai.OpenaiTTSHandler(c, resp, info)
	case constant.RelayModeAudioTranslation:
		fallthrough
	case constant.RelayModeAudioTranscription:
		err, usage = openai.OpenaiSTTHandler(c, resp, info, a.ResponseFormat)
	case constant.RelayModeImagesGenerations, constant.RelayModeImagesEdits:
		usage, err = openai.OpenaiHandlerWithUsage(c, info, resp)
	case constant.RelayModeResponses:
		if info.IsStream {
			usage, err = openai.OaiResponsesStreamHandler(c, info, resp)
		} else {
			usage, err = openai.OaiResponsesHandler(c, info, resp)
		}
	case constant.RelayModeResponsesCompact:
		usage, err = openai.OaiResponsesCompactionHandler(c, resp)
	default:
		if info.IsStream {
			usage, err = xAIStreamHandler(c, info, resp)
		} else {
			usage, err = xAIHandler(c, info, resp)
		}
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
