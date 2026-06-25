package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettleTestQuotaUsesTieredBilling(t *testing.T) {
	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:   "tiered_expr",
			ExprString:    `param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`,
			ExprHash:      billingexpr.ExprHashString(`param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`),
			GroupRatio:    1,
			EstimatedTier: "stream",
			QuotaPerUnit:  common.QuotaPerUnit,
			ExprVersion:   1,
		},
		BillingRequestInput: &billingexpr.RequestInput{
			Body: []byte(`{"stream":true}`),
		},
	}

	quota, result := settleTestQuota(info, types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 2,
	}, &dto.Usage{
		PromptTokens: 1000,
	})

	require.Equal(t, 1500, quota)
	require.NotNil(t, result)
	require.Equal(t, "stream", result.MatchedTier)
}

func TestBuildTestLogOtherInjectsTieredInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
			ExprString:  `tier("base", p * 2)`,
		},
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	priceData := types.PriceData{
		GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
	}
	usage := &dto.Usage{
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 12,
		},
	}

	other := buildTestLogOther(ctx, info, priceData, usage, &billingexpr.TieredResult{
		MatchedTier: "base",
	})

	require.Equal(t, "tiered_expr", other["billing_mode"])
	require.Equal(t, "base", other["matched_tier"])
	require.NotEmpty(t, other["expr_b64"])
}

func TestNormalizeChannelTestEndpointVolcEngineModels(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeVolcEngine}

	require.Equal(t, "", normalizeChannelTestEndpoint(channel, "doubao-seed-2-0-pro-260215", ""))
	require.Equal(t, string(constant.EndpointTypeEmbeddings), normalizeChannelTestEndpoint(channel, "doubao-embedding-text-240715", ""))
	require.Equal(t, string(constant.EndpointTypeEmbeddings), normalizeChannelTestEndpoint(channel, "doubao-embedding-vision-251215", ""))
	require.Equal(t, string(constant.EndpointTypeImageGeneration), normalizeChannelTestEndpoint(channel, "doubao-seedream-5-0-260128", ""))
	require.Equal(t, string(constant.EndpointTypeImageGeneration), normalizeChannelTestEndpoint(channel, "doubao-seededit-3-0-i2i-250628", ""))
	require.Equal(t, string(constant.EndpointTypeOpenAIVideo), normalizeChannelTestEndpoint(channel, "doubao-seedance-2-0-fast-260128", ""))
	require.Equal(t, string(constant.EndpointTypeOpenAIVideo), normalizeChannelTestEndpoint(channel, "wan2-1-14b-i2v-250225", ""))
	require.Equal(t, string(constant.EndpointTypeOpenAIVideo), normalizeChannelTestEndpoint(channel, "doubao-seed3d-2-0-260328", ""))
	require.Equal(t, string(constant.EndpointTypeOpenAIVideo), normalizeChannelTestEndpoint(channel, "hyper3d-gen2-260112", ""))
	require.Equal(t, string(constant.EndpointTypeOpenAI), normalizeChannelTestEndpoint(channel, "doubao-seedream-5-0-260128", string(constant.EndpointTypeOpenAI)))
}

func TestBuildTestVideoRequestVolcEngineModels(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeVolcEngine}

	req := buildTestVideoRequest("doubao-seedance-2-0-fast-260128", channel).(*relaycommon.TaskSubmitReq)
	require.Equal(t, "doubao-seedance-2-0-fast-260128", req.Model)
	require.Equal(t, "5", req.Seconds)
	require.Equal(t, "1280x720", req.Size)

	req = buildTestVideoRequest("wan2-1-14b-i2v-250225", channel).(*relaycommon.TaskSubmitReq)
	require.NotEmpty(t, req.Image)

	req = buildTestVideoRequest("wan2-1-14b-flf2v-250417", channel).(*relaycommon.TaskSubmitReq)
	require.Len(t, req.Images, 2)

	req = buildTestVideoRequest("doubao-seed3d-2-0-260328", channel).(*relaycommon.TaskSubmitReq)
	require.NotEmpty(t, req.Image)
	require.Contains(t, req.Prompt, "3D")
}
