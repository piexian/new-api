package opencode

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func claudeToGeminiResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("invalid response"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}
	var message dto.ClaudeResponse
	if err := common.Unmarshal(body, &message); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if upstreamErr := message.GetClaudeError(); upstreamErr != nil && upstreamErr.Type != "" {
		return nil, types.WithClaudeError(*upstreamErr, resp.StatusCode)
	}
	converted, err := relayconvert.ConvertResponse(c, info, types.RelayFormatGemini, &message)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	result, err := common.Marshal(converted.Value)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusBadGateway)
	}
	service.IOCopyBytesGracefully(c, resp, result)
	return converted.Usage, nil
}

func claudeToConvertedStream(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, target types.RelayFormat) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("invalid response"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(resp)

	state, err := relayconvert.NewResponseStreamState(types.RelayFormatClaude, target, relayconvert.ResponseStreamOptions{
		ID: helper.GetResponseID(c), Model: info.UpstreamModelName, Created: common.GetTimestamp(),
	})
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	if target == types.RelayFormatOpenAIResponses && info.ClaudeConvertInfo == nil {
		info.ClaudeConvertInfo = &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone}
	}
	var observedUsage dto.ClaudeUsage
	var hasObservedUsage bool
	var streamErr *types.NewAPIError
	writeResults := func(results []relayconvert.ResponseResult) {
		for _, result := range results {
			var writeErr error
			if target == types.RelayFormatOpenAIResponses {
				var event relayconvert.ChatToResponsesStreamEvent
				switch value := result.Value.(type) {
				case relayconvert.ChatToResponsesStreamEvent:
					event = value
				case *relayconvert.ChatToResponsesStreamEvent:
					if value == nil {
						continue
					}
					event = *value
				default:
					streamErr = types.NewOpenAIError(fmt.Errorf("unexpected Responses stream event: %T", result.Value), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
					return
				}
				data, err := common.Marshal(event.Payload)
				if err != nil {
					streamErr = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusBadGateway)
					return
				}
				writeErr = helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: event.Type}, string(data))
			} else {
				writeErr = helper.ObjectData(c, result.Value)
			}
			if writeErr != nil {
				streamErr = types.NewOpenAIError(writeErr, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
				return
			}
		}
	}
	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}
		var message dto.ClaudeResponse
		if err := common.UnmarshalJsonStr(data, &message); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
			sr.Stop(streamErr)
			return
		}
		if upstreamErr := message.GetClaudeError(); upstreamErr != nil && upstreamErr.Type != "" {
			streamErr = types.WithClaudeError(*upstreamErr, resp.StatusCode)
			sr.Stop(streamErr)
			return
		}
		if message.Message != nil && message.Message.Usage != nil {
			observedUsage = *message.Message.Usage
			hasObservedUsage = true
		}
		if message.Usage != nil {
			if message.Usage.InputTokens > 0 {
				observedUsage.InputTokens = message.Usage.InputTokens
			}
			if message.Usage.OutputTokens > 0 {
				observedUsage.OutputTokens = message.Usage.OutputTokens
			}
			if message.Usage.CacheCreationInputTokens > 0 {
				observedUsage.CacheCreationInputTokens = message.Usage.CacheCreationInputTokens
			}
			if message.Usage.CacheReadInputTokens > 0 {
				observedUsage.CacheReadInputTokens = message.Usage.CacheReadInputTokens
			}
			hasObservedUsage = true
		}
		results, err := relayconvert.ConvertStreamResponseChunk(c, info, state, &message)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
			sr.Stop(streamErr)
			return
		}
		writeResults(results)
		if streamErr != nil {
			sr.Stop(streamErr)
		}
	})
	if streamErr != nil {
		return nil, streamErr
	}
	results, err := relayconvert.FinalizeStreamResponse(c, info, state)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	writeResults(results)
	if streamErr != nil {
		return nil, streamErr
	}
	usage := state.Usage()
	if hasObservedUsage {
		usage = relayconvert.UsageFromClaudeAPIUsage(&observedUsage)
		state.SetUsage(usage)
	}
	if usage == nil || usage.TotalTokens == 0 {
		usage = service.ResponseText2Usage(c, state.UsageText(), info.UpstreamModelName, info.GetEstimatePromptTokens())
	}
	return usage, nil
}
