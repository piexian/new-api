package qwentokenplan

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Credential 是 Qwen Token Plan 渠道的绑定凭证：
// api_key 为 sk-sp- 推理密钥；console_token / access_key_id / access_key_secret
// 用于阿里云百炼 console 网关查询计划额度（bailian-cli 协议）。
// access_token / expires_at / user 为旧版千问 OAuth 字段，仅保留解析兼容。
type Credential struct {
	Type            string         `json:"type"`
	APIKey          string         `json:"api_key"`
	ConsoleToken    string         `json:"console_token,omitempty"`
	AccessKeyID     string         `json:"access_key_id,omitempty"`
	AccessKeySecret string         `json:"access_key_secret,omitempty"`
	AccessToken     string         `json:"access_token,omitempty"`
	ExpiresAt       string         `json:"expires_at,omitempty"`
	User            CredentialUser `json:"user,omitempty"`
}

type CredentialUser struct {
	ID       int64  `json:"id,omitempty"`
	Email    string `json:"email,omitempty"`
	AliyunID string `json:"aliyun_id,omitempty"`
}

func ExtractAPIKey(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "sk-sp-") {
		return trimmed, nil
	}
	if trimmed == "" {
		return "", errors.New("qwen token plan credential is empty")
	}

	var credential struct {
		Type   string `json:"type"`
		APIKey string `json:"api_key"`
	}
	if err := common.UnmarshalJsonStr(trimmed, &credential); err != nil {
		return "", errors.New("qwen token plan credential must be an sk-sp- API key or a valid JSON object")
	}
	if strings.TrimSpace(credential.Type) != "qwen_token_plan" {
		return "", errors.New("qwen token plan credential has an invalid type")
	}
	apiKey := strings.TrimSpace(credential.APIKey)
	if !strings.HasPrefix(apiKey, "sk-sp-") {
		return "", errors.New("qwen token plan credential must include an sk-sp- API key")
	}
	return apiKey, nil
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
	credential.ConsoleToken = strings.TrimSpace(credential.ConsoleToken)
	credential.AccessKeyID = strings.TrimSpace(credential.AccessKeyID)
	credential.AccessKeySecret = strings.TrimSpace(credential.AccessKeySecret)
	if credential.Type != "qwen_token_plan" {
		return nil, errors.New("qwen token plan credential has an invalid type")
	}
	if !strings.HasPrefix(credential.APIKey, "sk-sp-") {
		return nil, errors.New("qwen token plan credential must include an sk-sp- API key")
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

// HasConsoleCredential 判断是否具备查询计划额度的阿里云 console 凭证
// （手动回填的 console token，或可自动换签 token 的 AK/SK）。
func (credential *Credential) HasConsoleCredential() bool {
	if credential == nil {
		return false
	}
	return credential.ConsoleToken != "" ||
		(credential.AccessKeyID != "" && credential.AccessKeySecret != "")
}

// MergeAPIKey 在仅更换 sk-sp- 推理密钥时保留已绑定的 console 凭证（保活），
// 避免轮换密钥导致额度查询凭证丢失；整体 JSON 凭证则原样替换。
func MergeAPIKey(existing string, replacement string) (string, error) {
	return MergeChannelKey(existing, replacement, nil, nil, nil)
}

// MergeChannelKey 把渠道更新合并为最终凭证：
// key 为空表示保留已存 api_key；console 补丁字段为 nil 表示不变、
// 非nil空串表示清除；key 可为裸 sk-sp- 或完整 JSON 凭证（后者整体替换原凭证）。
func MergeChannelKey(existing string, key string, consoleToken *string, accessKeyID *string, accessKeySecret *string) (string, error) {
	credential := &Credential{}
	if existingCredential, err := ParseCredential(existing); err == nil {
		*credential = *existingCredential
	} else if apiKey, err := ExtractAPIKey(existing); err == nil {
		credential.APIKey = apiKey
	}

	if strings.TrimSpace(key) != "" {
		if replacement, err := ParseCredential(key); err == nil {
			*credential = *replacement
		} else {
			apiKey, err := ExtractAPIKey(key)
			if err != nil {
				return "", err
			}
			credential.APIKey = apiKey
		}
	}

	if credential.Type == "" {
		credential.Type = "qwen_token_plan"
	}
	if consoleToken != nil {
		credential.ConsoleToken = strings.TrimSpace(*consoleToken)
	}
	if accessKeyID != nil {
		credential.AccessKeyID = strings.TrimSpace(*accessKeyID)
	}
	if accessKeySecret != nil {
		credential.AccessKeySecret = strings.TrimSpace(*accessKeySecret)
	}
	if (credential.AccessKeyID == "") != (credential.AccessKeySecret == "") {
		return "", errors.New("qwen token plan AccessKey ID and Secret must be provided together")
	}
	if !strings.HasPrefix(credential.APIKey, "sk-sp-") {
		return "", errors.New("qwen token plan credential must include an sk-sp- API key")
	}
	return EncodeCredential(*credential)
}
