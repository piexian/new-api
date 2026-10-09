package model

import "gorm.io/gorm"

// SQLite 无法以 ADD COLUMN 直接加 UNIQUE 约束，先加可空列，
// 再由 User 的 AutoMigrate 创建唯一索引。
func migrateNodeLocUserID(db *gorm.DB) error {
	if db.Dialector.Name() != "sqlite" || !db.Migrator().HasTable(&User{}) || db.Migrator().HasColumn(&User{}, "NodeLocId") {
		return nil
	}
	type nodeLocColumn struct {
		NodeLocId string `gorm:"column:nodeloc_id;type:varchar(64);default:null"`
	}
	return db.Table("users").Migrator().AddColumn(&nodeLocColumn{}, "NodeLocId")
}
