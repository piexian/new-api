package controller

import (
	"context"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

// TypeSafe 渠道仅允许 typesafe 端点测试,显式指定其他端点须本地拦截
func TestChannelTestTypeSafeRejectsNonNativeEndpoint(t *testing.T) {
	channel := &model.Channel{
		Type:   constant.ChannelTypeTypeSafe,
		Name:   "typesafe-test",
		Models: "jev-latest",
	}

	for _, endpointType := range []string{"openai", "anthropic", "openai-response"} {
		result := testChannel(context.Background(), channel, 1, "jev-latest", endpointType, false)
		if result.localErr == nil {
			t.Fatalf("endpoint_type %q: expected local error, got nil", endpointType)
		}
		if !strings.Contains(result.localErr.Error(), "only supports the typesafe") {
			t.Fatalf("endpoint_type %q: unexpected error %q", endpointType, result.localErr.Error())
		}
	}
}
