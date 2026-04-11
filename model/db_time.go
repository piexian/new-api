package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func getDBTimestampWith(query *gorm.DB) int64 {
	if query == nil {
		query = DB
	}
	var ts int64
	var err error
	switch {
	case common.UsingPostgreSQL:
		err = query.Raw("SELECT EXTRACT(EPOCH FROM NOW())::bigint").Scan(&ts).Error
	case common.UsingSQLite:
		err = query.Raw("SELECT strftime('%s','now')").Scan(&ts).Error
	default:
		err = query.Raw("SELECT UNIX_TIMESTAMP()").Scan(&ts).Error
	}
	if err != nil || ts <= 0 {
		return common.GetTimestamp()
	}
	return ts
}

// GetDBTimestamp returns a UNIX timestamp from database time.
// Falls back to application time on error.
func GetDBTimestamp() int64 {
	return getDBTimestampWith(DB)
}

// GetDBTimestampTx returns a UNIX timestamp from the current transaction/database handle.
// Falls back to application time on error.
func GetDBTimestampTx(tx *gorm.DB) int64 {
	return getDBTimestampWith(tx)
}
