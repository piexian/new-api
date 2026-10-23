package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"golang.org/x/sync/singleflight"
)

// 共用会话标识风控：聚合日志中的会话标识/缓存关联标识，找出被多个用户共用的标识。
const (
	SessionRiskKindSession = "session"
	SessionRiskKindCache   = "cache"

	sessionRiskDefaultWindowSeconds = int64(30 * 24 * 60 * 60)
	SessionRiskMaxBatchBan          = 100
)

type SessionRiskUserRef struct {
	UserId       int    `json:"user_id"`
	Username     string `json:"username"`
	RequestCount int64  `json:"request_count"`
}

type SessionRiskTokenRef struct {
	TokenId      int    `json:"token_id"`
	TokenName    string `json:"token_name"`
	UserId       int    `json:"user_id"`
	Username     string `json:"username"`
	RequestCount int64  `json:"request_count"`
}

type SessionRiskItem struct {
	Kind         string                `json:"kind"`
	Identifier   string                `json:"identifier"`
	Sources      []string              `json:"sources"`
	UserCount    int64                 `json:"user_count"`
	TokenCount   int64                 `json:"token_count"`
	RequestCount int64                 `json:"request_count"`
	ErrorCount   int64                 `json:"error_count"`
	Quota        int64                 `json:"quota"`
	FirstSeenAt  int64                 `json:"first_seen_at"`
	LastSeenAt   int64                 `json:"last_seen_at"`
	Users        []SessionRiskUserRef  `json:"users"`
	Tokens       []SessionRiskTokenRef `json:"tokens"`
	IPs          []string              `json:"ips"`
}

type SessionRiskPage struct {
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Total    int               `json:"total"`
	Items    []SessionRiskItem `json:"items"`
	Start    int64             `json:"start"`
	End      int64             `json:"end"`
}

type sessionRiskRow struct {
	Identifier   string
	Hash         string
	Source       string
	IP           string `gorm:"column:ip"`
	TokenId      int    `gorm:"column:token_id"`
	TokenName    string `gorm:"column:token_name"`
	UserId       int    `gorm:"column:user_id"`
	Username     string `gorm:"column:username"`
	RequestCount int64  `gorm:"column:request_count"`
	ErrorCount   int64  `gorm:"column:error_count"`
	Quota        int64  `gorm:"column:quota"`
	FirstSeenAt  int64  `gorm:"column:first_seen_at"`
	LastSeenAt   int64  `gorm:"column:last_seen_at"`
}

func sessionRiskColumns(kind string) (idCol string, hashCol string, sourceCol string, err error) {
	switch kind {
	case SessionRiskKindSession:
		return "session_id", "session_hash", "session_source", nil
	case SessionRiskKindCache:
		return "cache_key", "cache_hash", "cache_source", nil
	default:
		return "", "", "", errors.New("invalid session risk kind")
	}
}

func normalizeSessionRiskWindow(start, end int64) (int64, int64) {
	if end <= 0 {
		end = time.Now().Unix()
	}
	if start <= 0 || start >= end {
		start = end - sessionRiskDefaultWindowSeconds
	}
	return start, end
}

// listSessionRiskStats 聚合完整时间窗内的原始请求，不做任何条数截断，
// 避免先截断再聚合漏掉共用同一标识的其他账号。列名只来自上面的白名单。
// 用 "> ''" 而非 "<> ''" 排除空标识：前者可以在 (hash, created_at) 索引上做范围扫描，
// 后者会让优化器退回按时间扫描整窗日志，而带标识的日志只占很小一部分。
func listSessionRiskStats(kind string, start, end int64, identifier string) ([]sessionRiskRow, error) {
	idCol, hashCol, sourceCol, err := sessionRiskColumns(kind)
	if err != nil {
		return nil, err
	}
	tx := model.LOG_DB.Model(&model.Log{}).
		Select(fmt.Sprintf(`MIN(%s) AS identifier, %s AS hash, %s AS source,
			ip, token_id, token_name, user_id, username, COUNT(*) AS request_count,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS error_count,
			SUM(quota) AS quota, MIN(created_at) AS first_seen_at, MAX(created_at) AS last_seen_at`, idCol, hashCol, sourceCol), model.LogTypeError).
		Where(hashCol+" > '' AND user_id > 0 AND type IN ? AND created_at >= ? AND created_at <= ?", []int{model.LogTypeConsume, model.LogTypeError}, start, end).
		Group(hashCol + ", " + sourceCol + ", ip, token_id, token_name, user_id, username")
	if identifier != "" {
		tx = tx.Where(hashCol+" = ?", common.SessionIdentifierHash(identifier))
	}
	var rows []sessionRiskRow
	err = tx.Find(&rows).Error
	return rows, err
}

