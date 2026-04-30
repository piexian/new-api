package model

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/samber/hot"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Subscription duration units
const (
	SubscriptionDurationYear   = "year"
	SubscriptionDurationMonth  = "month"
	SubscriptionDurationDay    = "day"
	SubscriptionDurationHour   = "hour"
	SubscriptionDurationCustom = "custom"
)

// Subscription quota reset period
const (
	SubscriptionResetNever   = "never"
	SubscriptionResetDaily   = "daily"
	SubscriptionResetWeekly  = "weekly"
	SubscriptionResetMonthly = "monthly"
	SubscriptionResetCustom  = "custom"
)

var (
	ErrSubscriptionOrderNotFound        = errors.New("subscription order not found")
	ErrSubscriptionOrderStatusInvalid   = errors.New("subscription order status invalid")
	ErrSubscriptionWalletQuotaNotEnough = errors.New("wallet quota not enough")
)

const (
	subscriptionPlanCacheNamespace     = "new-api:subscription_plan:v1"
	subscriptionPlanInfoCacheNamespace = "new-api:subscription_plan_info:v1"
)

var (
	subscriptionPlanCacheOnce     sync.Once
	subscriptionPlanInfoCacheOnce sync.Once

	subscriptionPlanCache     *cachex.HybridCache[SubscriptionPlan]
	subscriptionPlanInfoCache *cachex.HybridCache[SubscriptionPlanInfo]
)

func subscriptionPlanCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanInfoCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_TTL", 120)
	if ttlSeconds <= 0 {
		ttlSeconds = 120
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_CAP", 5000)
	if capacity <= 0 {
		capacity = 5000
	}
	return capacity
}

func subscriptionPlanInfoCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_CAP", 10000)
	if capacity <= 0 {
		capacity = 10000
	}
	return capacity
}

