package common

import "testing"

func TestCanonicalBuildInToolName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"web_search", "web_search"},
		{"web_search_premium", "web_search_premium"},
		{"web_search_preview", "web_search"},
		{"web_search_preview_2025_03_11", "web_search"},
		{"web_search_20250305", "web_search"},
		{"file_search", "file_search"},
		{"google_search", "google_search"},
		{"function", "function"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := CanonicalBuildInToolName(tc.name); got != tc.want {
			t.Errorf("CanonicalBuildInToolName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
