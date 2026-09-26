package gmicloud

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testImageModel = "hy-image-v3.5-preview"

func imageInfo(relayMode int) *relaycommon.RelayInfo {
	info := newInfo(relayMode, types.RelayFormatOpenAIImage)
	info.OriginModelName = testImageModel
	info.UpstreamModelName = testImageModel
	info.ApiKey = "test-key"
	return info
}

func TestResolveGMISize(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   string
		over2K bool
	}{
		{"空值走 Auto", "", "", false},
		{"auto 关键字", "auto", "", false},
		{"精确枚举 1:1 2K", "2048x2048", "2048x2048", false},
		{"精确枚举 QHD", "2560x1440", "2560x1440", false},
		{"精确枚举 4K 16:9", "3840x2160", "3840x2160", true},
		{"大写 X 归一", "1920X1080", "1920x1080", false},
		{"档位+比例", "4k+16:9", "3840x2160", true},
		{"档位+比例 9:16", "4K+9:16", "2160x3840", true},
		{"档位+比例 1:1", "2k+1:1", "2048x2048", false},
		{"档位+比例 QHD", "qhd+16:9", "2560x1440", false},
		{"档位+比例 4:3", "2k+4:3", "1536x1152", false},
		{"空格分隔", "1.5k + 1:1", "1536x1536", false},
		{"只给档位按 1:1", "4k", "4096x4096", true},
		{"只给档位 1k", "1k", "1024x1024", false},
		{"只给比例取最小档", "16:9", "1920x1080", false},
		{"只给比例 9:16", "9:16", "1080x1920", false},
		{"只给比例 4:3", "4:3", "1536x1152", false},
		{"只给比例 3:4", "3:4", "1152x1536", false},
		{"只给比例 1:1", "1:1", "1024x1024", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveGMISize(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, got.size)
			require.Equal(t, tc.over2K, got.over2K)
		})
	}
}

func TestResolveGMISizeRejectsUnknown(t *testing.T) {
	for _, input := range []string{"1024x2048", "3k+16:9", "8k", "21:9", "4k+4:3", "qhd+1:1", "4k+16:9+1:1", "landscape"} {
		_, err := resolveGMISize(input)
		require.Error(t, err, "size %q 应被本地拦截", input)
		var apiErr *types.NewAPIError
		require.ErrorAs(t, err, &apiErr)
		require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		require.True(t, types.IsSkipRetryError(apiErr), "非法 size 不应重试整个渠道池")
	}
}

func TestBuildImageRequestTextToImage(t *testing.T) {
	info := imageInfo(relayconstant.RelayModeImagesGenerations)
	body, err := buildImageRequest(nil, info, dto.ImageRequest{
		Model:  testImageModel,
		Prompt: "a red apple on a wooden table",
		Size:   "4k+16:9",
	})
	require.NoError(t, err)
	require.Equal(t, testImageModel, body.Model)
	require.Equal(t, "a red apple on a wooden table", body.Payload["prompt"])
	require.Equal(t, "3840x2160", body.Payload["size"])
	require.NotContains(t, body.Payload, "image")
	require.InDelta(t, gmiImageOver2KPriceRatio, info.PriceData.OtherRatioMultiplier(), 1e-9)
}

func TestBuildImageRequestAutoSizeUsesPixelBudget(t *testing.T) {
	info := imageInfo(relayconstant.RelayModeImagesGenerations)
	request := dto.ImageRequest{Model: testImageModel, Prompt: "sunrise"}
	request.Extra = map[string]json.RawMessage{}

	raw, err := common.Marshal(2359296)
	require.NoError(t, err)
	request.Extra["generate_max_pixels"] = raw

	body, err := buildImageRequest(nil, info, request)
	require.NoError(t, err)
	require.NotContains(t, body.Payload, "size")
	require.EqualValues(t, 2359296, body.Payload["generate_max_pixels"])
	require.Equal(t, 1.0, info.PriceData.OtherRatioMultiplier(), "Auto 档不叠加 4K 倍率")
}

