package xai

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestPreservesXAIExtrasAndExplicitZero(t *testing.T) {
	t.Parallel()

	n := uint(0)
	request := dto.ImageRequest{
		Prompt: "a neon skyline",
		N:      &n,
		Size:   "1792x1024",
		Extra: map[string]json.RawMessage{
			"seed":            []byte(`7`),
			"negative_prompt": []byte(`"low quality"`),
		},
		ExtraFields: []byte(`{
			"aspect_ratio": "4:3",
			"resolution": "2K",
			"seed": 9,
			"style_preset": "cinematic"
		}`),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-imagine-image-quality",
		},
	}

	got, err := convertImageRequest(nil, info, request)

	require.NoError(t, err)
	payload := decodeAnyMap(t, got)
	require.Equal(t, "grok-imagine-image-quality", payload["model"])
	require.Equal(t, "a neon skyline", payload["prompt"])
	require.Equal(t, float64(0), payload["n"])
	require.Equal(t, "4:3", payload["aspect_ratio"])
	require.Equal(t, "2K", payload["resolution"])
	require.Equal(t, float64(7), payload["seed"], "Extra should win over ExtraFields")
	require.Equal(t, "low quality", payload["negative_prompt"])
	require.Equal(t, "cinematic", payload["style_preset"])
}

func TestBuildImageLogDetailsIncludesXAINativeFields(t *testing.T) {
	t.Parallel()

	n := uint(0)
	details := BuildImageLogDetails(dto.ImageRequest{
		Model:  "grok-imagine-image-quality",
		Prompt: "a neon skyline",
		N:      &n,
		Size:   "1280x720",
		Extra: map[string]json.RawMessage{
			"resolution":      []byte(`"720p"`),
			"seed":            []byte(`7`),
			"negative_prompt": []byte(`"low quality"`),
			"image_url":       []byte(`"data:image/png;base64,abcdef"`),
		},
	}, nil, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-imagine-image-quality",
		},
	})

	require.Equal(t, "xai", details["provider"])
	require.Equal(t, "image", details["type"])
	require.Equal(t, "grok-imagine-image-quality", details["model"])
	require.Equal(t, "16:9", details["aspect_ratio"])
	require.Equal(t, "720p", details["resolution"])
	require.Equal(t, float64(0), details["n"])
	require.Equal(t, float64(7), details["seed"])
	require.Equal(t, "low quality", details["negative_prompt"])
	image, ok := details["image"].(map[string]any)
	require.True(t, ok, "image = %T", details["image"])
	require.Equal(t, "data:image/png;base64,<omitted>", image["url"])
	require.NotContains(t, details, "prompt")
}

func TestConvertImageRequestAcceptsXAIOfficialImageParams(t *testing.T) {
	t.Parallel()

	got, err := convertImageRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}, dto.ImageRequest{
		Model:  "grok-imagine-image-quality",
		Prompt: "make it a sketch",
		Extra: map[string]json.RawMessage{
			"image_url":    []byte(`"https://example.com/source.png"`),
			"image_format": []byte(`"base64"`),
		},
	})

	require.NoError(t, err)
	payload := decodeAnyMap(t, got)
	image, ok := payload["image"].(map[string]any)
	require.True(t, ok, "image = %T", payload["image"])
	require.Equal(t, "image_url", image["type"])
	require.Equal(t, "https://example.com/source.png", image["url"])
	require.Equal(t, "b64_json", payload["response_format"])
	require.NotContains(t, payload, "image_url")
	require.NotContains(t, payload, "image_format")
}

func TestConvertImageRequestAcceptsXAIOfficialImageURLsParam(t *testing.T) {
	t.Parallel()

	got, err := convertImageRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}, dto.ImageRequest{
		Model:  "grok-imagine-image-quality",
		Prompt: "combine these",
		Extra: map[string]json.RawMessage{
			"image_urls": []byte(`["https://example.com/one.png","https://example.com/two.png"]`),
		},
	})

	require.NoError(t, err)
	payload := decodeAnyMap(t, got)
	images, ok := payload["images"].([]any)
	require.True(t, ok, "images = %T", payload["images"])
	require.Len(t, images, 2)
	first, ok := images[0].(map[string]any)
	require.True(t, ok, "images[0] = %T", images[0])
	require.Equal(t, "image_url", first["type"])
	require.Equal(t, "https://example.com/one.png", first["url"])
	require.NotContains(t, payload, "image_urls")
}

func TestConvertImageEditMultipartConvertsFileToXAIJSONSource(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="image"; filename="source.png"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0})
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}

	got, err := convertImageRequest(c, info, dto.ImageRequest{
		Model:  "grok-imagine-image-quality",
		Prompt: "make it a sketch",
	})

	require.NoError(t, err)
	payload := decodeAnyMap(t, got)
	image, ok := payload["image"].(map[string]any)
	require.True(t, ok, "image = %T", payload["image"])
	require.Equal(t, "image_url", image["type"])
	require.True(t, strings.HasPrefix(image["url"].(string), "data:image/png;base64,"))
}

func TestConvertImageEditMultipartAcceptsXAIOfficialImageURLField(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("image_url", "https://example.com/source.png"))
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}

	got, err := convertImageRequest(c, info, dto.ImageRequest{
		Model:  "grok-imagine-image-quality",
		Prompt: "make it a sketch",
	})

	require.NoError(t, err)
	payload := decodeAnyMap(t, got)
	image, ok := payload["image"].(map[string]any)
	require.True(t, ok, "image = %T", payload["image"])
	require.Equal(t, "image_url", image["type"])
	require.Equal(t, "https://example.com/source.png", image["url"])
}

