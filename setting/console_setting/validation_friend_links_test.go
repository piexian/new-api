package console_setting

import (
	"strings"
	"testing"
)

func TestValidateFriendLinksOK(t *testing.T) {
	raw := `[
		{"name":"AstrBot","url":"https://astrbot.app","icon":"https://astrbot.app/icon.png","description":"bot","order":1,"enabled":true},
		{"name":"Docs","url":"https://docs.example.com","order":2}
	]`
	if err := ValidateConsoleSettings(raw, "FriendLinks"); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestValidateFriendLinksRejectsBadURL(t *testing.T) {
	raw := `[{"name":"x","url":"not-a-url"}]`
	err := ValidateConsoleSettings(raw, "FriendLinks")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateFriendLinksRejectsMissingName(t *testing.T) {
	raw := `[{"url":"https://example.com"}]`
	err := ValidateConsoleSettings(raw, "FriendLinks")
	if err == nil || !strings.Contains(err.Error(), "名称") {
		t.Fatalf("expected name error, got %v", err)
	}
}

func TestValidateFriendLinksMax30(t *testing.T) {
	items := make([]string, 0, 31)
	for range 31 {
		items = append(items, `{"name":"n","url":"https://example.com"}`)
	}
	raw := "[" + strings.Join(items, ",") + "]"
	err := ValidateConsoleSettings(raw, "FriendLinks")
	if err == nil || !strings.Contains(err.Error(), "30") {
		t.Fatalf("expected max 30 error, got %v", err)
	}
}

func TestGetFriendLinksFiltersAndSorts(t *testing.T) {
	cs := GetConsoleSetting()
	prevEnabled := cs.FriendLinksEnabled
	prevLinks := cs.FriendLinks
	t.Cleanup(func() {
		cs.FriendLinksEnabled = prevEnabled
		cs.FriendLinks = prevLinks
	})

	cs.FriendLinksEnabled = true
	cs.FriendLinks = `[
		{"name":"B","url":"https://b.example","order":2,"enabled":true},
		{"name":"Hidden","url":"https://h.example","order":0,"enabled":false},
		{"name":"A","url":"https://a.example","order":1,"enabled":true}
	]`
	got := GetFriendLinks()
	if len(got) != 2 {
		t.Fatalf("expected 2 enabled, got %d", len(got))
	}
	if got[0]["name"] != "A" || got[1]["name"] != "B" {
		t.Fatalf("unexpected order: %#v", got)
	}

	cs.FriendLinksEnabled = false
	if len(GetFriendLinks()) != 0 {
		t.Fatal("expected empty when disabled")
	}
}

func TestValidateFriendLinksAllowsEmojiIcon(t *testing.T) {
	raw := `[{"name":"Bot","url":"https://example.com","icon":"🤖","description":"emoji","order":1,"enabled":true}]`
	if err := ValidateConsoleSettings(raw, "FriendLinks"); err != nil {
		t.Fatalf("expected emoji icon ok, got %v", err)
	}
	// complex ZWJ emoji sequence
	raw2 := `[{"name":"Flags","url":"https://example.com","icon":"👨‍💻"}]`
	if err := ValidateConsoleSettings(raw2, "FriendLinks"); err != nil {
		t.Fatalf("expected complex emoji ok, got %v", err)
	}
}

func TestValidateFriendLinksRejectsDangerousIcon(t *testing.T) {
	raw := `[{"name":"x","url":"https://example.com","icon":"javascript:alert(1)"}]`
	err := ValidateConsoleSettings(raw, "FriendLinks")
	if err == nil {
		t.Fatal("expected dangerous icon rejected")
	}
}

func TestValidateFriendLinksStillAcceptsIconURL(t *testing.T) {
	raw := `[{"name":"x","url":"https://example.com","icon":"https://cdn.example.com/a.png"}]`
	if err := ValidateConsoleSettings(raw, "FriendLinks"); err != nil {
		t.Fatalf("expected icon url ok, got %v", err)
	}
}
