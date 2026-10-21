package opencode

import (
	"bytes"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenCodeIdentifierMatchesOfficialVectors(t *testing.T) {
	var generator openCodeIDGenerator
	const timestamp = int64(1790064000000)
	random := func() io.Reader { return bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}) }
	message, err := generator.next("msg", false, timestamp, random())
	require.NoError(t, err)
	require.Equal(t, "msg_0c820fc000010123456789ABCD", message)
	session, err := generator.next("ses", true, timestamp, random())
	require.NoError(t, err)
	require.Equal(t, "ses_f37df03ffffd0123456789ABCD", session)
	later, err := generator.next("msg", false, timestamp+1, random())
	require.NoError(t, err)
	require.Equal(t, "msg_0c820fc010010123456789ABCD", later)
	_, err = generator.next("msg", false, timestamp, bytes.NewReader(nil))
	require.ErrorIs(t, err, io.EOF)
}

func TestOpenCodeIdentifierConcurrent(t *testing.T) {
	var generator openCodeIDGenerator
	var wg sync.WaitGroup
	ids := make(chan string, 512)
	for i := 0; i < cap(ids); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := generator.next("ses", true, 1790064000000, rand.Reader)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)
	prefixes := make(map[string]bool)
	for id := range ids {
		require.Regexp(t, openCodeSessionIDPattern, id)
		require.False(t, prefixes[id[:16]], "counter must advance within the same millisecond")
		prefixes[id[:16]] = true
	}
	require.Len(t, prefixes, 512)
}

func TestOpenCodeHeadersPreserveCLIIdentity(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	identity := map[string]string{
		"x-opencode-session": "ses_f525e4699ffe5hmrr6t1FiPVca",
		"x-opencode-project": "project-test",
		"x-opencode-client":  "desktop",
		"User-Agent":         "opencode/latest/2.0.30/cli",
	}
	for name, value := range identity {
		c.Request.Header.Set(name, value)
	}
	c.Request.Header.Set("x-opencode-request", "msg_0ada1b968001cxm5rgh28JxlDH")
	headers := make(http.Header)
	require.NoError(t, applyOpenCodeHeaders(c, &headers, openCodeLaneV2, ""))
	for name, value := range identity {
		require.Equal(t, value, headers.Get(name), name)
	}
	require.Equal(t, identity["x-opencode-session"], headers.Get("x-opencode-session-id"))
	// The legacy correlation header is rejected by the free tier gate and must
	// never be forwarded.
	require.Empty(t, headers.Get("x-opencode-request"))
}

func TestOpenCodeHeadersFallbackAndRetryIsolation(t *testing.T) {
	newContext := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		c.Request.Header.Set("x-opencode-session", "newapi-invalid")
		c.Request.Header.Set("x-opencode-request", "not-a-message-id")
		c.Request.Header.Set("User-Agent", "some-other-client")
		return c
	}
	c := newContext()
	headers := make(http.Header)
	require.NoError(t, applyOpenCodeHeaders(c, &headers, openCodeLaneV2, ""))
	require.Equal(t, openCodeUserAgent, headers.Get("User-Agent"))
	require.Equal(t, "cli", headers.Get("x-opencode-client"))
	require.Equal(t, "global", headers.Get("x-opencode-project"))
	require.Regexp(t, openCodeSessionIDPattern, headers.Get("x-opencode-session"))
	require.Equal(t, headers.Get("x-opencode-session"), headers.Get("x-opencode-session-id"))
	require.Empty(t, headers.Get("x-opencode-request"))
	retry := make(http.Header)
	require.NoError(t, applyOpenCodeHeaders(c, &retry, openCodeLaneV2, ""))
	require.Equal(t, headers, retry)
	// The same signal re-encodes to the same canonical session (prompt-cache
	// affinity), while a different conversation keeps a different one.
	same := make(http.Header)
	require.NoError(t, applyOpenCodeHeaders(newContext(), &same, openCodeLaneV2, ""))
	require.Equal(t, headers.Get("x-opencode-session"), same.Get("x-opencode-session"))
	different := newContext()
	different.Request.Header.Set("x-opencode-session", "newapi-invalid-2")
	other := make(http.Header)
	require.NoError(t, applyOpenCodeHeaders(different, &other, openCodeLaneV2, ""))
	require.NotEqual(t, headers.Get("x-opencode-session"), other.Get("x-opencode-session"))
}

func TestOpenCodeProjectIDSanitizesCanonicalIDs(t *testing.T) {
	require.Equal(t, "global", openCodeProjectID("global"))
	require.Equal(t, "project-test", openCodeProjectID("project-test"))
	hashed := openCodeProjectID("prj_0abc123")
	require.Regexp(t, `^[0-9a-f]{40}$`, hashed)
	require.Equal(t, hashed, openCodeProjectID("prj_0abc123"))
	require.NotEqual(t, hashed, openCodeProjectID("prj_0def456"))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("x-opencode-project", "prj_0abc123")
	headers := make(http.Header)
	require.NoError(t, applyOpenCodeHeaders(c, &headers, openCodeLaneV2, ""))
	require.Equal(t, hashed, headers.Get("x-opencode-project"))
}

