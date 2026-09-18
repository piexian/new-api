package stepfun

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// StepFun TTS（/v1/audio/speech）官方字段白名单。
// docs 列出的字段之外一律不下发，避免严格校验的上游 400。
var stepFunTTSAllowedFields = []string{
	"model",
	"input",
	"voice",
	"response_format",
	"speed",
	"volume",
	"text_normalization",
	"voice_label",
	"instruction",
	"sample_rate",
	"pronunciation_map",
	"markdown_filter",
	"stream_format",
	"return_url",
	"timestamp",
}

// ConvertAudioRequest 仅对 TTS 做官方字段白名单重建：
// dto.AudioRequest 未声明 StepFun 扩展字段（volume/text_normalization/voice_label/
// instruction/sample_rate/pronunciation_map/markdown_filter/return_url/timestamp），
// 直接序列化会丢失，因此从原始请求体恢复；OpenAI 私有字段（instructions/metadata/
// vllm-omini 参数等）按白名单丢弃，其中 instructions 会映射为 StepFun 的 instruction。
// 转写/翻译为 multipart，沿用通用实现原样透传。
func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info == nil || info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return a.Adaptor.ConvertAudioRequest(c, info, request)
	}

	payload := map[string]any{}
	if base, err := common.Marshal(request); err == nil {
		_ = common.Unmarshal(base, &payload)
	}
	original, hasOriginal := rawJSONFields(c)
	if hasOriginal {
		if _, exists := original["instruction"]; !exists {
			if instruction := rawStringField(original, "instructions"); instruction != "" {
				payload["instruction"] = instruction
			}
		}
	}

	cleaned := make(map[string]any, len(stepFunTTSAllowedFields))
	for _, key := range stepFunTTSAllowedFields {
		if value, ok := payload[key]; ok {
			cleaned[key] = value
		}
	}
	if hasOriginal {
		for _, key := range stepFunTTSAllowedFields {
			if _, ok := cleaned[key]; ok {
				continue
			}
			if raw, ok := original[key]; ok && len(raw) > 0 {
				var value any
				if err := common.Unmarshal(raw, &value); err == nil {
					cleaned[key] = value
				}
			}
		}
	}

	body, err := common.Marshal(cleaned)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func rawStringField(fields map[string]json.RawMessage, key string) string {
	raw, ok := fields[key]
	if !ok || len(raw) == 0 {
		return ""
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// rawJSONFields 读取原始 JSON 请求体字段，用于恢复 DTO 未声明的上游扩展参数。
func rawJSONFields(c *gin.Context) (map[string]json.RawMessage, bool) {
	if c == nil || c.Request == nil || c.Request.Body == nil || c.Request.Body == http.NoBody {
		return nil, false
	}
	if !strings.Contains(strings.ToLower(c.Request.Header.Get("Content-Type")), "application/json") {
		return nil, false
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, false
	}
	data, err := storage.Bytes()
	if err != nil || len(data) == 0 {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(data, &fields); err != nil || len(fields) == 0 {
		return nil, false
	}
	return fields, true
}

// ttsJSONHandler 处理 return_url=true 的 TTS 响应：上游返回 JSON（音频下载 URL 与
// 可选字幕），必须原样透传，不能走二进制写入路径。计费沿用二进制的估算口径
// （prompt 估算 token；音频时长未知，不做 completion token 估算）。
func ttsJSONHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) *dto.Usage {
	defer service.CloseResponseBodyGracefully(resp)

	usage := &dto.Usage{}
	if info != nil {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	usage.TotalTokens = usage.PromptTokens
	usage.PromptTokensDetails.TextTokens = usage.PromptTokens

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.LogError(c, "failed to read StepFun TTS json response: "+err.Error())
		c.Writer.WriteHeaderNow()
		return usage
	}
	service.IOCopyBytesGracefully(c, resp, body)
	return usage
}
