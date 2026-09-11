package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertChannelPreservesCredentialAndSettings(t *testing.T) {
	for _, test := range []struct {
		name string
		typ  int
		key  string
	}{
		{"ordinary", constant.ChannelTypeOpenAI, "test-key"},
		{"mistral", constant.ChannelTypeMistralConsole, "session-value=="},
		{"vertex", constant.ChannelTypeVertexAi, "{\n  \"private_key\": \"line1\\nline2\"\n}"},
		{"codex", constant.ChannelTypeCodex, "{\n  \"access_token\": \"token\",\n  \"account_id\": \"account\"\n}"},
		{"qwen", constant.ChannelTypeQwenTokenPlan, "{\n  \"type\": \"qwen_token_plan\",\n  \"api_key\": \"sk-sp-test\"\n}"},
		{"multiline", constant.ChannelTypeOpenAI, "first\nsecond"},
		{"array credential", constant.ChannelTypeOpenAI, `["part1","part2"]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := setupChannelStatusControllerTestDB(t)
			channel := createChannelStatusControllerFixture(t, db)
			channel.Type, channel.Key = test.typ, test.key
			channel.Status = common.ChannelStatusManuallyDisabled
			require.NoError(t, db.Save(&channel).Error)
			ctx, w := conversionTestContext(http.MethodPost, "/", nil)
			ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(channel.Id)}}
			ConvertChannelToMultiKey(ctx)
			require.Contains(t, w.Body.String(), `"success":true`)
			require.NotContains(t, w.Body.String(), test.key)
			converted, err := model.GetChannelById(channel.Id, true)
			require.NoError(t, err)
			require.True(t, converted.ChannelInfo.IsMultiKey)
			require.Equal(t, 1, converted.ChannelInfo.MultiKeySize)
			require.Equal(t, constant.MultiKeyModeRandom, converted.ChannelInfo.MultiKeyMode)
			require.Equal(t, []string{test.key}, converted.GetKeys())
			key, _, apiErr := converted.GetNextEnabledKey()
			require.Nil(t, apiErr)
			require.Equal(t, test.key, key)
			// Only the key representation and pool metadata may change.
			converted.Key, converted.ChannelInfo = channel.Key, channel.ChannelInfo
			require.Equal(t, channel, *converted)
		})
	}
}

func TestMultiKeyConversionCannotBeReversedOrReset(t *testing.T) {
	db := setupChannelStatusControllerTestDB(t)
	channel := createChannelStatusControllerFixture(t, db)
	converted, err := model.ConvertChannelToMultiKey(channel.Id)
	require.NoError(t, err)
	converted.ChannelInfo.MultiKeyStatusList = map[int]int{0: common.ChannelStatusManuallyDisabled}
	require.NoError(t, converted.SaveChannelInfo())
	again, err := model.ConvertChannelToMultiKey(channel.Id)
	require.NoError(t, err)
	require.Equal(t, converted.ChannelInfo, again.ChannelInfo)
	require.Equal(t, converted.Key, again.Key)

	body := []byte(fmt.Sprintf(`{"id":%d,"channel_info":{"is_multi_key":false}}`, channel.Id))
	ctx, w := conversionTestContext(http.MethodPut, "/", body)
	UpdateChannel(ctx)
	require.Contains(t, w.Body.String(), `"success":false`)
	current, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.True(t, current.ChannelInfo.IsMultiKey)
	// A single-key edit loaded before conversion must not revert pool metadata.
	channel.Name = "renamed"
	require.NoError(t, channel.Update())
	require.True(t, channel.ChannelInfo.IsMultiKey)
}

func TestConvertedStructuredCredentialsSurviveEditAppendAndDelete(t *testing.T) {
	for _, typ := range []int{constant.ChannelTypeOpenAI, constant.ChannelTypeCodex, constant.ChannelTypeQwenTokenPlan} {
		t.Run(fmt.Sprint(typ), func(t *testing.T) {
			db := setupChannelStatusControllerTestDB(t)
			channel := createChannelStatusControllerFixture(t, db)
			key := "{\n\"access_token\":\"token\",\"account_id\":\"account\",\"type\":\"qwen_token_plan\",\"api_key\":\"sk-sp-test\"\n}"
			channel.Type, channel.Key = typ, key
			require.NoError(t, db.Save(&channel).Error)
			_, err := model.ConvertChannelToMultiKey(channel.Id)
			require.NoError(t, err)
			for _, patch := range []map[string]any{
				{"name": "renamed"},
				{"key": strings.ReplaceAll(key, "test", "second"), "key_mode": "append"},
			} {
				patch["id"], patch["type"] = channel.Id, typ
				body, err := common.Marshal(patch)
				require.NoError(t, err)
				ctx, w := conversionTestContext(http.MethodPut, "/", body)
				UpdateChannel(ctx)
				require.Contains(t, w.Body.String(), `"success":true`)
			}
			updated, err := model.GetChannelById(channel.Id, true)
			require.NoError(t, err)
			require.Len(t, updated.GetKeys(), 2)
			require.Equal(t, key, updated.GetKeys()[0])
			body := []byte(fmt.Sprintf(`{"channel_id":%d,"action":"delete_key","key_index":1}`, channel.Id))
			ctx, w := conversionTestContext(http.MethodPost, "/", body)
			ManageMultiKeys(ctx)
			require.Contains(t, w.Body.String(), `"success":true`)
			updated, err = model.GetChannelById(channel.Id, true)
			require.NoError(t, err)
			require.Equal(t, []string{key}, updated.GetKeys())
		})
	}
}

func conversionTestContext(method, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 1)
	c.Set("role", common.RoleRootUser)
	return c, w
}

func TestConvertedChannelCanDeleteAllDisabledKeys(t *testing.T) {
	db := setupChannelStatusControllerTestDB(t)
	channel := createChannelStatusControllerFixture(t, db)
	converted, err := model.ConvertChannelToMultiKey(channel.Id)
	require.NoError(t, err)
	converted.ChannelInfo.MultiKeyStatusList = map[int]int{0: common.ChannelStatusAutoDisabled}
	require.NoError(t, converted.SaveChannelInfo())
	body := []byte(fmt.Sprintf(`{"channel_id":%d,"action":"delete_disabled_keys"}`, channel.Id))
	ctx, w := conversionTestContext(http.MethodPost, "/", body)
	ManageMultiKeys(ctx)
	require.Contains(t, w.Body.String(), `"success":true`)
	updated, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Empty(t, updated.GetKeys())
	require.Zero(t, updated.ChannelInfo.MultiKeySize)
	_, _, apiErr := updated.GetNextEnabledKey()
	require.NotNil(t, apiErr)
}

func TestSingleCredentialWriteCannotOverwriteConvertedPool(t *testing.T) {
	db := setupChannelStatusControllerTestDB(t)
	channel := createChannelStatusControllerFixture(t, db)
	updated, err := model.UpdateSingleChannelKey(channel.Id, "refreshed-key")
	require.NoError(t, err)
	require.True(t, updated)
	converted, err := model.ConvertChannelToMultiKey(channel.Id)
	require.NoError(t, err)
	updated, err = model.UpdateSingleChannelKey(channel.Id, "stale-oauth-result")
	require.NoError(t, err)
	require.False(t, updated)
	current, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, converted.Key, current.Key)
	require.Equal(t, converted.ChannelInfo, current.ChannelInfo)
}
