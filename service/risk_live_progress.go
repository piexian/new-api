package service

import (
	"context"
	"encoding/base64"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/go-redis/redis/v8"
)

const (
	RiskLiveSourceProbeGuard = "probe_guard"
	RiskLiveSourceErrorBan   = "error_ban"
	RiskLiveProbeGuardRuleID = "probe_guard"

	riskLiveProgressIndexKey   = "risk:live-progress:index:v1"
	riskLiveProgressDataPrefix = "risk:live-progress:data:v1:"
	riskLiveProgressMaxRecords = 20000
)

type RiskLiveRuleSummary struct {
	Source               string `json:"source"`
	RuleId               string `json:"rule_id"`
	RuleName             string `json:"rule_name"`
	Enabled              bool   `json:"enabled"`
	ParentEnabled        bool   `json:"parent_enabled"`
	System               bool   `json:"system"`
	DryRun               bool   `json:"dry_run"`
	Dimension            string `json:"dimension"`
	Threshold            int    `json:"threshold"`
	WindowSeconds        int    `json:"window_seconds"`
	ActiveTargets        int    `json:"active_targets"`
	NearThresholdTargets int    `json:"near_threshold_targets"`
	TriggeredTargets     int    `json:"triggered_targets"`
	MaxProgressPercent   int    `json:"max_progress_percent"`
	LastSeenAt           int64  `json:"last_seen_at"`
}

type RiskLiveTarget struct {
	Id               string   `json:"id"`
	Source           string   `json:"source"`
	RuleId           string   `json:"rule_id"`
	RuleName         string   `json:"rule_name"`
	Dimension        string   `json:"dimension"`
	Target           string   `json:"target"`
	UserId           int      `json:"user_id"`
	Username         string   `json:"username"`
	Context          string   `json:"context"`
	CurrentCount     int64    `json:"current_count"`
	Threshold        int      `json:"threshold"`
	ProgressPercent  int      `json:"progress_percent"`
	WindowSeconds    int      `json:"window_seconds"`
	RemainingSeconds int64    `json:"remaining_seconds"`
	LastSeenAt       int64    `json:"last_seen_at"`
	ExpiresAt        int64    `json:"expires_at"`
	Status           string   `json:"status"`
	Members          []string `json:"members"`
}

type riskLiveProgressRecord struct {
	RiskLiveTarget
	WindowKey   string `json:"window_key"`
	CooldownKey string `json:"cooldown_key"`
}

type riskLiveProgressMemoryStore struct {
	mu   sync.RWMutex
	data map[string]riskLiveProgressRecord
}

var riskLiveProgressMemory = riskLiveProgressMemoryStore{data: make(map[string]riskLiveProgressRecord)}

func newRiskLiveProgressId(source, ruleId, dimension, target string) string {
	raw := strings.Join([]string{source, ruleId, dimension, target}, "\x00")
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func riskLiveProgressDataKey(id string) string {
	return riskLiveProgressDataPrefix + id
}

func normalizeRiskLiveProgress(record *riskLiveProgressRecord, now int64) {
	if record.Threshold <= 0 {
		record.Threshold = 1
	}
	progress := int(float64(record.CurrentCount) / float64(record.Threshold) * 100)
	if progress > 100 {
		progress = 100
	}
	if progress < 0 {
		progress = 0
	}
	record.ProgressPercent = progress
	record.RemainingSeconds = record.ExpiresAt - now
	if record.RemainingSeconds < 0 {
		record.RemainingSeconds = 0
	}
	switch {
	case record.CurrentCount >= int64(record.Threshold):
		record.Status = "threshold_reached"
	case progress >= 80:
		record.Status = "near_threshold"
	default:
		record.Status = "observing"
	}
}

func (s *riskLiveProgressMemoryStore) upsert(record riskLiveProgressRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := common.GetTimestamp()
	for id, current := range s.data {
		if current.ExpiresAt <= now {
			delete(s.data, id)
		}
	}
	if _, exists := s.data[record.Id]; !exists && len(s.data) >= riskLiveProgressMaxRecords {
		var oldestId string
		var oldestAt int64
		for id, current := range s.data {
			if oldestId == "" || current.LastSeenAt < oldestAt {
				oldestId = id
				oldestAt = current.LastSeenAt
			}
		}
		delete(s.data, oldestId)
	}
	s.data[record.Id] = record
}

func (s *riskLiveProgressMemoryStore) list(now int64) []riskLiveProgressRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]riskLiveProgressRecord, 0, len(s.data))
	for id, record := range s.data {
		if record.ExpiresAt <= now {
			delete(s.data, id)
			continue
		}
		records = append(records, record)
	}
	return records
}

func (s *riskLiveProgressMemoryStore) delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
}

