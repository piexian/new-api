package minimaxcode

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// MCP 媒体桥:MiniMax Code 订阅(Token Plan / M Plan)经 mavis MCP 工具接口提供
// 语音合成、图片生成与音乐生成,信封为 {code, message, output_url | success_items[], base_resp}。

const (
	mcpDefaultVoiceID     = "male-qn-qingse"
	mcpImageResolutionKey = "mcp_image_resolution"
)

type mcpSpeechRequest struct {
	Text    string   `json:"text"`
	VoiceID string   `json:"voice_id,omitempty"`
	Speed   *float64 `json:"speed,omitempty"`
	Volume  *float64 `json:"volume,omitempty"`
	Pitch   *int     `json:"pitch,omitempty"`
	Emotion string   `json:"emotion,omitempty"`
}

type mcpMusicRequest struct {
	Prompt     string `json:"prompt"`
	Lyrics     string `json:"lyrics,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
	Bitrate    int    `json:"bitrate,omitempty"`
	Format     string `json:"format,omitempty"`
}

type mcpMusicEnvelope struct {
	Requests []mcpMusicRequest `json:"requests"`
}

type mcpImageRequest struct {
	Prompt      string `json:"prompt"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
}

type mcpImageEnvelope struct {
	Requests []mcpImageRequest `json:"requests"`
}

type mcpMediaItem struct {
	IsSuccess    bool   `json:"is_success,omitempty"`
	OutputURL    string `json:"output_url,omitempty"`
	TextResponse string `json:"text_response,omitempty"`
}

type mcpMediaResponse struct {
	Code         int            `json:"code"`
	Message      string         `json:"message"`
	OutputURL    string         `json:"output_url,omitempty"`
	SuccessItems []mcpMediaItem `json:"success_items,omitempty"`
	FailedItems  []mcpMediaItem `json:"failed_items,omitempty"`
}

func isMiniMaxCodeMusicModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(model), "music")
}

func isMiniMaxCodeImageModel(model string) bool {
	return strings.HasPrefix(model, "MiniMax-H3-Image")
}

// convertMCPAudioRequest 把 OpenAI 语音请求转换为 MCP 工具请求:
// speech-* 走 synthesize_speech(text/voice_id/speed),music-* 走 batch_text_to_music(单条)。
func convertMCPAudioRequest(info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if isMiniMaxCodeMusicModel(info.UpstreamModelName) {
		musicReq := mcpMusicRequest{Prompt: request.Input, Format: "mp3"}
		if strings.TrimSpace(request.Instructions) != "" {
			musicReq.Lyrics = request.Instructions
		}
		// metadata 允许覆盖 lyrics/sample_rate/bitrate/format 等高级字段
		if len(request.Metadata) > 0 {
			var envelope mcpMusicEnvelope
			if err := common.Unmarshal(request.Metadata, &envelope); err == nil && len(envelope.Requests) > 0 {
				if envelope.Requests[0].Lyrics != "" {
					musicReq.Lyrics = envelope.Requests[0].Lyrics
				}
				musicReq.SampleRate = envelope.Requests[0].SampleRate
				musicReq.Bitrate = envelope.Requests[0].Bitrate
				if envelope.Requests[0].Format != "" {
					musicReq.Format = envelope.Requests[0].Format
				}
			}
		}
		body, err := common.Marshal(mcpMusicEnvelope{Requests: []mcpMusicRequest{musicReq}})
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(body), nil
	}

	speechReq := mcpSpeechRequest{Text: request.Input}
	if voice := strings.TrimSpace(request.Voice); voice != "" {
		speechReq.VoiceID = voice
	} else {
		speechReq.VoiceID = mcpDefaultVoiceID
	}
	speechReq.Speed = request.Speed
	body, err := common.Marshal(speechReq)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

// convertMCPImageRequest 把 OpenAI 图片请求转换为 MCP image_synthesize 请求
func convertMCPImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		return nil, fmt.Errorf("minimax code does not support the image edits endpoint")
	}
	imageReq := mcpImageRequest{Prompt: request.Prompt}
	if aspectRatio, ok := request.Extra["aspect_ratio"]; ok {
		var value string
		if common.Unmarshal(aspectRatio, &value) == nil && value != "" {
			imageReq.AspectRatio = value
		}
	}
	if resolution, ok := request.Extra["area"]; ok {
		var value string
		if common.Unmarshal(resolution, &value) == nil && value != "" {
			imageReq.Resolution = value
		}
	}
	if imageReq.Resolution == "" {
		if resolution, ok := request.Extra["resolution"]; ok {
			var value string
			if common.Unmarshal(resolution, &value) == nil && value != "" {
				imageReq.Resolution = value
			}
		}
	}
	// 请求分辨率落到 context,供 DoResponse 按档位计费
	c.Set(mcpImageResolutionKey, imageReq.Resolution)
	return mcpImageEnvelope{Requests: []mcpImageRequest{imageReq}}, nil
}

