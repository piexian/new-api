package service

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	multiAccountLoginWindowSeconds = int64(30 * 24 * 60 * 60)
	multiAccountLoginSignalLimit   = 50000
)

type MultiAccountStats struct {
	TotalClusters      int `json:"total_clusters"`
	HighRiskClusters   int `json:"high_risk_clusters"`
	RelatedAccounts    int `json:"related_accounts"`
	EmailConflicts     int `json:"email_conflicts"`
	SharedEnvironments int `json:"shared_environments"`
}

type MultiAccountUser struct {
	Id            int    `json:"id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	GitHubId      string `json:"github_id"`
	Role          int    `json:"role"`
	Status        int    `json:"status"`
	DisableReason string `json:"disable_reason"`
	DisabledUntil int64  `json:"disabled_until"`
	Deleted       bool   `json:"deleted"`
	CreatedAt     int64  `json:"created_at"`
	LastLoginAt   int64  `json:"last_login_at"`
	CanBan        bool   `json:"can_ban"`
}

type MultiAccountEvidence struct {
	Type        string `json:"type"`
	Email       string `json:"email"`
	IP          string `json:"ip"`
	UserAgent   string `json:"user_agent"`
	UserIds     []int  `json:"user_ids"`
	HitCount    int    `json:"hit_count"`
	FirstSeenAt int64  `json:"first_seen_at"`
	LastSeenAt  int64  `json:"last_seen_at"`
}

type MultiAccountCluster struct {
	Id         string                 `json:"id"`
	Rank       int                    `json:"rank"`
	RiskScore  int                    `json:"risk_score"`
	RiskLevel  string                 `json:"risk_level"`
	Accounts   []MultiAccountUser     `json:"accounts"`
	Evidence   []MultiAccountEvidence `json:"evidence"`
	LastSeenAt int64                  `json:"last_seen_at"`
}

type MultiAccountPage struct {
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Total    int                   `json:"total"`
	Items    []MultiAccountCluster `json:"items"`
	Stats    MultiAccountStats     `json:"stats"`
}

type loginEnvironmentUser struct {
	Count       int
	FirstSeenAt int64
	LastSeenAt  int64
}

type loginEnvironmentGroup struct {
	IP        string
	UserAgent string
	Users     map[int]*loginEnvironmentUser
}

type accountUnionFind struct {
	parent map[int]int
}

func newAccountUnionFind() *accountUnionFind {
	return &accountUnionFind{parent: make(map[int]int)}
}

func (u *accountUnionFind) add(id int) {
	if id > 0 {
		if _, ok := u.parent[id]; !ok {
			u.parent[id] = id
		}
	}
}

func (u *accountUnionFind) find(id int) int {
	parent, ok := u.parent[id]
	if !ok {
		return 0
	}
	if parent != id {
		u.parent[id] = u.find(parent)
	}
	return u.parent[id]
}

func (u *accountUnionFind) union(ids []int) {
	if len(ids) < 2 {
		return
	}
	for _, id := range ids {
		u.add(id)
	}
	root := u.find(ids[0])
	for _, id := range ids[1:] {
		otherRoot := u.find(id)
		if otherRoot != 0 && otherRoot != root {
			u.parent[otherRoot] = root
		}
	}
}

func collectMultiAccountEvidence() ([]MultiAccountEvidence, error) {
	stored, err := model.ListMultiAccountEvidence(20000)
	if err != nil {
		return nil, err
	}
	evidence := make([]MultiAccountEvidence, 0, len(stored))
	for _, item := range stored {
		evidence = append(evidence, MultiAccountEvidence{
			Type:        item.EvidenceType,
			Email:       item.Email,
			UserIds:     []int{item.PrimaryUserId, item.RelatedUserId},
			HitCount:    item.HitCount,
			FirstSeenAt: item.FirstSeenAt,
			LastSeenAt:  item.LastSeenAt,
		})
	}

	signals, err := model.ListRecentLoginEnvironmentSignals(common.GetTimestamp()-multiAccountLoginWindowSeconds, multiAccountLoginSignalLimit)
	if err != nil {
		return nil, err
	}
	groups := make(map[string]*loginEnvironmentGroup)
	for _, signal := range signals {
		key := signal.IP + "\x00" + signal.UserAgent
		group := groups[key]
		if group == nil {
			group = &loginEnvironmentGroup{
				IP:        signal.IP,
				UserAgent: signal.UserAgent,
				Users:     make(map[int]*loginEnvironmentUser),
			}
			groups[key] = group
		}
		user := group.Users[signal.UserId]
		if user == nil {
			user = &loginEnvironmentUser{FirstSeenAt: signal.CreatedAt, LastSeenAt: signal.CreatedAt}
			group.Users[signal.UserId] = user
		}
		user.Count++
		if signal.CreatedAt < user.FirstSeenAt {
			user.FirstSeenAt = signal.CreatedAt
		}
		if signal.CreatedAt > user.LastSeenAt {
			user.LastSeenAt = signal.CreatedAt
		}
	}
	for _, group := range groups {
		if len(group.Users) < 2 {
			continue
		}
		userIds := make([]int, 0, len(group.Users))
		hitCount := 0
		firstSeenAt := int64(0)
		lastSeenAt := int64(0)
		for userId, user := range group.Users {
			userIds = append(userIds, userId)
			hitCount += user.Count
			if firstSeenAt == 0 || user.FirstSeenAt < firstSeenAt {
				firstSeenAt = user.FirstSeenAt
			}
			if user.LastSeenAt > lastSeenAt {
				lastSeenAt = user.LastSeenAt
			}
		}
		sort.Ints(userIds)
		evidence = append(evidence, MultiAccountEvidence{
			Type:        "shared_ip_user_agent",
			IP:          group.IP,
			UserAgent:   group.UserAgent,
			UserIds:     userIds,
			HitCount:    hitCount,
			FirstSeenAt: firstSeenAt,
			LastSeenAt:  lastSeenAt,
		})
	}
	return evidence, nil
}

func multiAccountUserFromModel(user model.User) MultiAccountUser {
	return MultiAccountUser{
		Id:            user.Id,
		Username:      user.Username,
		Email:         user.Email,
		GitHubId:      user.GitHubId,
		Role:          user.Role,
		Status:        user.Status,
		DisableReason: user.DisableReason,
		DisabledUntil: user.DisabledUntil,
		Deleted:       user.DeletedAt.Valid,
		CreatedAt:     user.CreatedAt,
		LastLoginAt:   user.LastLoginAt,
		CanBan:        !user.DeletedAt.Valid && user.Role == common.RoleCommonUser && user.Status == common.UserStatusEnabled,
	}
}

func multiAccountClusterID(userIds []int) string {
	parts := make([]string, 0, len(userIds))
	for _, id := range userIds {
		parts = append(parts, strconv.Itoa(id))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, ",")))
	return fmt.Sprintf("%x", sum[:8])
}

func scoreMultiAccountCluster(accounts []MultiAccountUser, evidence []MultiAccountEvidence) (int, string) {
	score := 0
	types := make(map[string]struct{})
	for _, item := range evidence {
		types[item.Type] = struct{}{}
		switch item.Type {
		case model.MultiAccountEvidenceGitHubEmailConflict:
			score += 75 + min(item.HitCount, 15)
		case "shared_ip_user_agent":
			score += 30 + min(item.HitCount, 15)
			if len(item.UserIds) == 2 {
				score += 10
			} else if len(item.UserIds) == 3 {
				score += 5
			}
		}
	}
	if len(types) > 1 {
		score += 10
	}
	createdMin := int64(0)
	createdMax := int64(0)
	for _, account := range accounts {
		if account.CreatedAt <= 0 {
			continue
		}
		if createdMin == 0 || account.CreatedAt < createdMin {
			createdMin = account.CreatedAt
		}
		if account.CreatedAt > createdMax {
			createdMax = account.CreatedAt
		}
	}
	if len(accounts) >= 2 && createdMin > 0 {
		switch delta := createdMax - createdMin; {
		case delta <= 10*60:
			score += 20
		case delta <= 24*60*60:
			score += 10
		}
	}
	if score > 100 {
		score = 100
	}
	level := "low"
	if score >= 60 {
		level = "high"
	} else if score >= 35 {
		level = "medium"
	}
	return score, level
}

func buildMultiAccountClusters(evidence []MultiAccountEvidence) ([]MultiAccountCluster, MultiAccountStats, error) {
	unionFind := newAccountUnionFind()
	for _, item := range evidence {
		unionFind.union(item.UserIds)
	}
	allIds := make([]int, 0, len(unionFind.parent))
	for id := range unionFind.parent {
		allIds = append(allIds, id)
	}
	users, err := model.GetUsersByIDsUnscoped(allIds)
	if err != nil {
		return nil, MultiAccountStats{}, err
	}
	usersById := make(map[int]model.User, len(users))
	for _, user := range users {
		usersById[user.Id] = user
	}

	evidenceByRoot := make(map[int][]MultiAccountEvidence)
	for _, item := range evidence {
		if len(item.UserIds) == 0 {
			continue
		}
		root := unionFind.find(item.UserIds[0])
		if root > 0 {
			evidenceByRoot[root] = append(evidenceByRoot[root], item)
		}
	}
	idsByRoot := make(map[int][]int)
	for id := range unionFind.parent {
		root := unionFind.find(id)
		if _, ok := usersById[id]; ok {
			idsByRoot[root] = append(idsByRoot[root], id)
		}
	}

	clusters := make([]MultiAccountCluster, 0, len(idsByRoot))
	stats := MultiAccountStats{}
	relatedAccounts := make(map[int]struct{})
	for _, item := range evidence {
		switch item.Type {
		case model.MultiAccountEvidenceGitHubEmailConflict:
			stats.EmailConflicts++
		case "shared_ip_user_agent":
			stats.SharedEnvironments++
		}
	}
	for root, userIds := range idsByRoot {
		if len(userIds) < 2 {
			continue
		}
		sort.Ints(userIds)
		accounts := make([]MultiAccountUser, 0, len(userIds))
		for _, id := range userIds {
			accounts = append(accounts, multiAccountUserFromModel(usersById[id]))
			relatedAccounts[id] = struct{}{}
		}
		clusterEvidence := evidenceByRoot[root]
		sort.Slice(clusterEvidence, func(i, j int) bool {
			return clusterEvidence[i].LastSeenAt > clusterEvidence[j].LastSeenAt
		})
		lastSeenAt := int64(0)
		for _, item := range clusterEvidence {
			if item.LastSeenAt > lastSeenAt {
				lastSeenAt = item.LastSeenAt
			}
		}
		score, level := scoreMultiAccountCluster(accounts, clusterEvidence)
		clusters = append(clusters, MultiAccountCluster{
			Id:         multiAccountClusterID(userIds),
			RiskScore:  score,
			RiskLevel:  level,
			Accounts:   accounts,
			Evidence:   clusterEvidence,
			LastSeenAt: lastSeenAt,
		})
	}
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].RiskScore != clusters[j].RiskScore {
			return clusters[i].RiskScore > clusters[j].RiskScore
		}
		if clusters[i].LastSeenAt != clusters[j].LastSeenAt {
			return clusters[i].LastSeenAt > clusters[j].LastSeenAt
		}
		return clusters[i].Id < clusters[j].Id
	})
	for index := range clusters {
		clusters[index].Rank = index + 1
		if clusters[index].RiskLevel == "high" {
			stats.HighRiskClusters++
		}
	}
	stats.TotalClusters = len(clusters)
	stats.RelatedAccounts = len(relatedAccounts)
	return clusters, stats, nil
}

func multiAccountClusterMatches(cluster MultiAccountCluster, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	if strings.Contains(strings.ToLower(cluster.Id), keyword) {
		return true
	}
	for _, account := range cluster.Accounts {
		values := []string{strconv.Itoa(account.Id), account.Username, account.Email, account.GitHubId, account.DisableReason}
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), keyword) {
				return true
			}
		}
	}
	for _, item := range cluster.Evidence {
		for _, value := range []string{item.Email, item.IP, item.UserAgent} {
			if strings.Contains(strings.ToLower(value), keyword) {
				return true
			}
		}
	}
	return false
}

func ListMultiAccountClusters(page, pageSize int, keyword string) (*MultiAccountPage, error) {
	evidence, err := collectMultiAccountEvidence()
	if err != nil {
		return nil, err
	}
	clusters, stats, err := buildMultiAccountClusters(evidence)
	if err != nil {
		return nil, err
	}
	filtered := make([]MultiAccountCluster, 0, len(clusters))
	for _, cluster := range clusters {
		if multiAccountClusterMatches(cluster, keyword) {
			filtered = append(filtered, cluster)
		}
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = common.ItemsPerPage
	}
	if pageSize > 100 {
		pageSize = 100
	}
	start := (page - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := min(start+pageSize, len(filtered))
	return &MultiAccountPage{
		Page:     page,
		PageSize: pageSize,
		Total:    len(filtered),
		Items:    filtered[start:end],
		Stats:    stats,
	}, nil
}

func BanMultiAccountUser(userId, operatorId, durationMinutes int, reason string) (*MultiAccountUser, error) {
	user, err := model.DisableUserByMultiAccountReview(userId, reason, durationMinutes, operatorId, common.GetTimestamp())
	if err != nil {
		return nil, err
	}
	if err := model.InvalidateUserCache(user.Id); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate multi-account user cache for user %d: %s", user.Id, err.Error()))
	}
	if err := model.InvalidateUserTokensCache(user.Id); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate multi-account token cache for user %d: %s", user.Id, err.Error()))
	}
	NotifyAccountDisabled(*user)
	result := multiAccountUserFromModel(*user)
	return &result, nil
}
