package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var cfEmailHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	},
}

const cfEmailSendEndpoint = "https://api.cloudflare.com/client/v4/accounts/%s/email/sending/send"

type cfSendRequest struct {
	To      string `json:"to"`
	From    string `json:"from"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html"`
}

type cfSendResponse struct {
	Success  bool          `json:"success"`
	Errors   []cfAPIInfo   `json:"errors"`
	Messages []cfAPIInfo   `json:"messages"`
	Result   *cfSendResult `json:"result"`
}

type cfAPIInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cfSendResult struct {
	Delivered        []string `json:"delivered"`
	PermanentBounces []string `json:"permanent_bounces"`
	Queued           []string `json:"queued"`
}

func sendEmailViaCloudflare(subject string, receiver string, content string) error {
	if CFEmailAccountID == "" {
		return fmt.Errorf("Cloudflare Account ID not configured")
	}
	if CFEmailAPIToken == "" {
		return fmt.Errorf("Cloudflare API Token not configured")
	}
	if CFEmailFrom == "" {
		return fmt.Errorf("Cloudflare From address not configured")
	}

	if len(subject) > 998 {
		subject = subject[:998]
	}

	payload := cfSendRequest{
		To:      receiver,
		From:    CFEmailFrom,
		Subject: subject,
		Text:    content,
		HTML:    content,
	}

	body, err := Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Cloudflare email request: %w", err)
	}

	endpoint := fmt.Sprintf(cfEmailSendEndpoint, url.PathEscape(CFEmailAccountID))

	var resp cfSendResponse
	err = cfEmailDoRequest(endpoint, body, &resp)
	if err != nil {
		return err
	}

	if !resp.Success || len(resp.Errors) > 0 {
		errMsgs := ""
		for _, e := range resp.Errors {
			errMsgs += fmt.Sprintf("[%d] %s; ", e.Code, e.Message)
		}
		return fmt.Errorf("Cloudflare API error: %s", errMsgs)
	}

	if resp.Result != nil && len(resp.Result.PermanentBounces) > 0 {
		SysError(fmt.Sprintf("Cloudflare email permanent bounces: %v", resp.Result.PermanentBounces))
		return fmt.Errorf("email permanently bounced for recipients: %v", resp.Result.PermanentBounces)
	}

	return nil
}

func cfEmailDoRequest(endpoint string, body []byte, result *cfSendResponse) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create Cloudflare request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+CFEmailAPIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cfEmailHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("Cloudflare request failed: %w", err)
	}
	defer resp.Body.Close()

	err = DecodeJson(resp.Body, result)
	if err != nil {
		return fmt.Errorf("failed to decode Cloudflare response: %w", err)
	}

	switch resp.StatusCode {
	case 200:
		return nil
	case 400:
		return fmt.Errorf("Cloudflare bad request (HTTP %d): errors=%v", resp.StatusCode, result.Errors)
	case 401, 403:
		return fmt.Errorf("Cloudflare auth failed (HTTP %d): check API token and Email Sending permissions", resp.StatusCode)
	case 429:
		return fmt.Errorf("Cloudflare rate limited (HTTP 429)")
	default:
		if resp.StatusCode >= 500 {
			return fmt.Errorf("Cloudflare server error (HTTP %d)", resp.StatusCode)
		}
		return fmt.Errorf("Cloudflare unexpected status (HTTP %d): errors=%v", resp.StatusCode, result.Errors)
	}
}
