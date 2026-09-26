package gmicloud

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func chatImageInfo() *relaycommon.RelayInfo {
	// 真实 /v1/chat/completions 入站的 RelayMode 是 Unknown。
	info := newInfo(relayconstant.RelayModeUnknown, types.RelayFormatOpenAI)
	info.OriginModelName = testImageModel
	info.UpstreamModelName = testImageModel
	info.ApiKey = "test-key"
	return info
}

func userTextMessage(text string) dto.Message {
	return dto.Message{Role: "user", Content: text}
}

func userMediaMessage(text string, urls ...string) dto.Message {
	parts := make([]any, 0, len(urls)+1)
	if text != "" {
		parts = append(parts, map[string]any{"type": "text", "text": text})
	}
	for _, url := range urls {
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}})
	}
	return dto.Message{Role: "user", Content: parts}
}

func TestBuildChatImageRequestTextToImage(t *testing.T) {
	info := chatImageInfo()
	converted, err := buildChatImageRequest(nil, info, &dto.GeneralOpenAIRequest{
		Model:    testImageModel,
		Messages: []dto.Message{userTextMessage("a red apple on a wooden table")},
	})
	require.NoError(t, err)

	body := converted.(gmiSubmitRequest)
	require.Equal(t, testImageModel, body.Model)
	require.Equal(t, "a red apple on a wooden table", body.Payload["prompt"])
	require.NotContains(t, body.Payload, "size", "不传 size 即 Auto，由上游自选")
	require.NotContains(t, body.Payload, "image")
	require.Equal(t, 1.0, info.PriceData.OtherRatioMultiplier())
}

func TestBuildChatImageRequestSizeShorthand(t *testing.T) {
	info := chatImageInfo()
	converted, err := buildChatImageRequest(nil, info, &dto.GeneralOpenAIRequest{
		Model:    testImageModel,
		Size:     "4k+16:9",
		Messages: []dto.Message{userTextMessage("sunrise")},
	})
	require.NoError(t, err)
	require.Equal(t, "3840x2160", converted.(gmiSubmitRequest).Payload["size"])
	require.InDelta(t, gmiImageOver2KPriceRatio, info.PriceData.OtherRatioMultiplier(), 1e-9)
}

func TestBuildChatImageRequestImageToImage(t *testing.T) {
	info := chatImageInfo()
	converted, err := buildChatImageRequest(nil, info, &dto.GeneralOpenAIRequest{
		Model: testImageModel,
		Messages: []dto.Message{
			userTextMessage("first turn"),
			userMediaMessage("make it blue", "https://cdn.example.com/a.png", "https://cdn.example.com/b.png"),
		},
	})
	require.NoError(t, err)
	body := converted.(gmiSubmitRequest)
	require.Equal(t, "make it blue", body.Payload["prompt"], "prompt 取最后一条 user 消息")
	require.Equal(t, []string{"https://cdn.example.com/a.png", "https://cdn.example.com/b.png"}, body.Payload["image"])
}

// 参考图散落在更早的消息里也要收齐，prompt 仍取最后一条带文本的 user 消息。
func TestBuildChatImageRequestCollectsReferencesFromEarlierTurns(t *testing.T) {
	converted, err := buildChatImageRequest(nil, chatImageInfo(), &dto.GeneralOpenAIRequest{
		Model: testImageModel,
		Messages: []dto.Message{
			userMediaMessage("draw a cat", "https://cdn.example.com/a.png", "https://cdn.example.com/b.png"),
			dto.Message{Role: "assistant", Content: "here you go"},
			userTextMessage("make it blue"),
		},
	})
	require.NoError(t, err)
	body := converted.(gmiSubmitRequest)
	require.Equal(t, "make it blue", body.Payload["prompt"])
	require.Equal(t, []string{"https://cdn.example.com/a.png", "https://cdn.example.com/b.png"}, body.Payload["image"])
}