// sharedSessionItemsCacheTTL 整窗聚合代价高，翻页/排序/搜索在短期内复用同一时间窗的结果。
const sharedSessionItemsCacheTTL = time.Minute

type sharedSessionItemsEntry struct {
	items     []SessionRiskItem
	expiresAt time.Time
}

var (
	sharedSessionItemsCache = struct {
		sync.Mutex
		entries map[string]sharedSessionItemsEntry
	}{entries: map[string]sharedSessionItemsEntry{}}
	sharedSessionItemsGroup singleflight.Group
)

// cachedSharedSessionItems 返回时间窗内的共用标识簇；同一时间窗的并发请求只查询一次日志库。
// 返回的切片由多个请求共享，调用方不得原地修改。
func cachedSharedSessionItems(kind string, start, end int64) ([]SessionRiskItem, error) {
	key := fmt.Sprintf("%s:%d:%d", kind, start, end)
	sharedSessionItemsCache.Lock()
	entry, ok := sharedSessionItemsCache.entries[key]
	sharedSessionItemsCache.Unlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.items, nil
	}
	value, err, _ := sharedSessionItemsGroup.Do(key, func() (interface{}, error) {
		rows, err := listSessionRiskStats(kind, start, end, "")
		if err != nil {
			return nil, err
		}
		items := sharedSessionItems(rows, kind)
		now := time.Now()
		sharedSessionItemsCache.Lock()
		for k, e := range sharedSessionItemsCache.entries {
			if !now.Before(e.expiresAt) {
				delete(sharedSessionItemsCache.entries, k)
			}
		}
		sharedSessionItemsCache.entries[key] = sharedSessionItemsEntry{items: items, expiresAt: now.Add(sharedSessionItemsCacheTTL)}
		sharedSessionItemsCache.Unlock()
		return items, nil
	})
	if err != nil {
		return nil, err
	}
	return value.([]SessionRiskItem), nil
}

// sharedSessionItems 把明细行按标识哈希聚合成簇，只保留被至少两个用户共用的标识。
func sharedSessionItems(rows []sessionRiskRow, kind string) []SessionRiskItem {
	type bucket struct {
		item         SessionRiskItem
		users        map[int]*SessionRiskUserRef
		tokens       map[int]*SessionRiskTokenRef
		ips, sources map[string]struct{}
	}
	buckets := make(map[string]*bucket)
	for _, row := range rows {
		b := buckets[row.Hash]
		if b == nil {
			b = &bucket{
				item:    SessionRiskItem{Kind: kind, Identifier: row.Identifier},
				users:   map[int]*SessionRiskUserRef{},
				tokens:  map[int]*SessionRiskTokenRef{},
				ips:     map[string]struct{}{},
				sources: map[string]struct{}{},
			}
			buckets[row.Hash] = b
		}
		b.item.RequestCount += row.RequestCount
		b.item.ErrorCount += row.ErrorCount
		b.item.Quota += row.Quota
		if b.item.FirstSeenAt == 0 || row.FirstSeenAt < b.item.FirstSeenAt {
			b.item.FirstSeenAt = row.FirstSeenAt
		}
		if row.LastSeenAt > b.item.LastSeenAt {
			b.item.LastSeenAt = row.LastSeenAt
		}
		if b.users[row.UserId] == nil {
			b.users[row.UserId] = &SessionRiskUserRef{UserId: row.UserId, Username: row.Username}
		}
		b.users[row.UserId].RequestCount += row.RequestCount
		if row.TokenId > 0 {
			if b.tokens[row.TokenId] == nil {
				b.tokens[row.TokenId] = &SessionRiskTokenRef{TokenId: row.TokenId, TokenName: row.TokenName, UserId: row.UserId, Username: row.Username}
			}
			b.tokens[row.TokenId].RequestCount += row.RequestCount
		}
		if row.IP != "" {
			b.ips[row.IP] = struct{}{}
		}
		if row.Source != "" {
			b.sources[row.Source] = struct{}{}
		}
	}
	items := make([]SessionRiskItem, 0, len(buckets))
	for _, b := range buckets {
		if len(b.users) < 2 {
			continue
		}
		b.item.UserCount, b.item.TokenCount = int64(len(b.users)), int64(len(b.tokens))
		b.item.Users = sortedSessionRiskUsers(b.users)
		b.item.Tokens = sortedSessionRiskTokens(b.tokens)
		b.item.IPs = sortedSessionRiskStrings(b.ips)
		b.item.Sources = sortedSessionRiskStrings(b.sources)
		items = append(items, b.item)
	}
	return items
}

