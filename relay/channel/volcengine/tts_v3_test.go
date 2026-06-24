package volcengine

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenSpeechTTSV3BaseDetectionAndURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		wantURL string
	}{
		{
			name:    "alias",
			baseURL: openSpeechTTSV3BaseAlias,
			wantURL: openSpeechTTSV3DefaultURL,
		},
		{
			name:    "root openspeech host",
			baseURL: "https://openspeech.bytedance.com",
			wantURL: openSpeechTTSV3DefaultURL,
		},
		{
			name:    "full unidirectional url",
			baseURL: "https://openspeech.bytedance.com/api/v3/tts/unidirectional/sse",
			wantURL: "https://openspeech.bytedance.com/api/v3/tts/unidirectional/sse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.True(t, isOpenSpeechTTSV3Base(tt.baseURL))

			info := &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeAudioSpeech,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: tt.baseURL,
					ChannelType:    channelconstant.ChannelTypeVolcEngine,
				},
			}
			url, err := (&Adaptor{}).GetRequestURL(info)

			require.NoError(t, err)
			require.Equal(t, tt.wantURL, url)
		})
	}
}

func TestConvertAudioRequestUsesOpenSpeechTTSV3(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	speed := 1.1
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: openSpeechTTSV3BaseAlias,
			ChannelType:    channelconstant.ChannelTypeVolcEngine,
		},
	}
	request := dto.AudioRequest{
		Input:          "你好",
		Voice:          "nova",
		ResponseFormat: "opus",
		Speed:          &speed,
		Metadata:       []byte(`{"resource_id":"seed-tts-1.0-concurr"}`),
	}

	reader, err := (&Adaptor{}).ConvertAudioRequest(c, info, request)

	require.NoError(t, err)
	require.False(t, info.IsStream)
	require.True(t, isOpenSpeechTTSV3Context(c))
	require.Equal(t, "seed-tts-1.0-concurr", c.GetString(contextKeyTTSOpenSpeechV3Resource))

	body, err := io.ReadAll(reader)
	require.NoError(t, err)

	var converted volcengineTTSV3Request
	require.NoError(t, common.Unmarshal(body, &converted))
	require.Equal(t, "openai_relay_user", converted.User.UID)
	require.Equal(t, "你好", converted.ReqParams.Text)
	require.Equal(t, "zh_female_shuangkuaisisi_mars_bigtts", converted.ReqParams.Speaker)
	require.Equal(t, "ogg_opus", converted.ReqParams.AudioParams.Format)
	require.Equal(t, 24000, converted.ReqParams.AudioParams.SampleRate)
	require.NotNil(t, converted.ReqParams.SpeedRatio)
	require.Equal(t, speed, *converted.ReqParams.SpeedRatio)
}

func TestSetupRequestHeaderUsesOpenSpeechTTSV3Auth(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		apiKey    string
		wantKey   string
		wantAppID string
		wantToken string
	}{
		{
			name:      "legacy app id access key",
			apiKey:    "app-id|access-key",
			wantAppID: "app-id",
			wantToken: "access-key",
		},
		{
			name:    "new console api key",
			apiKey:  "volc-api-key",
			wantKey: "volc-api-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
			c.Set(contextKeyTTSOpenSpeechV3, true)
			c.Set(contextKeyTTSOpenSpeechV3Resource, "seed-tts-2.0")
			headers := make(http.Header)

			err := (&Adaptor{}).SetupRequestHeader(c, &headers, &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeAudioSpeech,
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiKey: tt.apiKey,
				},
			})

			require.NoError(t, err)
			require.Equal(t, "application/json", headers.Get("Content-Type"))
			require.Equal(t, "seed-tts-2.0", headers.Get("X-Api-Resource-Id"))
			require.NotEmpty(t, headers.Get("X-Api-Request-Id"))
			if tt.wantKey != "" {
				require.Equal(t, tt.wantKey, headers.Get("X-Api-Key"))
				require.Empty(t, headers.Get("X-Api-App-Id"))
				require.Empty(t, headers.Get("X-Api-Access-Key"))
				return
			}
			require.Equal(t, tt.wantAppID, headers.Get("X-Api-App-Id"))
			require.Equal(t, tt.wantToken, headers.Get("X-Api-Access-Key"))
			require.Equal(t, "Bearer;"+tt.wantToken, headers.Get("Authorization"))
		})
	}
}

func TestHandleTTSV3NdjsonResponseWritesAudio(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c := gin.CreateTestContextOnly(recorder, gin.New())
	audio := []byte("audio-bytes")
	body := strings.Join([]string{
		"event: audio",
		`{"code":0,"data":"` + base64.StdEncoding.EncodeToString(audio) + `"}`,
		"id: 1",
		`data: {"code":"20000000"}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	usage, apiErr := handleTTSV3NdjsonResponse(c, resp, &relaycommon.RelayInfo{}, "mp3")

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "audio/mpeg", recorder.Header().Get("Content-Type"))
	require.Equal(t, audio, recorder.Body.Bytes())
}
