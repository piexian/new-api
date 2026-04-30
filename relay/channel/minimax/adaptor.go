package minimax

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := claude.Adaptor{}
	return adaptor.ConvertClaudeRequest(c, info, req)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != constant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}

	if isMiniMaxMusicModel(info.OriginModelName) {
		return convertMusicRequest(c, info, request)
	}

	voiceID := request.Voice
	speed := lo.FromPtrOr(request.Speed, 0.0)
	audioFormat := normalizeMiniMaxAudioFormat(request.ResponseFormat)
	outputFormat := normalizeMiniMaxOutputFormat(request.ResponseFormat)

	minimaxRequest := MiniMaxTTSRequest{
		Model: info.OriginModelName,
		Text:  request.Input,
		VoiceSetting: VoiceSetting{
			VoiceID: voiceID,
			Speed:   speed,
		},
		AudioSetting: &AudioSetting{
			Format: audioFormat,
		},
		OutputFormat: outputFormat,
	}

	// 同步扩展字段的厂商自定义metadata
	if len(request.Metadata) > 0 {
		if err := common.Unmarshal(request.Metadata, &minimaxRequest); err != nil {
			return nil, fmt.Errorf("error unmarshalling metadata to minimax request: %w", err)
		}
	}
	minimaxRequest.OutputFormat = normalizeMiniMaxOutputFormat(minimaxRequest.OutputFormat)

	jsonData, err := common.Marshal(minimaxRequest)
	if err != nil {
		return nil, fmt.Errorf("error marshalling minimax request: %w", err)
	}

	c.Set("response_format", minimaxRequest.OutputFormat)

	// Debug: log the request structure
	// fmt.Printf("MiniMax TTS Request: %s\n", string(jsonData))

	return bytes.NewReader(jsonData), nil
}

func convertMusicRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	audioFormat := normalizeMiniMaxAudioFormat(request.ResponseFormat)
	outputFormat := normalizeMiniMaxOutputFormat(request.ResponseFormat)
	minimaxRequest := MiniMaxMusicRequest{
		Model:        info.OriginModelName,
		Prompt:       request.Input,
		Lyrics:       request.Instructions,
		OutputFormat: outputFormat,
		AudioSetting: &AudioSetting{
			Format: audioFormat,
		},
	}
	if len(request.Metadata) > 0 {
		if err := common.Unmarshal(request.Metadata, &minimaxRequest); err != nil {
			return nil, fmt.Errorf("error unmarshalling metadata to minimax music request: %w", err)
		}
	}
	minimaxRequest.OutputFormat = normalizeMiniMaxOutputFormat(minimaxRequest.OutputFormat)

	jsonData, err := common.Marshal(minimaxRequest)
	if err != nil {
		return nil, fmt.Errorf("error marshalling minimax music request: %w", err)
	}
	c.Set("response_format", minimaxRequest.OutputFormat)
	return bytes.NewReader(jsonData), nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info.RelayMode != constant.RelayModeImagesGenerations {
		return nil, fmt.Errorf("unsupported image relay mode: %d", info.RelayMode)
	}
	if isMiniMaxNativeImageEndpoint(c) {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return nil, err
		}
		body, err := storage.Bytes()
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(body), nil
	}
	return oaiImage2MiniMaxImageRequest(request), nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return GetRequestURL(info)
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
	if lo.FromPtrOr(request.MaxCompletionTokens, uint(0)) == 0 && lo.FromPtrOr(request.MaxTokens, uint(0)) != 0 {
		request.MaxCompletionTokens = request.MaxTokens
		request.MaxTokens = nil
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
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == constant.RelayModeAudioSpeech {
		return handleTTSResponse(c, resp, info)
	}
	if info.RelayMode == constant.RelayModeImagesGenerations {
		if isMiniMaxNativeImageEndpoint(c) {
			return miniMaxNativeImageHandler(c, resp)
		}
		return miniMaxImageHandler(c, resp, info)
	}

	switch info.RelayFormat {
	case types.RelayFormatClaude:
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	default:
		return handleChatCompletionResponse(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
