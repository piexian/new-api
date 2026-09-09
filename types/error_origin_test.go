package types

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestErrorOriginSurvivesWrappingAndProtocolConversion(t *testing.T) {
	upstream := WithOpenAIError(OpenAIError{Message: "upstream limit", Type: "rate_limit_error", Code: "rate_limit", NewAPIError: true}, 429)
	local := NewError(errors.New("blocked locally"), ErrorCodeAccessDenied)
	cases := []struct {
		name  string
		err   *NewAPIError
		local bool
	}{
		{"local", local, true},
		{"local OpenAI format", NewOpenAIError(errors.New("conversion failed"), ErrorCodeConvertRequestFailed, 400), true},
		{"local custom rule", WithOpenAIError(OpenAIError{Message: "blocked", Type: "invalid_request_error"}, 400, ErrOptionWithLocalError()), true},
		{"upstream", upstream, false},
		{"upstream Claude", WithClaudeError(ClaudeError{Message: "upstream rejection", Type: "invalid_request_error"}, 400), false},
		{"upstream fallback", InitOpenAIError(ErrorCodeBadResponseStatusCode, 502), false},
		{"upstream wrapped", NewError(fmt.Errorf("wrapped: %w", upstream), ErrorCodeBadResponse), false},
		{"upstream OpenAI wrapped", NewOpenAIError(upstream, ErrorCodeBadResponse, 502), false},
		{"upstream status wrapped", NewErrorWithStatusCode(upstream, ErrorCodeBadResponse, 502), false},
		{"explicit upstream", NewOpenAIError(errors.New("provider rejected"), ErrorCodePromptBlocked, 400, ErrOptionWithUpstreamError()), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.local, tc.err.IsLocalError())
			for _, payload := range []any{tc.err.ToOpenAIError(), tc.err.ToClaudeError()} {
				encoded, err := common.Marshal(payload)
				require.NoError(t, err)
				if tc.local {
					require.Contains(t, string(encoded), `"new_api_error":true`)
				} else {
					require.NotContains(t, string(encoded), `"new_api_error":`)
				}
			}
		})
	}
	require.Equal(t, http.StatusTooManyRequests, upstream.StatusCode)
	require.Equal(t, "rate_limit_error", upstream.ToOpenAIError().Type)
}

func TestClientErrorLabelsUpstreamWithoutChangingInternalMessage(t *testing.T) {
	upstream := WithOpenAIError(OpenAIError{Message: "rate limit exceeded", Type: "rate_limit_error", Code: "rate_limit"}, 429)
	client := upstream.ToClientOpenAIError()
	require.Equal(t, "上游返回：rate limit exceeded", client.Message)
	require.Equal(t, "rate_limit_error", client.Type)
	require.Equal(t, "rate_limit", client.Code)
	require.False(t, client.NewAPIError)
	require.Equal(t, "上游返回：rate limit exceeded", upstream.ToClientClaudeError().Message)
	require.Equal(t, "rate limit exceeded", upstream.Error())
	require.Equal(t, "rate limit exceeded", upstream.ToOpenAIError().Message)
	local := NewError(errors.New("blocked locally"), ErrorCodeAccessDenied)
	require.Equal(t, "blocked locally", local.ToClientOpenAIError().Message)
	require.True(t, local.ToClientOpenAIError().NewAPIError)
	require.Equal(t, "blocked locally", local.ToClientClaudeError().Message)
	message := "上游返回 HTML（HTTP 502）"
	require.Equal(t, message, FormatUpstreamErrorMessage(message))
	require.Equal(t, client.Message, FormatUpstreamErrorMessage(client.Message))
}