func TestBuildChatImageRequestImageURLsDeduped(t *testing.T) {
	converted, err := buildChatImageRequest(nil, chatImageInfo(), &dto.GeneralOpenAIRequest{
		Model: testImageModel,
		Messages: []dto.Message{
			userMediaMessage("", "https://cdn.example.com/a.png"),
			userMediaMessage("make it blue", "https://cdn.example.com/a.png"),
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"https://cdn.example.com/a.png"}, converted.(gmiSubmitRequest).Payload["image"])
}

func TestBuildChatImageRequestSeed(t *testing.T) {
	converted, err := buildChatImageRequest(nil, chatImageInfo(), &dto.GeneralOpenAIRequest{
		Model:    testImageModel,
		Seed:     common.GetPointer(float64(7)),
		Messages: []dto.Message{userTextMessage("x")},
	})
	require.NoError(t, err)
	require.EqualValues(t, 7, converted.(gmiSubmitRequest).Payload["seed"])
}

func TestBuildChatImageRequestExtraBodyPixelBudget(t *testing.T) {
	raw, err := common.Marshal(map[string]any{"generate_max_pixels": 1048576})
	require.NoError(t, err)
	converted, err := buildChatImageRequest(nil, chatImageInfo(), &dto.GeneralOpenAIRequest{
		Model:     testImageModel,
		ExtraBody: raw,
		Messages:  []dto.Message{userTextMessage("x")},
	})
	require.NoError(t, err)
	body := converted.(gmiSubmitRequest)
	require.NotContains(t, body.Payload, "size")
	require.EqualValues(t, 1048576, body.Payload["generate_max_pixels"])

	// 指定 size 时像素预算不参与。
	converted, err = buildChatImageRequest(nil, chatImageInfo(), &dto.GeneralOpenAIRequest{
		Model:     testImageModel,
		Size:      "1024x1024",
		ExtraBody: raw,
		Messages:  []dto.Message{userTextMessage("x")},
	})
	require.NoError(t, err)
	body = converted.(gmiSubmitRequest)
	require.Equal(t, "1024x1024", body.Payload["size"])
	require.NotContains(t, body.Payload, "generate_max_pixels")
}

func TestBuildChatImageRequestRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name    string
		request *dto.GeneralOpenAIRequest
		wantMsg string
	}{
		{
			"流式", &dto.GeneralOpenAIRequest{Model: testImageModel, Stream: common.GetPointer(true), Messages: []dto.Message{userTextMessage("x")}},
			"streaming image generation",
		},
		{
			"n 大于 1", &dto.GeneralOpenAIRequest{Model: testImageModel, N: common.GetPointer(2), Messages: []dto.Message{userTextMessage("x")}},
			"n must be 1",
		},
		{
			"缺 prompt", &dto.GeneralOpenAIRequest{Model: testImageModel, Messages: []dto.Message{{Role: "user", Content: "  "}}},
			"must contain a text prompt",
		},
		{
			"非法 size", &dto.GeneralOpenAIRequest{Model: testImageModel, Size: "3k+16:9", Messages: []dto.Message{userTextMessage("x")}},
			"unsupported size",
		},
		{
			"data URL 参考图", &dto.GeneralOpenAIRequest{Model: testImageModel, Messages: []dto.Message{userMediaMessage("x", "data:image/png;base64,AAAA")}},
			"data URLs are not supported",
		},
		{
			"seed 为小数", &dto.GeneralOpenAIRequest{Model: testImageModel, Seed: common.GetPointer(1.5), Messages: []dto.Message{userTextMessage("x")}},
			"seed must be a non-negative integer",
		},
		{
			"seed 为负", &dto.GeneralOpenAIRequest{Model: testImageModel, Seed: common.GetPointer(-1.0), Messages: []dto.Message{userTextMessage("x")}},
			"seed must be a non-negative integer",
		},
		{
			"参考图超 5 张", &dto.GeneralOpenAIRequest{Model: testImageModel, Messages: []dto.Message{userMediaMessage("x",
				"https://a/1.png", "https://a/2.png", "https://a/3.png", "https://a/4.png", "https://a/5.png", "https://a/6.png")}},
			"at most 5 reference images",
		},
		{
			"非图片模型", &dto.GeneralOpenAIRequest{Model: "MiniMaxAI/MiniMax-M2.7", Messages: []dto.Message{userTextMessage("x")}},
			"does not support /v1/chat/completions",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := chatImageInfo()
			if tc.request.Model != testImageModel {
				info.UpstreamModelName = tc.request.Model
			}
			_, err := buildChatImageRequest(nil, info, tc.request)
			require.ErrorContains(t, err, tc.wantMsg)
			var apiErr *types.NewAPIError
			require.ErrorAs(t, err, &apiErr)
			require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
			require.True(t, types.IsSkipRetryError(apiErr))
		})
	}
}

func TestHandleChatImageResponse(t *testing.T) {
	polls := 0
	server := newImageMockServer(t, &polls)
	info := chatImageInfo()
	info.ChannelBaseUrl = server.URL

	c, w := newImageGinContext()
	usage, apiErr := handleChatImageResponse(c, &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"request_id":"image-1",
			"status":"success",
			"created_at":1772184500,
			"outcome":{"media_urls":[{"id":"0","url":"https://cdn.example.com/out.png"}]}
		}`)),
	}, info)
	require.Nil(t, apiErr)
	require.Equal(t, 1, usage.TotalTokens)
	require.Zero(t, polls)

	var result dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
	require.Equal(t, "chat.completion", result.Object)
	require.Equal(t, testImageModel, result.Model)
	require.Len(t, result.Choices, 1)
	require.Equal(t, "assistant", result.Choices[0].Message.Role)
	require.Equal(t, "stop", result.Choices[0].FinishReason)
	require.Equal(t, "![image](https://cdn.example.com/out.png)", result.Choices[0].Message.Content)
}

func TestGetRequestURLChatImage(t *testing.T) {
	info := chatImageInfo()
	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, defaultRequestQueueBaseURL+submitRequestPath, url)
}
