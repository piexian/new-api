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
			id, err := generator.next("msg", false, 1790064000000, rand.Reader)
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
		require.Regexp(t, openCodeRequestIDPattern, id)
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
		"x-opencode-request": "msg_0ada1b968001cxm5rgh28JxlDH",
		"x-opencode-project": "project-test",
		"x-opencode-client":  "desktop",
		"User-Agent":         "opencode/1.18.31 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14",
	}
	for name, value := range identity {
		c.Request.Header.Set(name, value)
	}
	headers := make(http.Header)
	require.NoError(t, setupOpenCodeHeaders(c, &headers))
	for name, value := range identity {
		require.Equal(t, value, headers.Get(name), name)
	}
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
	require.NoError(t, setupOpenCodeHeaders(c, &headers))
	require.Equal(t, openCodeUserAgent, headers.Get("User-Agent"))
	require.Equal(t, "cli", headers.Get("x-opencode-client"))
	require.Equal(t, "global", headers.Get("x-opencode-project"))
	require.Regexp(t, openCodeSessionIDPattern, headers.Get("x-opencode-session"))
	require.Regexp(t, openCodeRequestIDPattern, headers.Get("x-opencode-request"))
	retry := make(http.Header)
	require.NoError(t, setupOpenCodeHeaders(c, &retry))
	require.Equal(t, headers, retry)
	other := make(http.Header)
	require.NoError(t, setupOpenCodeHeaders(newContext(), &other))
	require.NotEqual(t, headers.Get("x-opencode-session"), other.Get("x-opencode-session"))
	require.NotEqual(t, headers.Get("x-opencode-request"), other.Get("x-opencode-request"))
}
