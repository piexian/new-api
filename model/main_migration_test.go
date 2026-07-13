package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEnsureSubscriptionPlanTableSQLiteIncludesMergedColumns(t *testing.T) {
	testCases := []struct {
		name           string
		createOldTable bool
	}{
		{name: "create table"},
		{name: "migrate old table", createOldTable: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			oldDB := DB
			oldDatabaseType := common.MainDatabaseType()

			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			DB = db
			common.SetMainDatabaseType(common.DatabaseTypeSQLite)
			t.Cleanup(func() {
				DB = oldDB
				common.SetMainDatabaseType(oldDatabaseType)
				require.NoError(t, closeDB(db))
			})

			if testCase.createOldTable {
				require.NoError(t, DB.Exec("CREATE TABLE subscription_plans (id integer PRIMARY KEY)").Error)
			}

			require.NoError(t, ensureSubscriptionPlanTableSQLite())

			for _, column := range []string{
				"allow_balance_pay",
				"allow_wallet_overflow",
				"downgrade_group",
				"model_restrict_mode",
				"model_restrict_group",
				"allowed_models",
				"daily_quota_limit",
				"weekly_quota_limit",
				"monthly_quota_limit",
			} {
				assert.True(t, DB.Migrator().HasColumn("subscription_plans", column), column)
			}
		})
	}
}
