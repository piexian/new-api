package opencode

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/gin-gonic/gin"
)

const openCodeUserAgent = "opencode/1.19.0 ai-sdk/provider-utils/4.0.51 runtime/bun/1.3.14"
const openCodeIDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var openCodeSessionIDPattern = regexp.MustCompile(`^ses_[0-9a-f]{12}[0-9A-Za-z]{14}$`)
var openCodeRequestIDPattern = regexp.MustCompile(`^msg_[0-9a-f]{12}[0-9A-Za-z]{14}$`)
var openCodeIDs openCodeIDGenerator

type openCodeIDGenerator struct {
	mu            sync.Mutex
	lastTimestamp int64
	counter       uint64
}

// next matches OpenCode v1.19.0 packages/schema/src/identifier.ts.
func (g *openCodeIDGenerator) next(prefix string, descending bool, timestamp int64, random io.Reader) (string, error) {
	g.mu.Lock()
	if timestamp != g.lastTimestamp {
		g.lastTimestamp = timestamp
		g.counter = 0
	}
	g.counter++
	value := uint64(timestamp)*0x1000 + g.counter
	g.mu.Unlock()
	if descending {
		value = ^value
	}
	var timeBytes [6]byte
	for i := range timeBytes {
		timeBytes[i] = byte(value >> uint(40-8*i))
	}
	var suffix [14]byte
	if _, err := io.ReadFull(random, suffix[:]); err != nil {
		return "", err
	}
	for i, b := range suffix {
		suffix[i] = openCodeIDAlphabet[int(b)%len(openCodeIDAlphabet)]
	}
	return prefix + "_" + hex.EncodeToString(timeBytes[:]) + string(suffix[:]), nil
}

func openCodeRequestIdentifier(c *gin.Context, header, prefix string, descending bool, pattern *regexp.Regexp) (string, error) {
	if value := strings.TrimSpace(c.Request.Header.Get(header)); pattern.MatchString(value) {
		return value, nil
	}
	// Context-local IDs survive retries without merging unrelated conversations.
	key := "opencode.generated." + header
	if value := c.GetString(key); value != "" {
		return value, nil
	}
	value, err := openCodeIDs.next(prefix, descending, time.Now().UnixMilli(), rand.Reader)
	if err != nil {
		return "", err
	}
	c.Set(key, value)
	return value, nil
}

// passthroughClientHeaders forwards safe client headers verbatim. Go channels
// use it by default so upstream prompt-cache affinity (session, betas, client
// identity headers) survives the gateway.
func passthroughClientHeaders(c *gin.Context, headers *http.Header) {
	if c == nil || c.Request == nil || headers == nil {
		return
	}
	for name, values := range c.Request.Header {
		if channel.ShouldSkipPassthroughHeader(name) || len(values) == 0 {
			continue
		}
		if strings.TrimSpace(values[0]) == "" || headers.Get(name) != "" {
			continue
		}
		for _, value := range values {
			headers.Add(name, value)
		}
	}
}

// setupOpenCodeHeaders installs the OpenCode session/request identifiers and,
// when forceFingerprint is set (Zen), normalizes client/project/UA to the CLI
// fingerprint — Zen free models are restricted to the OpenCode client, so the
// gateway must present the official fingerprint regardless of the caller.
func setupOpenCodeHeaders(c *gin.Context, headers *http.Header, forceFingerprint bool) error {
	session, err := openCodeRequestIdentifier(c, "x-opencode-session", "ses", true, openCodeSessionIDPattern)
	if err != nil {
		return err
	}
	request, err := openCodeRequestIdentifier(c, "x-opencode-request", "msg", false, openCodeRequestIDPattern)
	if err != nil {
		return err
	}
	headers.Set("x-opencode-session", session)
	headers.Set("x-opencode-request", request)
	for name, fallback := range map[string]string{
		"x-opencode-client":  "cli",
		"x-opencode-project": "global",
	} {
		value := strings.TrimSpace(c.Request.Header.Get(name))
		if value == "" {
			value = fallback
		}
		headers.Set(name, value)
	}
	userAgent := strings.TrimSpace(c.Request.Header.Get("User-Agent"))
	if userAgent == "" || (forceFingerprint && !strings.HasPrefix(userAgent, "opencode/")) {
		userAgent = openCodeUserAgent
	}
	headers.Set("User-Agent", userAgent)
	if headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}
	return nil
}
