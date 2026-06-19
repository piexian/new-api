package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserAffCodeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	initCol()

	require.NoError(t, db.AutoMigrate(&User{}, &Option{}))

	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initCol()
	})

	return db
}

func TestRefreshAllAffCodesOnceRefreshesAllUsersOnlyOnce(t *testing.T) {
	db := setupUserAffCodeTestDB(t)
	users := []User{
		{Username: "aff-migrate-one", Password: "password", Group: "default", AffCode: "old1"},
		{Username: "aff-migrate-two", Password: "password", Group: "default", AffCode: "ABCDEF"},
	}
	require.NoError(t, db.Create(&users).Error)

	require.NoError(t, RefreshAllAffCodesOnce())

	var refreshed []User
	require.NoError(t, db.Order("id asc").Find(&refreshed).Error)
	require.Len(t, refreshed, len(users))
	firstRunCodes := make([]string, 0, len(refreshed))
	for i, user := range refreshed {
		require.Len(t, user.AffCode, AffCodeLength)
		require.NotEqual(t, users[i].AffCode, user.AffCode)
		firstRunCodes = append(firstRunCodes, user.AffCode)
	}

	var option Option
	require.NoError(t, db.First(&option, "key = ?", affCodeRefreshOptionKey).Error)
	require.Equal(t, "done", option.Value)

	require.NoError(t, RefreshAllAffCodesOnce())

	var secondRun []User
	require.NoError(t, db.Order("id asc").Find(&secondRun).Error)
	for i, user := range secondRun {
		require.Equal(t, firstRunCodes[i], user.AffCode)
	}
}

func TestTransferAffQuotaAllowsAmountBelowQuotaPerUnit(t *testing.T) {
	db := setupUserAffCodeTestDB(t)
	require.Greater(t, common.QuotaPerUnit, float64(1))

	user := User{
		Username: "aff-transfer-small",
		Password: "password",
		Group:    "default",
		AffCode:  "ABC123",
		AffQuota: 1,
		Quota:    10,
	}
	require.NoError(t, db.Create(&user).Error)

	require.NoError(t, user.TransferAffQuotaToQuota(1))

	var reloaded User
	require.NoError(t, db.First(&reloaded, user.Id).Error)
	require.Equal(t, 0, reloaded.AffQuota)
	require.Equal(t, 11, reloaded.Quota)
}
