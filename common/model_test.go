package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestIsOpenAIResponseCompactModel(t *testing.T) {
	t.Parallel()

	if !IsOpenAIResponseCompactModel("gpt-5-openai-compact") {
		t.Fatal("expected compact suffix model to be detected")
	}
	if IsOpenAIResponseCompactModel("gpt-5") {
		t.Fatal("did not expect regular model to be detected as compact")
	}
}

func TestGetEndpointTypesByChannelTypeForCompactModel(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "gpt-5-openai-compact")
	if len(endpoints) != 1 || endpoints[0] != constant.EndpointTypeOpenAIResponseCompact {
		t.Fatalf("expected compact model to expose only response compact endpoint, got %#v", endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForKilo(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeKilo, "gpt-5")
	if len(endpoints) != 1 || endpoints[0] != constant.EndpointTypeOpenAI {
		t.Fatalf("expected kilo channel to expose only chat completions endpoint, got %#v", endpoints)
	}
}