func getSubscriptionPlanCache() *cachex.HybridCache[SubscriptionPlan] {
	subscriptionPlanCacheOnce.Do(func() {
		ttl := subscriptionPlanCacheTTL()
		subscriptionPlanCache = cachex.NewHybridCache[SubscriptionPlan](cachex.HybridCacheConfig[SubscriptionPlan]{
			Namespace: cachex.Namespace(subscriptionPlanCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlan]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlan] {
				return hot.NewHotCache[string, SubscriptionPlan](hot.LRU, subscriptionPlanCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanCache
}

func getSubscriptionPlanInfoCache() *cachex.HybridCache[SubscriptionPlanInfo] {
	subscriptionPlanInfoCacheOnce.Do(func() {
		ttl := subscriptionPlanInfoCacheTTL()
		subscriptionPlanInfoCache = cachex.NewHybridCache[SubscriptionPlanInfo](cachex.HybridCacheConfig[SubscriptionPlanInfo]{
			Namespace: cachex.Namespace(subscriptionPlanInfoCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlanInfo]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlanInfo] {
				return hot.NewHotCache[string, SubscriptionPlanInfo](hot.LRU, subscriptionPlanInfoCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanInfoCache
}

func subscriptionPlanCacheKey(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func InvalidateSubscriptionPlanCache(planId int) {
	if planId <= 0 {
		return
	}
	cache := getSubscriptionPlanCache()
	_, _ = cache.DeleteMany([]string{subscriptionPlanCacheKey(planId)})
	infoCache := getSubscriptionPlanInfoCache()
	_ = infoCache.Purge()
}

// Subscription plan
type SubscriptionPlan struct {
	Id int `json:"id"`

	Title    string `json:"title" gorm:"type:varchar(128);not null"`
	Subtitle string `json:"subtitle" gorm:"type:varchar(255);default:''"`

	// Display money amount (follow existing code style: float64 for money)
	PriceAmount float64 `json:"price_amount" gorm:"type:decimal(10,6);not null;default:0"`
	Currency    string  `json:"currency" gorm:"type:varchar(8);not null;default:'USD'"`

	DurationUnit  string `json:"duration_unit" gorm:"type:varchar(16);not null;default:'month'"`
	DurationValue int    `json:"duration_value" gorm:"type:int;not null;default:1"`
	CustomSeconds int64  `json:"custom_seconds" gorm:"type:bigint;not null;default:0"`

	Enabled   bool `json:"enabled" gorm:"default:true"`
	SortOrder int  `json:"sort_order" gorm:"type:int;default:0"`

	StripePriceId  string `json:"stripe_price_id" gorm:"type:varchar(128);default:''"`
	CreemProductId string `json:"creem_product_id" gorm:"type:varchar(128);default:''"`

	// Max purchases per user (0 = unlimited)
	MaxPurchasePerUser int `json:"max_purchase_per_user" gorm:"type:int;default:0"`

	// Upgrade user group after purchase (empty = no change)
	UpgradeGroup string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`

	// Total quota (amount in quota units, 0 = unlimited)
	TotalAmount        int64  `json:"total_amount" gorm:"type:bigint;not null;default:0"`
	ModelRestrictMode  string `json:"model_restrict_mode" gorm:"type:varchar(16);default:''"`
	ModelRestrictGroup string `json:"model_restrict_group" gorm:"type:varchar(64);default:''"`
	AllowedModels      string `json:"allowed_models" gorm:"type:text"`
	DailyQuotaLimit    int64  `json:"daily_quota_limit" gorm:"type:bigint;not null;default:0"`
	WeeklyQuotaLimit   int64  `json:"weekly_quota_limit" gorm:"type:bigint;not null;default:0"`
	MonthlyQuotaLimit  int64  `json:"monthly_quota_limit" gorm:"type:bigint;not null;default:0"`

	// Quota reset period for plan
	QuotaResetPeriod        string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (p *SubscriptionPlan) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *SubscriptionPlan) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// Subscription order (payment -> webhook -> create UserSubscription)
type SubscriptionOrder struct {
	Id     int     `json:"id"`
	UserId int     `json:"user_id" gorm:"index"`
	PlanId int     `json:"plan_id" gorm:"index"`
	Money  float64 `json:"money"`

	TradeNo         string `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider string `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	Status          string `json:"status"`
	CreateTime      int64  `json:"create_time"`
	CompleteTime    int64  `json:"complete_time"`

	ProviderPayload string `json:"provider_payload" gorm:"type:text"`
}

func (o *SubscriptionOrder) Insert() error {
	if o.CreateTime == 0 {
		o.CreateTime = common.GetTimestamp()
	}
	return DB.Create(o).Error
}

func (o *SubscriptionOrder) Update() error {
	return DB.Save(o).Error
}

func GetSubscriptionOrderByTradeNo(tradeNo string) *SubscriptionOrder {
	if tradeNo == "" {
		return nil
	}
	var order SubscriptionOrder
	if err := DB.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		return nil
	}
	return &order
}

// User subscription instance
type UserSubscription struct {
	Id     int `json:"id"`
	UserId int `json:"user_id" gorm:"index;index:idx_user_sub_active,priority:1"`
	PlanId int `json:"plan_id" gorm:"index"`

	AmountTotal int64 `json:"amount_total" gorm:"type:bigint;not null;default:0"`
	AmountUsed  int64 `json:"amount_used" gorm:"type:bigint;not null;default:0"`

	StartTime int64  `json:"start_time" gorm:"bigint"`
	EndTime   int64  `json:"end_time" gorm:"bigint;index;index:idx_user_sub_active,priority:3"`
	Status    string `json:"status" gorm:"type:varchar(32);index;index:idx_user_sub_active,priority:2"` // active/expired/cancelled

	Source string `json:"source" gorm:"type:varchar(32);default:'order'"` // order/admin

	LastResetTime int64 `json:"last_reset_time" gorm:"type:bigint;default:0"`
	NextResetTime int64 `json:"next_reset_time" gorm:"type:bigint;default:0;index"`

	UpgradeGroup       string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`
	PrevUserGroup      string `json:"prev_user_group" gorm:"type:varchar(64);default:''"`
	DailyWindowStart   int64  `json:"daily_window_start" gorm:"type:bigint;default:0"`
	DailyWindowUsed    int64  `json:"daily_window_used" gorm:"type:bigint;not null;default:0"`
	WeeklyWindowStart  int64  `json:"weekly_window_start" gorm:"type:bigint;default:0"`
	WeeklyWindowUsed   int64  `json:"weekly_window_used" gorm:"type:bigint;not null;default:0"`
	MonthlyWindowStart int64  `json:"monthly_window_start" gorm:"type:bigint;default:0"`
	MonthlyWindowUsed  int64  `json:"monthly_window_used" gorm:"type:bigint;not null;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (s *UserSubscription) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (s *UserSubscription) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = common.GetTimestamp()
	return nil
}

type SubscriptionSummary struct {
	Subscription *UserSubscription `json:"subscription"`
	Plan         *SubscriptionPlan `json:"plan,omitempty"`
}

type defaultSubscriptionPlanAssignment struct {
	PlanId int `json:"plan_id"`
}

const (
	subscriptionDailyWindowSeconds   int64 = 24 * 3600
	subscriptionWeeklyWindowSeconds  int64 = 7 * 24 * 3600
	subscriptionMonthlyWindowSeconds int64 = 30 * 24 * 3600
)

func NormalizeSubscriptionModelRestrictMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "group", "custom":
		return strings.TrimSpace(mode)
	default:
		return ""
	}
}

func parseAllowedModels(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	var allowed []string
	if err := common.UnmarshalJsonStr(trimmed, &allowed); err != nil {
		return nil, err
	}
	normalized := make([]string, 0, len(allowed))
	seen := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		model := strings.TrimSpace(item)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		normalized = append(normalized, model)
	}
	return normalized, nil
}

func NormalizeAllowedModelsJSON(raw string) (string, error) {
	allowed, err := parseAllowedModels(raw)
	if err != nil {
		return "", err
	}
	data, err := common.Marshal(allowed)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseDefaultSubscriptionPlanAssignments(raw string) ([]defaultSubscriptionPlanAssignment, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "[]"
	}
	var assignments []defaultSubscriptionPlanAssignment
	if err := common.UnmarshalJsonStr(trimmed, &assignments); err != nil {
		return nil, err
	}
	normalized := make([]defaultSubscriptionPlanAssignment, 0, len(assignments))
	seen := make(map[int]struct{}, len(assignments))
	for _, assignment := range assignments {
		if assignment.PlanId <= 0 {
			return nil, fmt.Errorf("invalid plan_id: %d", assignment.PlanId)
		}
		if _, ok := seen[assignment.PlanId]; ok {
			return nil, fmt.Errorf("duplicate plan_id: %d", assignment.PlanId)
		}
		seen[assignment.PlanId] = struct{}{}
		normalized = append(normalized, defaultSubscriptionPlanAssignment{PlanId: assignment.PlanId})
	}
	return normalized, nil
}

func NormalizeDefaultSubscriptionPlansJSON(raw string) (string, error) {
	assignments, err := parseDefaultSubscriptionPlanAssignments(raw)
	if err != nil {
		return "", err
	}
	for _, assignment := range assignments {
		plan, err := GetSubscriptionPlanById(assignment.PlanId)
		if err != nil {
			return "", fmt.Errorf("plan_id=%d does not exist", assignment.PlanId)
		}
		if !plan.Enabled {
			return "", fmt.Errorf("plan_id=%d is disabled", assignment.PlanId)
		}
	}
	data, err := common.Marshal(assignments)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func getSubscriptionUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup == "" {
		return groupsCopy
	}
	if specialSettings, ok := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup); ok {
		for specialGroup, desc := range specialSettings {
			if strings.HasPrefix(specialGroup, "-:") {
				delete(groupsCopy, strings.TrimPrefix(specialGroup, "-:"))
				continue
			}
			if strings.HasPrefix(specialGroup, "+:") {
				groupsCopy[strings.TrimPrefix(specialGroup, "+:")] = desc
				continue
			}
			groupsCopy[specialGroup] = desc
		}
	}
	if _, ok := groupsCopy[userGroup]; !ok {
		groupsCopy[userGroup] = "用户分组"
	}
	return groupsCopy
}

func resolveSubscriptionModelRestrictGroup(plan *SubscriptionPlan, userGroup string) string {
	if plan == nil {
		return ""
	}
	if restrictGroup := strings.TrimSpace(plan.ModelRestrictGroup); restrictGroup != "" {
		return restrictGroup
	}
	if upgradeGroup := strings.TrimSpace(plan.UpgradeGroup); upgradeGroup != "" {
		return upgradeGroup
	}
	return strings.TrimSpace(userGroup)
}

func isModelEnabledForGroup(modelGroups []string, targetGroup string) bool {
	targetGroup = strings.TrimSpace(targetGroup)
	if targetGroup == "" || len(modelGroups) == 0 {
		return false
	}
	for _, group := range modelGroups {
		if group == "all" {
			return true
		}
		if strings.TrimSpace(group) == targetGroup {
			return true
		}
	}
	return false
}

func isSubscriptionGroupModelAllowed(plan *SubscriptionPlan, modelName string, userGroup string) bool {
	if plan == nil {
		return false
	}
	targetGroup := resolveSubscriptionModelRestrictGroup(plan, userGroup)
	if targetGroup == "" {
		return false
	}
	modelGroups := GetModelEnableGroups(strings.TrimSpace(modelName))
	return isModelEnabledForGroup(modelGroups, targetGroup)
}

func isSubscriptionModelAllowed(plan *SubscriptionPlan, modelName string, userGroup string) bool {
	if plan == nil {
		return false
	}
	switch NormalizeSubscriptionModelRestrictMode(plan.ModelRestrictMode) {
	case "":
		return true
	case "group":
		return isSubscriptionGroupModelAllowed(plan, modelName, userGroup)
	case "custom":
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			return false
		}
		allowed, err := parseAllowedModels(plan.AllowedModels)
		if err != nil || len(allowed) == 0 {
			return false
		}
		for _, pattern := range allowed {
			if strings.HasSuffix(pattern, "*") {
				prefix := strings.TrimSuffix(pattern, "*")
				if prefix == "" || strings.HasPrefix(modelName, prefix) {
					return true
				}
				continue
			}
			if modelName == pattern {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func maybeResetSubscriptionQuotaWindow(start *int64, used *int64, limit int64, now int64, windowSeconds int64) bool {
	if start == nil || used == nil || limit <= 0 {
		return false
	}
	originalStart := *start
	originalUsed := *used
	if *used < 0 {
		*used = 0
	}
	if *start <= 0 {
		*start = now
	} else if now-*start >= windowSeconds {
		*start = now
		*used = 0
	}
	return *start != originalStart || *used != originalUsed
}

func prepareSubscriptionQuotaWindows(sub *UserSubscription, plan *SubscriptionPlan, now int64) bool {
	if sub == nil || plan == nil {
		return false
	}
	changed := false
	if maybeResetSubscriptionQuotaWindow(&sub.DailyWindowStart, &sub.DailyWindowUsed, plan.DailyQuotaLimit, now, subscriptionDailyWindowSeconds) {
		changed = true
	}
	if maybeResetSubscriptionQuotaWindow(&sub.WeeklyWindowStart, &sub.WeeklyWindowUsed, plan.WeeklyQuotaLimit, now, subscriptionWeeklyWindowSeconds) {
		changed = true
	}
	if maybeResetSubscriptionQuotaWindow(&sub.MonthlyWindowStart, &sub.MonthlyWindowUsed, plan.MonthlyQuotaLimit, now, subscriptionMonthlyWindowSeconds) {
		changed = true
	}
	return changed
}

func exceedsSubscriptionQuotaWindows(sub *UserSubscription, plan *SubscriptionPlan, amount int64) bool {
	if sub == nil || plan == nil {
		return true
	}
	if plan.DailyQuotaLimit > 0 && sub.DailyWindowUsed+amount > plan.DailyQuotaLimit {
		return true
	}
	if plan.WeeklyQuotaLimit > 0 && sub.WeeklyWindowUsed+amount > plan.WeeklyQuotaLimit {
		return true
	}
	if plan.MonthlyQuotaLimit > 0 && sub.MonthlyWindowUsed+amount > plan.MonthlyQuotaLimit {
		return true
	}
	return false
}

func applySubscriptionQuotaWindowDelta(sub *UserSubscription, plan *SubscriptionPlan, delta int64) {
	if sub == nil || plan == nil || delta == 0 {
		return
	}
	if plan.DailyQuotaLimit > 0 {
		sub.DailyWindowUsed += delta
		if sub.DailyWindowUsed < 0 {
			sub.DailyWindowUsed = 0
		}
	}
	if plan.WeeklyQuotaLimit > 0 {
		sub.WeeklyWindowUsed += delta
		if sub.WeeklyWindowUsed < 0 {
			sub.WeeklyWindowUsed = 0
		}
	}
	if plan.MonthlyQuotaLimit > 0 {
		sub.MonthlyWindowUsed += delta
		if sub.MonthlyWindowUsed < 0 {
			sub.MonthlyWindowUsed = 0
		}
	}
}

func calcPlanEndTime(start time.Time, plan *SubscriptionPlan) (int64, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != SubscriptionDurationCustom {
		return 0, errors.New("duration_value must be > 0")
	}
	switch plan.DurationUnit {
	case SubscriptionDurationYear:
		return start.AddDate(plan.DurationValue, 0, 0).Unix(), nil
	case SubscriptionDurationMonth:
		return start.AddDate(0, plan.DurationValue, 0).Unix(), nil
	case SubscriptionDurationDay:
		return start.Add(time.Duration(plan.DurationValue) * 24 * time.Hour).Unix(), nil
	case SubscriptionDurationHour:
		return start.Add(time.Duration(plan.DurationValue) * time.Hour).Unix(), nil
	case SubscriptionDurationCustom:
		if plan.CustomSeconds <= 0 {
			return 0, errors.New("custom_seconds must be > 0")
		}
		return start.Add(time.Duration(plan.CustomSeconds) * time.Second).Unix(), nil
	default:
		return 0, fmt.Errorf("invalid duration_unit: %s", plan.DurationUnit)
	}
}

func NormalizeResetPeriod(period string) string {
	switch strings.TrimSpace(period) {
	case SubscriptionResetDaily, SubscriptionResetWeekly, SubscriptionResetMonthly, SubscriptionResetCustom:
		return strings.TrimSpace(period)
	default:
		return SubscriptionResetNever
	}
}

func calcNextResetTime(base time.Time, plan *SubscriptionPlan, endUnix int64) int64 {
	if plan == nil {
		return 0
	}
	period := NormalizeResetPeriod(plan.QuotaResetPeriod)
	if period == SubscriptionResetNever {
		return 0
	}
	var next time.Time
	switch period {
	case SubscriptionResetDaily:
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, 1)
	case SubscriptionResetWeekly:
		// Align to next Monday 00:00
		weekday := int(base.Weekday()) // Sunday=0
		// Convert to Monday=1..Sunday=7
		if weekday == 0 {
			weekday = 7
		}
		daysUntil := 8 - weekday
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, daysUntil)
	case SubscriptionResetMonthly:
		// Align to first day of next month 00:00
		next = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location()).
			AddDate(0, 1, 0)
	case SubscriptionResetCustom:
		if plan.QuotaResetCustomSeconds <= 0 {
			return 0
		}
		next = base.Add(time.Duration(plan.QuotaResetCustomSeconds) * time.Second)
	default:
		return 0
	}
	if endUnix > 0 && next.Unix() > endUnix {
		return 0
	}
	return next.Unix()
}

func GetSubscriptionPlanById(id int) (*SubscriptionPlan, error) {
	return getSubscriptionPlanByIdTx(nil, id)
}

func getSubscriptionPlanByIdTx(tx *gorm.DB, id int) (*SubscriptionPlan, error) {
	if id <= 0 {
		return nil, errors.New("invalid plan id")
	}
	key := subscriptionPlanCacheKey(id)
	if key != "" {
		if cached, found, err := getSubscriptionPlanCache().Get(key); err == nil && found {
			return &cached, nil
		}
	}
	var plan SubscriptionPlan
	query := DB
	if tx != nil {
		query = tx
	}
	if err := query.Where("id = ?", id).First(&plan).Error; err != nil {
		return nil, err
	}
	_ = getSubscriptionPlanCache().SetWithTTL(key, plan, subscriptionPlanCacheTTL())
	return &plan, nil
}

func CountUserSubscriptionsByPlan(userId int, planId int) (int64, error) {
	if userId <= 0 || planId <= 0 {
		return 0, errors.New("invalid userId or planId")
	}
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", userId, planId).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func getUserGroupByIdTx(tx *gorm.DB, userId int) (string, error) {
	if userId <= 0 {
		return "", errors.New("invalid userId")
	}
	if tx == nil {
		tx = DB
	}
	groupCol := commonGroupCol
	if strings.TrimSpace(groupCol) == "" {
		if common.UsingPostgreSQL {
			groupCol = `"group"`
		} else {
			groupCol = "`group`"
		}
	}
	var group string
	if err := tx.Model(&User{}).Where("id = ?", userId).Select(groupCol).Find(&group).Error; err != nil {
		return "", err
	}
	return group, nil
}

func downgradeUserGroupForSubscriptionTx(tx *gorm.DB, sub *UserSubscription, now int64) (string, error) {
	if tx == nil || sub == nil {
		return "", errors.New("invalid downgrade args")
	}
	upgradeGroup := strings.TrimSpace(sub.UpgradeGroup)
	if upgradeGroup == "" {
		return "", nil
	}
	currentGroup, err := getUserGroupByIdTx(tx, sub.UserId)
	if err != nil {
		return "", err
	}
	if currentGroup != upgradeGroup {
		return "", nil
	}
	var activeSub UserSubscription
	activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND id <> ? AND upgrade_group <> ''",
		sub.UserId, "active", now, sub.Id).
		Order("end_time desc, id desc").
		Limit(1).
		Find(&activeSub)
	if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
		return "", nil
	}
	prevGroup := strings.TrimSpace(sub.PrevUserGroup)
	if prevGroup == "" || prevGroup == currentGroup {
		return "", nil
	}
	if err := tx.Model(&User{}).Where("id = ?", sub.UserId).
		Update("group", prevGroup).Error; err != nil {
		return "", err
	}
	return prevGroup, nil
}

func CreateUserSubscriptionFromPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string) (*UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if source != "auto" && plan.MaxPurchasePerUser > 0 {
		var count int64
		if err := tx.Model(&UserSubscription{}).
			Where("user_id = ? AND plan_id = ?", userId, plan.Id).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return nil, errors.New("已达到该套餐购买上限")
		}
	}
	nowUnix := GetDBTimestampTx(tx)
	now := time.Unix(nowUnix, 0)
	endUnix, err := calcPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}
	resetBase := now
	nextReset := calcNextResetTime(resetBase, plan, endUnix)
	lastReset := int64(0)
	if nextReset > 0 {
		lastReset = now.Unix()
	}
	currentGroup, err := getUserGroupByIdTx(tx, userId)
	if err != nil {
		return nil, err
	}
	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	prevGroup := currentGroup
	if upgradeGroup != "" {
		if currentGroup != upgradeGroup {
			prevGroup = currentGroup
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", upgradeGroup).Error; err != nil {
				return nil, err
			}
		}
	}
	sub := &UserSubscription{
		UserId:        userId,
		PlanId:        plan.Id,
		AmountTotal:   plan.TotalAmount,
		AmountUsed:    0,
		StartTime:     now.Unix(),
		EndTime:       endUnix,
		Status:        "active",
		Source:        source,
		LastResetTime: lastReset,
		NextResetTime: nextReset,
		UpgradeGroup:  upgradeGroup,
		PrevUserGroup: prevGroup,
		CreatedAt:     nowUnix,
		UpdatedAt:     nowUnix,
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

func calcSubscriptionPlanRequiredQuota(plan *SubscriptionPlan) (int, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	if plan.PriceAmount <= 0 {
		return 0, nil
	}
	requiredQuota := decimal.NewFromFloat(plan.PriceAmount).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Round(0)
	if requiredQuota.IsNegative() {
		return 0, errors.New("invalid required quota")
	}
	return int(requiredQuota.IntPart()), nil
}

func GetSubscriptionPlanRequiredQuota(plan *SubscriptionPlan) (int, error) {
	return calcSubscriptionPlanRequiredQuota(plan)
}

func WalletPurchaseSubscription(userId int, planId int) (*SubscriptionOrder, error) {
	if userId <= 0 || planId <= 0 {
		return nil, errors.New("invalid userId or planId")
	}
	now := common.GetTimestamp()
	tradeNo := fmt.Sprintf("sub-wallet-%d-%d-%s", userId, time.Now().UnixMilli(), common.GetRandomString(6))
	var order *SubscriptionOrder
	remainingQuota := -1
	requiredQuota := 0
	upgradeGroup := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		plan, err := getSubscriptionPlanByIdTx(tx, planId)
		if err != nil {
			return err
		}
		if !plan.Enabled {
			return errors.New("套餐未启用")
		}
		requiredQuota, err = calcSubscriptionPlanRequiredQuota(plan)
		if err != nil {
			return err
		}
		if plan.MaxPurchasePerUser > 0 {
			var count int64
			if err := tx.Model(&UserSubscription{}).
				Where("user_id = ? AND plan_id = ?", userId, plan.Id).
				Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(plan.MaxPurchasePerUser) {
				return errors.New("已达到该套餐购买上限")
			}
		}
		if requiredQuota > 0 {
			var user User
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Select("id", "quota").
				Where("id = ?", userId).
				First(&user).Error; err != nil {
				return err
			}
			if user.Quota < requiredQuota {
				return ErrSubscriptionWalletQuotaNotEnough
			}
			remainingQuota = user.Quota - requiredQuota
			if err := tx.Model(&User{}).
				Where("id = ?", userId).
				Update("quota", remainingQuota).Error; err != nil {
				return err
			}
		}
		if _, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "wallet"); err != nil {
			return err
		}
		upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
		order = &SubscriptionOrder{
			UserId:        userId,
			PlanId:        plan.Id,
			Money:         plan.PriceAmount,
			TradeNo:       tradeNo,
			PaymentMethod: "wallet",
			Status:        common.TopUpStatusSuccess,
			CreateTime:    now,
			CompleteTime:  now,
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		return upsertSubscriptionTopUpTx(tx, order)
	})
	if err != nil {
		return nil, err
	}
	if requiredQuota > 0 && remainingQuota >= 0 {
		if err := updateUserQuotaCache(userId, remainingQuota); err != nil {
			common.SysLog(fmt.Sprintf("failed to update user quota cache after wallet subscription purchase: user_id=%d, error=%v", userId, err))
		}
	}
	if upgradeGroup != "" {
		if err := UpdateUserGroupCache(userId, upgradeGroup); err != nil {
			common.SysLog(fmt.Sprintf("failed to update user group cache after wallet subscription purchase: user_id=%d, group=%s, error=%v", userId, upgradeGroup, err))
		}
	}
	return order, nil
}

func AssignDefaultSubscriptionsToNewUser(userId int) {
	if userId <= 0 {
		return
	}
	assignments, err := parseDefaultSubscriptionPlanAssignments(common.DefaultSubscriptionPlans)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to parse DefaultSubscriptionPlans: user_id=%d, error=%v", userId, err))
		return
	}
	if len(assignments) == 0 {
		return
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Select("id").
			Where("id = ?", userId).
			First(&user).Error; err != nil {
			return err
		}
		for _, assignment := range assignments {
			var autoCount int64
			if err := tx.Model(&UserSubscription{}).
				Where("user_id = ? AND plan_id = ? AND source = ?", userId, assignment.PlanId, "auto").
				Count(&autoCount).Error; err != nil {
				common.SysLog(fmt.Sprintf("failed to check auto subscription grant: user_id=%d, plan_id=%d, error=%v", userId, assignment.PlanId, err))
				continue
			}
			if autoCount > 0 {
				continue
			}
			plan, err := getSubscriptionPlanByIdTx(tx, assignment.PlanId)
			if err != nil {
				common.SysLog(fmt.Sprintf("failed to load auto subscription plan: user_id=%d, plan_id=%d, error=%v", userId, assignment.PlanId, err))
				continue
			}
			if !plan.Enabled {
				common.SysLog(fmt.Sprintf("skip disabled auto subscription plan: user_id=%d, plan_id=%d", userId, assignment.PlanId))
				continue
			}
			if _, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "auto"); err != nil {
				common.SysLog(fmt.Sprintf("failed to auto assign subscription: user_id=%d, plan_id=%d, error=%v", userId, assignment.PlanId, err))
			}
		}
		return nil
	})
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to finalize auto subscriptions: user_id=%d, error=%v", userId, err))
		return
	}
	currentGroup, err := getUserGroupByIdTx(nil, userId)
	if err == nil && currentGroup != "" {
		if err := UpdateUserGroupCache(userId, currentGroup); err != nil {
			common.SysLog(fmt.Sprintf("failed to update user group cache after auto subscriptions: user_id=%d, group=%s, error=%v", userId, currentGroup, err))
		}
	}
}

// Complete a subscription order (idempotent). Creates a UserSubscription snapshot from the plan.
// expectedPaymentProvider guards against cross-gateway callback attacks (empty skips the check).
// actualPaymentMethod updates the order's PaymentMethod to reflect the real payment type used (empty skips update).
func CompleteSubscriptionOrder(tradeNo string, providerPayload string, expectedPaymentProvider string, actualPaymentMethod string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	var logUserId int
	var logPlanTitle string
	var logMoney float64
	var logPaymentMethod string
	var upgradeGroup string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status == common.TopUpStatusSuccess {
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		plan, err := GetSubscriptionPlanById(order.PlanId)
		if err != nil {
			return err
		}
		if !plan.Enabled {
			// still allow completion for already purchased orders
		}
		upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
		_, err = CreateUserSubscriptionFromPlanTx(tx, order.UserId, plan, "order")
		if err != nil {
			return err
		}
		if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
			return err
		}
		order.Status = common.TopUpStatusSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if actualPaymentMethod != "" && order.PaymentMethod != actualPaymentMethod {
			order.PaymentMethod = actualPaymentMethod
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		logUserId = order.UserId
		logPlanTitle = plan.Title
		logMoney = order.Money
		logPaymentMethod = order.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}
	if upgradeGroup != "" && logUserId > 0 {
		_ = UpdateUserGroupCache(logUserId, upgradeGroup)
	}
	if logUserId > 0 {
		msg := fmt.Sprintf("订阅购买成功，套餐: %s，支付金额: %.2f，支付方式: %s", logPlanTitle, logMoney, logPaymentMethod)
		RecordLog(logUserId, LogTypeTopup, msg)
	}
	return nil
}

func upsertSubscriptionTopUpTx(tx *gorm.DB, order *SubscriptionOrder) error {
	if tx == nil || order == nil {
		return errors.New("invalid subscription order")
	}
	now := common.GetTimestamp()
	var topup TopUp
	if err := tx.Where("trade_no = ?", order.TradeNo).First(&topup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			topup = TopUp{
				UserId:        order.UserId,
				Amount:        0,
				Money:         order.Money,
				TradeNo:       order.TradeNo,
				PaymentMethod: order.PaymentMethod,
				CreateTime:    order.CreateTime,
				CompleteTime:  now,
				Status:        common.TopUpStatusSuccess,
			}
			return tx.Create(&topup).Error
		}
		return err
	}
	topup.Money = order.Money
	if topup.PaymentMethod == "" {
		topup.PaymentMethod = order.PaymentMethod
	} else if topup.PaymentMethod != order.PaymentMethod {
		return ErrPaymentMethodMismatch
	}
	if topup.CreateTime == 0 {
		topup.CreateTime = order.CreateTime
	}
	topup.CompleteTime = now
	topup.Status = common.TopUpStatusSuccess
	return tx.Save(&topup).Error
}

func ExpireSubscriptionOrder(tradeNo string, expectedPaymentProvider string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
		order.Status = common.TopUpStatusExpired
		order.CompleteTime = common.GetTimestamp()
		return tx.Save(&order).Error
	})
}

// Admin bind (no payment). Creates a UserSubscription from a plan.
func AdminBindSubscription(userId int, planId int, sourceNote string) (string, error) {
	if userId <= 0 || planId <= 0 {
		return "", errors.New("invalid userId or planId")
	}
	plan, err := GetSubscriptionPlanById(planId)
	if err != nil {
		return "", err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "admin")
		return err
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(plan.UpgradeGroup) != "" {
		_ = UpdateUserGroupCache(userId, plan.UpgradeGroup)
		return fmt.Sprintf("用户分组将升级到 %s", plan.UpgradeGroup), nil
	}
	return "", nil
}

// GetAllActiveUserSubscriptions returns all active subscriptions for a user.
func GetAllActiveUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var subs []UserSubscription
	err := DB.Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// HasActiveUserSubscription returns whether the user has any active subscription.
// This is a lightweight existence check to avoid heavy pre-consume transactions.
func HasActiveUserSubscription(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetAllUserSubscriptions returns all subscriptions (active and expired) for a user.
func GetAllUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var subs []UserSubscription
	err := DB.Where("user_id = ?", userId).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

func buildSubscriptionSummaries(subs []UserSubscription) []SubscriptionSummary {
	if len(subs) == 0 {
		return []SubscriptionSummary{}
	}
	result := make([]SubscriptionSummary, 0, len(subs))
	for _, sub := range subs {
		subCopy := sub
		var planCopy *SubscriptionPlan
		if plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId); err == nil && plan != nil {
			planValue := *plan
			planCopy = &planValue
		}
		result = append(result, SubscriptionSummary{
			Subscription: &subCopy,
			Plan:         planCopy,
		})
	}
	return result
}

// AdminInvalidateUserSubscription marks a user subscription as cancelled and ends it immediately.
func AdminInvalidateUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		if err := tx.Model(&sub).Updates(map[string]interface{}{
			"status":     "cancelled",
			"end_time":   now,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		if err := tx.Where("id = ?", userSubscriptionId).Delete(&UserSubscription{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

type SubscriptionPreConsumeResult struct {
	UserSubscriptionId int
	PreConsumed        int64
	AmountTotal        int64
	AmountUsedBefore   int64
	AmountUsedAfter    int64
}

// ExpireDueSubscriptions marks expired subscriptions and handles group downgrade.
func ExpireDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("status = ? AND end_time > 0 AND end_time <= ?", "active", now).
		Order("end_time asc, id asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	expiredCount := 0
	userIds := make(map[int]struct{}, len(subs))
	for _, sub := range subs {
		if sub.UserId > 0 {
			userIds[sub.UserId] = struct{}{}
		}
	}
	for userId := range userIds {
		cacheGroup := ""
		err := DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&UserSubscription{}).
				Where("user_id = ? AND status = ? AND end_time > 0 AND end_time <= ?", userId, "active", now).
				Updates(map[string]interface{}{
					"status":     "expired",
					"updated_at": common.GetTimestamp(),
				})
			if res.Error != nil {
				return res.Error
			}
			expiredCount += int(res.RowsAffected)

			// If there's an active upgraded subscription, keep current group.
			var activeSub UserSubscription
			activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND upgrade_group <> ''",
				userId, "active", now).
				Order("end_time desc, id desc").
				Limit(1).
				Find(&activeSub)
			if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
				return nil
			}

			// No active upgraded subscription, downgrade to previous group if needed.
			var lastExpired UserSubscription
			expiredQuery := tx.Where("user_id = ? AND status = ? AND upgrade_group <> ''",
				userId, "expired").
				Order("end_time desc, id desc").
				Limit(1).
				Find(&lastExpired)
			if expiredQuery.Error != nil || expiredQuery.RowsAffected == 0 {
				return nil
			}
			upgradeGroup := strings.TrimSpace(lastExpired.UpgradeGroup)
			prevGroup := strings.TrimSpace(lastExpired.PrevUserGroup)
			if upgradeGroup == "" || prevGroup == "" {
				return nil
			}
			currentGroup, err := getUserGroupByIdTx(tx, userId)
			if err != nil {
				return err
			}
			if currentGroup != upgradeGroup || currentGroup == prevGroup {
				return nil
			}
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", prevGroup).Error; err != nil {
				return err
			}
			cacheGroup = prevGroup
			return nil
		})
		if err != nil {
			return expiredCount, err
		}
		if cacheGroup != "" {
			_ = UpdateUserGroupCache(userId, cacheGroup)
		}
	}
	return expiredCount, nil
}

// SubscriptionPreConsumeRecord stores idempotent pre-consume operations per request.
type SubscriptionPreConsumeRecord struct {
	Id                 int    `json:"id"`
	RequestId          string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId             int    `json:"user_id" gorm:"index"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
	PreConsumed        int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	Status             string `json:"status" gorm:"type:varchar(32);index"` // consumed/refunded
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint;index"`
}

func (r *SubscriptionPreConsumeRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *SubscriptionPreConsumeRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func fillSubscriptionPreConsumeResultTx(tx *gorm.DB, result *SubscriptionPreConsumeResult, record *SubscriptionPreConsumeRecord) error {
	if tx == nil || result == nil || record == nil {
		return errors.New("invalid pre-consume result args")
	}
	var sub UserSubscription
	if err := tx.Where("id = ?", record.UserSubscriptionId).First(&sub).Error; err != nil {
		return err
	}
	result.UserSubscriptionId = sub.Id
	result.PreConsumed = record.PreConsumed
	result.AmountTotal = sub.AmountTotal
	result.AmountUsedBefore = sub.AmountUsed
	result.AmountUsedAfter = sub.AmountUsed
	return nil
}

func maybeResetUserSubscriptionWithPlanTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, now int64) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid reset args")
	}
	if sub.NextResetTime > 0 && sub.NextResetTime > now {
		return nil
	}
	if NormalizeResetPeriod(plan.QuotaResetPeriod) == SubscriptionResetNever {
		return nil
	}
	baseUnix := sub.LastResetTime
	if baseUnix <= 0 {
		baseUnix = sub.StartTime
	}
	base := time.Unix(baseUnix, 0)
	next := calcNextResetTime(base, plan, sub.EndTime)
	advanced := false
	for next > 0 && next <= now {
		advanced = true
		base = time.Unix(next, 0)
		next = calcNextResetTime(base, plan, sub.EndTime)
	}
	if !advanced {
		if sub.NextResetTime == 0 && next > 0 {
			sub.NextResetTime = next
			sub.LastResetTime = base.Unix()
			return tx.Save(sub).Error
		}
		return nil
	}
	sub.AmountUsed = 0
	sub.LastResetTime = base.Unix()
	sub.NextResetTime = next
	return tx.Save(sub).Error
}

// PreConsumeUserSubscription pre-consumes from any active subscription total quota.
func PreConsumeUserSubscription(requestId string, userId int, modelName string, quotaType int, amount int64) (*SubscriptionPreConsumeResult, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	now := GetDBTimestamp()

	returnValue := &SubscriptionPreConsumeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing SubscriptionPreConsumeRecord
		query := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if existing.Status == "refunded" {
				return errors.New("subscription pre-consume already refunded")
			}
			return fillSubscriptionPreConsumeResultTx(tx, returnValue, &existing)
		}

		var subs []UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
			Order("end_time asc, id asc").
			Find(&subs).Error; err != nil {
			return errors.New("no active subscription")
		}
		if len(subs) == 0 {
			return errors.New("no active subscription")
		}
		currentUserGroup, err := getUserGroupByIdTx(tx, userId)
		if err != nil {
			return err
		}
		type preConsumeCandidate struct {
			sub           UserSubscription
			plan          *SubscriptionPlan
			planSortOrder int
		}

		candidates := make([]preConsumeCandidate, 0, len(subs))
		for _, candidate := range subs {
			sub := candidate
			var plan *SubscriptionPlan
			planSortOrder := 0
			if sub.PlanId > 0 {
				loadedPlan, planErr := getSubscriptionPlanByIdTx(tx, sub.PlanId)
				if planErr != nil {
					if errors.Is(planErr, gorm.ErrRecordNotFound) {
						common.SysLog(fmt.Sprintf("skip subscription pre-consume due missing plan: sub_id=%d, plan_id=%d", sub.Id, sub.PlanId))
						continue
					}
					return planErr
				}
				if loadedPlan == nil {
					common.SysLog(fmt.Sprintf("skip subscription pre-consume due nil plan: sub_id=%d, plan_id=%d", sub.Id, sub.PlanId))
					continue
				}
				plan = loadedPlan
				planSortOrder = plan.SortOrder
			}
			candidates = append(candidates, preConsumeCandidate{
				sub:           sub,
				plan:          plan,
				planSortOrder: planSortOrder,
			})
		}

		sort.SliceStable(candidates, func(i, j int) bool {
			left := candidates[i]
			right := candidates[j]
			if left.planSortOrder != right.planSortOrder {
				return left.planSortOrder > right.planSortOrder
			}
			if left.sub.EndTime != right.sub.EndTime {
				return left.sub.EndTime < right.sub.EndTime
			}
			return left.sub.Id < right.sub.Id
		})

		for _, candidate := range candidates {
			sub := candidate.sub
			plan := candidate.plan
			subscriptionUserGroup := strings.TrimSpace(sub.PrevUserGroup)
			if subscriptionUserGroup == "" {
				subscriptionUserGroup = currentUserGroup
			}
			if plan != nil {
				if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
					return err
				}
				if !isSubscriptionModelAllowed(plan, modelName, subscriptionUserGroup) {
					continue
				}
				if prepareSubscriptionQuotaWindows(&sub, plan, now) {
					if err := tx.Save(&sub).Error; err != nil {
						return err
					}
				}
				if exceedsSubscriptionQuotaWindows(&sub, plan, amount) {
					continue
				}
			}
			usedBefore := sub.AmountUsed
			if sub.AmountTotal > 0 {
				remain := sub.AmountTotal - usedBefore
				if remain < amount {
					continue
				}
			}
			record := &SubscriptionPreConsumeRecord{
				RequestId:          requestId,
				UserId:             userId,
				UserSubscriptionId: sub.Id,
				PreConsumed:        amount,
				Status:             "consumed",
			}
			if err := tx.Create(record).Error; err != nil {
				var dup SubscriptionPreConsumeRecord
				if err2 := tx.Where("request_id = ?", requestId).First(&dup).Error; err2 == nil {
					if dup.Status == "refunded" {
						return errors.New("subscription pre-consume already refunded")
					}
					return fillSubscriptionPreConsumeResultTx(tx, returnValue, &dup)
				}
				return err
			}
			sub.AmountUsed += amount
			if plan != nil {
				applySubscriptionQuotaWindowDelta(&sub, plan, amount)
			}
			if err := tx.Save(&sub).Error; err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = amount
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = usedBefore
			returnValue.AmountUsedAfter = sub.AmountUsed
			return nil
		}
		return fmt.Errorf("subscription quota insufficient, need=%d", amount)
	})
	if err != nil {
		return nil, err
	}
	return returnValue, nil
}

// RefundSubscriptionPreConsume is idempotent and refunds pre-consumed subscription quota by requestId.
func RefundSubscriptionPreConsume(requestId string) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return nil
		}
		if record.PreConsumed <= 0 {
			record.Status = "refunded"
			return tx.Save(&record).Error
		}
		if err := PostConsumeUserSubscriptionDelta(record.UserSubscriptionId, -record.PreConsumed); err != nil {
			return err
		}
		record.Status = "refunded"
		return tx.Save(&record).Error
	})
}

// ResetDueSubscriptions resets subscriptions whose next_reset_time has passed.
func ResetDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("next_reset_time > 0 AND next_reset_time <= ? AND status = ?", now, "active").
		Order("next_reset_time asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	resetCount := 0
	for _, sub := range subs {
		subCopy := sub
		plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
		if err != nil || plan == nil {
			continue
		}
		err = DB.Transaction(func(tx *gorm.DB) error {
			var locked UserSubscription
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ? AND next_reset_time > 0 AND next_reset_time <= ?", subCopy.Id, now).
				First(&locked).Error; err != nil {
				return nil
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &locked, plan, now); err != nil {
				return err
			}
			resetCount++
			return nil
		})
		if err != nil {
			return resetCount, err
		}
	}
	return resetCount, nil
}

// CleanupSubscriptionPreConsumeRecords removes old idempotency records to keep table small.
func CleanupSubscriptionPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		olderThanSeconds = 7 * 24 * 3600
	}
	cutoff := GetDBTimestamp() - olderThanSeconds
	res := DB.Where("updated_at < ?", cutoff).Delete(&SubscriptionPreConsumeRecord{})
	return res.RowsAffected, res.Error
}

