package opencode

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// The free tier blocks an egress IP, not a request, when it flags abusive
// traffic. Retrying the same exit cannot succeed and only extends the penalty,
// so a blocked or repeatedly failing exit is taken out of rotation and the
// relay loop moves on to another channel.
var (
	errFreeExitBlocked = errors.New("OpenCode free tier is blocking the exit IP; no retry is possible from this exit")
	errFreeExitPaused  = errors.New("OpenCode free tier lane is paused after repeated upstream failures")
)

const (
	// freeExitBlockTTL keeps a blocked exit out of rotation. The upstream does
	// not publish the block duration; this matches the observed recovery window
	// and stops the retry storm in the meantime.
	freeExitBlockTTL = 15 * time.Minute
	// Transport failures cool an exit off with exponential backoff.
	freeExitFailureBase = 15 * time.Second
	freeExitFailureMax  = 120 * time.Second
	// After this many consecutive failures the channel's free lane pauses with
	// an hour-long base that doubles per trip, up to a day.
	freeLaneBreakerThreshold = 6
	freeLaneBreakerBase      = time.Hour
	freeLaneBreakerMax       = 24 * time.Hour
	// freeBlockProbeLimit bounds how much of a 403 body is read to classify it.
	freeBlockProbeLimit = 16 << 10
)

// freeExitState tracks one channel exit (channel + proxy setting). It is
// process-local: a restart forgets blocks, which is acceptable because the
// upstream block outlives the process anyway.
type freeExitState struct {
	mu            sync.Mutex
	failures      uint32
	consecutive   uint32
	strikes       uint32
	cooldownUntil time.Time
	blockedUntil  time.Time
	pausedUntil   time.Time
}

var freeExits sync.Map

func freeExitKey(info *relaycommon.RelayInfo) string {
	if info == nil {
		return "unknown"
	}
	channelID := 0
	proxy := ""
	if info.ChannelMeta != nil {
		channelID = info.ChannelId
		proxy = strings.TrimSpace(info.ChannelSetting.Proxy)
	}
	return strconv.Itoa(channelID) + "\x00" + proxy
}

// freeLaneClient returns the Bun-fingerprinted client for the Zen free lanes
// and nil for Go, whose keyed traffic keeps the standard relay client.
func freeLaneClient(info *relaycommon.RelayInfo) (*http.Client, error) {
	if info == nil || info.ChannelMeta == nil || openCodeLaneFor(info.ChannelBaseUrl) == openCodeLaneGo {
		return nil, nil
	}
	return freeTransportClient(info.ChannelSetting.Proxy)
}

// freeExitFor returns the state of this channel's free exit.
func freeExitFor(info *relaycommon.RelayInfo) *freeExitState {
	key := freeExitKey(info)
	if cached, ok := freeExits.Load(key); ok {
		return cached.(*freeExitState)
	}
	state := &freeExitState{}
	cached, _ := freeExits.LoadOrStore(key, state)
	return cached.(*freeExitState)
}

// available reports why this exit cannot serve a free request right now.
func (e *freeExitState) available(now time.Time) error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if now.Before(e.blockedUntil) {
		return fmt.Errorf("%w (blocked for another %s)", errFreeExitBlocked, e.blockedUntil.Sub(now).Round(time.Second))
	}
	if now.Before(e.pausedUntil) {
		return fmt.Errorf("%w (paused for another %s)", errFreeExitPaused, e.pausedUntil.Sub(now).Round(time.Second))
	}
	if now.Before(e.cooldownUntil) {
		return fmt.Errorf("OpenCode free tier exit is cooling down for another %s", e.cooldownUntil.Sub(now).Round(time.Second))
	}
	return nil
}

func (e *freeExitState) noteSuccess() {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failures = 0
	e.consecutive = 0
	e.strikes = 0
	e.cooldownUntil = time.Time{}
	e.pausedUntil = time.Time{}
}

