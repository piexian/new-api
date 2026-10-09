package openai

import (
	"encoding/base64"
	"encoding/binary"
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func decodeBase64Embedding(t *testing.T, s string) []float32 {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(s)
	require.NoError(t, err)
	require.Zero(t, len(raw)%4)
	out := make([]float32, len(raw)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return out
}

func TestConvertEmbeddingArraysToBase64(t *testing.T) {
	t.Run("float arrays convert to base64 float32 LE", func(t *testing.T) {
		body := `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[1.5,-2.25,0]}],"model":"m","usage":{"prompt_tokens":3,"total_tokens":3}}`
		out, err := convertEmbeddingArraysToBase64([]byte(body))
		require.NoError(t, err)
		encoded := gjson.GetBytes(out, "data.0.embedding").String()
		require.Equal(t, []float32{1.5, -2.25, 0}, decodeBase64Embedding(t, encoded))
		// 其余字段保持原样
		require.Equal(t, "list", gjson.GetBytes(out, "object").String())
		require.Equal(t, int64(3), gjson.GetBytes(out, "usage.prompt_tokens").Int())
	})

	t.Run("existing base64 strings are kept", func(t *testing.T) {
		original := base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4})
		body := `{"data":[{"index":0,"embedding":"` + original + `"}]}`
		out, err := convertEmbeddingArraysToBase64([]byte(body))
		require.NoError(t, err)
		require.Equal(t, original, gjson.GetBytes(out, "data.0.embedding").String())
	})

	t.Run("batch input converts every item by index", func(t *testing.T) {
		body := `{"data":[{"index":0,"embedding":[0.5]},{"index":1,"embedding":[-0.5]}]}`
		out, err := convertEmbeddingArraysToBase64([]byte(body))
		require.NoError(t, err)
		require.Equal(t, []float32{0.5}, decodeBase64Embedding(t, gjson.GetBytes(out, "data.0.embedding").String()))
		require.Equal(t, []float32{-0.5}, decodeBase64Embedding(t, gjson.GetBytes(out, "data.1.embedding").String()))
	})

	t.Run("non-list body passes through", func(t *testing.T) {
		body := `{"error":{"message":"bad"}}`
		out, err := convertEmbeddingArraysToBase64([]byte(body))
		require.NoError(t, err)
		require.JSONEq(t, body, string(out))
	})
}

func TestEmbeddingBase64Transformer(t *testing.T) {
	t.Run("nil when request is not base64", func(t *testing.T) {
		info := &relaycommon.RelayInfo{Request: &dto.EmbeddingRequest{Model: "m", EncodingFormat: "float"}}
		require.Nil(t, embeddingBase64Transformer(info))
	})

	t.Run("nil when request type mismatch", func(t *testing.T) {
		require.Nil(t, embeddingBase64Transformer(&relaycommon.RelayInfo{}))
	})

	t.Run("set when base64 requested", func(t *testing.T) {
		info := &relaycommon.RelayInfo{Request: &dto.EmbeddingRequest{Model: "m", EncodingFormat: "base64"}}
		require.NotNil(t, embeddingBase64Transformer(info))
	})
}

func TestConvertEmbeddingRequestStripsBase64ForXunfeiMaas(t *testing.T) {
	a := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeXunfeiMaaS}}
	converted, err := a.ConvertEmbeddingRequest(nil, info, dto.EmbeddingRequest{Model: "m", EncodingFormat: "base64"})
	require.NoError(t, err)
	req, ok := converted.(dto.EmbeddingRequest)
	require.True(t, ok)
	require.Empty(t, req.EncodingFormat)
	raw, err := common.Marshal(req)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "encoding_format")

	info.ChannelMeta.ChannelType = constant.ChannelTypeOpenAI
	converted, err = a.ConvertEmbeddingRequest(nil, info, dto.EmbeddingRequest{Model: "m", EncodingFormat: "base64"})
	require.NoError(t, err)
	require.Equal(t, "base64", converted.(dto.EmbeddingRequest).EncodingFormat)
}