func recordRiskLiveProgress(record riskLiveProgressRecord) {
	now := common.GetTimestamp()
	record.Id = newRiskLiveProgressId(record.Source, record.RuleId, record.Dimension, record.Target)
	record.LastSeenAt = now
	record.ExpiresAt = now + int64(record.WindowSeconds)
	record.Members = nil
	normalizeRiskLiveProgress(&record, now)
	riskLiveProgressMemory.upsert(record)

	if !common.RedisEnabled {
		return
	}
	payload, err := common.Marshal(record)
	if err != nil {
		common.SysLog("risk live progress marshal failed: " + err.Error())
		return
	}
	ttl := time.Duration(record.WindowSeconds+120) * time.Second
	pipe := common.RDB.TxPipeline()
	pipe.Set(context.Background(), riskLiveProgressDataKey(record.Id), payload, ttl)
	pipe.ZAdd(context.Background(), riskLiveProgressIndexKey, &redis.Z{Score: float64(record.ExpiresAt), Member: record.Id})
	pipe.ZRemRangeByScore(context.Background(), riskLiveProgressIndexKey, "-inf", strconv.FormatInt(now, 10))
	pipe.ZRemRangeByRank(context.Background(), riskLiveProgressIndexKey, 0, -riskLiveProgressMaxRecords-1)
	if _, err := pipe.Exec(context.Background()); err != nil {
		common.SysLog("risk live progress redis update failed: " + err.Error())
	}
}

func loadRedisRiskLiveProgress(now int64) []riskLiveProgressRecord {
	ids, err := common.RDB.ZRevRangeByScore(context.Background(), riskLiveProgressIndexKey, &redis.ZRangeBy{
		Min:    strconv.FormatInt(now+1, 10),
		Max:    "+inf",
		Offset: 0,
		Count:  riskLiveProgressMaxRecords,
	}).Result()
	if err != nil || len(ids) == 0 {
		return nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = riskLiveProgressDataKey(id)
	}
	values, err := common.RDB.MGet(context.Background(), keys...).Result()
	if err != nil {
		return nil
	}
	records := make([]riskLiveProgressRecord, 0, len(values))
	staleIds := make([]interface{}, 0)
	for i, value := range values {
		text, ok := value.(string)
		if !ok || text == "" {
			staleIds = append(staleIds, ids[i])
			continue
		}
		var record riskLiveProgressRecord
		if err := common.UnmarshalJsonStr(text, &record); err != nil {
			staleIds = append(staleIds, ids[i])
			continue
		}
		if record.ExpiresAt > now {
			records = append(records, record)
		}
	}
	if len(staleIds) > 0 {
		_ = common.RDB.ZRem(context.Background(), riskLiveProgressIndexKey, staleIds...).Err()
	}
	return records
}

func deleteRiskLiveProgressRecord(record riskLiveProgressRecord) {
	riskLiveProgressMemory.delete(record.Id)
	if common.RedisEnabled {
		pipe := common.RDB.TxPipeline()
		pipe.Del(context.Background(), riskLiveProgressDataKey(record.Id))
		pipe.ZRem(context.Background(), riskLiveProgressIndexKey, record.Id)
		_, _ = pipe.Exec(context.Background())
	}
}

func loadRiskLiveProgress(exact bool) []riskLiveProgressRecord {
	now := common.GetTimestamp()
	merged := make(map[string]riskLiveProgressRecord)
	for _, record := range riskLiveProgressMemory.list(now) {
		merged[record.Id] = record
	}
	if common.RedisEnabled {
		for _, record := range loadRedisRiskLiveProgress(now) {
			current, exists := merged[record.Id]
			if !exists || record.LastSeenAt >= current.LastSeenAt {
				merged[record.Id] = record
			}
		}
	}

	records := make([]riskLiveProgressRecord, 0, len(merged))
	for _, record := range merged {
		if exact {
			record.CurrentCount = riskWindowCount(record.WindowKey, record.WindowSeconds)
			if record.CurrentCount == 0 {
				deleteRiskLiveProgressRecord(record)
				continue
			}
			if record.Source == RiskLiveSourceProbeGuard {
				record.Members = riskWindowMembers(record.WindowKey)
			}
		}
		normalizeRiskLiveProgress(&record, now)
		records = append(records, record)
	}
	return records
}

