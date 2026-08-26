package common

import "strings"

const AnthropicBillingHeaderName = "x-anthropic-billing-header"

var blockedUpstreamHeaderNamesLower = map[string]struct{}{
	AnthropicBillingHeaderName: {},

	// Client-controlled proxy/IP identity must not reach provider APIs. Besides
	// leaking internal topology, these headers can be spoofed by callers.
	"forwarded":           {},
	"true-client-ip":      {},
	"x-client-ip":         {},
	"x-cluster-client-ip": {},
	"x-forwarded-for":     {},
	"x-forwarded":         {},
	"x-originating-ip":    {},
	"x-real-ip":           {},
	"x-remote-addr":       {},
	"x-remote-ip":         {},
	"cf-connecting-ip":    {},
	"fastly-client-ip":    {},
}

func IsBlockedUpstreamHeader(name string) bool {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if normalized == "" {
		return false
	}
	_, ok := blockedUpstreamHeaderNamesLower[normalized]
	return ok
}
