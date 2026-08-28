package gmicloud

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func nativeMusicInfo(outputFormat string) *relaycommon.RelayInfo {
	info := newInfo(relayconstant.RelayModeMiniMaxMusicGeneration, types.RelayFormatMiniMax)
	info.OriginModelName = "minimax-music-3.0"
	info.ApiKey = "test-key"
	info.Request = &dto.MiniMaxMusicGenerationRequest{
		Model:        "minimax-music-3.0",
		OutputFormat: outputFormat,
	}
	return info
}

func TestBuildMiniMaxMusicRequestBody(t *testing.T) {
	info := nativeMusicInfo("url")
	reader, err := buildMiniMaxMusicRequestBody(info, strings.NewReader(`{
		"model":"music-3.0",
		"prompt":"indie folk",
		"lyrics":"[Verse]\\nHello world",
		"output_format":"url",
		"audio_setting":{"sample_rate":44100,"bitrate":256000,"format":"wav"}
	}`))
	require.NoError(t, err)

	body := readRequestJSON(t, reader)
	require.Equal(t, "minimax-music-3.0", body["model"])
	payload := body["payload"].(map[string]any)
	require.Equal(t, "[Verse]\\nHello world", payload["lyrics"])
	require.Equal(t, "indie folk", payload["prompt"])
	require.EqualValues(t, 44100, payload["sample_rate"])
	require.EqualValues(t, 256000, payload["bitrate"])
	require.Equal(t, "wav", payload["format"])
	require.NotContains(t, payload, "output_format")
}

// 合法性校验统一在 ValidateMiniMaxMusicRequest；handler 侧先校验，build 只做转换。
func TestValidateMiniMaxMusicRequest(t *testing.T) {
	require.NoError(t, ValidateMiniMaxMusicRequest(&dto.MiniMaxMusicGenerationRequest{Lyrics: "song", OutputFormat: "hex"}))
	require.NoError(t, ValidateMiniMaxMusicRequest(&dto.MiniMaxMusicGenerationRequest{Lyrics: "song", OutputFormat: "url"}))
	require.ErrorContains(t, ValidateMiniMaxMusicRequest(&dto.MiniMaxMusicGenerationRequest{Lyrics: "song", Stream: true}), "streaming")
	require.ErrorContains(t, ValidateMiniMaxMusicRequest(&dto.MiniMaxMusicGenerationRequest{Lyrics: "song", AudioURL: "https://example.com/audio.mp3"}), "cover")
	require.ErrorContains(t, ValidateMiniMaxMusicRequest(&dto.MiniMaxMusicGenerationRequest{Lyrics: "song", OutputFormat: "mp3"}), "output_format")
	require.ErrorContains(t, ValidateMiniMaxMusicRequest(&dto.MiniMaxMusicGenerationRequest{Lyrics: "  "}), "lyrics")
}

func newMusicMockServer(t *testing.T, downloads *int) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/ie/requestqueue/apikey/requests/") && r.Method == http.MethodGet:
			require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			body := strings.ReplaceAll(`{

				"request_id":"music-1",

				"model":"minimax-music-3.0",

				"status":"success",

				"outcome":{

					"audio_url":"SERVER_URL/music.mp3",

					"duration_ms":25364,

					"sample_rate":44100,

					"channels":2,

					"bitrate":256000

				}

			}`, "SERVER_URL", server.URL)
			_, _ = w.Write([]byte(body))
		case r.URL.Path == "/music.mp3":
			*downloads++
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("music-bytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newMusicGinContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/music_generation", nil)
	return c, w
}

// allowPrivateDownloadFetch 放行 SSRF 防护下的回环下载（httptest），并确保受保护客户端已初始化。
func allowPrivateDownloadFetch(t *testing.T) {
	t.Helper()
	service.InitHttpClient()
	setting := system_setting.GetFetchSetting()
	origAllowPrivateIp, origPorts := setting.AllowPrivateIp, setting.AllowedPorts
	setting.AllowPrivateIp = true
	setting.AllowedPorts = []string{} // httptest 随机端口
	t.Cleanup(func() {
		setting.AllowPrivateIp = origAllowPrivateIp
		setting.AllowedPorts = origPorts
	})
}

func TestHandleMiniMaxMusicResponseURL(t *testing.T) {
	downloads := 0
	server := newMusicMockServer(t, &downloads)
	info := nativeMusicInfo("url")
	info.ChannelBaseUrl = server.URL
	info.SetEstimatePromptTokens(12)

	c, w := newMusicGinContext()
	usage, apiErr := handleMiniMaxMusicResponse(c, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"music-1","status":"success"}`)),
	}, info)
	require.Nil(t, apiErr)
	require.Equal(t, 12, usage.PromptTokens)
	require.Zero(t, downloads)
	require.Equal(t, http.StatusOK, w.Code)

	var result miniMaxMusicResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
	require.Equal(t, server.URL+"/music.mp3", result.Data.Audio)
	require.Equal(t, 2, result.Data.Status)
	require.Equal(t, "music-1", result.TraceID)
	require.EqualValues(t, 25364, result.ExtraInfo.MusicDuration)
	require.EqualValues(t, 44100, result.ExtraInfo.MusicSampleRate)
	require.Equal(t, 2, result.ExtraInfo.MusicChannel)
	require.EqualValues(t, 256000, result.ExtraInfo.Bitrate)
	require.EqualValues(t, 0, result.BaseResp.StatusCode)
	require.Equal(t, "success", result.BaseResp.StatusMsg)
}

func TestHandleMiniMaxMusicResponseHex(t *testing.T) {
	allowPrivateDownloadFetch(t)
	downloads := 0
	server := newMusicMockServer(t, &downloads)
	info := nativeMusicInfo("")
	info.ChannelBaseUrl = server.URL

	c, w := newMusicGinContext()
	usage, apiErr := handleMiniMaxMusicResponse(c, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"music-1","status":"success"}`)),
	}, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 1, downloads)

	var result miniMaxMusicResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
	require.Equal(t, "6d757369632d6279746573", result.Data.Audio)
	require.EqualValues(t, len("music-bytes"), result.ExtraInfo.MusicSize)
}
