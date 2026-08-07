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

func isKimiK3ShortContextOverflow(resp *http.Response) bool {
	if resp == nil || resp.StatusCode != http.StatusBadRequest || resp.Body == nil {
		return false
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return false
	}
	message := strings.ToLower(string(body))
	return strings.Contains(message, "exceeded model token limit: 262144")
}