func sortedSessionRiskUsers(m map[int]*SessionRiskUserRef) []SessionRiskUserRef {
	out := make([]SessionRiskUserRef, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RequestCount != out[j].RequestCount {
			return out[i].RequestCount > out[j].RequestCount
		}
		return out[i].UserId < out[j].UserId
	})
	return out
}

func sortedSessionRiskTokens(m map[int]*SessionRiskTokenRef) []SessionRiskTokenRef {
	out := make([]SessionRiskTokenRef, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RequestCount != out[j].RequestCount {
			return out[i].RequestCount > out[j].RequestCount
		}
		return out[i].TokenId < out[j].TokenId
	})
	return out
}

func sortedSessionRiskStrings(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// withoutDisabledSessionRiskUsers 按当前账号状态剔除已封禁/已删除的用户及其令牌，
// 启用账号不足两个的标识不再算作共用。账号状态每次实时查询，封禁后无需等待聚合缓存过期。
// 请求量、错误数、额度、IP 等仍是该标识在时间窗内的整体统计。
func withoutDisabledSessionRiskUsers(items []SessionRiskItem) ([]SessionRiskItem, error) {
	ids := make([]int, 0)
	seen := make(map[int]struct{})
	for _, item := range items {
		for _, user := range item.Users {
			if _, ok := seen[user.UserId]; !ok {
				seen[user.UserId] = struct{}{}
				ids = append(ids, user.UserId)
			}
		}
	}
	enabled, err := model.ListEnabledUserIds(ids)
	if err != nil {
		return nil, err
	}
	out := make([]SessionRiskItem, 0, len(items))
	for _, item := range items {
		users := make([]SessionRiskUserRef, 0, len(item.Users))
		for _, user := range item.Users {
			if _, ok := enabled[user.UserId]; ok {
				users = append(users, user)
			}
		}
		if len(users) < 2 {
			continue
		}
		tokens := make([]SessionRiskTokenRef, 0, len(item.Tokens))
		for _, token := range item.Tokens {
			if _, ok := enabled[token.UserId]; ok {
				tokens = append(tokens, token)
			}
		}
		// item 是缓存条目的值拷贝，替换切片不会影响缓存。
		item.Users, item.Tokens = users, tokens
		item.UserCount, item.TokenCount = int64(len(users)), int64(len(tokens))
		out = append(out, item)
	}
	return out, nil
}

func sessionRiskItemMatchesKeyword(item SessionRiskItem, keyword string) bool {
	if keyword == "" {
		return true
	}
	if strings.Contains(strings.ToLower(item.Identifier), keyword) {
		return true
	}
	for _, user := range item.Users {
		if strings.Contains(strings.ToLower(user.Username), keyword) {
			return true
		}
	}
	for _, token := range item.Tokens {
		if strings.Contains(strings.ToLower(token.TokenName), keyword) {
			return true
		}
	}
	for _, ip := range item.IPs {
		if strings.Contains(strings.ToLower(ip), keyword) {
			return true
		}
	}
	for _, source := range item.Sources {
		if strings.Contains(strings.ToLower(source), keyword) {
			return true
		}
	}
	return false
}

// ListSharedSessions 列出时间窗内被多个用户共用的会话标识/缓存关联标识。
func ListSharedSessions(kind string, page, pageSize int, keyword string, start, end int64, sortKey, sortOrder string) (SessionRiskPage, error) {
	start, end = normalizeSessionRiskWindow(start, end)
	shared, err := cachedSharedSessionItems(kind, start, end)
	if err != nil {
		return SessionRiskPage{}, err
	}
	shared, err = withoutDisabledSessionRiskUsers(shared)
	if err != nil {
		return SessionRiskPage{}, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	items := make([]SessionRiskItem, 0)
	// 先构建完整关联再按关键字过滤，避免搜索隐藏同一标识下的其他用户。
	for _, item := range shared {
		if sessionRiskItemMatchesKeyword(item, keyword) {
			items = append(items, item)
		}
	}
	numeric := func(item SessionRiskItem, key string) int64 {
		switch key {
		case "token_count":
			return item.TokenCount
		case "request_count":
			return item.RequestCount
		case "error_count":
			return item.ErrorCount
		case "quota":
			return item.Quota
		case "first_seen_at":
			return item.FirstSeenAt
		case "user_count":
			return item.UserCount
		default:
			return item.LastSeenAt
		}
	}
	asc := sortOrder == "asc"
	sort.Slice(items, func(i, j int) bool {
		if sortKey == "identifier" {
			if items[i].Identifier != items[j].Identifier {
				if asc {
					return items[i].Identifier < items[j].Identifier
				}
				return items[i].Identifier > items[j].Identifier
			}
		} else if sortKey != "" {
			vi, vj := numeric(items[i], sortKey), numeric(items[j], sortKey)
			if vi != vj {
				if asc {
					return vi < vj
				}
				return vi > vj
			}
		}
		if items[i].LastSeenAt != items[j].LastSeenAt {
			return items[i].LastSeenAt > items[j].LastSeenAt
		}
		return items[i].Identifier < items[j].Identifier
	})
	total := len(items)
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	from := (page - 1) * pageSize
	if from > total {
		from = total
	}
	to := from + pageSize
	if to > total {
		to = total
	}
	return SessionRiskPage{Page: page, PageSize: pageSize, Total: total, Items: items[from:to], Start: start, End: end}, nil
}

type SessionRiskBanRequest struct {
	UserIds         []int  `json:"user_ids"`
	Reason          string `json:"reason"`
	DurationMinutes int    `json:"duration_minutes"`
	Kind            string `json:"kind"`
	Identifier      string `json:"identifier"`
	Start           int64  `json:"start"`
	End             int64  `json:"end"`
}

// BanSharedSessionUsers 批量封禁共用标识的关联用户；封禁前在同一时间窗内复核，
// 选定用户必须确实关联该标识，防止凭过期数据误封。
func BanSharedSessionUsers(req SessionRiskBanRequest, operatorID int) (map[string]interface{}, error) {
	identifier := common.NormalizeSessionIdentifier(req.Identifier)
	if identifier == "" {
		return nil, errors.New("invalid session identifier")
	}
	if _, _, _, err := sessionRiskColumns(req.Kind); err != nil {
		return nil, err
	}
	if req.Start <= 0 || req.End < req.Start {
		return nil, errors.New("a valid session risk time window is required")
	}
	seen := map[int]bool{}
	selected := make([]int, 0, len(req.UserIds))
	for _, id := range req.UserIds {
		if id > 0 && !seen[id] {
			seen[id] = true
			selected = append(selected, id)
		}
	}
	if len(selected) == 0 || len(selected) > SessionRiskMaxBatchBan {
		return nil, fmt.Errorf("select 1-%d associated users", SessionRiskMaxBatchBan)
	}
	rows, err := listSessionRiskStats(req.Kind, req.Start, req.End, identifier)
	if err != nil {
		return nil, err
	}
	items := sharedSessionItems(rows, req.Kind)
	if len(items) != 1 || items[0].Identifier != identifier {
		return nil, errors.New("no shared session found in the selected window")
	}
	associated := map[int]bool{}
	for _, user := range items[0].Users {
		associated[user.UserId] = true
	}
	for _, id := range selected {
		if !associated[id] {
			return nil, errors.New("selected user is not associated with this identifier in the selected window")
		}
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "shared " + req.Kind + " risk ban"
	}
	success := 0
	failures := make([]map[string]interface{}, 0)
	sort.Ints(selected)
	for _, id := range selected {
		user, err := model.DisableUserBySessionRiskReview(id, reason, req.DurationMinutes, operatorID, time.Now().Unix())
		if err != nil {
			failures = append(failures, map[string]interface{}{"id": id, "message": err.Error()})
			continue
		}
		if err := model.InvalidateUserCache(user.Id); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate session-risk user cache for user %d: %s", user.Id, err.Error()))
		}
		if err := model.InvalidateUserTokensCache(user.Id); err != nil {
			common.SysLog(fmt.Sprintf("failed to invalidate session-risk token cache for user %d: %s", user.Id, err.Error()))
		}
		NotifyAccountDisabled(*user)
		success++
	}
	return map[string]interface{}{"total_users": len(selected), "success": success, "failures": failures}, nil
}
