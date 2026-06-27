package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestPickOAuthUsername(t *testing.T) {
	setupUserSelfControllerTestDB(t)

	// Seed usernames that should be considered "taken" (including soft-deleted lookups,
	// since CheckUserExistOrDeleted uses Unscoped).
	taken := []string{
		"existinguser",         // 12 chars
		"abcde",                // 5
		"abcd",                 // 4
		"abc",                  // 3 (== minLen)
		"taken20charsname1234", // 20
	}
	for _, name := range taken {
		u := &model.User{
			Username: name,
			Password: "password123",
			Role:     common.RoleCommonUser,
			Status:   common.UserStatusEnabled,
			Group:    "default",
			AffCode:  name + "-aff",
		}
		if err := model.DB.Create(u).Error; err != nil {
			t.Fatalf("failed to seed user %q: %v", name, err)
		}
	}

	cases := []struct {
		login string
		want  string
	}{
		{"", ""},           // empty -> placeholder
		{"alice", "alice"}, // short & free -> as-is
		{"abcdefghijklmnopqrst", "abcdefghijklmnopqrst"},    // exactly maxLen, free -> as-is
		{"nopuberegavu71-netizen", "nopuberegavu71-netiz"},  // 22 -> truncate to 20
		{"existinguser", "existinguse"},                     // 12 taken -> shorten to 11
		{"abcde", ""},                                       // 5,4,3 all taken -> placeholder
		{"taken20charsname12345678", "taken20charsname123"}, // 24 -> 20(taken) -> 19
	}
	for _, tc := range cases {
		t.Run(tc.login, func(t *testing.T) {
			got := pickOAuthUsername(tc.login)
			if got != tc.want {
				t.Errorf("pickOAuthUsername(%q) = %q, want %q", tc.login, got, tc.want)
			}
		})
	}
}
