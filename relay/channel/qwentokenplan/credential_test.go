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
