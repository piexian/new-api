package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQwenOAuthDeviceFlow(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cli/device/code":
			require.Equal(t, "client-1", r.URL.Query().Get("client_id"))
			require.NotEmpty(t, r.URL.Query().Get("code_challenge"))
			require.Equal(t, "S256", r.URL.Query().Get("code_challenge_method"))
			_, _ = w.Write([]byte(`{"Success":true,"Data":{"Token":"device-token","VerificationUrl":"https://verify.example","ExpiresIn":600,"Interval":5}}`))
		case "/cli/device/token":
			require.Equal(t, "client-1", r.URL.Query().Get("client_id"))
			require.Equal(t, "device-token", r.URL.Query().Get("token"))
			require.NotEmpty(t, r.URL.Query().Get("code_verifier"))
			_, _ = w.Write([]byte(`{"Success":true,"Data":{"Status":"complete","Credentials":{"AccessToken":"access-token","ExpireTime":"2099-01-01T00:00:00Z","User":{"Id":123,"Email":"user@example.com","AliyunId":"aliyun-1"}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	flow, err := createQwenOAuthAuthorizationFlow(context.Background(), server.Client(), server.URL, "client-1")
	require.NoError(t, err)
	require.Equal(t, "device-token", flow.Token)
	require.Equal(t, "https://verify.example", flow.VerificationURL)

	result, err := pollQwenOAuthAuthorization(context.Background(), server.Client(), server.URL, flow.ClientID, flow.Token, flow.Verifier)
	require.NoError(t, err)
	require.Equal(t, "complete", result.Status)
	require.NotNil(t, result.Credentials)
	require.Equal(t, "access-token", result.Credentials.AccessToken)
	require.Equal(t, "aliyun-1", result.Credentials.User.AliyunID)
}