func TestBuildImageRequestBelow2KHasNoPriceRatio(t *testing.T) {
	info := imageInfo(relayconstant.RelayModeImagesGenerations)
	_, err := buildImageRequest(nil, info, dto.ImageRequest{
		Model:  testImageModel,
		Prompt: "sunrise",
		Size:   "qhd+16:9",
	})
	require.NoError(t, err)
	require.Equal(t, 1.0, info.PriceData.OtherRatioMultiplier(), "QHD 仍在 2K 计价档内")
}

func TestBuildImageRequestImageToImage(t *testing.T) {
	info := imageInfo(relayconstant.RelayModeImagesEdits)
	raw, err := common.Marshal([]string{"https://cdn.example.com/a.png", "https://cdn.example.com/b.png"})
	require.NoError(t, err)

	body, err := buildImageRequest(nil, info, dto.ImageRequest{
		Model:  testImageModel,
		Prompt: "make it blue",
		Size:   "1024x1024",
		Image:  raw,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"https://cdn.example.com/a.png", "https://cdn.example.com/b.png"}, body.Payload["image"])
}

// /v1/images/edits 也接受 multipart 表单里的单个 image URL。
func TestBuildImageRequestImageToImageSingleString(t *testing.T) {
	info := imageInfo(relayconstant.RelayModeImagesEdits)
	raw, err := common.Marshal("https://cdn.example.com/a.png")
	require.NoError(t, err)

	body, err := buildImageRequest(nil, info, dto.ImageRequest{
		Model:  testImageModel,
		Prompt: "make it blue",
		Image:  raw,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"https://cdn.example.com/a.png"}, body.Payload["image"])
}

func TestBuildImageRequestRejectsInvalidInput(t *testing.T) {
	rawURL := func(value string) []byte {
		raw, err := common.Marshal(value)
		require.NoError(t, err)
		return raw
	}
	rawList := func(values ...string) []byte {
		raw, err := common.Marshal(values)
		require.NoError(t, err)
		return raw
	}

	cases := []struct {
		name          string
		mode          int
		request       dto.ImageRequest
		wantMsg       string
		upstreamModel string
	}{
		{"缺 prompt", relayconstant.RelayModeImagesGenerations, dto.ImageRequest{Model: testImageModel}, "prompt is required", ""},
		{"n 大于 1", relayconstant.RelayModeImagesGenerations, dto.ImageRequest{Model: testImageModel, Prompt: "x", N: common.GetPointer(uint(2))}, "n must be 1", ""},
		{"b64_json", relayconstant.RelayModeImagesGenerations, dto.ImageRequest{Model: testImageModel, Prompt: "x", ResponseFormat: "b64_json"}, "b64_json is not supported", ""},
		{"非法 size", relayconstant.RelayModeImagesGenerations, dto.ImageRequest{Model: testImageModel, Prompt: "x", Size: "1024x2048"}, "unsupported size", ""},
		{"上游没有的档位+比例", relayconstant.RelayModeImagesGenerations, dto.ImageRequest{Model: testImageModel, Prompt: "x", Size: "4k+4:3"}, "unsupported size", ""},
		{"参考图非 http", relayconstant.RelayModeImagesGenerations, dto.ImageRequest{Model: testImageModel, Prompt: "x", Image: rawURL("ftp://cdn.example.com/a.png")}, "must be a public HTTP(S) URL", ""},
		{"参考图超 5 张", relayconstant.RelayModeImagesGenerations, dto.ImageRequest{Model: testImageModel, Prompt: "x", Image: rawList("https://a/1.png", "https://a/2.png", "https://a/3.png", "https://a/4.png", "https://a/5.png", "https://a/6.png")}, "at most 5 reference images", ""},
		{"edits 缺参考图", relayconstant.RelayModeImagesEdits, dto.ImageRequest{Model: testImageModel, Prompt: "x"}, "requires at least one reference image", ""},
		{"非图片模型", relayconstant.RelayModeImagesGenerations, dto.ImageRequest{Model: "MiniMaxAI/MiniMax-M2.7", Prompt: "x"}, "does not support /v1/images/generations", "MiniMaxAI/MiniMax-M2.7"},
		{"错误端点", relayconstant.RelayModeChatCompletions, dto.ImageRequest{Model: testImageModel, Prompt: "x"}, "only serves /v1/images/generations", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := imageInfo(tc.mode)
			if tc.upstreamModel != "" {
				info.UpstreamModelName = tc.upstreamModel
			}
			_, err := buildImageRequest(nil, info, tc.request)
			require.ErrorContains(t, err, tc.wantMsg)
			var apiErr *types.NewAPIError
			require.ErrorAs(t, err, &apiErr)
			require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
			require.True(t, types.IsSkipRetryError(apiErr))
		})
	}
}

