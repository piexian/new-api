package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// models.dev catalog 缓存：精简字段，TTL 24h
const (
	modelsDevCatalogURL    = "https://models.dev/api.json"
	modelsDevCatalogTTL    = 24 * time.Hour
	modelsDevFetchTimeout  = 30 * time.Second
)

// ModelsDevCatalogEntry 精简的模型条目
type ModelsDevCatalogEntry struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Family          string                 `json:"family,omitempty"`
	Reasoning       bool                   `json:"reasoning"`
	ReasoningOptions []map[string]any      `json:"reasoning_options,omitempty"`
	ToolCall        bool                   `json:"tool_call"`
	Attachment      bool                   `json:"attachment"`
	Modalities      ModelsDevModalities    `json:"modalities"`
}

type ModelsDevModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type ModelsDevCatalogResponse struct {
	Success   bool                              `json:"success"`
	Message   string                            `json:"message"`
	Data      map[string]ModelsDevCatalogEntry  `json:"data"`
	FetchedAt int64                              `json:"fetched_at"`
}

// 内存缓存
var (
	modelsDevCatalogCache     map[string]ModelsDevCatalogEntry
	modelsDevCatalogCacheTime time.Time
	modelsDevCatalogMutex      sync.RWMutex
	modelsDevFetching          bool
)

// GetModelsDevCatalog 获取 models.dev 精简分类目录（带内存缓存）
func GetModelsDevCatalog(c *gin.Context) {
	modelsDevCatalogMutex.RLock()
	cached := modelsDevCatalogCache
	cacheTime := modelsDevCatalogCacheTime
	modelsDevCatalogMutex.RUnlock()

	// 缓存有效且未过期
	if cached != nil && time.Since(cacheTime) < modelsDevCatalogTTL {
		c.JSON(http.StatusOK, ModelsDevCatalogResponse{
			Success:   true,
			Message:   "",
			Data:      cached,
			FetchedAt: cacheTime.Unix(),
		})
		return
	}

	// 防止并发重复拉取
	modelsDevCatalogMutex.Lock()
	if modelsDevFetching {
		modelsDevCatalogMutex.Unlock()
		// 已有其他请求在拉取，返回旧缓存或空
		if cached != nil {
			c.JSON(http.StatusOK, ModelsDevCatalogResponse{
				Success:   true,
				Message:   "serving stale cache while refreshing",
				Data:      cached,
				FetchedAt: cacheTime.Unix(),
			})
			return
		}
		c.JSON(http.StatusOK, ModelsDevCatalogResponse{
			Success: false,
			Message: "catalog is being loaded, please retry shortly",
			Data:    map[string]ModelsDevCatalogEntry{},
		})
		return
	}
	modelsDevFetching = true
	modelsDevCatalogMutex.Unlock()

	// 异步拉取不阻塞当前请求——但首次加载需要同步等待
	entries, err := fetchModelsDevCatalog()
	modelsDevCatalogMutex.Lock()
	modelsDevFetching = false
	if err != nil {
		modelsDevCatalogMutex.Unlock()
		// 拉取失败时返回旧缓存
		if cached != nil {
			c.JSON(http.StatusOK, ModelsDevCatalogResponse{
				Success:   true,
				Message:   "serving stale cache after fetch error: " + err.Error(),
				Data:      cached,
				FetchedAt: cacheTime.Unix(),
			})
			return
		}
		c.JSON(http.StatusOK, ModelsDevCatalogResponse{
			Success: false,
			Message: "failed to fetch catalog: " + err.Error(),
			Data:    map[string]ModelsDevCatalogEntry{},
		})
		return
	}
	modelsDevCatalogCache = entries
	modelsDevCatalogCacheTime = time.Now()
	modelsDevCatalogMutex.Unlock()

	c.JSON(http.StatusOK, ModelsDevCatalogResponse{
		Success:   true,
		Message:   "",
		Data:      entries,
		FetchedAt: modelsDevCatalogCacheTime.Unix(),
	})
}

// fetchModelsDevCatalog 拉取并精简 models.dev/api.json
func fetchModelsDevCatalog() (map[string]ModelsDevCatalogEntry, error) {
	client := &http.Client{Timeout: modelsDevFetchTimeout}
	resp, err := client.Get(modelsDevCatalogURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 原始结构：map[provider] -> { models: map[modelId] -> {...} }
	var raw map[string]json.RawMessage
	if err := common.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	entries := make(map[string]ModelsDevCatalogEntry, 1024)

	for _, providerRaw := range raw {
		// 尝试解析为 provider 结构
		var provider struct {
			Models map[string]json.RawMessage `json:"models"`
		}
		if err := common.Unmarshal(providerRaw, &provider); err != nil {
			continue
		}
		if provider.Models == nil {
			continue
		}

		for _, modelRaw := range provider.Models {
			var m struct {
				ID               string            `json:"id"`
				Name             string            `json:"name"`
				Family           string            `json:"family"`
				Reasoning         bool              `json:"reasoning"`
				ReasoningOptions []map[string]any  `json:"reasoning_options"`
				ToolCall         bool              `json:"tool_call"`
				Attachment       bool              `json:"attachment"`
				Modalities       ModelsDevModalities `json:"modalities"`
			}
			if err := common.Unmarshal(modelRaw, &m); err != nil {
				continue
			}
			// key 优先用 model.ID（含 provider 前缀如 openai/gpt-4o），
			// 同时也用裸模型名（gpt-4o）作为别名 key 以匹配后端 model name
			entry := ModelsDevCatalogEntry{
				ID:               m.ID,
				Name:             m.Name,
				Family:           m.Family,
				Reasoning:         m.Reasoning,
				ReasoningOptions: m.ReasoningOptions,
				ToolCall:          m.ToolCall,
				Attachment:       m.Attachment,
				Modalities:       m.Modalities,
			}
			entries[m.ID] = entry
			// 裸模型名别名（取 / 后半部分）
			if idx := strings.LastIndex(m.ID, "/"); idx >= 0 && idx < len(m.ID)-1 {
				bareName := m.ID[idx+1:]
				if _, exists := entries[bareName]; !exists {
					entries[bareName] = entry
				}
			} else {
				if _, exists := entries[m.ID]; !exists {
					entries[m.ID] = entry
				}
			}
		}
	}

	return entries, nil
}
