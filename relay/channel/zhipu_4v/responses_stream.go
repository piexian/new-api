package zhipu_4v

import (
	"errors"
	"fmt"
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

func zhipuClaudeResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	responseID := helper.GetResponseID(c)
	state, err := relayconvert.NewResponseStreamState(types.RelayFormatClaude, types.RelayFormatOpenAIResponses, relayconvert.ResponseStreamOptions{
		ID:      responseID,
		Model:   info.UpstreamModelName,
		Created: common.GetTimestamp(),
	})
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	if info.ClaudeConvertInfo == nil {
		info.ClaudeConvertInfo = &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone}
	}
	var observedUsage dto.ClaudeUsage
	var hasObservedUsage bool

	var streamErr *types.NewAPIError
	var sendResult func(relayconvert.ResponseResult) bool
	sendResult = func(result relayconvert.ResponseResult) bool {
		switch value := result.Value.(type) {
		case relayconvert.ChatToResponsesStreamEvent:
			data, marshalErr := common.Marshal(value.Payload)
			if marshalErr != nil {
				streamErr = types.NewOpenAIError(marshalErr, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
				return false
			}
			if writeErr := helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: value.Type}, string(data)); writeErr != nil {
				streamErr = types.NewOpenAIError(writeErr, types.ErrorCodeBadResponse, http.StatusInternalServerError)
				return false
			}
			return true
		case *relayconvert.ChatToResponsesStreamEvent:
			if value == nil {
				return true
			}
			return sendResult(relayconvert.ResponseResult{Value: *value})
		default:
			streamErr = types.NewOpenAIError(fmt.Errorf("expected OpenAI Responses stream event, got %T", result.Value), types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}
		var claudeResponse dto.ClaudeResponse
		if err := common.UnmarshalJsonStr(data, &claudeResponse); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			sr.Stop(streamErr)
			return
		}
		if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
			streamErr = types.WithClaudeError(*claudeError, http.StatusInternalServerError)
			sr.Stop(streamErr)
			return
		}
		if claudeResponse.Message != nil && claudeResponse.Message.Usage != nil {
			observedUsage = *claudeResponse.Message.Usage
			hasObservedUsage = true
		}
		if claudeResponse.Usage != nil {
			if claudeResponse.Usage.InputTokens > 0 {
				observedUsage.InputTokens = claudeResponse.Usage.InputTokens
			}
			if claudeResponse.Usage.OutputTokens > 0 {
				observedUsage.OutputTokens = claudeResponse.Usage.OutputTokens
			}
			if claudeResponse.Usage.CacheCreationInputTokens > 0 {
				observedUsage.CacheCreationInputTokens = claudeResponse.Usage.CacheCreationInputTokens
			}
			if claudeResponse.Usage.CacheReadInputTokens > 0 {
				observedUsage.CacheReadInputTokens = claudeResponse.Usage.CacheReadInputTokens
			}
			hasObservedUsage = true
		}
		results, err := relayconvert.ConvertStreamResponseChunk(c, info, state, &claudeResponse)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			sr.Stop(streamErr)
			return
		}
		for _, result := range results {
			if !sendResult(result) {
				sr.Stop(streamErr)
				return
			}
		}
	})
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
		state.SetUsage(usage)
	}
	finalResults, err := relayconvert.FinalizeStreamResponse(c, info, state)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	for _, result := range finalResults {
		if !sendResult(result) {
			return nil, streamErr
		}
	}
	return usage, nil
}
