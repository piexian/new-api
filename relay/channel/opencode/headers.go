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

	"github.com/gin-gonic/gin"
)

const openCodeUserAgent = "opencode/1.18.31 ai-sdk/provider-utils/4.0.40 runtime/bun/1.3.14"
const openCodeIDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var openCodeSessionIDPattern = regexp.MustCompile(`^ses_[0-9a-f]{12}[0-9A-Za-z]{14}$`)
var openCodeRequestIDPattern = regexp.MustCompile(`^msg_[0-9a-f]{12}[0-9A-Za-z]{14}$`)
var openCodeIDs openCodeIDGenerator

type openCodeIDGenerator struct {
	mu            sync.Mutex
	lastTimestamp int64
	counter       uint64
}

// next matches OpenCode v1.18.31 packages/schema/src/identifier.ts.
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

func setupOpenCodeHeaders(c *gin.Context, headers *http.Header) error {
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
	if !strings.HasPrefix(userAgent, "opencode/") {
		userAgent = openCodeUserAgent
	}
	headers.Set("User-Agent", userAgent)
	headers.Set("Content-Type", "application/json")
	return nil
}
