package qwentokenplan

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type Credential struct {
	Type        string         `json:"type"`
	APIKey      string         `json:"api_key"`
	AccessToken string         `json:"access_token"`
	ExpiresAt   string         `json:"expires_at"`
	User        CredentialUser `json:"user"`
}

type CredentialUser struct {
	ID       int64  `json:"id,omitempty"`
	Email    string `json:"email,omitempty"`
	AliyunID string `json:"aliyun_id,omitempty"`
}

func ParseCredential(raw string) (*Credential, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("qwen token plan credential is empty")
	}
	var credential Credential
	if err := common.UnmarshalJsonStr(trimmed, &credential); err != nil {
		return nil, errors.New("qwen token plan credential must be a valid JSON object")
	}
	credential.Type = strings.TrimSpace(credential.Type)
	credential.APIKey = strings.TrimSpace(credential.APIKey)
	credential.AccessToken = strings.TrimSpace(credential.AccessToken)
	credential.ExpiresAt = strings.TrimSpace(credential.ExpiresAt)
	if credential.Type != "qwen_token_plan" {
		return nil, errors.New("qwen token plan credential has an invalid type")
	}
	if !strings.HasPrefix(credential.APIKey, "sk-sp-") {
		return nil, errors.New("qwen token plan credential must include an sk-sp- API key")
	}
	if credential.AccessToken == "" {
		return nil, errors.New("qwen token plan credential must include an OAuth access token")
	}
	if credential.ExpiresAt == "" {
		return nil, errors.New("qwen token plan credential must include OAuth expiration time")
	}
	return &credential, nil
}

func EncodeCredential(credential Credential) (string, error) {
	encoded, err := common.Marshal(credential)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (credential *Credential) OAuthExpired(now time.Time) bool {
	if credential == nil {
		return true
	}
	expiresAt, ok := parseOAuthExpiration(credential.ExpiresAt)
	return !ok || !expiresAt.After(now)
}

func parseOAuthExpiration(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
