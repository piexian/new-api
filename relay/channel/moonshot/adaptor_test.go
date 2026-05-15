package moonshot

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestGetRequestURLUsesMessagesForKimiCodingPlan(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "special base",
			baseURL: "kimi-coding-plan",
			want:    "https://api.kimi.com/coding/v1/messages",
		},
		{
			name:    "custom coding base",
			baseURL: "https://example.com/coding",
			want:    "https://example.com/coding/v1/messages",
		},
		{
			name:    "custom coding v1 base",
			baseURL: "https://example.com/coding/v1",
			want:    "https://example.com/coding/v1/messages",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			adaptor := &Adaptor{}
			got, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
				RelayMode:   relayconstant.RelayModeChatCompletions,
				RelayFormat: types.RelayFormatOpenAI,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: testCase.baseURL,
				},
			})
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("GetRequestURL() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestGetRequestURLKeepsOpenAIEndpointForRegularMoonshot(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	got, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.moonshot.cn",
		},
	})
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	want := "https://api.moonshot.cn/v1/chat/completions"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestConvertOpenAIRequestReturnsClaudeRequestForKimiCodingPlan(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	adaptor := &Adaptor{}
	converted, err := adaptor.ConvertOpenAIRequest(c, &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "kimi-k2.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "kimi-coding-plan",
			UpstreamModelName: "kimi-k2.5",
		},
	}, &dto.GeneralOpenAIRequest{
		Model: "kimi-k2.5",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hi",
			},
		},
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	if _, ok := converted.(*dto.ClaudeRequest); !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.ClaudeRequest", converted)
	}
}

func TestSetupRequestHeaderKeepsBearerAuthForKimiCodingPlan(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	adaptor := &Adaptor{}
	headers := make(http.Header)
	err := adaptor.SetupRequestHeader(c, &headers, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:         "kimi-key",
			ChannelBaseUrl: "kimi-coding-plan",
		},
	})
	if err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if headers.Get("Authorization") != "Bearer kimi-key" {
		t.Fatalf("Authorization = %q, want Bearer kimi-key", headers.Get("Authorization"))
	}
}
