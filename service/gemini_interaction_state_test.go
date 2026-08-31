package service

import (
	"testing"
)

func TestGeminiInteractionStateMemoryStore(t *testing.T) {
	// 未启用 Redis 时走进程内存实现
	state := &GeminiInteractionState{
		ChannelID:  7,
		Key:        "test-key",
		UserID:     42,
		TokenID:    9,
		Model:      "gemini-3.6-flash",
		Background: true,
	}
	id := "v1_test_interaction"
	SaveGeminiInteractionState(id, state)

	got, ok := GetGeminiInteractionState(id)
	if !ok {
		t.Fatal("state should be found after save")
	}
	if got.ChannelID != 7 || got.Key != "test-key" || got.UserID != 42 || got.Model != "gemini-3.6-flash" || !got.Background {
		t.Fatalf("state roundtrip mismatch: %+v", got)
	}

	if _, ok := GetGeminiInteractionState("missing"); ok {
		t.Fatal("missing id should not be found")
	}

	// 计费认领:仅首次成功
	if !ClaimGeminiInteractionBilling(id) {
		t.Fatal("first claim should succeed")
	}
	if ClaimGeminiInteractionBilling(id) {
		t.Fatal("second claim should fail")
	}

	DeleteGeminiInteractionState(id)
	if _, ok := GetGeminiInteractionState(id); ok {
		t.Fatal("state should be deleted")
	}
	// 删除后可重新认领(新生命周期)
	if !ClaimGeminiInteractionBilling(id) {
		t.Fatal("claim after delete should succeed")
	}
}

func TestGeminiInteractionStateEmptyGuards(t *testing.T) {
	SaveGeminiInteractionState("", nil)
	if _, ok := GetGeminiInteractionState(""); ok {
		t.Fatal("empty id should never be found")
	}
	if ClaimGeminiInteractionBilling("") {
		t.Fatal("empty id claim should fail")
	}
}
