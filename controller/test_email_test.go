package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type testEmailResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func performTestEmailRequest(t *testing.T, body string) testEmailResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/option/test_email", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	TestEmailDelivery(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response testEmailResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestEmailDeliveryRejectsInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, body := range []string{
		`{"receiver":"not-an-email"}`,
		`{"receiver":""}`,
		`{`,
	} {
		response := performTestEmailRequest(t, body)
		require.False(t, response.Success)
		require.NotEmpty(t, response.Message)
	}
}

func TestEmailDeliveryReturnsProviderError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalProvider := common.EmailProvider
	originalAccountID := common.CFEmailAccountID
	originalAPIToken := common.CFEmailAPIToken
	originalFrom := common.CFEmailFrom
	originalDailyLimit := common.EmailDailyLimit
	originalRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		common.EmailProvider = originalProvider
		common.CFEmailAccountID = originalAccountID
		common.CFEmailAPIToken = originalAPIToken
		common.CFEmailFrom = originalFrom
		common.EmailDailyLimit = originalDailyLimit
		common.RedisEnabled = originalRedisEnabled
	})

	common.EmailProvider = "cloudflare"
	common.CFEmailAccountID = ""
	common.CFEmailAPIToken = ""
	common.CFEmailFrom = ""
	common.EmailDailyLimit = 0
	common.RedisEnabled = false

	response := performTestEmailRequest(t, `{"receiver":"admin@example.com"}`)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "Cloudflare Account ID not configured")
}