type SubscriptionPlanInfo struct {
	PlanId    int
	PlanTitle string
}

func GetSubscriptionPlanInfoByUserSubscriptionId(userSubscriptionId int) (*SubscriptionPlanInfo, error) {
	if userSubscriptionId <= 0 {
		return nil, errors.New("invalid userSubscriptionId")
	}
	cacheKey := fmt.Sprintf("sub:%d", userSubscriptionId)
	if cached, found, err := getSubscriptionPlanInfoCache().Get(cacheKey); err == nil && found {
		return &cached, nil
	}
	var sub UserSubscription
	if err := DB.Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
		return nil, err
	}
	plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
	if err != nil {
		return nil, err
	}
	info := &SubscriptionPlanInfo{
		PlanId:    sub.PlanId,
		PlanTitle: plan.Title,
	}
	_ = getSubscriptionPlanInfoCache().SetWithTTL(cacheKey, *info, subscriptionPlanInfoCacheTTL())
	return info, nil
}

// Update subscription used amount by delta (positive consume more, negative refund).
func PostConsumeUserSubscriptionDelta(userSubscriptionId int, delta int64) error {
	if userSubscriptionId <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		now := GetDBTimestampTx(tx)
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).
			First(&sub).Error; err != nil {
			return err
		}
		var plan *SubscriptionPlan
		if sub.PlanId > 0 {
			loadedPlan, planErr := getSubscriptionPlanByIdTx(tx, sub.PlanId)
			if planErr != nil {
				if errors.Is(planErr, gorm.ErrRecordNotFound) {
					common.SysLog(fmt.Sprintf("subscription post-consume failed due missing plan: sub_id=%d, plan_id=%d", sub.Id, sub.PlanId))
				}
				return planErr
			}
			if loadedPlan == nil {
				common.SysLog(fmt.Sprintf("subscription post-consume failed due nil plan: sub_id=%d, plan_id=%d", sub.Id, sub.PlanId))
				return errors.New("subscription plan is nil")
			}
			plan = loadedPlan
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
				return err
			}
			if prepareSubscriptionQuotaWindows(&sub, plan, now) {
				if err := tx.Save(&sub).Error; err != nil {
					return err
				}
			}
		}
		newUsed := sub.AmountUsed + delta
		if newUsed < 0 {
			newUsed = 0
		}
		if plan != nil && delta > 0 && exceedsSubscriptionQuotaWindows(&sub, plan, delta) {
			return errors.New("subscription window quota exceeded")
		}
		if sub.AmountTotal > 0 && newUsed > sub.AmountTotal {
			return fmt.Errorf("subscription used exceeds total, used=%d total=%d", newUsed, sub.AmountTotal)
		}
		sub.AmountUsed = newUsed
		if plan != nil {
			applySubscriptionQuotaWindowDelta(&sub, plan, delta)
		}
		return tx.Save(&sub).Error
	})
}
