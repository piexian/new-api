package gemini

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	rootconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Interactions API 版本路径白名单,入站版本原样镜像到上游
var geminiInteractionsPathPrefixes = []string{
	"/v1beta/interactions",
	"/v1beta2/interactions",
	"/v1/interactions",
}

// geminiInteractionsRequestURL 按入站路径构造上游 interactions URL(含 {id}/cancel 子路径与 query)
func geminiInteractionsRequestURL(info *relaycommon.RelayInfo) (string, error) {
	requestPath := info.RequestURLPath
	query := ""
	if idx := strings.Index(requestPath, "?"); idx >= 0 {
		query = requestPath[idx:]
		requestPath = requestPath[:idx]
	}
	for _, prefix := range geminiInteractionsPathPrefixes {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return fmt.Sprintf("%s%s%s", strings.TrimRight(info.ChannelBaseUrl, "/"), requestPath, query), nil
		}
	}
	return "", fmt.Errorf("unsupported gemini interactions path: %s", requestPath)
}

// saveGeminiInteractionState 创建成功后记录路由映射(interaction 上游状态按 key 隔离)
func saveGeminiInteractionState(info *relaycommon.RelayInfo, interaction *dto.GeminiInteraction, background bool, billed bool) {
	if info == nil || interaction == nil || interaction.ID == "" {
		return
	}
	service.SaveGeminiInteractionState(interaction.ID, &service.GeminiInteractionState{
		ChannelID:  info.ChannelId,
		Key:        info.ApiKey,
		UserID:     info.UserId,
		TokenID:    info.TokenId,
		Model:      info.OriginModelName,
		Background: background,
		Billed:     billed,
	})
}

// geminiInteractionRequestBackground 读取请求中的 background 标记
func geminiInteractionRequestBackground(info *relaycommon.RelayInfo) bool {
	if req, ok := info.Request.(*dto.GeminiInteractionsRequest); ok && req.Background != nil {
		return *req.Background
	}
	return false
}

// GeminiInteractionsHandler 非流式透传:解析 usage 与 id,原样回写响应体
func GeminiInteractionsHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	logger.LogDebug(c, "Gemini interactions response body: %s", responseBody)

	var interaction dto.GeminiInteraction
	if err := common.Unmarshal(responseBody, &interaction); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var usage *dto.Usage
	billed := false
	if interaction.Usage != nil && interaction.Usage.HasTokens() {
		usage = interaction.Usage.ToUsage(info.GetEstimatePromptTokens())
		billed = true
	} else {
		usage = &dto.Usage{}
	}
	saveGeminiInteractionState(info, &interaction, geminiInteractionRequestBackground(info), billed)

	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

// GeminiInteractionsStreamHandler 流式透传:逐事件回写 SSE(保留 event 行),
// 从 interaction.completed 提取 usage;断流时按已累积文本估算
func GeminiInteractionsStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	helper.SetEventStreamHeaders(c)

	var usage *dto.Usage
	var interactionID string
	var responseText strings.Builder
	background := geminiInteractionRequestBackground(info)

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		var event dto.GeminiInteractionSseEvent
		if err := common.UnmarshalJsonStr(data, &event); err != nil {
			sr.Stop(fmt.Errorf("unmarshal interactions sse event: %w", err))
			return
		}
		eventName := event.EventName()

		if event.Interaction != nil {
			if interactionID == "" && event.Interaction.ID != "" {
				interactionID = event.Interaction.ID
				saveGeminiInteractionState(info, event.Interaction, background, false)
			}
			if dto.IsGeminiInteractionTerminal(event.Interaction.Status) &&
				event.Interaction.Usage != nil && event.Interaction.Usage.HasTokens() {
				usage = event.Interaction.Usage.ToUsage(info.GetEstimatePromptTokens())
			}
		}
		// 兜底文本统计: step.delta 中的文本增量
		if eventName == "step.delta" {
			if text := gjson.Get(data, "delta.text").String(); text != "" {
				responseText.WriteString(text)
			}
		}

		if err := interactionsSseWrite(c, eventName, data); err != nil {
			logger.LogError(c, "failed to write interactions stream data: "+err.Error())
			sr.Stop(fmt.Errorf("write stream data: %w", err))
			return
		}
		info.SendResponseCount++
	})

	if usage == nil {
		if info.ReceivedResponseCount > 0 && responseText.Len() > 0 {
			usage = service.ResponseText2Usage(c, responseText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
			common.SetContextKey(c, rootconstant.ContextKeyLocalCountTokens, true)
		} else {
			usage = &dto.Usage{}
		}
	}
	// 已按 completed usage 计费的流,更新映射为已计费,避免后续 GET 重复结算
	if interactionID != "" && usage != nil && usage.TotalTokens > 0 {
		saveGeminiInteractionState(info, &dto.GeminiInteraction{ID: interactionID}, background, true)
	}
	return usage, nil
}

// interactionsSseWrite 按 SSE 规范回写 event + data 两行
func interactionsSseWrite(c *gin.Context, eventName string, data string) error {
	if eventName == "" {
		return helper.StringData(c, data)
	}
	c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("event: %s\n", eventName)})
	c.Render(-1, common.CustomEvent{Data: "data: " + data + "\n"})
	return helper.FlushWriter(c)
}

// GeminiInteractionsStatePassthrough get/cancel/delete 的非流式透传。
// onInteraction 回调返回解析出的 interaction 摘要,供调用方做异步结算与映射清理
func GeminiInteractionsStatePassthrough(c *gin.Context, resp *http.Response, onInteraction func(*dto.GeminiInteraction)) *types.NewAPIError {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	logger.LogDebug(c, "Gemini interactions state response body: %s", responseBody)

	if onInteraction != nil && len(responseBody) > 0 {
		var interaction dto.GeminiInteraction
		if err := common.Unmarshal(responseBody, &interaction); err == nil && interaction.ID != "" {
			onInteraction(&interaction)
		}
	}
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return nil
}

// GeminiInteractionsStateStreamPassthrough GET ?stream=true 续传的 SSE 透传,终态事件回调用于结算
func GeminiInteractionsStateStreamPassthrough(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, onInteraction func(*dto.GeminiInteraction)) *types.NewAPIError {
	helper.SetEventStreamHeaders(c)

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		var event dto.GeminiInteractionSseEvent
		if err := common.UnmarshalJsonStr(data, &event); err != nil {
			sr.Stop(fmt.Errorf("unmarshal interactions sse event: %w", err))
			return
		}
		eventName := event.EventName()
		if onInteraction != nil && event.Interaction != nil && dto.IsGeminiInteractionTerminal(event.Interaction.Status) {
			onInteraction(event.Interaction)
		}
		if err := interactionsSseWrite(c, eventName, data); err != nil {
			logger.LogError(c, "failed to write interactions stream data: "+err.Error())
			sr.Stop(fmt.Errorf("write stream data: %w", err))
			return
		}
		info.SendResponseCount++
	})
	return nil
}
