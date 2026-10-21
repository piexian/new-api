package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelKeysDisplayText(t *testing.T) {
	multi := func(key string) *Channel {
		return &Channel{Key: key, ChannelInfo: ChannelInfo{IsMultiKey: true}}
	}

	// Pools stored by SetKeys (JSON array) are shown one key per line.
	pool := multi("")
	require.NoError(t, pool.SetKeys([]string{"sk-a", "sk-b", "sk-c"}))
	assert.Equal(t, "sk-a\nsk-b\nsk-c", pool.KeysDisplayText())

	// Newline-separated pools are unchanged.
	assert.Equal(t, "sk-a\nsk-b", multi("sk-a\nsk-b\n").KeysDisplayText())

	// JSON credentials cannot round-trip as lines and keep the stored form.
	vertex := multi("")
	require.NoError(t, vertex.SetKeys([]string{"{\n  \"type\": \"service_account\"\n}", `{"type":"service_account"}`}))
	assert.Equal(t, vertex.Key, vertex.KeysDisplayText())

	// Single-key channels return the key as stored.
	single := &Channel{Key: `["not-a-pool"]`}
	assert.Equal(t, `["not-a-pool"]`, single.KeysDisplayText())
}
