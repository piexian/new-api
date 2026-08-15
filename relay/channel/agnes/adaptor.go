package agnes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	openai.Adaptor
}

// agnes 原生支持 OpenAI/Responses/Anthropic 三种端点，Claude 格式直接透传 /v1/messages
func useClaudeAPI(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	if info.RelayMode == relayconstant.RelayModeClaudeCountTokens {
		return true
	}
	return info.RelayFormat == types.RelayFormatClaude &&
		info.RelayMode != relayconstant.RelayModeResponses &&
		info.RelayMode != relayconstant.RelayModeResponsesCompact
}

type imageRequest struct {
	Model        string          `json:"model"`
	Prompt       string          `json:"prompt"`
	Size         string          `json:"size,omitempty"`
	Image        []string        `json:"image,omitempty"`
	ReturnBase64 json.RawMessage `json:"return_base64,omitempty"`
	ExtraBody    map[string]any  `json:"extra_body,omitempty"`
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	relayMode := relayconstant.RelayModeImagesGenerations
	if info != nil {
		relayMode = info.RelayMode
	}
	switch relayMode {
	case relayconstant.RelayModeImagesGenerations:
		return convertImageRequest(info, request, false)
	case relayconstant.RelayModeImagesEdits:
		return convertImageRequest(info, request, true)
	default:
		return a.Adaptor.ConvertImageRequest(c, info, request)
	}
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if useClaudeAPI(info) {
		baseURL := info.ChannelBaseUrl
		if baseURL == "" {
			baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeAgnesAI]
		}
		path := "/v1/messages"
		if info.RelayMode == relayconstant.RelayModeClaudeCountTokens {
			path = "/v1/messages/count_tokens"
		}
		return relaycommon.GetFullRequestURL(baseURL, path, info.ChannelType), nil
	}
	if info != nil && info.RelayMode == relayconstant.RelayModeImagesEdits {
		baseURL := info.ChannelBaseUrl
		if baseURL == "" {
			baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeAgnesAI]
		}
		return relaycommon.GetFullRequestURL(baseURL, "/v1/images/generations", info.ChannelType), nil
	}
	return a.Adaptor.GetRequestURL(info)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if useClaudeAPI(info) {
		adaptor := claude.Adaptor{}
		return adaptor.ConvertClaudeRequest(c, info, request)
	}
	return a.Adaptor.ConvertClaudeRequest(c, info, request)
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	if useClaudeAPI(info) {
		channel.SetupApiRequestHeader(info, c, header)
		header.Set("x-api-key", info.ApiKey)
		anthropicVersion := c.Request.Header.Get("anthropic-version")
		if anthropicVersion == "" {
			anthropicVersion = "2023-06-01"
		}
		header.Set("anthropic-version", anthropicVersion)
		claude.CommonClaudeHeadersOperation(c, header, info)
		return nil
	}
	if err := a.Adaptor.SetupRequestHeader(c, header, info); err != nil {
		return err
	}
	if info == nil {
		return nil
	}
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		header.Set("Content-Type", gin.MIMEJSON)
	}
	return nil
}

// 必须自己实现 DoRequest 并以 a 调 DoApiRequest：内嵌 openai.Adaptor.DoRequest
// 会把内嵌实例传给 DoApiRequest，导致 GetRequestURL/SetupRequestHeader 覆写被绕过
func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if useClaudeAPI(info) {
		return channel.DoApiRequest(a, c, info, requestBody)
	}
	return a.Adaptor.DoRequest(c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if useClaudeAPI(info) {
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	}
	return a.Adaptor.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func convertImageRequest(info *relaycommon.RelayInfo, request dto.ImageRequest, requireImage bool) (any, error) {
	if request.N != nil && *request.N > 1 {
		return nil, errors.New("agnes image API does not support n > 1")
	}

	modelName := strings.TrimSpace(request.Model)
	if info != nil && info.ChannelMeta != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		modelName = strings.TrimSpace(info.UpstreamModelName)
	}
	if modelName == "" {
		modelName = ModelImage21Flash
	}

	extraBody, images, err := buildImageFields(request)
	if err != nil {
		return nil, err
	}
	if requireImage && len(images) == 0 {
		return nil, errors.New("agnes image edits require an image URL in image or extra_body.image; file uploads are not supported")
	}

	return imageRequest{
		Model:        modelName,
		Prompt:       request.Prompt,
		Size:         request.Size,
		Image:        images,
		ReturnBase64: getRawExtra(request, "return_base64"),
		ExtraBody:    extraBody,
	}, nil
}

func buildImageFields(request dto.ImageRequest) (map[string]any, []string, error) {
	extraBody := make(map[string]any)
	var images []string

	if request.Extra != nil {
		if raw, ok := request.Extra["extra_body"]; ok && len(bytes.TrimSpace(raw)) > 0 {
			var parsed map[string]json.RawMessage
			if err := common.Unmarshal(raw, &parsed); err != nil {
				return nil, nil, fmt.Errorf("invalid extra_body: %w", err)
			}
			for key, value := range parsed {
				if len(bytes.TrimSpace(value)) == 0 {
					continue
				}
				if key == "image" {
					normalized, ok, err := normalizeImageValue(value)
					if err != nil {
						return nil, nil, err
					}
					if ok {
						images = normalized
					}
					continue
				}
				extraBody[key] = value
			}
		}
	}

	if len(bytes.TrimSpace(request.Image)) > 0 {
		normalized, ok, err := normalizeImageValue(request.Image)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			images = normalized
		}
	}

	if _, ok := extraBody["response_format"]; !ok && strings.TrimSpace(request.ResponseFormat) != "" {
		extraBody["response_format"] = strings.TrimSpace(request.ResponseFormat)
	}

	if len(extraBody) == 0 {
		return nil, images, nil
	}
	return extraBody, images, nil
}

func getRawExtra(request dto.ImageRequest, key string) json.RawMessage {
	if request.Extra == nil {
		return nil
	}
	raw := request.Extra[key]
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	return raw
}

func normalizeImageValue(raw json.RawMessage) ([]string, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false, nil
	}

	var images []string
	if err := common.Unmarshal(trimmed, &images); err == nil {
		images = compactStrings(images)
		return images, len(images) > 0, nil
	}

	var image string
	if err := common.Unmarshal(trimmed, &image); err == nil {
		image = strings.TrimSpace(image)
		if image == "" {
			return nil, false, nil
		}
		return []string{image}, true, nil
	}

	return nil, false, errors.New("agnes image input must be a URL string or an array of URL strings")
}

func compactStrings(values []string) []string {
	compact := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			compact = append(compact, value)
		}
	}
	return compact
}
