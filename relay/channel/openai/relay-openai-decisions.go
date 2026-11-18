package openai

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	dto "github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// OpenAIDecisionsHandler 处理原生 Decisions 响应:官方无流式,body 原样透传,仅抽取 usage 计费。
func OpenAIDecisionsHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (usage any, err *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewError(fmt.Errorf("empty decisions response"), types.ErrorCodeBadResponse)
	}
	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewError(fmt.Errorf("failed to read decisions response: %w", readErr), types.ErrorCodeReadResponseBodyFailed)
	}
	service.CloseResponseBodyGracefully(resp)

	var parsed kitdto.OpenAIDecisionsResponse
	if unmarshalErr := common.Unmarshal(responseBody, &parsed); unmarshalErr != nil {
		// 解析失败仍把原文透传,计费走 usage 缺失路径
		service.IOCopyBytesGracefully(c, resp, responseBody)
		return nil, nil
	}
	service.IOCopyBytesGracefully(c, resp, responseBody)

	usage = parsed.Usage.ToUsage(info.GetEstimatePromptTokens())
	return usage, nil
}

// OpenAIDecisionsSystemOneHandler 处理 systemone 转换路径的 Decisions 响应:
// answers 数组按 name 回填 SystemOne 映射表(字段反向映射),usage 键名同构直接透传。
func OpenAIDecisionsSystemOneHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (usage any, err *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewError(fmt.Errorf("empty decisions response"), types.ErrorCodeBadResponse)
	}
	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewError(fmt.Errorf("failed to read decisions response: %w", readErr), types.ErrorCodeReadResponseBodyFailed)
	}
	service.CloseResponseBodyGracefully(resp)

	var parsed kitdto.OpenAIDecisionsResponse
	if unmarshalErr := common.Unmarshal(responseBody, &parsed); unmarshalErr != nil {
		// 解析失败仍把原文透传,计费走 usage 缺失路径
		service.IOCopyBytesGracefully(c, resp, responseBody)
		return nil, nil
	}

	response := dto.SystemOneResponse{
		Model:   info.UpstreamModelName,
		Answers: dto.SystemOneAnswersFromOpenAIDecisions(parsed.Answers),
	}
	if parsed.Usage != nil {
		response.Usage = &dto.SystemOneUsage{
			InputTokens:  parsed.Usage.InputTokens,
			OutputTokens: parsed.Usage.OutputTokens,
		}
	}
	if body, marshalErr := common.Marshal(response); marshalErr == nil {
		service.IOCopyBytesGracefully(c, resp, body)
	} else {
		service.IOCopyBytesGracefully(c, resp, responseBody)
	}

	usage = parsed.Usage.ToUsage(info.GetEstimatePromptTokens())
	return usage, nil
}
