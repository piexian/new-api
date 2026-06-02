package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestXAINativeRequestBodyPreservesNonJSONBody(t *testing.T) {
	t.Parallel()

	body := "raw-audio-bytes"
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/stt", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/octet-stream")
	info := &relaycommon.RelayInfo{}

	reader, closer, err := xAINativeRequestBody(c, info)
	if closer != nil {
		defer closer.Close()
	}

	require.NoError(t, err)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, body, string(got))
	require.Equal(t, int64(len(body)), info.UpstreamRequestBodySize)
}

func TestXAINativeRequestBodyUsesEmptyReaderForGET(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tts/voices", nil)
	info := &relaycommon.RelayInfo{}

	reader, closer, err := xAINativeRequestBody(c, info)

	require.NoError(t, err)
	require.Nil(t, closer)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Zero(t, info.UpstreamRequestBodySize)
}

func TestXAINativeProxyWebSocketForwardsTextAndBinaryFrames(t *testing.T) {
	gin.SetMode(gin.TestMode)

	clientServerConn, downstreamClientConn, cleanupClient := newTestWebSocketPair(t)
	defer cleanupClient()
	targetServerConn, targetDialerConn, cleanupTarget := newTestWebSocketPair(t)
	defer cleanupTarget()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/stt?sample_rate=16000", nil)
	info := &relaycommon.RelayInfo{
		ClientWs: clientServerConn,
		TargetWs: targetDialerConn,
	}

	done := make(chan struct{})
	go func() {
		xAINativeProxyWebSocket(c, info)
		close(done)
	}()

	require.NoError(t, downstreamClientConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"audio.done"}`)))
	messageType, message, err := targetServerConn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, messageType)
	require.Equal(t, `{"type":"audio.done"}`, string(message))

	require.NoError(t, targetServerConn.WriteMessage(websocket.BinaryMessage, []byte{0x01, 0x02, 0x03}))
	messageType, message, err = downstreamClientConn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.BinaryMessage, messageType)
	require.Equal(t, []byte{0x01, 0x02, 0x03}, message)

	require.NoError(t, downstreamClientConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")))
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("xAI native websocket proxy did not exit after client close")
	}
}

func newTestWebSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	acceptedCh := make(chan *websocket.Conn, 1)
	errCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			errCh <- err
			return
		}
		acceptedCh <- conn
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialerConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	var serverConn *websocket.Conn
	select {
	case serverConn = <-acceptedCh:
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for websocket server connection")
	}

	cleanup := func() {
		_ = dialerConn.Close()
		_ = serverConn.Close()
		server.Close()
	}
	return serverConn, dialerConn, cleanup
}