func TestConvertImageRequestAcceptsOfficialXAIImageFields(t *testing.T) {
	t.Parallel()

	got, err := convertImageRequest(nil, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-imagine-image-quality",
		},
	}, dto.ImageRequest{
		Prompt: "combine these references",
		Extra: map[string]json.RawMessage{
			"image_url":    []byte(`"https://example.com/source.png"`),
			"image_urls":   []byte(`["https://example.com/ref-1.png","https://example.com/ref-2.png"]`),
			"image_format": []byte(`"base64"`),
		},
	})

	require.NoError(t, err)
	payload := decodeAnyMap(t, got)
	require.Equal(t, "grok-imagine-image-quality", payload["model"])
	require.Equal(t, "b64_json", payload["response_format"])
	require.NotContains(t, payload, "image_url")
	require.NotContains(t, payload, "image_urls")
	require.NotContains(t, payload, "image_format")

	image, ok := payload["image"].(map[string]any)
	require.True(t, ok, "image = %T", payload["image"])
	require.Equal(t, "image_url", image["type"])
	require.Equal(t, "https://example.com/source.png", image["url"])

	images, ok := payload["images"].([]any)
	require.True(t, ok, "images = %T", payload["images"])
	require.Len(t, images, 2)
	first, ok := images[0].(map[string]any)
	require.True(t, ok, "images[0] = %T", images[0])
	require.Equal(t, "image_url", first["type"])
	require.Equal(t, "https://example.com/ref-1.png", first["url"])
}

func TestSetupRequestHeaderForXAIImagesUsesJSONContentType(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	header := http.Header{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "test-key",
		},
	}

	err := (&Adaptor{}).SetupRequestHeader(c, &header, info)

	require.NoError(t, err)
	require.Equal(t, "Bearer test-key", header.Get("Authorization"))
	require.Equal(t, "application/json", header.Get("Content-Type"))
}

func TestConvertTTSRequestPreservesNativeXAIFields(t *testing.T) {
	t.Parallel()

	speed := 0.0
	got, err := convertTTSRequest(dto.AudioRequest{
		Input:          "hello",
		Voice:          "eve",
		ResponseFormat: "mp3",
		Speed:          &speed,
		Metadata: []byte(`{
			"language": "en",
			"output_format": {"codec": "wav", "sample_rate": 24000},
			"text_normalization": true
		}`),
	})

	require.NoError(t, err)
	require.Equal(t, "hello", got["text"])
	require.Equal(t, "eve", got["voice_id"])
	require.Equal(t, float64(0), got["speed"])
	require.Equal(t, "en", got["language"])
	require.Equal(t, true, got["text_normalization"])
	outputFormat, ok := got["output_format"].(map[string]any)
	require.True(t, ok, "output_format = %T", got["output_format"])
	require.Equal(t, "wav", outputFormat["codec"])
	require.Equal(t, float64(24000), outputFormat["sample_rate"])
}

func TestGetRequestURLMapsXAIAudioAndRealtimeEndpoints(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	for _, test := range []struct {
		name string
		mode int
		base string
		path string
		want string
	}{
		{
			name: "tts",
			mode: relayconstant.RelayModeAudioSpeech,
			base: "https://api.x.ai",
			path: "/v1/audio/speech",
			want: "https://api.x.ai/v1/tts",
		},
		{
			name: "stt transcription",
			mode: relayconstant.RelayModeAudioTranscription,
			base: "https://api.x.ai",
			path: "/v1/audio/transcriptions",
			want: "https://api.x.ai/v1/stt",
		},
		{
			name: "realtime websocket",
			mode: relayconstant.RelayModeRealtime,
			base: "https://api.x.ai",
			path: "/v1/realtime?model=grok-voice-latest",
			want: "wss://api.x.ai/v1/realtime?model=grok-voice-latest",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			info := &relaycommon.RelayInfo{
				RelayMode:      test.mode,
				RequestURLPath: test.path,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: test.base,
					ChannelType:    appconstant.ChannelTypeXai,
				},
			}

			got, err := adaptor.GetRequestURL(info)

			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestGetRequestURLPassesThroughXAINativeEndpoint(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeXAINative,
		RequestURLPath: "/v1/tts/voices/eve",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.x.ai",
			ChannelType:    appconstant.ChannelTypeXai,
		},
	}

	got, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	require.Equal(t, "https://api.x.ai/v1/tts/voices/eve", got)
}

func TestGetRequestURLUsesWebSocketSchemeForXAINativeRealtime(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		path string
		want string
	}{
		{
			name: "stt streaming",
			path: "/v1/stt?sample_rate=16000&encoding=pcm",
			want: "wss://api.x.ai/v1/stt?sample_rate=16000&encoding=pcm",
		},
		{
			name: "responses websocket mode",
			path: "/v1/responses",
			want: "wss://api.x.ai/v1/responses",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			info := &relaycommon.RelayInfo{
				RelayMode:      relayconstant.RelayModeXAINative,
				RelayFormat:    types.RelayFormatXAIRealtime,
				RequestURLPath: test.path,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: "https://api.x.ai",
					ChannelType:    appconstant.ChannelTypeXai,
				},
			}

			got, err := (&Adaptor{}).GetRequestURL(info)

			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestSetupRequestHeaderForXAINativeOmitsEmptyContentHeaders(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tts/voices", nil)
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeXAINative,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "test-key",
		},
	}
	header := http.Header{}

	err := (&Adaptor{}).SetupRequestHeader(c, &header, info)

	require.NoError(t, err)
	require.Equal(t, "Bearer test-key", header.Get("Authorization"))
	require.Empty(t, header.Values("Content-Type"))
	require.Empty(t, header.Values("Accept"))
}

func decodeAnyMap(t *testing.T, value any) map[string]any {
	t.Helper()

	data, err := common.Marshal(value)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	return payload
}
