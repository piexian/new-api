package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundNativeModelAndGroup(t *testing.T) {
	for _, path := range []string{"/pg/chat/completions", "/pg/responses", "/pg/messages", "/pg/v1beta/models/native-model:streamGenerateContent?alt=sse"} {
		t.Run(path, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", path, strings.NewReader(`{"model":"body-model","group":"selected-group","contents":[{"parts":[{"text":"hello"}]}]}`))
			c.Request.Header.Set("Content-Type", "application/json")
			request, selectChannel, err := getModelRequest(c)
			require.NoError(t, err)
			require.True(t, selectChannel)
			require.Equal(t, "selected-group", request.Group)
			require.Equal(t, "selected-group", common.GetContextKeyString(c, constant.ContextKeyTokenGroup))
			if strings.Contains(path, "v1beta") {
				require.Equal(t, "native-model", request.Model)
				require.Equal(t, relayconstant.RelayModeGemini, relayconstant.Path2RelayMode(c.Request.URL.Path))
			} else {
				require.Equal(t, "body-model", request.Model)
			}
		})
	}
}
