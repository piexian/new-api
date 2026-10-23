package minimaxcode

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// OAuthKey 是 MiniMax Code 渠道的密钥格式（OAuth 设备码登录产物）。
type OAuthKey struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Subject      string `json:"subject,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
	Email        string `json:"email,omitempty"`
	UserName     string `json:"user_name,omitempty"`
	Region       string `json:"region,omitempty"`
	GroupID      string `json:"group_id,omitempty"`
	LastRefresh  string `json:"last_refresh,omitempty"`
	Expired      string `json:"expired,omitempty"`
	Type         string `json:"type,omitempty"`
}

func ParseOAuthKey(raw string) (*OAuthKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty oauth key")
	}
	if !strings.HasPrefix(raw, "{") {
		return nil, errors.New("channel key must be a JSON object (obtained via OAuth device login)")
	}
	var key OAuthKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("invalid oauth key json")
	}
	if strings.TrimSpace(key.AccessToken) == "" {
		return nil, errors.New("oauth key missing access_token")
	}
	return &key, nil
}

// NormalizeRegion 兼容历史 region 取值。
func NormalizeRegion(region string) string {
	region = strings.ToLower(strings.TrimSpace(region))
	switch region {
	case RegionGlobal, "io", "en", "overseas":
		return RegionGlobal
	default:
		return RegionCN
	}
}
