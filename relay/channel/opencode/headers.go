package opencode

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// openCodeUserAgent mirrors the OpenCode 2.x CLI shape. The 2026-10-14 free
// tier gate rejects the v1-style "opencode/<ver> (<runtime>...)" form; the
// version number itself is not validated upstream.
const openCodeUserAgent = "opencode/latest/2.0.32/cli"

// openCodeAnthropicBeta is the fixed beta set the 2.x client sends on the
// /inference gateway; the v2 gate fingerprints it on Messages requests.
const openCodeAnthropicBeta = "interleaved-thinking-2025-05-14"

const openCodeIDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var openCodeSessionIDPattern = regexp.MustCompile(`^ses_[0-9a-f]{12}[0-9A-Za-z]{14}$`)
var openCodeDescendingTimePartPattern = regexp.MustCompile(`^[0-9a-f]{9}f[c-f][0-9a-f]$`)
var openCodeCLIUserAgentPattern = regexp.MustCompile(`^opencode/latest/[^/]+/cli$`)
var openCodeIDs openCodeIDGenerator

// openCodeLane names the upstream surface a request is shaped for: the legacy
// v1 Zen gateway keeps the 1.x request shape, the v2 /inference gateway the
// 2.x shape, and Go forwards the client's own headers.
type openCodeLane int

const (
	openCodeLaneV1 openCodeLane = iota
	openCodeLaneV2
	openCodeLaneGo
)

func openCodeLaneFor(baseURL string) openCodeLane {
	if IsGoBase(baseURL) {
		return openCodeLaneGo
	}
	if strings.HasSuffix(NormalizeRoot(baseURL), "/zen") {
		return openCodeLaneV1
	}
	return openCodeLaneV2
}

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

// openCodeSessionScope isolates gateway-minted upstream sessions per channel
// and downstream token, so two users whose conversations begin identically do
// not share an upstream session, provider cache or rate-limit bucket.
func openCodeSessionScope(info *relaycommon.RelayInfo) string {
	if info == nil {
		return ""
	}
	channelID := 0
	if info.ChannelMeta != nil {
		channelID = info.ChannelId
	}
	return strconv.Itoa(channelID) + "#" + strconv.Itoa(info.TokenId)
}

// openCodeOfficialSessionID reports whether signal carries the encoding the
// official client mints with Identifier.descending(). The free tier accepts
// counters 1-63 (low 12 bits 0xFC0-0xFFF) and rejects every other encoding of
// the right ses_ shape with 403 FreeTierError.
func openCodeOfficialSessionID(signal string) bool {
	return openCodeSessionIDPattern.MatchString(signal) &&
		openCodeDescendingTimePartPattern.MatchString(signal[4:16])
}

// openCodeCanonicalSessionID keeps an official client session verbatim and
// deterministically re-encodes every other signal into the accepted layout, so
// a client that sends a well-shaped but wrongly encoded session still gets a
// stable upstream session instead of a 403.
func openCodeCanonicalSessionID(scope, signal string) string {
	if openCodeOfficialSessionID(signal) {
		return signal
	}
	sum := sha256.Sum256([]byte("ses\x00" + scope + "\x00" + signal))
	timePart := openCodeDescendingTimePart(sum[:6])
	randomPart := openCodeBase62(new(big.Int).SetBytes(sum[6:16]), 14)
	return "ses_" + timePart + randomPart
}

// openCodeDescendingTimePart renders six hash bytes as Identifier.descending()
// does: a 36-bit timestamp shifted into a 48-bit word, ORed with counter 1 and
// inverted, leaving the low 12 bits at ffe.
func openCodeDescendingTimePart(hash []byte) string {
	current := (uint64(hash[0])<<28 | uint64(hash[1])<<20 | uint64(hash[2])<<12 |
		uint64(hash[3])<<4 | uint64(hash[4])>>4) << 12
	current |= 1
	inverted := ^current & 0xFFFFFFFFFFFF
	return hex.EncodeToString([]byte{
		byte(inverted >> 40), byte(inverted >> 32), byte(inverted >> 24),
		byte(inverted >> 16), byte(inverted >> 8), byte(inverted),
	})
}

func openCodeBase62(n *big.Int, width int) string {
	base := big.NewInt(62)
	out := make([]byte, width)
	remainder := new(big.Int)
	for i := width - 1; i >= 0; i-- {
		n.DivMod(n, base, remainder)
		out[i] = openCodeIDAlphabet[remainder.Int64()]
	}
	return string(out)
}

// openCodeSessionIdentifier returns the conversation-affinity session. An
// official client value is preserved; a foreign value (a UUID, another
// gateway's session, a shape-correct but wrongly encoded id) is re-encoded
// deterministically; no signal at all mints a fresh official-shaped id that
// survives retries of the same request.
func openCodeSessionIdentifier(c *gin.Context, scope string) (string, error) {
	signal := ""
	if c != nil && c.Request != nil {
		for _, name := range []string{"x-opencode-session", "x-opencode-session-id", "x-session-affinity", "x-session-id"} {
			if value := strings.TrimSpace(c.Request.Header.Get(name)); value != "" {
				signal = value
				break
			}
		}
	}
	if c == nil {
		return openCodeIDs.next("ses", true, time.Now().UnixMilli(), rand.Reader)
	}
	// Context-local IDs survive retries without merging unrelated conversations.
	const key = "opencode.generated.x-opencode-session"
	if value := c.GetString(key); value != "" {
		return value, nil
	}
	var value string
	if signal == "" {
		generated, err := openCodeIDs.next("ses", true, time.Now().UnixMilli(), rand.Reader)
		if err != nil {
			return "", err
		}
		value = generated
	} else {
		value = openCodeCanonicalSessionID(scope, signal)
	}
	c.Set(key, value)
	return value, nil
}

