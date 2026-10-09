package openai

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// OpenaiEmbeddingHandler 复用通用 OpenAI 响应处理，仅在客户端请求
// encoding_format=base64 而上游返回 float 数组时，网关侧转换为 float32
// 小端 Base64（讯飞星辰 MaaS 等只返回 float 的上游）。
func OpenaiEmbeddingHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	return OpenaiHandlerWithBodyTransformer(c, info, resp, embeddingBase64Transformer(info))
}

func embeddingBase64Transformer(info *relaycommon.RelayInfo) func([]byte) ([]byte, error) {
	req, ok := info.Request.(*dto.EmbeddingRequest)
	if !ok || req.EncodingFormat != "base64" {
		return nil
	}
	return convertEmbeddingArraysToBase64
}

func convertEmbeddingArraysToBase64(body []byte) ([]byte, error) {
	data := gjson.GetBytes(body, "data")
	if !data.IsArray() {
		return body, nil
	}
	result := body
	for i, item := range data.Array() {
		emb := item.Get("embedding")
		if !emb.IsArray() {
			// 上游已返回 base64 字符串（或字段缺失），保持原样
			continue
		}
		var floats []float64
		if err := common.Unmarshal([]byte(emb.Raw), &floats); err != nil {
			return nil, err
		}
		buf := make([]byte, len(floats)*4)
		for j, f := range floats {
			binary.LittleEndian.PutUint32(buf[j*4:], math.Float32bits(float32(f)))
		}
		encoded, err := sjson.SetBytes(result, fmt.Sprintf("data.%d.embedding", i), base64.StdEncoding.EncodeToString(buf))
		if err != nil {
			return nil, err
		}
		result = encoded
	}
	return result, nil
}
