package minimax

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type miniMaxNativeMusicResponse struct {
	BaseResp MiniMaxBaseResp `json:"base_resp"`
}

func isMiniMaxNativeMusicRelayMode(relayMode int) bool {
	switch relayMode {
	case relayconstant.RelayModeMiniMaxMusicGeneration,
		relayconstant.RelayModeMiniMaxMusicCoverPreprocess,
		relayconstant.RelayModeMiniMaxLyricsGeneration:
		return true
	default:
		return false
	}
}

func miniMaxNativeMusicHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid minimax music response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	if strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		usage := miniMaxNativeMusicUsage(info)
		streamMiniMaxNativeResponse(c, resp)
		return usage, nil
	}

	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(readErr, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	if len(bytes.TrimSpace(responseBody)) == 0 {
		return nil, types.NewOpenAIError(fmt.Errorf("empty minimax music response"), types.ErrorCodeEmptyResponse, http.StatusInternalServerError)
	}

	var nativeResp miniMaxNativeMusicResponse
	if err := common.Unmarshal(responseBody, &nativeResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if nativeResp.BaseResp.StatusCode != 0 {
		message := nativeResp.BaseResp.StatusMsg
		if message == "" {
			message = fmt.Sprintf("minimax music error: %d", nativeResp.BaseResp.StatusCode)
		}
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: message,
			Type:    "minimax_music_error",
			Code:    fmt.Sprintf("%d", nativeResp.BaseResp.StatusCode),
		}, http.StatusBadRequest)
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)

	return miniMaxNativeMusicUsage(info), nil
}

func miniMaxNativeMusicUsage(info *relaycommon.RelayInfo) *dto.Usage {
	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens <= 0 {
		promptTokens = 1
	}
	return &dto.Usage{
		PromptTokens: promptTokens,
		TotalTokens:  promptTokens,
	}
}

func streamMiniMaxNativeResponse(c *gin.Context, resp *http.Response) {
	defer service.CloseResponseBodyGracefully(resp)

	for k, v := range resp.Header {
		if k == "Content-Length" || len(v) == 0 {
			continue
		}
		c.Writer.Header().Set(k, v[0])
	}
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
	c.Writer.Flush()
}