func TestBuildImageRequestRejectsBadExtras(t *testing.T) {
	build := func(t *testing.T, extras map[string]json.RawMessage) error {
		t.Helper()
		request := dto.ImageRequest{Model: testImageModel, Prompt: "x", Size: "1024x1024", Extra: extras}
		_, err := buildImageRequest(nil, imageInfo(relayconstant.RelayModeImagesGenerations), request)
		return err
	}

	raw, err := common.Marshal(1234567)
	require.NoError(t, err)
	require.ErrorContains(t, build(t, map[string]json.RawMessage{"generate_max_pixels": raw}), "generate_max_pixels must be one of")

	raw, err = common.Marshal(-1)
	require.NoError(t, err)
	require.ErrorContains(t, build(t, map[string]json.RawMessage{"seed": raw}), "seed must not be negative")

	raw, err = common.Marshal("abc")
	require.NoError(t, err)
	require.ErrorContains(t, build(t, map[string]json.RawMessage{"seed": raw}), "seed must be an integer")

	raw, err = common.Marshal(7)
	require.NoError(t, err)
	body, err := buildImageRequest(nil, imageInfo(relayconstant.RelayModeImagesGenerations), dto.ImageRequest{
		Model: testImageModel, Prompt: "x", Size: "1024x1024",
		Extra: map[string]json.RawMessage{"seed": raw},
	})
	require.NoError(t, err)
	require.EqualValues(t, 7, body.Payload["seed"])
}

// 消费日志要记实际计费尺寸，否则简写入参无法反推落在哪一档。
func TestBuildImageRequestRecordsResolvedSizeForLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, err := buildImageRequest(c, imageInfo(relayconstant.RelayModeImagesGenerations), dto.ImageRequest{
		Model: testImageModel, Prompt: "x", Size: "4k+16:9",
	})
	require.NoError(t, err)
	details, ok := c.Get("image_request_detail")
	require.True(t, ok)
	require.Equal(t, map[string]interface{}{"size": "3840x2160"}, details)

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, err = buildImageRequest(c2, imageInfo(relayconstant.RelayModeImagesGenerations), dto.ImageRequest{
		Model: testImageModel, Prompt: "x",
	})
	require.NoError(t, err)
	details2, ok := c2.Get("image_request_detail")
	require.True(t, ok)
	require.Equal(t, map[string]interface{}{"size": "auto"}, details2)
}

// 上游自己去拉参考图，文件上传没有可公开访问的地址，必须显式拒绝而不是静默退化成文生图。
func TestBuildImageRequestRejectsUploadedFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	c.Request.MultipartForm = &multipartFormWithFile

	_, err := buildImageRequest(c, imageInfo(relayconstant.RelayModeImagesEdits), dto.ImageRequest{
		Model:  testImageModel,
		Prompt: "make it blue",
	})
	require.ErrorContains(t, err, "uploaded image files are not supported")
}

