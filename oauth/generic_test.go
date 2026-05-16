package oauth

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestNormalizeProviderUsernamePrefersDisplayName(t *testing.T) {
	got := normalizeProviderUsername("Alice Nick", "alice@example.com")

	if got != "Alice Nick" {
		t.Fatalf("expected display name, got %q", got)
	}
}

func TestNormalizeProviderUsernameKeepsShortFallbackValues(t *testing.T) {
	username := "short@example.io"

	got := normalizeProviderUsername("", username)

	if got != username {
		t.Fatalf("expected %q, got %q", username, got)
	}
}

func TestNormalizeProviderUsernameUsesEmailLocalPartFallbackForLongEmails(t *testing.T) {
	got := normalizeProviderUsername("", "very.long.google.user@example.com")

	if got != "very.long.google.use" {
		t.Fatalf("unexpected username: %q", got)
	}
	if len(got) > model.UserNameMaxLength {
		t.Fatalf("username length = %d, want <= %d", len(got), model.UserNameMaxLength)
	}
}

func TestNormalizeProviderUsernameLeavesLongNonEmailValuesForFallback(t *testing.T) {
	username := strings.Repeat("a", model.UserNameMaxLength+1)

	got := normalizeProviderUsername("", username)

	if got != strings.Repeat("a", model.UserNameMaxLength) {
		t.Fatalf("unexpected truncated username: %q", got)
	}
}

func TestNormalizeProviderUsernameTruncatesLongDisplayName(t *testing.T) {
	got := normalizeProviderUsername("A very long Google display name", "alice@example.com")

	if got != "A very long Google d" {
		t.Fatalf("unexpected username: %q", got)
	}
}
