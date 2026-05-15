package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestUserMarshalJSONUsesUsernameAsDisplayName(t *testing.T) {
	user := User{
		Username:    "alice",
		DisplayName: "Alice Display",
	}

	data, err := common.Marshal(user)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := common.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}

	if payload["display_name"] != user.Username {
		t.Fatalf("display_name = %v, want %q", payload["display_name"], user.Username)
	}
}
