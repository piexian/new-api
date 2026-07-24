package oauth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func useGitHubAPIServer(t *testing.T, handler http.Handler) {
	t.Helper()

	server := httptest.NewServer(handler)
	previousBase := githubAPIBase
	githubAPIBase = server.URL
	t.Cleanup(func() {
		githubAPIBase = previousBase
		server.Close()
	})
}

func TestGitHubGetUserInfoUsesPublicEmail(t *testing.T) {
	emailsRequested := false
	useGitHubAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":123,"login":"octocat","name":"The Octocat","email":"public@example.com","created_at":"2011-01-25T18:44:36Z"}`)
		case "/user/emails":
			emailsRequested = true
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))

	user, err := (&GitHubProvider{}).GetUserInfo(context.Background(), &OAuthToken{AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("GetUserInfo returned error: %v", err)
	}
	if user.Email != "public@example.com" {
		t.Errorf("Email = %q, want public@example.com", user.Email)
	}
	if emailsRequested {
		t.Error("GetUserInfo requested /user/emails despite a public profile email")
	}
}

func TestGitHubGetUserInfoFallsBackToVerifiedPrimaryEmail(t *testing.T) {
	useGitHubAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/user":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":123,"login":"octocat","name":"The Octocat","email":null,"created_at":"2011-01-25T18:44:36Z"}`)
		case "/user/emails":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[
				{"email":"unverified@example.com","primary":true,"verified":false},
				{"email":"secondary@example.com","primary":false,"verified":true},
				{"email":"primary@example.com","primary":true,"verified":true}
			]`)
		default:
			http.NotFound(w, r)
		}
	}))

	user, err := (&GitHubProvider{}).GetUserInfo(context.Background(), &OAuthToken{AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("GetUserInfo returned error: %v", err)
	}
	if user.Email != "primary@example.com" {
		t.Errorf("Email = %q, want primary@example.com", user.Email)
	}
}

func TestGitHubGetUserInfoContinuesWhenEmailLookupFails(t *testing.T) {
	useGitHubAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":123,"login":"octocat","name":"The Octocat","email":null,"created_at":"2011-01-25T18:44:36Z"}`)
		case "/user/emails":
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))

	user, err := (&GitHubProvider{}).GetUserInfo(context.Background(), &OAuthToken{AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("GetUserInfo returned error: %v", err)
	}
	if user.Email != "" {
		t.Errorf("Email = %q, want empty", user.Email)
	}
}