// openCodeChildSessionIdentifier preserves the client's own session id, which a
// subagent mints separately from the parent conversation.
func openCodeChildSessionIdentifier(c *gin.Context, scope, root string) string {
	if c != nil && c.Request != nil {
		if value := strings.TrimSpace(c.Request.Header.Get("x-opencode-session-id")); value != "" {
			return openCodeCanonicalSessionID(scope, value)
		}
	}
	return root
}

// openCodeParentSessionIdentifier returns the canonical parent session when the
// client reported one; the parent, not the child, carries the affinity.
func openCodeParentSessionIdentifier(c *gin.Context, scope string) string {
	if c == nil || c.Request == nil {
		return ""
	}
	for _, name := range []string{"x-opencode-parent-session-id", "x-parent-session-id"} {
		if value := strings.TrimSpace(c.Request.Header.Get(name)); value != "" {
			return openCodeCanonicalSessionID(scope, value)
		}
	}
	return ""
}

// openCodeProjectID normalizes the project header into a shape the free tier
// accepts: canonical prj_* ids are rejected with 403, while the official
// client's bare 40-hex value (or any non-canonical string such as "global")
// passes. Hashing keeps one conversation pinned to one upstream project.
func openCodeProjectID(value string) string {
	if !strings.HasPrefix(value, "prj_") {
		return value
	}
	sum := sha256.Sum256([]byte("opencode-project\x00" + value))
	return hex.EncodeToString(sum[:20])
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

// setupOpenCodeHeaders installs the lane's request shape. Zen free models are
// restricted to the OpenCode client, so the gateway presents the official
// fingerprint regardless of the caller.
//
// The 2026-10-14 free tier gate requires both x-opencode-session and
// x-opencode-session-id, and rejects the legacy x-opencode-request correlation
// header with 403, so that header is never generated here; a genuine value a
// real client sent still passes through on Go lanes via passthroughClientHeaders.
func setupOpenCodeHeaders(c *gin.Context, headers *http.Header, info *relaycommon.RelayInfo) error {
	lane := openCodeLaneV2
	if info != nil {
		lane = openCodeLaneFor(info.ChannelBaseUrl)
	}
	return applyOpenCodeHeaders(c, headers, lane, openCodeSessionScope(info))
}

func applyOpenCodeHeaders(c *gin.Context, headers *http.Header, lane openCodeLane, scope string) error {
	session, err := openCodeSessionIdentifier(c, scope)
	if err != nil {
		return err
	}
	if lane == openCodeLaneV2 {
		applyOpenCodeV2SessionHeaders(c, headers, scope, session)
	} else {
		headers.Set("x-opencode-session", session)
		headers.Set("x-opencode-session-id", session)
	}
	for name, fallback := range map[string]string{
		"x-opencode-client":  "cli",
		"x-opencode-project": "global",
	} {
		value := ""
		if c != nil && c.Request != nil {
			value = strings.TrimSpace(c.Request.Header.Get(name))
		}
		if value == "" {
			value = fallback
		}
		if name == "x-opencode-project" {
			value = openCodeProjectID(value)
		}
		headers.Set(name, value)
	}
	if lane == openCodeLaneV2 {
		headers.Set("Accept", "application/json, text/event-stream")
	}
	userAgent := ""
	if c != nil && c.Request != nil {
		userAgent = strings.TrimSpace(c.Request.Header.Get("User-Agent"))
	}
	if userAgent == "" || (lane == openCodeLaneV2 && !openCodeCLIUserAgentPattern.MatchString(userAgent)) {
		userAgent = openCodeUserAgent
	}
	headers.Set("User-Agent", userAgent)
	if headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}
	return nil
}

// applyOpenCodeV2SessionHeaders renders the OpenCode 2.x session set: the
// child session as x-opencode-session-id, the parent (or the session itself)
// as conversation affinity on x-opencode-session, x-session-affinity and
// X-Session-Id, plus the parent headers when a subagent reported one.
func applyOpenCodeV2SessionHeaders(c *gin.Context, headers *http.Header, scope, session string) {
	root := session
	child := openCodeChildSessionIdentifier(c, scope, session)
	if parent := openCodeParentSessionIdentifier(c, scope); parent != "" {
		root = parent
		headers.Set("x-opencode-parent-session-id", parent)
		headers.Set("x-parent-session-id", parent)
	}
	headers.Set("x-session-affinity", root)
	headers.Set("X-Session-Id", root)
	headers.Set("x-opencode-session", root)
	headers.Set("x-opencode-session-id", child)
}
