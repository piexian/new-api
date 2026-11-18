package gemini

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

// GeminiDecisionsHandler 处理原生 Decisions 响应:官方无流式,body 原样透传,仅抽取 usage 计费。
func GeminiDecisionsHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (usage any, err *types.NewAPIError) {
	return decisionsResponse(c, info, resp, "")
}

// GeminiDecisionsSystemOneHandler 处理 systemone 转换路径的 Decisions 响应:
// 回填 SystemOne 形状(model 回填、usage 键名映射),answers 原样透传。
func GeminiDecisionsSystemOneHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (usage any, err *types.NewAPIError) {
	return decisionsResponse(c, info, resp, info.UpstreamModelName)
}

// decisionsResponse 读取上游 Decisions 响应:解析出 usage 供计费;
// systemOneModel 为空时 body 原样写回,非空时重打包为 SystemOne 响应。
func decisionsResponse(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, systemOneModel string) (usage any, newAPIError *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewError(fmt.Errorf("empty decisions response"), types.ErrorCodeBadResponse)
	}
	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewError(fmt.Errorf("failed to read decisions response: %w", readErr), types.ErrorCodeReadResponseBodyFailed)
	}
	service.CloseResponseBodyGracefully(resp)

	var parsed kitdto.GeminiDecisionsResponse
	if unmarshalErr := common.Unmarshal(responseBody, &parsed); unmarshalErr != nil {
		// 解析失败仍把原文透传,计费走 usage 缺失路径
		service.IOCopyBytesGracefully(c, resp, responseBody)
		return nil, nil
	}

	if systemOneModel == "" {
		service.IOCopyBytesGracefully(c, resp, responseBody)
	} else if systemOneBody, marshalErr := buildSystemOneResponse(systemOneModel, parsed); marshalErr == nil {
		service.IOCopyBytesGracefully(c, resp, systemOneBody)
	} else {
		service.IOCopyBytesGracefully(c, resp, responseBody)
	}

	usage = parsed.Usage.ToUsage(info.GetEstimatePromptTokens())
	return usage, nil
}

// buildSystemOneResponse 把 Decisions 响应回填为 SystemOne 形状:
// answers 原样保留,usage 映射 input_tokens/output_tokens;prompt_feedback 为
// SystemOne 协议没有的字段,被拦截的请求 answers 为空,直接丢弃不影响语义。
func buildSystemOneResponse(model string, parsed kitdto.GeminiDecisionsResponse) ([]byte, error) {
	response := dto.SystemOneResponse{
		Model:   model,
		Answers: parsed.Answers,
	}
	if parsed.Usage != nil {
		response.Usage = &dto.SystemOneUsage{
			InputTokens:  parsed.Usage.TotalInputTokens,
			OutputTokens: parsed.Usage.TotalOutputTokens,
		}
	}
	return common.Marshal(response)
}