// doMCPSpeechResponse 处理 synthesize_speech 响应:下载 output_url 音频并回给客户端,
// 计费沿用字符口径(与普通 MiniMax 渠道一致)
func doMCPSpeechResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	mediaResp, err := readMCPMediaResponse(resp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	if mediaResp.Code != 0 || mediaResp.OutputURL == "" {
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: fmt.Sprintf("minimax code speech api error: %s (code %d)", mediaResp.Message, mediaResp.Code),
			Type:    "minimax_code_error",
		}, http.StatusBadGateway)
	}

	audioData, err := downloadMediaFile(mediaResp.OutputURL)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	c.Data(http.StatusOK, "audio/mpeg", audioData)

	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens <= 0 {
		promptTokens = 1
	}
	return &dto.Usage{PromptTokens: promptTokens, TotalTokens: promptTokens}, nil
}

// doMCPMusicResponse 处理 batch_text_to_music 响应:下载 success_items[0].output_url 并回给客户端;
// music-3.0 走 per-call 固定价,usage 仅作记录
func doMCPMusicResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	mediaResp, err := readMCPMediaResponse(resp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	outputURL := firstMCPSuccessURL(mediaResp)
	if outputURL == "" {
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: fmt.Sprintf("minimax code music api error: %s (code %d)", mediaResp.Message, mediaResp.Code),
			Type:    "minimax_code_error",
		}, http.StatusBadGateway)
	}

	audioData, err := downloadMediaFile(outputURL)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	c.Data(http.StatusOK, "audio/mpeg", audioData)

	return &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil
}

// doMCPImageResponse 处理 image_synthesize 响应:回填 OpenAI 图片响应(url 形式),
// 计费按请求分辨率的官方单张参考价折算 token 当量(走该模型的 token 倍率)
func doMCPImageResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	mediaResp, err := readMCPMediaResponse(resp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	imageResponse := &dto.ImageResponse{Created: info.StartTime.Unix()}
	for _, item := range mediaResp.SuccessItems {
		if item.OutputURL != "" {
			imageResponse.Data = append(imageResponse.Data, dto.ImageData{Url: item.OutputURL})
		}
	}
	if len(imageResponse.Data) == 0 {
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: fmt.Sprintf("minimax code image api error: %s (code %d)", mediaResp.Message, mediaResp.Code),
			Type:    "minimax_code_error",
		}, http.StatusBadGateway)
	}
	jsonResponse, err := common.Marshal(imageResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponse)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(http.StatusOK)
	if _, err := c.Writer.Write(jsonResponse); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponse)
	}

	callTokens := mcpImageCallTokens(c.GetString(mcpImageResolutionKey))
	return &dto.Usage{PromptTokens: callTokens, TotalTokens: callTokens}, nil
}

// mcpImageCallTokens 官方单张参考价(1K ¥0.143 / 2K ¥0.153 / 4K ¥0.452)
// 按文本输入 ¥15/M 折算 token 当量;未指定分辨率按官方缺省 4K
func mcpImageCallTokens(resolution string) int {
	switch strings.ToUpper(strings.TrimSpace(resolution)) {
	case "1K":
		return 9531
	case "2K":
		return 10206
	default:
		return 30145
	}
}

// readMCPMediaResponse 读取上游 MCP 工具响应
func readMCPMediaResponse(resp *http.Response) (*mcpMediaResponse, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("empty media response")
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read media response: %w", err)
	}
	service.CloseResponseBodyGracefully(resp)

	var mediaResp mcpMediaResponse
	if err := common.Unmarshal(responseBody, &mediaResp); err != nil {
		return nil, fmt.Errorf("failed to parse media response: %w", err)
	}
	return &mediaResp, nil
}

// firstMCPSuccessURL 取第一个成功条目的产物地址
func firstMCPSuccessURL(mediaResp *mcpMediaResponse) string {
	for _, item := range mediaResp.SuccessItems {
		if item.IsSuccess && item.OutputURL != "" {
			return item.OutputURL
		}
	}
	return ""
}

// downloadMediaFile 下载 MCP 产物(公开 CDN 地址,无需鉴权)
func downloadMediaFile(fileURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	client := service.GetHttpClient()
	if client == nil {
		// 单测环境未初始化全局客户端时回退默认实例
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download media file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download media file status: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read media file: %w", err)
	}
	return data, nil
}