func newImageMockServer(t *testing.T, polls *int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, requestStatusPath) && r.Method == http.MethodGet {
			*polls++
			_, _ = w.Write([]byte(`{
				"request_id":"image-1",
				"model":"` + testImageModel + `",
				"status":"success",
				"created_at":1772184500,
				"outcome":{
					"request_id":"upstream-1",
					"media_urls":[{"id":"0","type":"image","url":"https://cdn.example.com/out.png","width":1024,"height":1024}],
					"thumbnail_image_url":"https://cdn.example.com/out.png"
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	return server
}

func newImageGinContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	return c, w
}

func TestHandleImageResponseSubmitSuccessSkipsPolling(t *testing.T) {
	polls := 0
	server := newImageMockServer(t, &polls)
	info := imageInfo(relayconstant.RelayModeImagesGenerations)
	info.ChannelBaseUrl = server.URL

	c, w := newImageGinContext()
	usage, apiErr := handleImageResponse(c, &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"request_id":"image-1",
			"status":"success",
			"created_at":1772184500,
			"outcome":{"media_urls":[{"id":"0","url":"https://cdn.example.com/out.png"}]}
		}`)),
	}, info)
	require.Nil(t, apiErr)
	require.Equal(t, 1, usage.PromptTokens)
	require.Equal(t, 1, usage.TotalTokens)
	require.Zero(t, polls, "提交已返回终态时不应再轮询")
	require.Equal(t, http.StatusOK, w.Code)

	var result dto.ImageResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
	require.EqualValues(t, 1772184500, result.Created)
	require.Len(t, result.Data, 1)
	require.Equal(t, "https://cdn.example.com/out.png", result.Data[0].Url)
}

func TestHandleImageResponsePollsUntilTerminal(t *testing.T) {
	polls := 0
	server := newImageMockServer(t, &polls)
	info := imageInfo(relayconstant.RelayModeImagesGenerations)
	info.ChannelBaseUrl = server.URL

	c, w := newImageGinContext()
	usage, apiErr := handleImageResponse(c, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"image-1","status":"queued"}`)),
	}, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 1, polls)

	var result dto.ImageResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
	require.Equal(t, "https://cdn.example.com/out.png", result.Data[0].Url)
}

func TestHandleImageResponseUpstreamError(t *testing.T) {
	info := imageInfo(relayconstant.RelayModeImagesGenerations)
	c, _ := newImageGinContext()

	_, apiErr := handleImageResponse(c, &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(`{"error":"size must be formatted as {width}x{height}"}`)),
	}, info)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.ErrorContains(t, apiErr, "size must be formatted")
}

func TestGetRequestURLImage(t *testing.T) {
	for _, mode := range []int{relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits} {
		info := imageInfo(mode)
		url, err := (&Adaptor{}).GetRequestURL(info)
		require.NoError(t, err)
		require.Equal(t, defaultRequestQueueBaseURL+submitRequestPath, url)
	}
}

// 图片模型只服务图片端点和 chat 端点，responses 等端点必须本地拒绝。
func TestGetRequestURLRejectsImageModelOnUnsupportedEndpoint(t *testing.T) {
	for _, mode := range []int{relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesInputTokens, relayconstant.RelayModeEmbeddings} {
		info := imageInfo(mode)
		_, err := (&Adaptor{}).GetRequestURL(info)
		require.ErrorContains(t, err, "image model", "relay mode %d", mode)
	}

	info := imageInfo(relayconstant.RelayModeChatCompletions)
	info.OriginModelName = "MiniMaxAI/MiniMax-M2.7"
	info.UpstreamModelName = "MiniMaxAI/MiniMax-M2.7"
	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, defaultLLMBaseURL+"/v1/chat/completions", url)
}

var multipartFormWithFile = multipart.Form{
	File: map[string][]*multipart.FileHeader{
		"image": {{Filename: "cat.png"}},
	},
}