// noteFailure applies exponential backoff to transport-level failures (dial
// errors, resets, timeouts) and trips the lane breaker after a run of them. An
// upstream Retry-After lengthens the cooldown instead of shortening it.
func (e *freeExitState) noteFailure(now time.Time, retryAfter time.Duration) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failures++
	e.consecutive++
	backoff := freeExitFailureBase
	for i := uint32(1); i < e.failures && backoff < freeExitFailureMax; i++ {
		backoff *= 2
	}
	if backoff > freeExitFailureMax {
		backoff = freeExitFailureMax
	}
	if retryAfter > backoff {
		backoff = retryAfter
	}
	e.cooldownUntil = now.Add(backoff)
	if e.consecutive >= freeLaneBreakerThreshold {
		e.strikes++
		pause := freeLaneBreakerBase
		for i := uint32(1); i < e.strikes && pause < freeLaneBreakerMax; i++ {
			pause *= 2
		}
		if pause > freeLaneBreakerMax {
			pause = freeLaneBreakerMax
		}
		e.pausedUntil = now.Add(pause)
		e.consecutive = 0
	}
}

// noteIPBlock records an upstream IP-abuse block. The block belongs to the
// egress IP, so nothing a new request carries can clear it before the TTL.
func (e *freeExitState) noteIPBlock(now time.Time) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.blockedUntil = now.Add(freeExitBlockTTL)
	e.cooldownUntil = time.Time{}
}

// resetFreeExitState clears the process-local exit state (tests).
func resetFreeExitState() {
	freeExits.Range(func(key, _ any) bool {
		freeExits.Delete(key)
		return true
	})
}

// isFreeNetworkAbuseBlock reports whether status and body are the upstream's
// IP-abuse rejection rather than the request-shape gate. The shape gate's 403
// ("free tier can only be used from within OpenCode") can still be fixed by
// reshaping the request; an abuse block cannot.
func isFreeNetworkAbuseBlock(status int, body []byte) bool {
	if status != http.StatusForbidden {
		return false
	}
	text := strings.ToLower(string(body))
	if strings.Contains(text, "abusive traffic") ||
		strings.Contains(text, "abuse traffic") ||
		strings.Contains(text, "abuse of traffic") {
		return true
	}
	return strings.Contains(text, "free models aren't available") &&
		strings.Contains(text, "from your current network")
}

// peekFreeResponseBody reads the start of a response body and puts it back in
// front of the rest, so classification and downstream error handling both see
// the full payload.
func peekFreeResponseBody(resp *http.Response) []byte {
	if resp == nil || resp.Body == nil {
		return nil
	}
	prefix, err := io.ReadAll(io.LimitReader(resp.Body, freeBlockProbeLimit))
	resp.Body = &replayBody{Reader: io.MultiReader(bytes.NewReader(prefix), resp.Body), closer: resp.Body}
	if err != nil {
		return nil
	}
	return prefix
}

type replayBody struct {
	io.Reader
	closer io.Closer
}

func (b *replayBody) Close() error { return b.closer.Close() }

// freeRetryAfter parses an upstream Retry-After header, capped so a hostile
// value cannot park an exit for hours.
func freeRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	value := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return min(time.Duration(seconds)*time.Second, freeExitFailureMax)
	}
	if when, err := http.ParseTime(value); err == nil {
		if wait := time.Until(when); wait > 0 {
			return min(wait, freeExitFailureMax)
		}
	}
	return 0
}

// freeExitRequest tracks one free-tier attempt's outcome against its exit.
func noteFreeExitResult(exit *freeExitState, resp *http.Response, err error) error {
	now := time.Now()
	if err != nil {
		exit.noteFailure(now, 0)
		return err
	}
	if resp == nil {
		return nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		exit.noteFailure(now, freeRetryAfter(resp))
		return nil
	}
	if resp.StatusCode == http.StatusForbidden && isFreeNetworkAbuseBlock(resp.StatusCode, peekFreeResponseBody(resp)) {
		exit.noteIPBlock(now)
		_ = resp.Body.Close()
		return errFreeExitBlocked
	}
	if resp.StatusCode < 400 {
		exit.noteSuccess()
	}
	return nil
}
