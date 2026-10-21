package opencode

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

func TestFreeNetworkAbuseBlockClassification(t *testing.T) {
	require.True(t, isFreeNetworkAbuseBlock(http.StatusForbidden, []byte("Forbidden, OpenCode Free Model Gateway Detect Abuse Traffic")))
	require.True(t, isFreeNetworkAbuseBlock(http.StatusForbidden, []byte("Abuse of traffic detected from this network")))
	require.True(t, isFreeNetworkAbuseBlock(http.StatusForbidden, []byte("Free models aren't available from your current network because we've detected abusive traffic from this IP address")))
	// The request-shape rejection is a different error and must stay retryable.
	require.False(t, isFreeNetworkAbuseBlock(http.StatusForbidden, []byte("free tier can only be used from within OpenCode")))
	require.False(t, isFreeNetworkAbuseBlock(http.StatusTooManyRequests, []byte("abusive traffic")))
}

func TestFreeExitCooldownGrowsAndBreaks(t *testing.T) {
	resetFreeExitState()
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelId:      4242,
		ChannelSetting: dto.ChannelSettings{Proxy: "http://127.0.0.1:9"},
	}}
	exit := freeExitFor(info)
	now := time.Now()

	exit.noteFailure(now, 0)
	require.Error(t, exit.available(now.Add(time.Second)))
	require.NoError(t, exit.available(now.Add(freeExitFailureBase+time.Second)))

	exit.noteFailure(now, 0)
	require.Error(t, exit.available(now.Add(freeExitFailureBase+time.Second)))
	require.NoError(t, exit.available(now.Add(2*freeExitFailureBase+time.Second)))

	// An upstream Retry-After lengthens the cooldown instead of shortening it.
	exit.noteFailure(now, freeExitFailureMax)
	require.Error(t, exit.available(now.Add(freeExitFailureMax-time.Second)))

	// A run of consecutive failures trips the lane breaker.
	for i := 0; i < freeLaneBreakerThreshold; i++ {
		exit.noteFailure(now, 0)
	}
	require.ErrorIs(t, exit.available(now.Add(freeExitFailureMax+time.Second)), errFreeExitPaused)

	// A success clears cooldown, breaker strikes and the failure run.
	exit.noteSuccess()
	require.NoError(t, exit.available(now))
}

func TestFreeExitIPBlockStopsTheExit(t *testing.T) {
	resetFreeExitState()
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 77}}
	exit := freeExitFor(info)
	now := time.Now()
	body := `{"error":{"message":"Free models aren't available from your current network because we've detected abusive traffic from this IP address"}}`
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	require.ErrorIs(t, noteFreeExitResult(exit, resp, nil), errFreeExitBlocked)
	require.ErrorIs(t, exit.available(now.Add(time.Minute)), errFreeExitBlocked)
	require.NoError(t, exit.available(now.Add(freeExitBlockTTL+time.Minute)))
}

func TestFreeExitClassifiesTransientFailures(t *testing.T) {
	resetFreeExitState()
	exit := freeExitFor(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 78}})
	now := time.Now()

	require.ErrorIs(t, noteFreeExitResult(exit, nil, io.ErrUnexpectedEOF), io.ErrUnexpectedEOF)
	require.Error(t, exit.available(now.Add(time.Second)))

	// A plain upstream 400 is a request problem: the exit stays healthy.
	resetFreeExitState()
	exit = freeExitFor(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 79}})
	resp := &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("invalid request"))}
	require.NoError(t, noteFreeExitResult(exit, resp, nil))
	require.NoError(t, exit.available(now))
}

func TestBunClientHelloMatchesBunShape(t *testing.T) {
	spec := bunClientHello()
	require.Len(t, spec.CipherSuites, 17)
	alpn := ""
	keyShares := 0
	for _, extension := range spec.Extensions {
		switch value := extension.(type) {
		case *utls.ALPNExtension:
			require.Len(t, value.AlpnProtocols, 1)
			alpn = value.AlpnProtocols[0]
		case *utls.KeyShareExtension:
			keyShares = len(value.KeyShares)
			require.Equal(t, utls.X25519MLKEM768, value.KeyShares[0].Group)
		}
	}
	require.Equal(t, "http/1.1", alpn)
	require.Equal(t, 2, keyShares)
	// No GREASE values: Bun's BoringSSL does not send them.
	for _, suite := range spec.CipherSuites {
		require.NotEqual(t, uint16(0x0a0a), suite&0x0f0f)
	}
}

func TestFreeProxyURLMarkers(t *testing.T) {
	parsed, err := parseFreeProxyURL("pool+http://user:pass@127.0.0.1:7890")
	require.NoError(t, err)
	require.Equal(t, "http", parsed.Scheme)
	require.Equal(t, "127.0.0.1:7890", parsed.Host)

	_, err = parseFreeProxyURL("socks5h://127.0.0.1:1080#pool")
	require.NoError(t, err)

	_, err = parseFreeProxyURL("ftp://127.0.0.1:21")
	require.Error(t, err)
}
