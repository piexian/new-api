package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

func TestChannelHasAvailableKey(t *testing.T) {
	tests := []struct {
		name    string
		channel *Channel
		want    bool
	}{
		{
			name:    "single key channel with empty key is unavailable",
			channel: &Channel{},
			want:    false,
		},
		{
			name: "single key channel with key is available",
			channel: &Channel{
				Key: "sk-test",
			},
			want: true,
		},
		{
			name: "multi key channel with only blank keys is unavailable",
			channel: &Channel{
				Key: " \n\t",
				ChannelInfo: ChannelInfo{
					IsMultiKey: true,
				},
			},
			want: false,
		},
		{
			name: "multi key channel skips disabled key and keeps enabled key available",
			channel: &Channel{
				Key: "disabled-key\nenabled-key",
				ChannelInfo: ChannelInfo{
					IsMultiKey: true,
					MultiKeyStatusList: map[int]int{
						0: common.ChannelStatusAutoDisabled,
					},
				},
			},
			want: true,
		},
		{
			name: "multi key channel with all keys disabled is unavailable",
			channel: &Channel{
				Key: "disabled-key-1\ndisabled-key-2",
				ChannelInfo: ChannelInfo{
					IsMultiKey: true,
					MultiKeyStatusList: map[int]int{
						0: common.ChannelStatusAutoDisabled,
						1: common.ChannelStatusAutoDisabled,
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.channel.HasAvailableKey(); got != tt.want {
				t.Fatalf("HasAvailableKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNextEnabledKeySkipsBlankMultiKeys(t *testing.T) {
	channel := &Channel{
		Key: " \nvalid-key",
		ChannelInfo: ChannelInfo{
			IsMultiKey:   true,
			MultiKeyMode: constant.MultiKeyModeRandom,
		},
	}

	key, index, err := channel.GetNextEnabledKey()
	if err != nil {
		t.Fatalf("GetNextEnabledKey() returned error: %v", err)
	}
	if key != "valid-key" || index != 1 {
		t.Fatalf("GetNextEnabledKey() = (%q, %d), want (%q, %d)", key, index, "valid-key", 1)
	}
}
