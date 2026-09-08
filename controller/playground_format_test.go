package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundNativeChatFormats(t *testing.T) {
	for path, expected := range map[string]types.RelayFormat{
		"/pg/chat/completions": types.RelayFormatOpenAI,
		"/pg/responses":        types.RelayFormatOpenAIResponses,
		"/pg/messages":         types.RelayFormatClaude,
		"/pg/v1beta/models/gemini-test:generateContent":               types.RelayFormatGemini,
		"/pg/v1beta/models/gemini-test:streamGenerateContent?alt=sse": types.RelayFormatGemini,
	} {
		t.Run(path, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", path, nil)
			require.Equal(t, expected, playgroundRelayFormat(c))
			if expected == types.RelayFormatGemini {
				info := relaycommon.GenRelayInfoGemini(c, nil)
				require.True(t, strings.HasPrefix(info.RequestURLPath, "/v1beta/models/"))
			}
		})
	}
}
