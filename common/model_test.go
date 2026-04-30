package common

import (
	"reflect"
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

func TestGetEndpointTypesByChannelTypeForPoeClaudeModel(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypePoe, "Claude-Sonnet-4.6")
	want := []constant.EndpointType{
		constant.EndpointTypeAnthropic,
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Poe Claude model endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForPoeNonClaudeModel(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypePoe, "GPT-5.4")
	want := []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Poe non-Claude model endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForZhipuClaudeModel(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeZhipu, "claude-3-7-sonnet")
	want := []constant.EndpointType{
		constant.EndpointTypeAnthropic,
		constant.EndpointTypeOpenAI,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Zhipu Claude model endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForZhipuNonClaudeModel(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeZhipu, "glm-4-plus")
	want := []constant.EndpointType{
		constant.EndpointTypeOpenAI,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Zhipu non-Claude model endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForDeepSeek(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeDeepSeek, "deepseek-chat")
	want := []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeAnthropic,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected DeepSeek endpoints %#v, got %#v", want, endpoints)
	}
}

func TestGetEndpointTypesByChannelTypeForXunfeiMaaSImage(t *testing.T) {
	t.Parallel()

	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeXunfeiMaaSImage, "idxskolorss2b6")
	want := []constant.EndpointType{
		constant.EndpointTypeImageGeneration,
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("expected Xunfei MaaS image channel endpoints %#v, got %#v", want, endpoints)
	}
}
