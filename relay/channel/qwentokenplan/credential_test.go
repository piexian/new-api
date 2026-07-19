package qwentokenplan

import (
	"testing"
	"time"

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
			name: "complete bound credential",
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

func TestMergeAPIKey(t *testing.T) {
	t.Parallel()

	validExisting := `{"type":"qwen_token_plan","api_key":"sk-sp-old","access_token":"oauth-token","expires_at":"2099-01-01T00:00:00Z","user":{"email":"user@example.com"}}`
	merged, err := MergeAPIKey(validExisting, "sk-sp-new", time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	credential, err := ParseCredential(merged)
	require.NoError(t, err)
	require.Equal(t, "sk-sp-new", credential.APIKey)
	require.Equal(t, "oauth-token", credential.AccessToken)
	require.Equal(t, "user@example.com", credential.User.Email)

	expiredExisting := `{"type":"qwen_token_plan","api_key":"sk-sp-old","access_token":"expired-token","expires_at":"2025-01-01T00:00:00Z"}`
	merged, err = MergeAPIKey(expiredExisting, "sk-sp-new", time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, "sk-sp-new", merged)
}
