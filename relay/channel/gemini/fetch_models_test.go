package gemini

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchGeminiModelsFollowsOfficialPagination(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1beta/models", r.URL.Path)
		require.Equal(t, "test-key", r.URL.Query().Get("key"))
		require.Equal(t, "1000", r.URL.Query().Get("pageSize"))
		require.Empty(t, r.Header.Get("x-goog-api-key"))

		switch requestCount {
		case 1:
			require.Empty(t, r.URL.Query().Get("pageToken"))
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-2.5-pro"}],"nextPageToken":"1"}`))
		case 2:
			require.Equal(t, "1", r.URL.Query().Get("pageToken"))
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-2.5-flash"}]}`))
		default:
			t.Fatalf("unexpected request %d", requestCount)
		}
	}))
	defer server.Close()

	models, err := FetchGeminiModels(server.URL, "test-key", "")
	require.NoError(t, err)
	require.Equal(t, []string{"gemini-2.5-pro", "gemini-2.5-flash"}, models)
}
