package stepfun

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
)

// ValidateEndpointForModel 校验模型与入站端点能力是否匹配。
// 依据为上游 GET /v1/models/{id} 的 supported_protocols 快照（constant/stepfun.go）：
// 例如 step-3.5-flash 只支持 chat/messages，收到 /v1/responses 时本地直接 400，
// 不必把必然失败的请求打到上游。未收录模型不做限制（代理与自定义 base 场景不误伤）。
func ValidateEndpointForModel(info *relaycommon.RelayInfo) *types.NewAPIError {
	if info == nil || info.ChannelType != constant.ChannelTypeStepFun {
		return nil
	}
	if !isStepFunBase(info.ChannelBaseUrl) {
		// 非白名单 base 走通用转换，模型能力不可推断，跳过校验
		return nil
	}
	modelName := strings.ToLower(strings.TrimSpace(info.UpstreamModelName))
	if modelName == "" && info.ChannelMeta != nil {
		modelName = strings.ToLower(strings.TrimSpace(info.ChannelMeta.UpstreamModelName))
	}
	if modelName == "" {
		modelName = strings.ToLower(strings.TrimSpace(info.OriginModelName))
	}
	if modelName == "" {
		return nil
	}

	switch {
	case info.RelayMode == relayconstant.RelayModeResponses && !constant.StepFunModelSupportsResponses(modelName):
		return stepFunEndpointError(modelName, "/v1/responses",
			"the model does not support the Responses API; call it via /v1/chat/completions or /v1/messages")
	case info.RelayMode == relayconstant.RelayModeResponsesCompact:
		return stepFunEndpointError(modelName, "/v1/responses/compact",
			"the compact endpoint is not supported on StepFun channels")
	case info.RelayFormat == types.RelayFormatClaude && !constant.StepFunModelSupportsMessages(modelName):
		return stepFunEndpointError(modelName, "/v1/messages",
			"the model does not support the Messages API; call it via /v1/chat/completions")
	}
	return nil
}

func stepFunEndpointError(modelName, endpoint, detail string) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		fmt.Errorf("StepFun model %q cannot be used with %s: %s", modelName, endpoint, detail),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithSkipSensitiveMask(),
	)
}
