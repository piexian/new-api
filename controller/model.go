package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/ai360"
	"github.com/QuantumNous/new-api/relay/channel/lingyiwanwu"
	"github.com/QuantumNous/new-api/relay/channel/minimax"
	"github.com/QuantumNous/new-api/relay/channel/moonshot"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// https://platform.openai.com/docs/api-reference/models/list

var openAIModels []dto.OpenAIModels
var openAIModelsMap map[string]dto.OpenAIModels
var channelId2Models map[int][]string
var geminiCompatibleModels map[string]bool

const (
	geminiListDefaultPageSize = 50
	geminiListMaxPageSize     = 1000
)

func init() {
	// https://platform.openai.com/docs/models/model-endpoint-compatibility
	for i := 0; i < constant.APITypeDummy; i++ {
		if i == constant.APITypeAIProxyLibrary {
			continue
		}
		adaptor := relay.GetAdaptor(i)
		channelName := adaptor.GetChannelName()
		modelNames := adaptor.GetModelList()
		for _, modelName := range modelNames {
			openAIModels = append(openAIModels, dto.OpenAIModels{
				Id:      modelName,
				Object:  "model",
				Created: 1626777600,
				OwnedBy: channelName,
			})
		}
	}
	for _, modelName := range ai360.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: ai360.ChannelName,
		})
	}
	for _, modelName := range moonshot.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: moonshot.ChannelName,
		})
	}
	for _, modelName := range lingyiwanwu.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: lingyiwanwu.ChannelName,
		})
	}
	for _, modelName := range minimax.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: minimax.ChannelName,
		})
	}
	for modelName, _ := range constant.MidjourneyModel2Action {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: "midjourney",
		})
	}
	openAIModelsMap = make(map[string]dto.OpenAIModels)
	for _, aiModel := range openAIModels {
		openAIModelsMap[aiModel.Id] = aiModel
	}
	channelId2Models = make(map[int][]string)
	for i := 1; i <= constant.ChannelTypeDummy; i++ {
		apiType, success := common.ChannelType2APIType(i)
		if !success || apiType == constant.APITypeAIProxyLibrary {
			continue
		}
		meta := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: i,
		}}
		adaptor := relay.GetAdaptor(apiType)
		adaptor.Init(meta)
		channelId2Models[i] = adaptor.GetModelList()
	}
	geminiCompatibleModels = make(map[string]bool)
	for _, channelType := range []int{constant.ChannelTypeGemini, constant.ChannelTypeVertexAi} {
		for _, modelName := range channelId2Models[channelType] {
			geminiCompatibleModels[modelName] = true
		}
	}
	openAIModels = lo.UniqBy(openAIModels, func(m dto.OpenAIModels) string {
		return m.Id
	})
}

func shouldIncludeModelForType(modelName string, modelType int) bool {
	switch modelType {
	case constant.ChannelTypeGemini:
		endpointTypes := model.GetModelSupportEndpointTypes(modelName)
		if len(endpointTypes) > 0 {
			return lo.Contains(endpointTypes, constant.EndpointTypeGemini)
		}
		return geminiCompatibleModels[modelName]
	default:
		return true
	}
}

func appendModelIfEligible(userOpenAiModels *[]dto.OpenAIModels, modelName string, acceptUnsetRatioModel bool, modelType int) {
	if !acceptUnsetRatioModel {
		_, _, exist := ratio_setting.GetModelRatioOrPrice(modelName)
		if !exist {
			return
		}
	}
	if !shouldIncludeModelForType(modelName, modelType) {
		return
	}
	if oaiModel, ok := openAIModelsMap[modelName]; ok {
		oaiModel.SupportedEndpointTypes = model.GetModelSupportEndpointTypes(modelName)
		*userOpenAiModels = append(*userOpenAiModels, oaiModel)
		return
	}
	*userOpenAiModels = append(*userOpenAiModels, dto.OpenAIModels{
		Id:                     modelName,
		Object:                 "model",
		Created:                1626777600,
		OwnedBy:                "custom",
		SupportedEndpointTypes: model.GetModelSupportEndpointTypes(modelName),
	})
}

func getGeminiSupportedGenerationMethods(endpointTypes []constant.EndpointType, modelName string) []string {
	switch {
	case strings.HasPrefix(modelName, "veo-"):
		return []string{"predictLongRunning"}
	case strings.HasPrefix(modelName, "imagen"):
		return []string{"predict"}
	case strings.HasPrefix(modelName, "text-embedding"),
		strings.HasPrefix(modelName, "embedding"),
		strings.HasPrefix(modelName, "gemini-embedding"):
		return []string{"embedContent", "batchEmbedContents"}
	}

	methods := make([]string, 0, 4)
	if lo.Contains(endpointTypes, constant.EndpointTypeGemini) || geminiCompatibleModels[modelName] {
		methods = append(methods, "generateContent", "streamGenerateContent")
	}
	if lo.Contains(endpointTypes, constant.EndpointTypeEmbeddings) {
		methods = append(methods, "embedContent", "batchEmbedContents")
	}
	return lo.Uniq(methods)
}

func buildGeminiModel(openAIModel dto.OpenAIModels) dto.GeminiModel {
	endpointTypes := openAIModel.SupportedEndpointTypes
	if len(endpointTypes) == 0 {
		endpointTypes = model.GetModelSupportEndpointTypes(openAIModel.Id)
	}
	return dto.GeminiModel{
		Name:                       fmt.Sprintf("models/%s", openAIModel.Id),
		BaseModelId:                openAIModel.Id,
		DisplayName:                openAIModel.Id,
		SupportedGenerationMethods: getGeminiSupportedGenerationMethods(endpointTypes, openAIModel.Id),
	}
}

