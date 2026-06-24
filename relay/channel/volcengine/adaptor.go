package volcengine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

const (
	contextKeyTTSRequest              = "volcengine_tts_request"
	contextKeyResponseFormat          = "response_format"
	contextKeyTTSOpenSpeechV3         = "volcengine_tts_openspeech_v3"
	contextKeyTTSOpenSpeechV3Resource = "volcengine_tts_openspeech_v3_resource_id"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	if shouldUseVolcengineClaudeMessagesEndpoint(info) {
		adaptor := claude.Adaptor{}
		return adaptor.ConvertClaudeRequest(c, info, req)
	}
	adaptor := openai.Adaptor{}
	return adaptor.ConvertClaudeRequest(c, info, req)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != constant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}

	baseURL := info.ChannelBaseUrl
	if baseURL == "" {
		baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeVolcEngine]
	}
	if isOpenSpeechTTSV3Base(baseURL) {
		encoding := mapEncoding(request.ResponseFormat)
		c.Set(contextKeyResponseFormat, encoding)
		c.Set(contextKeyTTSOpenSpeechV3, true)
		c.Set(contextKeyTTSOpenSpeechV3Resource, parseTTSV3ResourceID(request.Metadata))

		info.IsStream = false
		jsonData, buildErr := buildTTSV3RequestBody(request.Input, mapVoiceType(request.Voice), encoding, request.Speed)
		if buildErr != nil {
			return nil, fmt.Errorf("error marshalling openspeech v3 request: %w", buildErr)
		}
		return bytes.NewReader(jsonData), nil
	}

	appID, token, err := parseVolcengineAuth(info.ApiKey)
	if err != nil {
		return nil, err
	}

	voiceType := mapVoiceType(request.Voice)
	speedRatio := lo.FromPtrOr(request.Speed, 0.0)
	encoding := mapEncoding(request.ResponseFormat)

	c.Set(contextKeyResponseFormat, encoding)

	volcRequest := VolcengineTTSRequest{
		App: VolcengineTTSApp{
			AppID:   appID,
			Token:   token,
			Cluster: "volcano_tts",
		},
		User: VolcengineTTSUser{
			UID: "openai_relay_user",
		},
		Audio: VolcengineTTSAudio{
			VoiceType:  voiceType,
			Encoding:   encoding,
			SpeedRatio: speedRatio,
			Rate:       24000,
		},
		Request: VolcengineTTSReqInfo{
			ReqID:     generateRequestID(),
			Text:      request.Input,
			Operation: "submit",
			Model:     info.OriginModelName,
		},
	}

	if len(request.Metadata) > 0 {
		if err = common.Unmarshal(request.Metadata, &volcRequest); err != nil {
			return nil, fmt.Errorf("error unmarshalling metadata to volcengine request: %w", err)
		}
	}

	c.Set(contextKeyTTSRequest, volcRequest)

	if volcRequest.Request.Operation == "submit" {
		info.IsStream = true
	}

	jsonData, err := common.Marshal(volcRequest)
	if err != nil {
		return nil, fmt.Errorf("error marshalling volcengine request: %w", err)
	}

	return bytes.NewReader(jsonData), nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	switch info.RelayMode {
	case constant.RelayModeImagesGenerations:
		return request, nil
	// 根据官方文档,并没有发现豆包生图支持表单请求:https://www.volcengine.com/docs/82379/1824121
	//case constant.RelayModeImagesEdits:
	//
	//	var requestBody bytes.Buffer
	//	writer := multipart.NewWriter(&requestBody)
	//
	//	writer.WriteField("model", request.Model)
	//
	//	formData := c.Request.PostForm
	//	for key, values := range formData {
	//		if key == "model" {
	//			continue
	//		}
	//		for _, value := range values {
	//			writer.WriteField(key, value)
	//		}
	//	}
	//
	//	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
	//		return nil, errors.New("failed to parse multipart form")
	//	}
	//
	//	if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil {
	//		var imageFiles []*multipart.FileHeader
	//		var exists bool
	//
	//		if imageFiles, exists = c.Request.MultipartForm.File["image"]; !exists || len(imageFiles) == 0 {
	//			if imageFiles, exists = c.Request.MultipartForm.File["image[]"]; !exists || len(imageFiles) == 0 {
	//				foundArrayImages := false
	//				for fieldName, files := range c.Request.MultipartForm.File {
	//					if strings.HasPrefix(fieldName, "image[") && len(files) > 0 {
	//						foundArrayImages = true
	//						for _, file := range files {
	//							imageFiles = append(imageFiles, file)
	//						}
	//					}
	//				}
	//
	//				if !foundArrayImages && (len(imageFiles) == 0) {
	//					return nil, errors.New("image is required")
	//				}
	//			}
	//		}
	//
	//		for i, fileHeader := range imageFiles {
	//			file, err := fileHeader.Open()
	//			if err != nil {
	//				return nil, fmt.Errorf("failed to open image file %d: %w", i, err)
	//			}
	//			defer file.Close()
	//
	//			fieldName := "image"
	//			if len(imageFiles) > 1 {
	//				fieldName = "image[]"
	//			}
	//
	//			mimeType := detectImageMimeType(fileHeader.Filename)
	//
	//			h := make(textproto.MIMEHeader)
	//			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileHeader.Filename))
	//			h.Set("Content-Type", mimeType)
	//
	//			part, err := writer.CreatePart(h)
	//			if err != nil {
	//				return nil, fmt.Errorf("create form part failed for image %d: %w", i, err)
	//			}
	//
	//			if _, err := io.Copy(part, file); err != nil {
	//				return nil, fmt.Errorf("copy file failed for image %d: %w", i, err)
	//			}
	//		}
	//
	//		if maskFiles, exists := c.Request.MultipartForm.File["mask"]; exists && len(maskFiles) > 0 {
	//			maskFile, err := maskFiles[0].Open()
	//			if err != nil {
	//				return nil, errors.New("failed to open mask file")
	//			}
	//			defer maskFile.Close()
	//
	//			mimeType := detectImageMimeType(maskFiles[0].Filename)
	//
	//			h := make(textproto.MIMEHeader)
	//			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="mask"; filename="%s"`, maskFiles[0].Filename))
	//			h.Set("Content-Type", mimeType)
	//
	//			maskPart, err := writer.CreatePart(h)
	//			if err != nil {
	//				return nil, errors.New("create form file failed for mask")
	//			}
	//
	//			if _, err := io.Copy(maskPart, maskFile); err != nil {
	//				return nil, errors.New("copy mask file failed")
	//			}
	//		}
	//	} else {
	//		return nil, errors.New("no multipart form data found")
	//	}
	//
	//	writer.Close()
	//	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	//	return bytes.NewReader(requestBody.Bytes()), nil

	default:
		return request, nil
	}
}

func detectImageMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		if strings.HasPrefix(ext, ".jp") {
			return "image/jpeg"
		}
		return "image/png"
	}
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func volcengineV3BaseURL(baseUrl string) string {
	baseUrl = strings.TrimRight(baseUrl, "/")
	if strings.HasSuffix(baseUrl, "/api/v3") ||
		strings.HasSuffix(baseUrl, "/api/plan/v3") ||
		strings.HasSuffix(baseUrl, "/api/coding/v3") {
		return baseUrl
	}
	return fmt.Sprintf("%s/api/v3", baseUrl)
}

func volcengineOpenAIBaseURL(baseUrl string, specialPlan channelconstant.ChannelSpecialBase, hasSpecialPlan bool) string {
	if hasSpecialPlan && specialPlan.OpenAIBaseURL != "" {
		return strings.TrimRight(specialPlan.OpenAIBaseURL, "/")
	}
	return volcengineV3BaseURL(baseUrl)
}

func volcengineClaudeBaseURL(baseUrl string, specialPlan channelconstant.ChannelSpecialBase, hasSpecialPlan bool) string {
	if hasSpecialPlan && specialPlan.ClaudeBaseURL != "" {
		return strings.TrimRight(specialPlan.ClaudeBaseURL, "/")
	}

	baseUrl = strings.TrimRight(baseUrl, "/")
	lowerBaseURL := strings.ToLower(baseUrl)
	switch {
	case strings.HasSuffix(lowerBaseURL, "/api/compatible/v1"),
		strings.HasSuffix(lowerBaseURL, "/api/plan/v1"),
		strings.HasSuffix(lowerBaseURL, "/api/coding/v1"):
		return strings.TrimRight(baseUrl[:len(baseUrl)-len("/v1")], "/")
	case strings.HasSuffix(lowerBaseURL, "/api/compatible"),
		strings.HasSuffix(lowerBaseURL, "/api/plan"),
		strings.HasSuffix(lowerBaseURL, "/api/coding"):
		return baseUrl
	case strings.HasSuffix(lowerBaseURL, "/api/plan/v3"),
		strings.HasSuffix(lowerBaseURL, "/api/coding/v3"):
		return strings.TrimRight(baseUrl[:len(baseUrl)-len("/v3")], "/")
	case strings.HasSuffix(lowerBaseURL, "/api/v3"):
		return strings.TrimRight(baseUrl[:len(baseUrl)-len("/api/v3")], "/") + "/api/compatible"
	default:
		return baseUrl + "/api/compatible"
	}
}

