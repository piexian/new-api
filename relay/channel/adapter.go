package channel

import (
	"io"
	"net/http"

	dto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor interface {
	// Init IsStream bool
	Init(info *relaycommon.RelayInfo)
	GetRequestURL(info *relaycommon.RelayInfo) (string, error)
	SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error
	ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *kitdto.GeneralOpenAIRequest) (any, error)
	ConvertRerankRequest(c *gin.Context, relayMode int, request kitdto.RerankRequest) (any, error)
	ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request kitdto.EmbeddingRequest) (any, error)
	ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request kitdto.AudioRequest) (io.Reader, error)
	ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request kitdto.ImageRequest) (any, error)
	ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request kitdto.OpenAIResponsesRequest) (any, error)
	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
	DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError)
	GetModelList() []string
	GetChannelName() string
	ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *kitdto.ClaudeRequest) (any, error)
	ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *kitdto.GeminiChatRequest) (any, error)
}

// BlockedInboundRoutes 声明该渠道不支持的入站路径，命中即在入口拦截并返回 404。
// 用于能力缺失的渠道（如仅支持 rerank/embedding/生图的专用渠道），
// 避免请求被降级后发到不支持的上游端点。
type InboundRouteBlocker interface {
	BlockedInboundRoutes() []string
}

// ProtectedHeaderAdaptor 声明由渠道自身写入的身份/指纹请求头（不区分大小写）。
// Header Override（含渠道亲和性模板的 pass_headers）不得新增或覆盖这些头：
// 客户端的 User-Agent、会话 ID 等被透传后会让 OAuth 凭证或客户端指纹校验失败。
type ProtectedHeaderAdaptor interface {
	ProtectedRequestHeaders(info *relaycommon.RelayInfo) []string
}

type TaskAdaptor interface {
	Init(info *relaycommon.RelayInfo)

	ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError

	// ── Billing ──────────────────────────────────────────────────────

	// EstimateBilling returns OtherRatios for pre-charge based on user request.
	// Called after ValidateRequestAndSetAction, before price calculation.
	// Adaptors should extract duration, resolution, etc. from the parsed request
	// and return them as ratio multipliers (e.g. {"seconds": 5, "size": 1.666}).
	// Return nil to use the base model price without extra ratios.
	EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64

	// AdjustBillingOnSubmit returns adjusted OtherRatios from the upstream
	// submit response. Called after a successful DoResponse.
	// If the upstream returned actual parameters that differ from the estimate
	// (e.g. actual seconds), return updated ratios so the caller can recalculate
	// the quota and settle the delta with the pre-charge.
	// Return nil if no adjustment is needed.
	AdjustBillingOnSubmit(info *relaycommon.RelayInfo, taskData []byte) map[string]float64

	// AdjustBillingOnComplete returns the actual quota when a task reaches a
	// terminal state (success/failure) during polling.
	// Called by the polling loop after ParseTaskResult.
	// Return a positive value to trigger delta settlement (supplement / refund).
	// Return 0 to keep the pre-charged amount unchanged.
	AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int

	// ── Request / Response ───────────────────────────────────────────

	BuildRequestURL(info *relaycommon.RelayInfo) (string, error)
	BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error
	BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error)

	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error)
	DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, err *dto.TaskError)

	GetModelList() []string
	GetChannelName() string

	// ── Polling ──────────────────────────────────────────────────────

	FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error)
	ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error)
}

type OpenAIVideoConverter interface {
	ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error)
}