func TestOpenCodeGoBasePassthroughsClientHeaders(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("User-Agent", "claude-cli/2.0")
	c.Request.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
	c.Request.Header.Set("x-opencode-session", "ses_f525e4699ffe5hmrr6t1FiPVca")
	c.Request.Header.Set("Authorization", "Bearer client-should-not-win")

	headers := make(http.Header)
	headers.Set("Authorization", "Bearer channel-key")
	require.False(t, IsGoBase("https://opencode.ai/inference"))
	passthroughClientHeaders(c, &headers)
	require.NoError(t, applyOpenCodeHeaders(c, &headers, openCodeLaneGo, ""))

	require.Equal(t, "claude-cli/2.0", headers.Get("User-Agent"))
	require.Equal(t, "prompt-caching-2024-07-31", headers.Get("anthropic-beta"))
	require.Equal(t, "Bearer channel-key", headers.Get("Authorization"))
	require.Equal(t, "ses_f525e4699ffe5hmrr6t1FiPVca", headers.Get("x-opencode-session"))
	require.Len(t, headers.Values("Authorization"), 1)
	require.Len(t, headers.Values("User-Agent"), 1)
}

func TestOpenCodeZenBaseForcesFingerprintUserAgent(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("User-Agent", "claude-cli/2.0")

	headers := make(http.Header)
	require.False(t, IsGoBase("https://opencode.ai/inference"))
	require.True(t, IsGoBase("https://opencode.ai/inference/go"))
	require.NoError(t, applyOpenCodeHeaders(c, &headers, openCodeLaneV2, ""))

	require.Equal(t, openCodeUserAgent, headers.Get("User-Agent"))
	require.Equal(t, "cli", headers.Get("x-opencode-client"))
	require.Equal(t, "application/json", headers.Get("Content-Type"))
}

func TestOpenCodeLaneDetection(t *testing.T) {
	require.Equal(t, openCodeLaneV2, openCodeLaneFor("https://opencode.ai/inference"))
	require.Equal(t, openCodeLaneV2, openCodeLaneFor(""))
	require.Equal(t, openCodeLaneGo, openCodeLaneFor("https://opencode.ai/inference/go"))
	require.Equal(t, openCodeLaneGo, openCodeLaneFor("https://opencode.ai/zen/go"))
	require.Equal(t, openCodeLaneV1, openCodeLaneFor("https://opencode.ai/zen"))
}

func TestOpenCodeSessionEncodingWindow(t *testing.T) {
	// Canonical shape but a foreign (ascending / raw hash) encoding: the free
	// tier rejects it, so it is re-encoded instead of forwarded.
	foreign := "ses_0c820fc000010123456789ABCD"
	require.False(t, openCodeOfficialSessionID(foreign))
	canonical := openCodeCanonicalSessionID("1#2", foreign)
	require.Regexp(t, openCodeSessionIDPattern, canonical)
	require.True(t, openCodeOfficialSessionID(canonical))
	require.Equal(t, canonical, openCodeCanonicalSessionID("1#2", foreign))
	require.NotEqual(t, canonical, openCodeCanonicalSessionID("1#3", foreign))

	// An official client session is preserved for prompt-cache affinity.
	official := "ses_f37df03ffffd0123456789ABCD"
	require.True(t, openCodeOfficialSessionID(official))
	require.Equal(t, official, openCodeCanonicalSessionID("1#2", official))

	// Counter 65 leaves the accepted 1-64 window and must be re-encoded.
	require.False(t, openCodeOfficialSessionID("ses_f37df03fffbf0123456789ABCD"))
}

func TestOpenCodeV2SessionAffinityHeaders(t *testing.T) {
	parent := "ses_f37df03ffffd0123456789ABCD"
	child := "ses_f37df03ffffd123456789ABCDE"
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("x-opencode-session", parent)
	c.Request.Header.Set("x-opencode-session-id", child)
	c.Request.Header.Set("x-parent-session-id", parent)

	headers := make(http.Header)
	require.NoError(t, applyOpenCodeHeaders(c, &headers, openCodeLaneV2, "7#8"))
	require.Equal(t, parent, headers.Get("x-opencode-session"))
	require.Equal(t, parent, headers.Get("x-session-affinity"))
	require.Equal(t, parent, headers.Get("X-Session-Id"))
	require.Equal(t, child, headers.Get("x-opencode-session-id"))
	require.Equal(t, parent, headers.Get("x-opencode-parent-session-id"))
	require.Equal(t, "application/json, text/event-stream", headers.Get("Accept"))
}

func TestOpenCodeV1LaneKeepsFlatSessionHeaders(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("x-opencode-session", "ses_f37df03ffffd0123456789ABCD")
	headers := make(http.Header)
	require.NoError(t, applyOpenCodeHeaders(c, &headers, openCodeLaneV1, ""))
	require.Equal(t, headers.Get("x-opencode-session"), headers.Get("x-opencode-session-id"))
	require.Empty(t, headers.Get("x-session-affinity"))
	require.Empty(t, headers.Get("X-Session-Id"))
}
