package controller

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestValidateAndFillReturnsUserNotFound(t *testing.T) {
	setupUserSelfControllerTestDB(t)

	// Seed a real user. Insert hashes the password with bcrypt.
	realUser := &model.User{Username: "realuser", Password: "correctpass"}
	if err := realUser.Insert(0); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	// Seed a disabled user to verify the disabled path is still distinct.
	disabledUser := &model.User{Username: "banned", Password: "bannedpass"}
	if err := disabledUser.Insert(0); err != nil {
		t.Fatalf("failed to seed disabled user: %v", err)
	}
	if err := model.DB.Model(&model.User{}).Where("id = ?", disabledUser.Id).
		Update("status", common.UserStatusDisabled).Error; err != nil {
		t.Fatalf("failed to disable user: %v", err)
	}

	cases := []struct {
		name     string
		username string
		password string
		want     error
	}{
		{"nonexistent user returns ErrUserNotFound", "ghost", "whatever", model.ErrUserNotFound},
		{"wrong password returns ErrInvalidCredentials", "realuser", "wrongpass", model.ErrInvalidCredentials},
		{"correct credentials return nil", "realuser", "correctpass", nil},
		{"disabled user returns ErrUserDisabled", "banned", "bannedpass", model.ErrUserDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := model.User{Username: tc.username, Password: tc.password}
			err := u.ValidateAndFill()
			switch {
			case tc.want == nil && err != nil:
				t.Errorf("ValidateAndFill(%q) = %v, want nil", tc.username, err)
			case tc.want != nil && !errors.Is(err, tc.want):
				t.Errorf("ValidateAndFill(%q) = %v, want %v", tc.username, err, tc.want)
			}
		})
	}
}
