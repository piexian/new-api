package minimaxcode

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	// 上游只接受 Messages 协议；内部直调（如渠道测试）复用 claude 的 OpenAI→Claude 转换。
	claudeAdaptor := claude.Adaptor{}
	return claudeAdaptor.ConvertOpenAIRequest(c, info, request)
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("minimax code does not support the responses protocol")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("minimax code does not support the gemini protocol")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("minimax code does not support audio endpoints")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("minimax code does not support image endpoints")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("minimax code does not support rerank endpoints")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("minimax code does not support embedding endpoints")
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info != nil && info.RelayMode == relayconstant.RelayModeClaudeCountTokens {
		return "", errors.New("minimax code does not support the count_tokens endpoint")
	}
	return NormalizeBaseURL(info.ChannelBaseUrl) + MessagesPath, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	oauthKey, err := ParseOAuthKey(info.ApiKey)
	if err != nil {
		return err
	}

	req.Set("Authorization", "Bearer "+strings.TrimSpace(oauthKey.AccessToken))
	req.Set("User-Agent", UserAgent)
	req.Set("X-Mavis-Session-Id", stableUUID("mavis-session|"+oauthKey.AccessToken))
	req.Set("X-Mavis-Agent-Id", AgentID)
	req.Set("X-Mavis-Timezone-Offset", fmt.Sprintf("%d", localTimezoneOffsetSeconds()))
	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else {
		req.Set("Accept", "application/json")
	}
	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	claudeAdaptor := claude.Adaptor{}
	return claudeAdaptor.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

// NormalizeBaseURL 允许粘贴带 LLM 前缀的完整地址，同时兼容海外域名。
func NormalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return DefaultBaseURL
	}
	for _, suffix := range []string{
		MessagesPath,
		LLMPathPrefix,
	} {
		if strings.HasSuffix(baseURL, suffix) {
			baseURL = strings.TrimSuffix(baseURL, suffix)
			break
		}
	}
	return strings.TrimRight(baseURL, "/")
}

// RegionBaseURL 返回区域对应的 LLM 域名。
func RegionBaseURL(region string) string {
	if NormalizeRegion(region) == RegionGlobal {
		return GlobalBaseURL
	}
	return DefaultBaseURL
}

// stableUUID 从种子派生形如 8-4-4-4-12 的稳定 UUID（v4 风格位）。
func stableUUID(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	var parts [4]uint64
	for i := 0; i < 4; i++ {
		parts[i] = binary.BigEndian.Uint64(sum[i*8 : (i+1)*8])
	}
	b := make([]byte, 16)
	for i := 0; i < 2; i++ {
		binary.BigEndian.PutUint64(b[i*8:(i+1)*8], parts[i])
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// localTimezoneOffsetSeconds 与客户端 getTimezoneOffset()*-60 一致：东八区为 28800。
func localTimezoneOffsetSeconds() int {
	_, offset := time.Now().Zone()
	return offset
}
