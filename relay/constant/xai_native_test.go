package constant

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPath2RelayModeSupportsXAINativeRoutes(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/v1/realtime/client_secrets",
		"/v1/tts",
		"/v1/tts/voices",
		"/v1/tts/voices/eve",
		"/v1/stt",
		"/v1/custom-voices",
		"/v1/custom-voices/voice123/audio",
		"/v1/responses/resp_123",
		"/v1/chat/deferred-completion/req_123",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, RelayModeXAINative, Path2RelayMode(path))
		})
	}
}

func TestPath2RelayModeKeepsXAICompatibleRoutes(t *testing.T) {
	t.Parallel()

	require.Equal(t, RelayModeRealtime, Path2RelayMode("/v1/realtime"))
	require.Equal(t, RelayModeResponsesCompact, Path2RelayMode("/v1/responses/compact"))
	require.Equal(t, RelayModeResponses, Path2RelayMode("/v1/responses"))
}

func TestPath2RelayModeSupportsMoarkNativeRoutes(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/v1/async/music/generations",
		"/v1/async/images/edits",
		"/v1/tasks",
		"/v1/tasks/available-quota",
		"/v1/task/task_123",
		"/v1/task/task_123/get",
		"/v1/task/task_123/status",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, RelayModeMoarkNative, Path2RelayMode(path))
		})
	}
}
