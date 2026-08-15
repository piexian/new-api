package baidu_v2

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
)

// Claude 请求必须走本渠道的 ConvertOpenAIRequest（-search 后缀剥离 + web_search 注入），
// 不能委托给新建 openai.Adaptor 实例导致定制被跳过。
func TestConvertClaudeRequestAppliesSearchSuffix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := &dto.ClaudeRequest{
		Model:     "ernie-x1-search",
		MaxTokens: lo.ToPtr(uint(64)),
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "hi"},
		},
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "ernie-x1-search"},
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, req)
	if err != nil {
		t.Fatalf("convert claude request: %v", err)
	}

	payload, ok := converted.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload with web_search injected, got %T", converted)
	}
	if payload["model"] != "ernie-x1" {
		t.Fatalf("model suffix not stripped: %v", payload["model"])
	}
	ws, ok := payload["web_search"].(map[string]any)
	if !ok || ws["enable"] != true {
		t.Fatalf("web_search not injected: %v", payload["web_search"])
	}
}
