package oauth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// swapVar temporarily sets *dst to val and returns a restore func. Used to point
// the package-level endpoint vars (and the Web API key) at a local httptest
// server for the duration of a test.
func swapVar(dst *string, val string) func() {
	old := *dst
	*dst = val
	return func() { *dst = old }
}

// newCallbackContext builds a *gin.Context whose request URL carries the given
// OpenID query params, mimicking the Steam callback that HandleOAuth forwards.
func newCallbackContext(t *testing.T, params url.Values) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = &http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Path: "/oauth/steam", RawQuery: params.Encode()},
	}
	return c
}

func TestExtractSteamID(t *testing.T) {
	tests := []struct {
		name      string
		claimedID string
		want      string
	}{
		{"standard", "https://steamcommunity.com/openid/id/76561197960287930", "76561197960287930"},
		{"with trailing query", "https://steamcommunity.com/openid/id/76561197960287930?foo=bar", "76561197960287930"},
		{"with fragment", "https://steamcommunity.com/openid/id/76561197960287930#anchor", "76561197960287930"},
		{"trailing slash only", "https://steamcommunity.com/openid/id/", ""},
		{"empty", "", ""},
		{"non-numeric id", "https://steamcommunity.com/openid/id/notanumber", ""},
		{"mixed alphanumeric", "https://steamcommunity.com/openid/id/765abc", ""},
		{"bare numeric id", "76561197960287930", "76561197960287930"},
		{"whitespace padded", "  https://steamcommunity.com/openid/id/76561197960287930  ", "76561197960287930"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractSteamID(tt.claimedID); got != tt.want {
				t.Errorf("extractSteamID(%q) = %q, want %q", tt.claimedID, got, tt.want)
			}
		})
	}
}

func TestIsAssertionValid(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"valid", "ns:http://specs.openid.net/auth/2.0\nis_valid:true\n", true},
		{"invalid", "ns:http://specs.openid.net/auth/2.0\nis_valid:false\n", false},
		{"missing field", "ns:http://specs.openid.net/auth/2.0\n", false},
		{"empty body", "", false},
		{"valid with surrounding spaces", "is_valid: true\n", true},
		{"false explicit", "is_valid:false\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAssertionValid(tt.body); got != tt.want {
				t.Errorf("isAssertionValid(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

// TestBuildVerificationValues verifies the security-critical invariant of the
// OpenID check_authentication replay: every openid.* param is forwarded, the
// mode is flipped to check_authentication, and non-openid params (notably the
// CSRF state) are dropped so they cannot pollute the signed assertion.
func TestBuildVerificationValues(t *testing.T) {
	query := url.Values{
		"openid.mode":        {"id_res"},
		"openid.claimed_id":  {"https://steamcommunity.com/openid/id/76561197960287930"},
		"openid.identity":    {"https://steamcommunity.com/openid/id/76561197960287930"},
		"openid.ns":          {"http://specs.openid.net/auth/2.0"},
		"openid.op_endpoint": {"https://steamcommunity.com/openid/login"},
		"openid.signed":      {"op_endpoint,claimed_id,identity"},
		"state":              {"csrf-token"},
		"noise":              {"should-be-dropped"},
	}
	got := buildVerificationValues(query)

	if got.Get("openid.mode") != "check_authentication" {
		t.Errorf("openid.mode = %q, want check_authentication", got.Get("openid.mode"))
	}
	for _, k := range []string{"openid.claimed_id", "openid.identity", "openid.ns", "openid.op_endpoint", "openid.signed"} {
		if got.Get(k) == "" {
			t.Errorf("openid param %q was not forwarded", k)
		}
	}
	for _, k := range []string{"state", "noise"} {
		if got.Get(k) != "" {
			t.Errorf("non-openid param %q leaked into verification request: %q", k, got.Get(k))
		}
	}
}

func TestExchangeToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		f := r.PostForm
		// The verification request must flip mode to check_authentication.
		if f.Get("openid.mode") != "check_authentication" {
			t.Errorf("server received openid.mode=%q, want check_authentication", f.Get("openid.mode"))
		}
		// The signed assertion params must be forwarded verbatim.
		if f.Get("openid.claimed_id") == "" || f.Get("openid.signed") == "" {
			t.Errorf("assertion params not forwarded: claimed_id=%q signed=%q", f.Get("openid.claimed_id"), f.Get("openid.signed"))
		}
		// Non-openid params (state) must NOT reach the verification endpoint.
		if f.Get("state") != "" {
			t.Errorf("state leaked into verification request: %q", f.Get("state"))
		}
		fmt.Fprintf(w, "ns:http://specs.openid.net/auth/2.0\nis_valid:true\n")
	}))
	defer srv.Close()
	defer swapVar(&steamOpenIDEndpoint, srv.URL)()

	p := &SteamProvider{}
	params := url.Values{
		"openid.mode":        {"id_res"},
		"openid.claimed_id":  {"https://steamcommunity.com/openid/id/76561197960287930"},
		"openid.identity":    {"https://steamcommunity.com/openid/id/76561197960287930"},
		"openid.ns":          {"http://specs.openid.net/auth/2.0"},
		"openid.op_endpoint": {"https://steamcommunity.com/openid/login"},
		"openid.signed":      {"op_endpoint,claimed_id,identity"},
		"openid.sig":         {"fakebase64sig=="},
		"state":              {"csrf-token"},
	}
	c := newCallbackContext(t, params)

	token, err := p.ExchangeToken(context.Background(), "", c)
	if err != nil {
		t.Fatalf("ExchangeToken returned error: %v", err)
	}
	if token == nil || token.AccessToken != "76561197960287930" {
		t.Fatalf("expected token carrying SteamID 76561197960287930, got %+v", token)
	}
}

