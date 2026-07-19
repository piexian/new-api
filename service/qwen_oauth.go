package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const qwenOAuthBaseURL = "https://t.qianwenai.com"

type QwenOAuthAuthorizationFlow struct {
	ClientID        string
	Token           string
	Verifier        string
	VerificationURL string
	ExpiresIn       int
	Interval        int
}

type QwenOAuthPollResult struct {
	Status      string
	Credentials *QwenOAuthCredentials
}

type QwenOAuthCredentials struct {
	AccessToken string
	ExpiresAt   string
	User        QwenOAuthUser
}

type QwenOAuthUser struct {
	ID       int64
	Email    string
	AliyunID string
}

type qwenOAuthInitResponse struct {
	Success bool `json:"Success"`
	Data    struct {
		Token           string `json:"Token"`
		VerificationURL string `json:"VerificationUrl"`
		ExpiresIn       int    `json:"ExpiresIn"`
		Interval        int    `json:"Interval"`
	} `json:"Data"`
}

type qwenOAuthPollResponse struct {
	Success bool `json:"Success"`
	Data    struct {
		Status      string `json:"Status"`
		Credentials struct {
			AccessToken string `json:"AccessToken"`
			ExpireTime  string `json:"ExpireTime"`
			User        struct {
				ID           int64  `json:"Id"`
				Email        string `json:"Email"`
				AliyunID     string `json:"AliyunId"`
				Organization string `json:"Organization"`
			} `json:"User"`
		} `json:"Credentials"`
	} `json:"Data"`
	Status      string `json:"status"`
	Credentials struct {
		AccessToken string `json:"access_token"`
		ExpireTime  string `json:"expire_time"`
		User        struct {
			ID           int64  `json:"id"`
			Email        string `json:"email"`
			AliyunID     string `json:"aliyunId"`
			Organization string `json:"organization"`
		} `json:"user"`
	} `json:"credentials"`
}

func CreateQwenOAuthAuthorizationFlow(ctx context.Context, proxyURL string, clientID string) (*QwenOAuthAuthorizationFlow, error) {
	client, err := NewProxyHttpClient(proxyURL)
	if err != nil {
		return nil, err
	}
	return createQwenOAuthAuthorizationFlow(ctx, client, qwenOAuthBaseURL, clientID)
}

func PollQwenOAuthAuthorization(ctx context.Context, proxyURL string, clientID string, token string, verifier string) (*QwenOAuthPollResult, error) {
	client, err := NewProxyHttpClient(proxyURL)
	if err != nil {
		return nil, err
	}
	return pollQwenOAuthAuthorization(ctx, client, qwenOAuthBaseURL, clientID, token, verifier)
}

func createQwenOAuthAuthorizationFlow(ctx context.Context, client *http.Client, authBaseURL string, clientID string) (*QwenOAuthAuthorizationFlow, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, errors.New("qwen oauth client id is empty")
	}
	verifier, challenge, err := generatePKCEPair()
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("client_id", clientID)
	values.Set("code_challenge", challenge)
	values.Set("code_challenge_method", "S256")
	requestURL := strings.TrimRight(authBaseURL, "/") + "/cli/device/code?" + values.Encode()

	body, err := doQwenOAuthRequest(ctx, client, requestURL)
	if err != nil {
		return nil, err
	}
	var payload qwenOAuthInitResponse
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if !payload.Success || strings.TrimSpace(payload.Data.Token) == "" || strings.TrimSpace(payload.Data.VerificationURL) == "" {
		return nil, errors.New("qwen device authorization returned an invalid response")
	}
	return &QwenOAuthAuthorizationFlow{
		ClientID:        clientID,
		Token:           strings.TrimSpace(payload.Data.Token),
		Verifier:        verifier,
		VerificationURL: strings.TrimSpace(payload.Data.VerificationURL),
		ExpiresIn:       payload.Data.ExpiresIn,
		Interval:        payload.Data.Interval,
	}, nil
}

func pollQwenOAuthAuthorization(ctx context.Context, client *http.Client, authBaseURL string, clientID string, token string, verifier string) (*QwenOAuthPollResult, error) {
	values := url.Values{}
	values.Set("client_id", strings.TrimSpace(clientID))
	values.Set("token", strings.TrimSpace(token))
	values.Set("code_verifier", strings.TrimSpace(verifier))
	requestURL := strings.TrimRight(authBaseURL, "/") + "/cli/device/token?" + values.Encode()
	body, err := doQwenOAuthRequest(ctx, client, requestURL)
	if err != nil {
		message := strings.ToLower(err.Error())
		for _, status := range []string{"expired_token", "access_denied", "slow_down"} {
			if strings.Contains(message, status) {
				return &QwenOAuthPollResult{Status: status}, nil
			}
		}
		return nil, err
	}

	var payload qwenOAuthPollResponse
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	status := strings.ToLower(strings.TrimSpace(firstQwenOAuthString(payload.Data.Status, payload.Status)))
	if status == "" {
		status = "authorization_pending"
	}
	result := &QwenOAuthPollResult{Status: status}
	if status != "complete" {
		return result, nil
	}

	accessToken := firstQwenOAuthString(payload.Data.Credentials.AccessToken, payload.Credentials.AccessToken)
	expiresAt := firstQwenOAuthString(payload.Data.Credentials.ExpireTime, payload.Credentials.ExpireTime)
	if accessToken == "" || expiresAt == "" {
		return nil, errors.New("qwen oauth completed without credentials")
	}
	aliyunID := firstQwenOAuthString(
		payload.Data.Credentials.User.AliyunID,
		payload.Data.Credentials.User.Organization,
		payload.Credentials.User.AliyunID,
		payload.Credentials.User.Organization,
	)
	userID := payload.Data.Credentials.User.ID
	if userID == 0 {
		userID = payload.Credentials.User.ID
	}
	result.Credentials = &QwenOAuthCredentials{
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
		User: QwenOAuthUser{
			ID:       userID,
			Email:    firstQwenOAuthString(payload.Data.Credentials.User.Email, payload.Credentials.User.Email),
			AliyunID: aliyunID,
		},
	}
	return result, nil
}

func doQwenOAuthRequest(ctx context.Context, client *http.Client, requestURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "qianwen-cli/new-api")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("qwen oauth status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func firstQwenOAuthString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
