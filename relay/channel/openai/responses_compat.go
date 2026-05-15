package openai

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/responsescompat"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ChatCompletionResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &chatResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	responsesResp, usage := responsescompat.ChatCompletionToResponse(c, info, &chatResp)
	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

func ChatCompletionResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	var streamErr *types.NewAPIError
	emitter := responsescompat.NewStreamEmitter(c, info)

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}
		var streamResp dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResp); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			sr.Stop(streamErr)
			return
		}
		if streamResp.Model != "" {
			emitter.SetModel(streamResp.Model)
		}
		if streamResp.Created != 0 {
			emitter.SetCreatedAt(streamResp.Created)
		}
		if service.ValidUsage(streamResp.Usage) {
			emitter.SetUsage(streamResp.Usage)
		}
		for _, choice := range streamResp.Choices {
			if delta := choice.Delta.GetReasoningContent(); delta != "" {
				if !emitter.SendTextDelta(delta) {
					sr.Stop(emitter.Err())
					return
				}
			}
			if delta := choice.Delta.GetContentString(); delta != "" {
				if !emitter.SendTextDelta(delta) {
					sr.Stop(emitter.Err())
					return
				}
			}
		}
	})
	if streamErr != nil {
		return nil, streamErr
	}
	if !emitter.Complete() {
		return nil, emitter.Err()
	}
	return emitter.Usage(), nil
}
