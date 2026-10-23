package service

// MiniMax Code 官方模型目录：桌面端托管 runtime 从云端同步的快照
// （official-model-config-sync.js）。桌面端的 /minimax-desktop/api/v1/models/list
// 由本地 runtime 提供，云端返回 404，不能直接调用。

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	minimaxCodeModelsPath    = "/mavis/api/v1/models"
	minimaxCodeModelsTimeout = 15 * time.Second
	// 快照里官方托管 provider 的 ID。
	minimaxCodeManagedProviderID = "minimax"
)

type miniMaxCodeModelSnapshot struct {
	Providers []struct {
		ProviderID string `json:"providerId"`
		Config     struct {
			Models map[string]struct {
				Enabled *bool `json:"enabled"`
			} `json:"models"`
			ModelOrder []string `json:"model_order"`
		} `json:"config"`
	} `json:"providers"`
}

// managedModelIDs 与官方 configuredModelIds 一致：跳过 enabled=false，
// 先按 model_order，余下模型按 ID 排序保证结果稳定。
func (s *miniMaxCodeModelSnapshot) managedModelIDs() ([]string, error) {
	for _, provider := range s.Providers {
		if provider.ProviderID != minimaxCodeManagedProviderID {
			continue
		}
		remaining := make([]string, 0, len(provider.Config.Models))
		for id, m := range provider.Config.Models {
			id = strings.TrimSpace(id)
			if id == "" || (m.Enabled != nil && !*m.Enabled) {
				continue
			}
			remaining = append(remaining, id)
		}
		sort.Strings(remaining)
		ids := make([]string, 0, len(remaining))
		for _, id := range provider.Config.ModelOrder {
			if idx := slices.Index(remaining, id); idx >= 0 {
				ids = append(ids, id)
				remaining = slices.Delete(remaining, idx, idx+1)
			}
		}
		ids = append(ids, remaining...)
		if len(ids) == 0 {
			break
		}
		return ids, nil
	}
	return nil, errors.New("minimax code returned no models")
}

// ListModels 拉取官方模型目录。与官方客户端一致：带令牌被拒（401/403）时匿名重试。
func (cl *MiniMaxCodeAccountClient) ListModels(ctx context.Context) ([]string, error) {
	snapshot, status, err := cl.fetchModelSnapshot(ctx, cl.accessToken)
	if err != nil && cl.accessToken != "" && (status == http.StatusUnauthorized || status == http.StatusForbidden) {
		snapshot, _, err = cl.fetchModelSnapshot(ctx, "")
	}
	if err != nil {
		return nil, err
	}
	return snapshot.managedModelIDs()
}

func (cl *MiniMaxCodeAccountClient) fetchModelSnapshot(ctx context.Context, accessToken string) (*miniMaxCodeModelSnapshot, int, error) {
	region := "cn"
	if cl.region == "global" {
		region = "en"
	}
	query := url.Values{}
	query.Set("region", region)
	query.Set("buildEnv", "prod")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		MiniMaxCodeMatrixOrigin(cl.region)+minimaxCodeModelsPath+"?"+query.Encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	resp, err := cl.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("minimax code model catalog request failed: http %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	var snapshot miniMaxCodeModelSnapshot
	if err := common.Unmarshal(raw, &snapshot); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("minimax code model catalog invalid response: %w", err)
	}
	return &snapshot, resp.StatusCode, nil
}

// FetchMiniMaxCodeModels 拉取官方模型目录；目录接口允许匿名访问，无需刷新凭证。
func FetchMiniMaxCodeModels(ctx context.Context, proxyURL string, oauthKey *MiniMaxCodeOAuthKey) ([]string, error) {
	client, err := NewMiniMaxCodeAccountClient(proxyURL, oauthKey.Region, oauthKey.AccessToken, oauthKey.UserID)
	if err != nil {
		return nil, err
	}
	listCtx, cancel := context.WithTimeout(ctx, minimaxCodeModelsTimeout)
	defer cancel()
	return client.ListModels(listCtx)
}
