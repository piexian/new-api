package dto

import (
	"net/http"

	"github.com/QuantumNous/new-api/relaykit/types"
)

type Request interface {
	GetTokenCountMeta() *types.TokenCountMeta
	IsStream(c *http.Request) bool
	SetModelName(modelName string)
}

type BaseRequest struct {
}

func (b *BaseRequest) GetTokenCountMeta() *types.TokenCountMeta {
	return &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
}
func (b *BaseRequest) IsStream(c *http.Request) bool {
	return false
}
func (b *BaseRequest) SetModelName(modelName string) {}
