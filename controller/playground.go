package controller

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// playgroundRelayFormat 根据请求路径推断 RelayFormat
func playgroundRelayFormat(c *gin.Context) types.RelayFormat {
	path := c.Request.URL.Path
	switch {
	case strings.Contains(path, "/chat/completions"):
		return types.RelayFormatOpenAI
	case strings.Contains(path, "/responses"):
		return types.RelayFormatOpenAIResponses
	case strings.Contains(path, "/messages"):
		return types.RelayFormatClaude
	case strings.Contains(path, "/images/"):
		return types.RelayFormatOpenAIImage
	case strings.Contains(path, "/video"):
		return types.RelayFormatTask
	case strings.Contains(path, "/audio/"):
		return types.RelayFormatOpenAIAudio
	case strings.Contains(path, "/embeddings"):
		return types.RelayFormatEmbedding
	case strings.Contains(path, "/rerank"):
		return types.RelayFormatRerank
	default:
		return types.RelayFormatOpenAI
	}
}

func Playground(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		newAPIError = types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	relayFormat := playgroundRelayFormat(c)

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, nil, nil)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return
	}

	userId := c.GetInt("id")

	// Write user context to ensure acceptUnsetRatio is available
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		return
	}
	userCache.WriteContext(c)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-%s", relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	Relay(c, relayFormat)
}
