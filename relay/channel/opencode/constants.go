package opencode

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
)

var (
	ModelList = channelconstant.OpenCodeModelList
	GoModels  = channelconstant.OpenCodeGoModels
)

var ChannelName = "opencode"

func NormalizeRoot(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = channelconstant.OpenCodeZenBaseURLAlias
	}

	switch baseURL {
	case channelconstant.OpenCodeZenBaseURLAlias:
		return channelconstant.OpenCodeZenBaseURL
	case channelconstant.OpenCodeGoBaseURLAlias:
		return channelconstant.OpenCodeGoBaseURL
	}

	baseURL = strings.TrimRight(baseURL, "/")
	for _, suffix := range []string{
		"/v1/chat/completions",
		"/v1/messages/count_tokens",
		"/v1/messages",
		"/v1/responses/compact",
		"/v1/responses",
		"/v1/models",
		"/v1",
	} {
		if strings.HasSuffix(baseURL, suffix) {
			return strings.TrimSuffix(baseURL, suffix)
		}
	}
	return baseURL
}

func IsGoBase(baseURL string) bool {
	root := NormalizeRoot(baseURL)
	return root == channelconstant.OpenCodeGoBaseURL || strings.HasSuffix(root, "/zen/go")
}

func ModelsURL(baseURL string) (string, bool) {
	if IsGoBase(baseURL) {
		return "", false
	}
	return NormalizeRoot(baseURL) + "/v1/models", true
}

func StaticModelListForBase(baseURL string) []string {
	if IsGoBase(baseURL) {
		return GoModels
	}
	return nil
}

func ParseModelsResponse(body []byte) ([]string, error) {
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(response.Data)+len(response.Models))
	seen := make(map[string]struct{})
	add := func(model string) {
		model = strings.TrimSpace(strings.TrimPrefix(model, "models/"))
		if model == "" {
			return
		}
		if _, ok := seen[model]; ok {
			return
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}
	for _, item := range response.Data {
		add(item.ID)
	}
	for _, item := range response.Models {
		switch {
		case item.ID != "":
			add(item.ID)
		case item.Name != "":
			add(item.Name)
		default:
			add(item.Model)
		}
	}
	return models, nil
}
