package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type channelUsageKeySelection struct {
	KeyIndex       int
	KeyCount       int
	KeyLabel       string
	Key            string
	KeyStatus      int
	DisabledReason string
	DisabledTime   int64
}

func resolveChannelUsageKeySelection(channel *model.Channel, rawKeyIndex string) (*channelUsageKeySelection, error) {
	keys := channel.GetKeys()
	if len(keys) == 0 {
		if trimmed := strings.TrimSpace(channel.Key); trimmed != "" {
			keys = []string{trimmed}
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("channel has no keys")
	}

	keyIndex := 0
	if trimmed := strings.TrimSpace(rawKeyIndex); trimmed != "" {
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid key_index: %w", err)
		}
		keyIndex = parsed
	}
	if keyIndex < 0 || keyIndex >= len(keys) {
		return nil, fmt.Errorf("key_index out of range: %d", keyIndex)
	}

	key := strings.TrimSpace(keys[keyIndex])
	if key == "" {
		return nil, fmt.Errorf("selected key is empty")
	}

	keyStatus, disabledReason, disabledTime := channelKeyStatusMeta(channel, keyIndex)
	return &channelUsageKeySelection{
		KeyIndex:       keyIndex,
		KeyCount:       len(keys),
		KeyLabel:       fmt.Sprintf("Key #%d", keyIndex+1),
		Key:            key,
		KeyStatus:      keyStatus,
		DisabledReason: disabledReason,
		DisabledTime:   disabledTime,
	}, nil
}

func channelKeyStatusMeta(channel *model.Channel, keyIndex int) (status int, disabledReason string, disabledTime int64) {
	status = common.ChannelStatusEnabled
	if channel == nil {
		return status, "", 0
	}
	if channel.ChannelInfo.MultiKeyStatusList != nil {
		if value, ok := channel.ChannelInfo.MultiKeyStatusList[keyIndex]; ok {
			status = value
		}
	}
	if channel.ChannelInfo.MultiKeyDisabledReason != nil {
		disabledReason = strings.TrimSpace(channel.ChannelInfo.MultiKeyDisabledReason[keyIndex])
	}
	if channel.ChannelInfo.MultiKeyDisabledTime != nil {
		disabledTime = channel.ChannelInfo.MultiKeyDisabledTime[keyIndex]
	}
	return status, disabledReason, disabledTime
}
