package moonshot

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

const (
	kimiK3ShortContextModel  = "k3-256k"
	kimiK3FullContextModel   = "k3"
	kimiK3FallbackContextKey = "moonshot_k3_short_context_fallback"

	kimiK3ShortContextRouteLabel        = "K3 Auto Route (k3 -> k3-256k)"
	kimiK3FullContextFallbackRouteLabel = "K3 Context Fallback (k3-256k -> k3)"
	kimiK3DirectContextRouteLabel       = "K3 Auto Route (k3 direct: estimated > 256K)"
)

func markKimiK3ShortContextFallback(c *gin.Context, info *relaycommon.RelayInfo) {
	if c != nil {
		c.Set(kimiK3FallbackContextKey, true)
	}
	info.AppendRequestModelRouting(kimiK3ShortContextRouteLabel)
}

func shouldRetryKimiK3WithFullContext(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if c == nil || !shouldUseKimiK3ShortContext(info, getUpstreamModelName(info, "")) {
		return false
	}
	value, exists := c.Get(kimiK3FallbackContextKey)
	marked, ok := value.(bool)
	return exists && ok && marked
}

func replaceKimiModelInRequestBody(body []byte, model string) ([]byte, bool, error) {
	var payload map[string]json.RawMessage
	if err := common.Unmarshal(body, &payload); err != nil {
		return body, false, err
	}
	rawModel, ok := payload["model"]
	if !ok {
		return body, false, nil
	}
	var currentModel string
	if err := common.Unmarshal(rawModel, &currentModel); err != nil ||
		!strings.EqualFold(strings.TrimSpace(currentModel), kimiK3ShortContextModel) {
		return body, false, nil
	}
	encodedModel, err := common.Marshal(model)
	if err != nil {
		return body, false, err
	}
	payload["model"] = encodedModel
	encodedPayload, err := common.Marshal(payload)
	if err != nil {
		return body, false, err
	}
	return encodedPayload, true, nil
}

// isKimiK3ShortContextOverflow 识别 k3-256k 的 256K 上下文溢出错误。
// 两种已观测形态：400 + "exceeded model token limit: 262144"；
// 401 + "supports only" + "256k context"（套餐/模型档位限制措辞）。
// rewriteKimiK3AutoRouteOverflowStatus 把自动路由（用户请求 k3）场景下最终仍溢出的
// 响应状态码改写为 429：可触发外层换渠道重试，且避免 401 落入渠道自动禁用区间。
// 显式 k3-256k 请求不满足 shouldUseKimiK3ShortContext，原状态码保持不变。
func rewriteKimiK3AutoRouteOverflowStatus(info *relaycommon.RelayInfo, resp *http.Response) {
	if resp == nil {
		return
	}
	if !shouldUseKimiK3ShortContext(info, getUpstreamModelName(info, "")) {
		return
	}
	if isKimiK3ShortContextOverflow(resp) {
		resp.StatusCode = http.StatusTooManyRequests
	}
}

func isKimiK3ShortContextOverflow(resp *http.Response) bool {
	if resp == nil || resp.Body == nil {
		return false
	}
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnauthorized {
		return false
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return false
	}
	message := strings.ToLower(string(body))
	switch resp.StatusCode {
	case http.StatusBadRequest:
		return strings.Contains(message, "exceeded model token limit: 262144")
	default: // http.StatusUnauthorized
		return strings.Contains(message, "supports only") && strings.Contains(message, "256k context")
	}
}
