package perplexity

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
)

// Claude 请求必须走本渠道的 ConvertOpenAIRequest（top_p 钳制 + perplexity 映射），
// 不能委托给新建 openai.Adaptor 实例导致定制被跳过。
func TestConvertClaudeRequestAppliesPerplexityMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	topP := 1.0
	req := &dto.ClaudeRequest{
		Model:     "sonar-pro",
		MaxTokens: lo.ToPtr(uint(64)),
		TopP:      &topP,
		Messages: []dto.ClaudeMessage{
			{
				Role:    "user",
				Content: "hi",
			},
		},
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "sonar-pro"},
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, req)
	if err != nil {
		t.Fatalf("convert claude request: %v", err)
	}

	// requestOpenAI2Perplexity 返回的结构应带 perplexity 风格字段，且 top_p 被钳制到 0.99
	switch v := converted.(type) {
	case *dto.GeneralOpenAIRequest:
		if v.TopP == nil || *v.TopP >= 1 {
			t.Fatalf("top_p not clamped: %+v", v.TopP)
		}
	case map[string]any:
		if tp, ok := v["top_p"].(float64); ok && tp >= 1 {
			t.Fatalf("top_p not clamped: %v", tp)
		}
	default:
		t.Fatalf("unexpected converted type %T", converted)
	}
}
