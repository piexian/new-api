package vertex

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGeminiConvertInfo(modelName string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeVertexAi,
			UpstreamModelName: modelName,
		},
	}
}

func newFunctionResponseRequest() *dto.GeminiChatRequest {
	return &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "user",
				Parts: []dto.GeminiPart{
					{
						FunctionResponse: &dto.GeminiFunctionResponse{
							Name:     "get_weather",
							Response: map[string]interface{}{"result": "sunny"},
							ID:       json.RawMessage(`"fc_upstream_1"`),
						},
					},
				},
			},
		},
	}
}

func TestConvertGeminiRequestKeepsFunctionResponseIDForGemini3(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	prev := model_setting.GetGeminiSettings().RemoveFunctionResponseIdEnabled
	model_setting.GetGeminiSettings().RemoveFunctionResponseIdEnabled = true
	defer func() { model_setting.GetGeminiSettings().RemoveFunctionResponseIdEnabled = prev }()

	adaptor := &Adaptor{RequestMode: RequestModeGemini}
	converted, err := adaptor.ConvertGeminiRequest(c, newGeminiConvertInfo("gemini-3.5-flash"), newFunctionResponseRequest())
	require.NoError(t, err)

	req := converted.(*dto.GeminiChatRequest)
	require.NotNil(t, req.Contents[0].Parts[0].FunctionResponse.ID, "3.x 严格匹配要求保留上游下发的 id")
}

func TestConvertGeminiRequestStripsFunctionResponseIDForGemini2(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	prev := model_setting.GetGeminiSettings().RemoveFunctionResponseIdEnabled
	model_setting.GetGeminiSettings().RemoveFunctionResponseIdEnabled = true
	defer func() { model_setting.GetGeminiSettings().RemoveFunctionResponseIdEnabled = prev }()

	adaptor := &Adaptor{RequestMode: RequestModeGemini}
	converted, err := adaptor.ConvertGeminiRequest(c, newGeminiConvertInfo("gemini-2.5-flash"), newFunctionResponseRequest())
	require.NoError(t, err)

	req := converted.(*dto.GeminiChatRequest)
	require.Nil(t, req.Contents[0].Parts[0].FunctionResponse.ID, "2.x 系列上游不支持 functionResponse.id")
}

func TestConvertGeminiRequestKeepsIDWhenStripDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	prev := model_setting.GetGeminiSettings().RemoveFunctionResponseIdEnabled
	model_setting.GetGeminiSettings().RemoveFunctionResponseIdEnabled = false
	defer func() { model_setting.GetGeminiSettings().RemoveFunctionResponseIdEnabled = prev }()

	adaptor := &Adaptor{RequestMode: RequestModeGemini}
	converted, err := adaptor.ConvertGeminiRequest(c, newGeminiConvertInfo("gemini-2.5-flash"), newFunctionResponseRequest())
	require.NoError(t, err)

	req := converted.(*dto.GeminiChatRequest)
	require.NotNil(t, req.Contents[0].Parts[0].FunctionResponse.ID)
}
