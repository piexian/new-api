package dto

import (
	"encoding/json"
	"net/http"

	"github.com/QuantumNous/new-api/relaykit/types"
)

// DeepSeekToolsRequest 是 /v1/tools/{search,fetch} 的网关请求信封:
// model 决定路由与计费(deepseek-search / deepseek-fetch),其余字段原样透传上游。
type DeepSeekToolsRequest struct {
	Model string          `json:"model,omitempty"`
	Query string          `json:"query,omitempty"`
	URL   string          `json:"url,omitempty"`
	Raw   json.RawMessage `json:"-"`
}

func (r *DeepSeekToolsRequest) GetRequestModel() string {
	if r == nil {
		return ""
	}
	return r.Model
}

func (r *DeepSeekToolsRequest) IsStream(c *http.Request) bool {
	return false
}

func (r *DeepSeekToolsRequest) SetModelName(modelName string) {
	if r != nil && modelName != "" {
		r.Model = modelName
	}
}
func (r *DeepSeekToolsRequest) GetTokenCountMeta() *types.TokenCountMeta {
	return &types.TokenCountMeta{}
}