func shouldUseVolcengineClaudeMessagesEndpoint(info *relaycommon.RelayInfo) bool {
	if info == nil || info.RelayFormat != types.RelayFormatClaude {
		return false
	}
	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	if _, ok := channelconstant.ChannelSpecialBases[baseURL]; ok {
		return true
	}
	return !strings.HasPrefix(info.UpstreamModelName, "bot")
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseUrl := info.ChannelBaseUrl
	if baseUrl == "" {
		baseUrl = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeVolcEngine]
	}
	baseUrl = strings.TrimRight(baseUrl, "/")
	specialPlan, hasSpecialPlan := channelconstant.ChannelSpecialBases[baseUrl]

	switch info.RelayFormat {
	case types.RelayFormatClaude:
		if shouldUseVolcengineClaudeMessagesEndpoint(info) {
			return fmt.Sprintf("%s/v1/messages", volcengineClaudeBaseURL(baseUrl, specialPlan, hasSpecialPlan)), nil
		}
		if strings.HasPrefix(info.UpstreamModelName, "bot") {
			return fmt.Sprintf("%s/bots/chat/completions", volcengineV3BaseURL(baseUrl)), nil
		}
		return fmt.Sprintf("%s/chat/completions", volcengineV3BaseURL(baseUrl)), nil
	default:
		switch info.RelayMode {
		case constant.RelayModeChatCompletions:
			openAIBaseURL := volcengineOpenAIBaseURL(baseUrl, specialPlan, hasSpecialPlan)
			if hasSpecialPlan && specialPlan.OpenAIBaseURL != "" {
				return fmt.Sprintf("%s/chat/completions", openAIBaseURL), nil
			}
			if strings.HasPrefix(info.UpstreamModelName, "bot") {
				return fmt.Sprintf("%s/bots/chat/completions", openAIBaseURL), nil
			}
			return fmt.Sprintf("%s/chat/completions", openAIBaseURL), nil
		case constant.RelayModeEmbeddings:
			return fmt.Sprintf("%s/embeddings", volcengineOpenAIBaseURL(baseUrl, specialPlan, hasSpecialPlan)), nil
		//豆包的图生图也走generations接口: https://www.volcengine.com/docs/82379/1824121
		case constant.RelayModeImagesGenerations, constant.RelayModeImagesEdits:
			return fmt.Sprintf("%s/images/generations", volcengineOpenAIBaseURL(baseUrl, specialPlan, hasSpecialPlan)), nil
		//case constant.RelayModeImagesEdits:
		//	return fmt.Sprintf("%s/api/v3/images/edits", baseUrl), nil
		case constant.RelayModeRerank:
			return fmt.Sprintf("%s/rerank", volcengineOpenAIBaseURL(baseUrl, specialPlan, hasSpecialPlan)), nil
		case constant.RelayModeResponses:
			return fmt.Sprintf("%s/responses", volcengineOpenAIBaseURL(baseUrl, specialPlan, hasSpecialPlan)), nil
		case constant.RelayModeAudioSpeech:
			if isOpenSpeechTTSV3Base(baseUrl) {
				return openSpeechTTSV3RequestURL(baseUrl), nil
			}
			if baseUrl == channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeVolcEngine] {
				return "wss://openspeech.bytedance.com/api/v1/tts/ws_binary", nil
			}
			return fmt.Sprintf("%s/v1/audio/speech", baseUrl), nil
		default:
		}
	}
	return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	if c != nil && c.Request != nil {
		for name := range c.Request.Header {
			if strings.HasPrefix(strings.ToLower(name), "ark-beta-") {
				if value := strings.TrimSpace(c.Request.Header.Get(name)); value != "" {
					req.Set(name, value)
				}
			}
		}
	}

	if shouldUseVolcengineClaudeMessagesEndpoint(info) {
		req.Set("Authorization", "Bearer "+info.ApiKey)
		req.Set("x-api-key", info.ApiKey)
		anthropicVersion := c.Request.Header.Get("anthropic-version")
		if anthropicVersion == "" {
			anthropicVersion = "2023-06-01"
		}
		req.Set("anthropic-version", anthropicVersion)
		claude.CommonClaudeHeadersOperation(c, req, info)
		return nil
	}

	if info.RelayMode == constant.RelayModeAudioSpeech {
		if isOpenSpeechTTSV3Context(c) {
			resourceID := defaultTTSV3ResourceID
			if value, ok := c.Get(contextKeyTTSOpenSpeechV3Resource); ok {
				if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
					resourceID = strings.TrimSpace(s)
				}
			}
			return setupOpenSpeechTTSV3Header(req, info.ApiKey, resourceID)
		}
		parts := strings.Split(info.ApiKey, "|")
		if len(parts) == 2 {
			req.Set("Authorization", "Bearer;"+parts[1])
		}
		req.Set("Content-Type", "application/json")
		return nil
	} else if info.RelayMode == constant.RelayModeImagesEdits {
		req.Set("Content-Type", gin.MIMEJSON)
	}

	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	if !model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) &&
		strings.HasSuffix(info.UpstreamModelName, "-thinking") &&
		strings.HasPrefix(info.UpstreamModelName, "deepseek") {
		info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-thinking")
		request.Model = info.UpstreamModelName
		request.THINKING = json.RawMessage(`{"type": "enabled"}`)
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponses
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info.RelayMode == constant.RelayModeAudioSpeech {
		baseUrl := info.ChannelBaseUrl
		if baseUrl == "" {
			baseUrl = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeVolcEngine]
		}

		if baseUrl == channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeVolcEngine] {
			if info.IsStream {
				return nil, nil
			}
		}
	}
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if shouldUseVolcengineClaudeMessagesEndpoint(info) {
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	}

	if info.RelayMode == constant.RelayModeAudioSpeech {
		encoding := mapEncoding(c.GetString(contextKeyResponseFormat))
		if isOpenSpeechTTSV3Context(c) {
			return handleTTSV3NdjsonResponse(c, resp, info, encoding)
		}
		if info.IsStream {
			volcRequestInterface, exists := c.Get(contextKeyTTSRequest)
			if !exists {
				return nil, types.NewErrorWithStatusCode(
					errors.New("volcengine TTS request not found in context"),
					types.ErrorCodeBadRequestBody,
					http.StatusInternalServerError,
				)
			}

			volcRequest, ok := volcRequestInterface.(VolcengineTTSRequest)
			if !ok {
				return nil, types.NewErrorWithStatusCode(
					errors.New("invalid volcengine TTS request type"),
					types.ErrorCodeBadRequestBody,
					http.StatusInternalServerError,
				)
			}

			// Get the WebSocket URL
			requestURL, urlErr := a.GetRequestURL(info)
			if urlErr != nil {
				return nil, types.NewErrorWithStatusCode(
					urlErr,
					types.ErrorCodeBadRequestBody,
					http.StatusInternalServerError,
				)
			}
			return handleTTSWebSocketResponse(c, requestURL, volcRequest, info, encoding)
		}
		return handleTTSResponse(c, resp, info, encoding)
	}

	adaptor := openai.Adaptor{}
	usage, err = adaptor.DoResponse(c, resp, info)
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