func getGeminiPagination(c *gin.Context, total int) (start int, end int, nextPageToken string, err error) {
	pageSize := geminiListDefaultPageSize
	if pageSizeValue := c.Query("pageSize"); pageSizeValue != "" {
		parsedPageSize, parseErr := strconv.Atoi(pageSizeValue)
		if parseErr == nil && parsedPageSize > 0 {
			pageSize = parsedPageSize
		}
	}
	if pageSize > geminiListMaxPageSize {
		pageSize = geminiListMaxPageSize
	}

	start = 0
	if pageToken := c.Query("pageToken"); pageToken != "" {
		start, err = strconv.Atoi(pageToken)
		if err != nil || start < 0 {
			return 0, 0, "", fmt.Errorf("invalid pageToken")
		}
	}
	if start >= total {
		return total, total, "", nil
	}

	end = start + pageSize
	if end >= total {
		return start, total, "", nil
	}
	return start, end, strconv.Itoa(end), nil
}

func renderGeminiError(c *gin.Context, statusCode int, status string, message string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"code":    statusCode,
			"message": message,
			"status":  status,
		},
	})
}

func ListModels(c *gin.Context, modelType int) {
	userOpenAiModels := make([]dto.OpenAIModels, 0)

	acceptUnsetRatioModel := operation_setting.SelfUseModeEnabled
	if !acceptUnsetRatioModel {
		userId := c.GetInt("id")
		if userId > 0 {
			userSettings, _ := model.GetUserSetting(userId, false)
			if userSettings.AcceptUnsetRatioModel {
				acceptUnsetRatioModel = true
			}
		}
	}

	modelLimitEnable := common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
	if modelLimitEnable {
		s, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
		var tokenModelLimit map[string]bool
		if ok {
			tokenModelLimit = s.(map[string]bool)
		} else {
			tokenModelLimit = map[string]bool{}
		}
		for allowModel, _ := range tokenModelLimit {
			appendModelIfEligible(&userOpenAiModels, allowModel, acceptUnsetRatioModel, modelType)
		}
	} else {
		userId := c.GetInt("id")
		userGroup, err := model.GetUserGroup(userId, false)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "get user group failed",
			})
			return
		}
		group := userGroup
		tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		if tokenGroup != "" {
			group = tokenGroup
		}
		var models []string
		if tokenGroup == "auto" {
			for _, autoGroup := range service.GetUserAutoGroup(userGroup) {
				groupModels := model.GetGroupEnabledModels(autoGroup)
				for _, g := range groupModels {
					if !common.StringsContains(models, g) {
						models = append(models, g)
					}
				}
			}
		} else {
			models = model.GetGroupEnabledModels(group)
		}
		for _, modelName := range models {
			appendModelIfEligible(&userOpenAiModels, modelName, acceptUnsetRatioModel, modelType)
		}
	}

	switch modelType {
	case constant.ChannelTypeAnthropic:
		useranthropicModels := make([]dto.AnthropicModel, len(userOpenAiModels))
		for i, model := range userOpenAiModels {
			useranthropicModels[i] = dto.AnthropicModel{
				ID:          model.Id,
				CreatedAt:   time.Unix(int64(model.Created), 0).UTC().Format(time.RFC3339),
				DisplayName: model.Id,
				Type:        "model",
			}
		}
		c.JSON(200, gin.H{
			"data":     useranthropicModels,
			"first_id": useranthropicModels[0].ID,
			"has_more": false,
			"last_id":  useranthropicModels[len(useranthropicModels)-1].ID,
		})
	case constant.ChannelTypeGemini:
		userGeminiModels := make([]dto.GeminiModel, len(userOpenAiModels))
		for i, modelItem := range userOpenAiModels {
			userGeminiModels[i] = buildGeminiModel(modelItem)
		}
		start, end, nextPageToken, err := getGeminiPagination(c, len(userGeminiModels))
		if err != nil {
			renderGeminiError(c, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
			return
		}
		response := gin.H{
			"models": userGeminiModels[start:end],
		}
		if nextPageToken != "" {
			response["nextPageToken"] = nextPageToken
		}
		c.JSON(http.StatusOK, response)
	default:
		c.JSON(200, gin.H{
			"success": true,
			"data":    userOpenAiModels,
			"object":  "list",
		})
	}
}

func ChannelListModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    openAIModels,
	})
}

func DashboardListModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    channelId2Models,
	})
}

func EnabledListModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    model.GetEnabledModels(),
	})
}

func RetrieveModel(c *gin.Context, modelType int) {
	modelId := c.Param("model")
	if aiModel, ok := openAIModelsMap[modelId]; ok {
		aiModel.SupportedEndpointTypes = model.GetModelSupportEndpointTypes(aiModel.Id)
		switch modelType {
		case constant.ChannelTypeAnthropic:
			c.JSON(200, dto.AnthropicModel{
				ID:          aiModel.Id,
				CreatedAt:   time.Unix(int64(aiModel.Created), 0).UTC().Format(time.RFC3339),
				DisplayName: aiModel.Id,
				Type:        "model",
			})
		case constant.ChannelTypeGemini:
			if !shouldIncludeModelForType(aiModel.Id, modelType) {
				renderGeminiError(c, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("model not found: models/%s", modelId))
				return
			}
			c.JSON(http.StatusOK, buildGeminiModel(aiModel))
		default:
			c.JSON(200, aiModel)
		}
	} else {
		if modelType == constant.ChannelTypeGemini {
			renderGeminiError(c, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("model not found: models/%s", modelId))
			return
		}
		openAIError := types.OpenAIError{
			Message: fmt.Sprintf("The model '%s' does not exist", modelId),
			Type:    "invalid_request_error",
			Param:   "model",
			Code:    "model_not_found",
		}
		c.JSON(200, gin.H{
			"error": openAIError,
		})
	}
}
