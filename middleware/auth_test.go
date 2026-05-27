package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestResolveTokenUsingGroup(t *testing.T) {
	tests := []struct {
		name             string
		userGroup        string
		tokenGroup       string
		userUsableGroups map[string]string
		expectedGroup    string
		expectedOK       bool
	}{
		{
			name:             "empty token group uses user group",
			userGroup:        "user",
			tokenGroup:       "",
			userUsableGroups: map[string]string{"user": "用户分组"},
			expectedGroup:    "user",
			expectedOK:       true,
		},
		{
			name:             "token group in usable groups becomes effective group",
			userGroup:        "vip",
			tokenGroup:       "model",
			userUsableGroups: map[string]string{"vip": "用户分组", "model": "模型分组"},
			expectedGroup:    "model",
			expectedOK:       true,
		},
		{
			name:             "token group matching user group is allowed",
			userGroup:        "91vip",
			tokenGroup:       "91vip",
			userUsableGroups: map[string]string{"91vip": "用户分组"},
			expectedGroup:    "91vip",
			expectedOK:       true,
		},
		{
			name:             "auto token group is allowed for downstream auto selection",
			userGroup:        "user",
			tokenGroup:       "auto",
			userUsableGroups: map[string]string{"user": "用户分组"},
			expectedGroup:    "auto",
			expectedOK:       true,
		},
		{
			name:             "token group outside usable groups is rejected",
			userGroup:        "user",
			tokenGroup:       "model",
			userUsableGroups: map[string]string{"user": "用户分组"},
			expectedGroup:    "",
			expectedOK:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, ok := resolveTokenUsingGroup(tt.userGroup, tt.tokenGroup, tt.userUsableGroups)
			require.Equal(t, tt.expectedOK, ok)
			require.Equal(t, tt.expectedGroup, group)
		})
	}
}

func TestFormatAvailableGroups(t *testing.T) {
	require.Equal(t, "无", formatAvailableGroups(nil))
	require.Equal(t, "model、user", formatAvailableGroups(map[string]string{
		"user":  "用户分组",
		"model": "模型分组",
	}))
}

// Ensure model types are used to satisfy import
var _ = &model.Token{}
var _ = &model.UserBase{}
