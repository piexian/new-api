package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// recordSessionIdentity 将分销商中间件捕获的原始会话标识落到日志上；仅在风控中心开启采集时生效。
func recordSessionIdentity(c *gin.Context, log *Log) {
	if c == nil || !risk_setting.GetRiskCenterSetting().SessionRecordingEnabled {
		return
	}
	value, ok := c.Get(common.RequestSessionIdentityKey)
	identity, valid := value.(common.RequestSessionIdentity)
	if !ok || !valid {
		return
	}
	log.SessionRecorded = true
	log.SessionId, log.SessionSource = identity.SessionID, identity.SessionSource
	log.CacheKey, log.CacheSource = identity.CacheKey, identity.CacheSource
	// 哈希保留大小写敏感的标识，避免大小写不敏感的 collation 把不同会话聚合到一起。
	log.SessionHash = common.SessionIdentifierHash(identity.SessionID)
	log.CacheHash = common.SessionIdentifierHash(identity.CacheKey)
}

// LogSessionFilter 日志按会话标识/缓存关联标识筛选。
type LogSessionFilter struct {
	SessionID string
	CacheKey  string
}

func applyLogSessionFilters(tx *gorm.DB, filters []LogSessionFilter) *gorm.DB {
	for _, filter := range filters {
		for column, value := range map[string]string{"session_hash": filter.SessionID, "cache_hash": filter.CacheKey} {
			if value == "" {
				continue
			}
			value = common.NormalizeSessionIdentifier(value)
			if value == "" {
				tx = tx.Where("1 = 0")
			} else {
				tx = tx.Where("logs."+column+" = ?", common.SessionIdentifierHash(value))
			}
		}
	}
	return tx
}
