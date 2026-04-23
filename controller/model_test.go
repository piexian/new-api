package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiListResponse struct {
	Models        []dto.GeminiModel `json:"models"`
	NextPageToken string            `json:"nextPageToken"`
}

type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func TestListModelsGeminiFiltersByEndpointAndPaginates(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	previousSelfUseMode := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = previousSelfUseMode
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models?pageSize=1", nil)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{
		"gpt-4o":               true,
		"gemini-2.5-pro":       true,
		"gemini-2.5-flash":     true,
		"gemini-embedding-001": true,
	})

	ListModels(c, constant.ChannelTypeGemini)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response geminiListResponse
	err := common.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Models, 1)
	require.Equal(t, "1", response.NextPageToken)
	require.NotEqual(t, "models/gpt-4o", response.Models[0].Name)
	require.NotEmpty(t, response.Models[0].BaseModelId)
	require.NotEmpty(t, response.Models[0].DisplayName)
	require.NotEmpty(t, response.Models[0].SupportedGenerationMethods)
}

func TestListModelsGeminiRejectsInvalidPageToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	previousSelfUseMode := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = previousSelfUseMode
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models?pageToken=bad-token", nil)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{
		"gemini-2.5-pro": true,
	})

	ListModels(c, constant.ChannelTypeGemini)

	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var response geminiErrorResponse
	err := common.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "INVALID_ARGUMENT", response.Error.Status)
}

func TestRetrieveModelGeminiUsesNativeShape(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	t.Run("native model", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Params = gin.Params{{Key: "model", Value: "gemini-2.5-pro"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models/gemini-2.5-pro", nil)

		RetrieveModel(c, constant.ChannelTypeGemini)

		require.Equal(t, http.StatusOK, recorder.Code)

		var response dto.GeminiModel
		err := common.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, "models/gemini-2.5-pro", response.Name)
		require.Equal(t, "gemini-2.5-pro", response.BaseModelId)
		require.Contains(t, response.SupportedGenerationMethods, "generateContent")
	})

	t.Run("unsupported openai-only model", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Params = gin.Params{{Key: "model", Value: "gpt-4o"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models/gpt-4o", nil)

		RetrieveModel(c, constant.ChannelTypeGemini)

		require.Equal(t, http.StatusNotFound, recorder.Code)

		var response geminiErrorResponse
		err := common.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, "NOT_FOUND", response.Error.Status)
	})
}