func GetRiskLiveRuleSummaries() []RiskLiveRuleSummary {
	probeSetting := risk_setting.GetProbeGuardSetting()
	errorSetting := risk_setting.GetErrorBanSetting()
	summaries := make([]RiskLiveRuleSummary, 0, len(errorSetting.Rules)+1)
	summaries = append(summaries, RiskLiveRuleSummary{
		Source:        RiskLiveSourceProbeGuard,
		RuleId:        RiskLiveProbeGuardRuleID,
		RuleName:      "Probe Guard",
		Enabled:       probeSetting.Enabled,
		ParentEnabled: true,
		System:        true,
		DryRun:        probeSetting.DryRun,
		Dimension:     probeSetting.BanDimension,
		Threshold:     probeSetting.DistinctModelCount,
		WindowSeconds: probeSetting.WindowSeconds,
	})
	for _, rule := range errorSetting.Rules {
		summaries = append(summaries, RiskLiveRuleSummary{
			Source:        RiskLiveSourceErrorBan,
			RuleId:        rule.Id,
			RuleName:      rule.Name,
			Enabled:       rule.Enabled,
			ParentEnabled: errorSetting.Enabled,
			DryRun:        errorSetting.DryRun,
			Dimension:     errorSetting.ResolveDimension(rule.Dimension),
			Threshold:     rule.Threshold,
			WindowSeconds: errorSetting.WindowSeconds,
		})
	}

	index := make(map[string]int, len(summaries))
	for i := range summaries {
		index[summaries[i].Source+"\x00"+summaries[i].RuleId] = i
	}
	recordsByRule := make(map[string][]riskLiveProgressRecord)
	for _, record := range loadRiskLiveProgress(false) {
		i, ok := index[record.Source+"\x00"+record.RuleId]
		if !ok {
			continue
		}
		summary := &summaries[i]
		summary.ActiveTargets++
		if record.LastSeenAt > summary.LastSeenAt {
			summary.LastSeenAt = record.LastSeenAt
		}
		key := record.Source + "\x00" + record.RuleId
		recordsByRule[key] = append(recordsByRule[key], record)
	}
	for key, records := range recordsByRule {
		i := index[key]
		summary := &summaries[i]
		sort.Slice(records, func(i, j int) bool {
			return records[i].ProgressPercent > records[j].ProgressPercent
		})
		for _, record := range records {
			// Exact reads are required for near-threshold counts. Below 80%, stop
			// once the snapshot cannot beat the exact maximum already found.
			if record.ProgressPercent < 80 && record.ProgressPercent <= summary.MaxProgressPercent {
				break
			}
			record.CurrentCount = riskWindowCount(record.WindowKey, record.WindowSeconds)
			normalizeRiskLiveProgress(&record, common.GetTimestamp())
			if record.Status == "near_threshold" {
				summary.NearThresholdTargets++
			}
			if record.Status == "threshold_reached" {
				summary.TriggeredTargets++
			}
			if record.ProgressPercent > summary.MaxProgressPercent {
				summary.MaxProgressPercent = record.ProgressPercent
			}
		}
	}
	return summaries
}

func GetRiskLiveTargets(source, ruleId, dimension, keyword string, startIdx, num int) ([]RiskLiveTarget, int) {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	records := loadRiskLiveProgress(false)
	filtered := make([]riskLiveProgressRecord, 0, len(records))
	for _, record := range records {
		if record.Source != source || record.RuleId != ruleId {
			continue
		}
		if dimension != "" && record.Dimension != dimension {
			continue
		}
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{record.Target, record.Username, record.Context}, " "))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		filtered = append(filtered, record)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].LastSeenAt == filtered[j].LastSeenAt {
			return filtered[i].Id < filtered[j].Id
		}
		return filtered[i].LastSeenAt > filtered[j].LastSeenAt
	})
	total := len(filtered)
	if startIdx < 0 {
		startIdx = 0
	}
	if num <= 0 {
		num = 10
	}
	if startIdx >= total {
		return []RiskLiveTarget{}, total
	}
	end := startIdx + num
	if end > total {
		end = total
	}
	items := make([]RiskLiveTarget, 0, end-startIdx)
	for _, record := range filtered[startIdx:end] {
		record.CurrentCount = riskWindowCount(record.WindowKey, record.WindowSeconds)
		if record.Source == RiskLiveSourceProbeGuard {
			record.Members = riskWindowMembers(record.WindowKey)
		}
		normalizeRiskLiveProgress(&record, common.GetTimestamp())
		items = append(items, record.RiskLiveTarget)
	}
	return items, total
}

// ClearRiskLiveProgress 清理与目标匹配的实时窗口、冷却状态和观察索引。
func ClearRiskLiveProgress(source, dimension, target string) {
	for _, record := range loadRiskLiveProgress(false) {
		if record.Source != source || record.Dimension != dimension || record.Target != target {
			continue
		}
		riskWindowDelete(record.WindowKey)
		if record.CooldownKey != "" {
			riskCooldownDelete(record.CooldownKey)
		}
		deleteRiskLiveProgressRecord(record)
	}
}

func resetRiskLiveProgressForTest() {
	riskLiveProgressMemory.mu.Lock()
	riskLiveProgressMemory.data = make(map[string]riskLiveProgressRecord)
	riskLiveProgressMemory.mu.Unlock()
}
