package common

import "strings"

const AnthropicBillingHeaderName = "x-anthropic-billing-header"

var blockedUpstreamHeaderNamesLower = map[string]struct{}{
	AnthropicBillingHeaderName: {},
}

func IsBlockedUpstreamHeader(name string) bool {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if normalized == "" {
		return false
	}
	_, ok := blockedUpstreamHeaderNamesLower[normalized]
	return ok
}
