package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type pricingAPIResponse struct {
	Success     bool               `json:"success"`
	Data        []model.Pricing    `json:"data"`
	GroupRatio  map[string]float64 `json:"group_ratio"`
	UsableGroup map[string]string  `json:"usable_group"`
	AutoGroups  []string           `json:"auto_groups"`
}

func withPricingGroupSettings(t *testing.T, userUsableGroups string, groupRatio string) {
	t.Helper()

	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	model.InvalidatePricingCache()

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(userUsableGroups))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(groupRatio))

	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		model.InvalidatePricingCache()
	})
}

func TestAnonymousPricingUsesPublicGroups(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	withPricingGroupSettings(t, `{}`, `{"default":1}`)

	require.NoError(t, db.Create(&model.Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "test-channel",
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "anonymous-visible-model",
		ChannelId: 1,
		Enabled:   true,
	}).Error)
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload pricingAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Contains(t, payload.UsableGroup, "default")
	require.Contains(t, payload.GroupRatio, "default")
	require.Len(t, payload.Data, 1)
	require.Equal(t, "anonymous-visible-model", payload.Data[0].ModelName)
	require.Equal(t, []string{"default"}, payload.AutoGroups)
}