func TestExchangeToken_InvalidAssertion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ns:http://specs.openid.net/auth/2.0\nis_valid:false\n")
	}))
	defer srv.Close()
	defer swapVar(&steamOpenIDEndpoint, srv.URL)()

	p := &SteamProvider{}
	params := url.Values{
		"openid.mode":       {"id_res"},
		"openid.claimed_id": {"https://steamcommunity.com/openid/id/76561197960287930"},
	}
	c := newCallbackContext(t, params)

	if _, err := p.ExchangeToken(context.Background(), "", c); err == nil {
		t.Fatal("expected error for invalid (is_valid:false) assertion, got nil")
	}
}

func TestExchangeToken_UserCancelled(t *testing.T) {
	p := &SteamProvider{}
	// openid.mode=cancel must short-circuit before any network call.
	params := url.Values{"openid.mode": {"cancel"}}
	c := newCallbackContext(t, params)

	_, err := p.ExchangeToken(context.Background(), "", c)
	if err == nil {
		t.Fatal("expected error for cancelled Steam auth, got nil")
	}
	if _, ok := err.(*AccessDeniedError); !ok {
		t.Errorf("expected *AccessDeniedError, got %T (%v)", err, err)
	}
}

func TestGetUserInfo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("key") != "test-api-key" {
			t.Errorf("steamids call key=%q, want test-api-key", r.FormValue("key"))
		}
		if r.FormValue("steamids") != "76561197960287930" {
			t.Errorf("steamids=%q, want 76561197960287930", r.FormValue("steamids"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"response":{"players":[{"steamid":"76561197960287930","personaname":"GordonFreeman","profileurl":"https://steamcommunity.com/id/gordon","avatar":"https://a.co/av.jpg","avatarmedium":"https://a.co/avm.jpg","avatarfull":"https://a.co/avf.jpg","communityvisibilitystate":3}]}}`)
	}))
	defer srv.Close()
	defer swapVar(&steamAPIBase, srv.URL)()
	defer swapVar(&common.SteamWebAPIKey, "test-api-key")()

	p := &SteamProvider{}
	user, err := p.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "76561197960287930"})
	if err != nil {
		t.Fatalf("GetUserInfo returned error: %v", err)
	}
	if user.ProviderUserID != "76561197960287930" {
		t.Errorf("ProviderUserID = %q, want 76561197960287930", user.ProviderUserID)
	}
	if user.Username != "GordonFreeman" {
		t.Errorf("Username = %q, want GordonFreeman", user.Username)
	}
	if user.DisplayName != "GordonFreeman" {
		t.Errorf("DisplayName = %q, want GordonFreeman", user.DisplayName)
	}
	// Steam never exposes an email — this invariant must hold.
	if user.Email != "" {
		t.Errorf("Email = %q, want empty (Steam provides no email)", user.Email)
	}
	if user.Extra["avatarfull"] != "https://a.co/avf.jpg" {
		t.Errorf("Extra[avatarfull] = %v, want https://a.co/avf.jpg", user.Extra["avatarfull"])
	}
	if user.Extra["profileurl"] != "https://steamcommunity.com/id/gordon" {
		t.Errorf("Extra[profileurl] = %v", user.Extra["profileurl"])
	}
}

func TestGetUserInfo_NoAPIKey(t *testing.T) {
	defer swapVar(&common.SteamWebAPIKey, "")()

	p := &SteamProvider{}
	_, err := p.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "76561197960287930"})
	if err == nil {
		t.Fatal("expected error when SteamWebAPIKey is empty, got nil")
	}
}

func TestGetUserInfo_EmptyPlayers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"response":{"players":[]}}`)
	}))
	defer srv.Close()
	defer swapVar(&steamAPIBase, srv.URL)()
	defer swapVar(&common.SteamWebAPIKey, "test-api-key")()

	p := &SteamProvider{}
	_, err := p.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "76561197960287930"})
	if err == nil {
		t.Fatal("expected error for empty players array, got nil")
	}
}
