package relay

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel/stepfun"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// StepFunNativeWssHelper 原始双向透传 StepFun 的 WebSocket 端点
// （/v1/realtime/audio 流式 TTS、/v1/audio/asr/stream 双向识别、/v1/realtime 双向对话）：
// 三套协议各不相同，网关只做帧级转发，不改写任何事件。
func StepFunNativeWssHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.ChannelType != constant.ChannelTypeStepFun {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("StepFun websocket endpoints require a StepFun channel, got channel type %d", info.ChannelType),
			types.ErrorCodeInvalidApiType,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	if info.ClientWs == nil {
		return types.NewError(fmt.Errorf("invalid client websocket connection"), types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}

	pathOnly, _ := stepfun.SplitPathQuery(info.RequestURLPath)
	endpoint, ok := stepfun.LookupWssEndpoint(pathOnly)
	if !ok {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("unsupported StepFun websocket endpoint %q", info.RequestURLPath),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	if !endpoint.AvailableOnBase(info.ChannelBaseUrl) {
		// 实测：双向 ASR 仅开放平台提供，Step Plan 通道上游 404
		return types.NewErrorWithStatusCode(
			fmt.Errorf("endpoint %s is only available on the StepFun open platform, not on the Step Plan channel", endpoint.Path),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	resp, err := adaptor.DoRequest(c, info, nil)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	targetWs, ok := resp.(*websocket.Conn)
	if !ok || targetWs == nil {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid websocket response type: %T", resp), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	info.TargetWs = targetWs
	defer targetWs.Close()

	stepFunProxyWebSocket(c, info)
	// 上游 WS 不返回用量，按请求体估算 prompt token 计费（与 xAI 原生 WS 透传口径一致）。
	service.PostTextConsumeQuota(c, info, stepFunNativeUsage(info), []string{"StepFun native WebSocket passthrough"})
	return nil
}

func stepFunProxyWebSocket(c *gin.Context, info *relaycommon.RelayInfo) {
	done := make(chan struct{}, 2)
	errChan := make(chan error, 2)

	gopool.Go(func() {
		stepFunCopyWebSocket(c, "client to StepFun", info.ClientWs, info.TargetWs, nil, done, errChan)
	})
	gopool.Go(func() {
		stepFunCopyWebSocket(c, "StepFun to client", info.TargetWs, info.ClientWs, func() {
			info.SetFirstResponseTime()
		}, done, errChan)
	})

	select {
	case <-done:
	case err := <-errChan:
		logger.LogError(c, "StepFun native websocket proxy error: "+err.Error())
	case <-c.Done():
	}

	_ = info.ClientWs.Close()
	_ = info.TargetWs.Close()
}

func stepFunCopyWebSocket(c *gin.Context, direction string, src *websocket.Conn, dst *websocket.Conn, beforeWrite func(), done chan<- struct{}, errChan chan<- error) {
	defer func() {
		if r := recover(); r != nil {
			errChan <- fmt.Errorf("panic in %s websocket copy: %v", direction, r)
		}
	}()

	for {
		messageType, message, err := src.ReadMessage()
		if err != nil {
			if stepFunIsNormalWebSocketClose(err) {
				done <- struct{}{}
			} else {
				errChan <- fmt.Errorf("read %s failed: %w", direction, err)
			}
			return
		}
		if beforeWrite != nil {
			beforeWrite()
		}
		if err := dst.WriteMessage(messageType, message); err != nil {
			if stepFunIsNormalWebSocketClose(err) {
				done <- struct{}{}
			} else {
				errChan <- fmt.Errorf("write %s failed: %w", direction, err)
			}
			return
		}
	}
}

func stepFunIsNormalWebSocketClose(err error) bool {
	if err == nil {
		return true
	}
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) ||
		strings.Contains(err.Error(), "use of closed network connection")
}
