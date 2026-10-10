package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

const testImageBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC"

func TestClaudeConvertersPreserveImageContent(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "json"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			for _, legacy := range []bool{false, true} {
				t.Run(map[bool]string{false: "registry", true: "legacy"}[legacy], func(t *testing.T) {
					var req dto.GeneralOpenAIRequest
					require.NoError(t, common.Unmarshal([]byte(`{"model":"claude-test","messages":[{"role":"user","content":[{"type":"text","text":"Describe this image"},{"type":"image_url","image_url":{"url":"data:image/png;base64,`+testImageBase64+`"}}]}]}`), &req))
					if typed {
						req.Messages[0].SetMediaContent(req.Messages[0].ParseContent())
					}
					// kit 转换器要求 max_tokens（Claude API 强制），与生产请求一致。
					maxTokens := uint(1024)
					req.MaxTokens = &maxTokens
					copied, err := common.DeepCopy(&req)
					require.NoError(t, err)
					var converted *dto.ClaudeRequest
					if legacy {
						converted, err = RequestOpenAI2ClaudeMessage(nil, nil, *copied)
					} else {
						var result *relayconvert.RequestResult
						result, err = relayconvert.ConvertRequest(nil, nil, types.RelayFormatClaude, copied)
						if err == nil {
							converted = result.Value.(*dto.ClaudeRequest)
						}
					}
					require.NoError(t, err)
					wire, err := common.Marshal(converted)
					require.NoError(t, err)
					var sent dto.ClaudeRequest
					require.NoError(t, common.Unmarshal(wire, &sent))
					require.Len(t, sent.Messages, 1)
					parts, err := sent.Messages[0].ParseContent()
					require.NoError(t, err)
					require.Len(t, parts, 2)
					require.Equal(t, "Describe this image", parts[0].GetText())
					require.Equal(t, "image", parts[1].Type)
					require.Equal(t, "image/png", parts[1].Source.MediaType)
					require.Equal(t, testImageBase64, parts[1].Source.Data)
				})
			}
		})
	}
}
