package qwentokenplan

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{
			name: "raw token plan key",
			raw:  "  sk-sp-test-key  ",
			want: "sk-sp-test-key",
		},
		{
			name: "console bound credential",
			raw:  `{"type":"qwen_token_plan","api_key":"sk-sp-bound","console_token":"console-token","access_key_id":"ak","access_key_secret":"sk"}`,
			want: "sk-sp-bound",
		},
		{
			name: "legacy oauth credential",
			raw:  `{"type":"qwen_token_plan","api_key":"sk-sp-bound","access_token":"oauth-token","expires_at":"2099-01-01T00:00:00Z"}`,
			want: "sk-sp-bound",
		},
		{
			name: "partial credential can be reauthorized",
			raw:  `{"type":"qwen_token_plan","api_key":"sk-sp-reusable"}`,
			want: "sk-sp-reusable",
		},
		{
			name:    "wrong credential type",
			raw:     `{"type":"other","api_key":"sk-sp-test"}`,
			wantErr: "invalid type",
		},
		{
			name:    "missing token plan key",
			raw:     `{"type":"qwen_token_plan","api_key":"sk-test"}`,
			wantErr: "must include an sk-sp- API key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ExtractAPIKey(test.raw)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestParseCredentialConsoleFields(t *testing.T) {
	t.Parallel()

	credential, err := ParseCredential(`{"type":"qwen_token_plan","api_key":"sk-sp-bound","console_token":" ct ","access_key_id":" ak "}`)
	require.NoError(t, err)
	require.Equal(t, "ct", credential.ConsoleToken)
	require.Equal(t, "ak", credential.AccessKeyID)
	require.Equal(t, "", credential.AccessKeySecret)
	require.True(t, credential.HasConsoleCredential())
}

func TestHasConsoleCredential(t *testing.T) {
	t.Parallel()

	require.False(t, (*Credential)(nil).HasConsoleCredential())

	bare, err := ParseCredential(`{"type":"qwen_token_plan","api_key":"sk-sp-x"}`)
	require.NoError(t, err)
	require.False(t, bare.HasConsoleCredential())

	consoleOnly, err := ParseCredential(`{"type":"qwen_token_plan","api_key":"sk-sp-x","console_token":"ct"}`)
	require.NoError(t, err)
	require.True(t, consoleOnly.HasConsoleCredential())

	akOnly, err := ParseCredential(`{"type":"qwen_token_plan","api_key":"sk-sp-x","access_key_id":"ak"}`)
	require.NoError(t, err)
	require.False(t, akOnly.HasConsoleCredential())

	akSk, err := ParseCredential(`{"type":"qwen_token_plan","api_key":"sk-sp-x","access_key_id":"ak","access_key_secret":"sk"}`)
	require.NoError(t, err)
	require.True(t, akSk.HasConsoleCredential())

	legacy, err := ParseCredential(`{"type":"qwen_token_plan","api_key":"sk-sp-x","access_token":"oauth","expires_at":"2099-01-01T00:00:00Z"}`)
	require.NoError(t, err)
	require.False(t, legacy.HasConsoleCredential())
}

func TestMergeAPIKey(t *testing.T) {
	t.Parallel()

	existing := `{"type":"qwen_token_plan","api_key":"sk-sp-old","console_token":"ct","access_key_id":"ak","access_key_secret":"sk","user":{"email":"user@example.com"}}`
	merged, err := MergeAPIKey(existing, "sk-sp-new")
	require.NoError(t, err)
	credential, err := ParseCredential(merged)
	require.NoError(t, err)
	require.Equal(t, "sk-sp-new", credential.APIKey)
	require.Equal(t, "ct", credential.ConsoleToken)
	require.Equal(t, "ak", credential.AccessKeyID)
	require.Equal(t, "sk", credential.AccessKeySecret)
	require.Equal(t, "user@example.com", credential.User.Email)

	legacyExisting := `{"type":"qwen_token_plan","api_key":"sk-sp-old","access_token":"legacy-token","expires_at":"2025-01-01T00:00:00Z"}`
	merged, err = MergeAPIKey(legacyExisting, "sk-sp-new")
	require.NoError(t, err)
	legacyCredential, err := ParseCredential(merged)
	require.NoError(t, err)
	require.Equal(t, "sk-sp-new", legacyCredential.APIKey)
	require.Equal(t, "legacy-token", legacyCredential.AccessToken)

	invalidExisting := "not-json"
	merged, err = MergeAPIKey(invalidExisting, "sk-sp-new")
	require.NoError(t, err)
	normalized, err := ParseCredential(merged)
	require.NoError(t, err)
	require.Equal(t, "sk-sp-new", normalized.APIKey)
}

func TestMergeChannelKeyPatchOnly(t *testing.T) {
	t.Parallel()

	existing := `{"type":"qwen_token_plan","api_key":"sk-sp-old","console_token":"old-ct"}`
	patch := "new-ct"
	merged, err := MergeChannelKey(existing, "", &patch, nil, nil)
	require.NoError(t, err)
	credential, err := ParseCredential(merged)
	require.NoError(t, err)
	require.Equal(t, "sk-sp-old", credential.APIKey)
	require.Equal(t, "new-ct", credential.ConsoleToken)

	clear := ""
	merged, err = MergeChannelKey(existing, "", &clear, nil, nil)
	require.NoError(t, err)
	credential, err = ParseCredential(merged)
	require.NoError(t, err)
	require.Equal(t, "", credential.ConsoleToken)

	ak := "ak-1"
	sk := "sk-1"
	merged, err = MergeChannelKey(existing, "sk-sp-new", &patch, &ak, &sk)
	require.NoError(t, err)
	credential, err = ParseCredential(merged)
	require.NoError(t, err)
	require.Equal(t, "sk-sp-new", credential.APIKey)
	require.Equal(t, "ak-1", credential.AccessKeyID)
	require.Equal(t, "sk-1", credential.AccessKeySecret)

	brokenAK := "ak-1"
	_, err = MergeChannelKey(existing, "", nil, &brokenAK, nil)
	require.ErrorContains(t, err, "together")

	_, err = MergeChannelKey("not-json", "", nil, nil, nil)
	require.ErrorContains(t, err, "sk-sp-")

	jsonKey := `{"type":"qwen_token_plan","api_key":"sk-sp-json","console_token":"ct-from-json"}`
	merged, err = MergeChannelKey(existing, jsonKey, &patch, nil, nil)
	require.NoError(t, err)
	credential, err = ParseCredential(merged)
	require.NoError(t, err)
	require.Equal(t, "sk-sp-json", credential.APIKey)
	require.Equal(t, "new-ct", credential.ConsoleToken)
}
