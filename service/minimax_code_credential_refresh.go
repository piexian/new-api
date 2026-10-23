package service

// MiniMax Code 渠道凭证：JSON OAuth key 解析、刷新与落库。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type MiniMaxCodeOAuthKey struct {
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
	UserID       string `json:"user_id,omitempty"`
	LastRefresh  string `json:"last_refresh,omitempty"`
	Expired      string `json:"expired,omitempty"`
	Type         string `json:"type,omitempty"`
}

func ParseMiniMaxCodeOAuthKey(raw string) (*MiniMaxCodeOAuthKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty oauth key")
	}
	var key MiniMaxCodeOAuthKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("invalid oauth key json")
	}
	if strings.TrimSpace(key.AccessToken) == "" {
		return nil, errors.New("oauth key missing access_token")
	}
	return &key, nil
}

func (k *MiniMaxCodeOAuthKey) Marshal() (string, error) {
	encoded, err := common.Marshal(k)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

type MiniMaxCodeCredentialRefreshOptions struct {
	ResetCaches bool
}

// RefreshMiniMaxCodeChannelCredential 刷新渠道访问令牌并回写数据库。
func RefreshMiniMaxCodeChannelCredential(ctx context.Context, channelID int, opts MiniMaxCodeCredentialRefreshOptions) (*MiniMaxCodeOAuthKey, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeMiniMaxCode {
		return nil, nil, fmt.Errorf("channel type is not supported for credential refresh")
	}
	if ch.ChannelInfo.IsMultiKey {
		return nil, nil, errors.New("credential refresh does not support multi-key channels")
	}

	oauthKey, err := ParseMiniMaxCodeOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return nil, nil, fmt.Errorf("refresh_token is required to refresh credential")
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	grant, err := RefreshMiniMaxCodeToken(refreshCtx, ch.GetSetting().Proxy, oauthKey.Region, oauthKey.RefreshToken)
	if err != nil {
		return nil, nil, err
	}

	oauthKey.AccessToken = grant.AccessToken
	oauthKey.RefreshToken = grant.RefreshToken
	oauthKey.Scope = firstNonEmpty(grant.Scope, oauthKey.Scope)
	oauthKey.Subject = firstNonEmpty(grant.Subject, oauthKey.Subject)
	oauthKey.AccountID = firstNonEmpty(grant.AccountID, oauthKey.AccountID)
	oauthKey.LastRefresh = time.Now().Format(time.RFC3339)
	oauthKey.Expired = grant.ExpiresAt.Format(time.RFC3339)
	if strings.TrimSpace(oauthKey.Type) == "" {
		oauthKey.Type = "minimax_code"
	}

	// 补齐缺失的分组/身份信息（失败不阻断刷新）。
	if strings.TrimSpace(oauthKey.GroupID) == "" || strings.TrimSpace(oauthKey.UserID) == "" {
		if identity := tryMiniMaxCodeIdentity(refreshCtx, ch, oauthKey); identity != nil {
			oauthKey.GroupID = firstNonEmpty(identity.GroupID, oauthKey.GroupID)
			oauthKey.UserID = firstNonEmpty(identity.RealUserID, oauthKey.UserID)
		}
	}

	encoded, err := oauthKey.Marshal()
	if err != nil {
		return nil, nil, err
	}
	updated, err := model.UpdateSingleChannelKey(ch.Id, encoded)
	if err != nil {
		return nil, nil, err
	}
	if !updated {
		return nil, nil, errors.New("credential refresh does not support multi-key channels")
	}

	if opts.ResetCaches {
		model.InitChannelCache()
	}
	return oauthKey, ch, nil
}

func tryMiniMaxCodeIdentity(ctx context.Context, ch *model.Channel, oauthKey *MiniMaxCodeOAuthKey) *MiniMaxCodeIdentity {
	identityCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client, err := NewMiniMaxCodeAccountClient(ch.GetSetting().Proxy, oauthKey.Region, oauthKey.AccessToken, oauthKey.UserID)
	if err != nil {
		return nil
	}
	identity, err := client.FetchMiniMaxCodeIdentity(identityCtx)
	if err != nil {
		return nil
	}
	return identity
}
