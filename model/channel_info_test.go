package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func testChannelInfo() ChannelInfo {
	return ChannelInfo{
		IsMultiKey:             true,
		MultiKeySize:           2,
		MultiKeyStatusList:     map[int]int{0: 1, 1: 2},
		MultiKeyDisabledReason: map[int]string{1: "manual"},
		MultiKeyMode:           constant.MultiKeyModePolling,
	}
}

func TestChannelInfoValueReturnsJSONText(t *testing.T) {
	info := testChannelInfo()
	value, err := info.Value()
	require.NoError(t, err)

	jsonText, ok := value.(string)
	require.True(t, ok, "ChannelInfo must use text values so PostgreSQL simple protocol does not encode JSON as bytea")
	expected, err := common.Marshal(&info)
	require.NoError(t, err)
	require.JSONEq(t, string(expected), jsonText)
}

func TestChannelInfoScanSupportsDatabaseJSONTypes(t *testing.T) {
	original := testChannelInfo()
	encoded, err := common.Marshal(&original)
	require.NoError(t, err)

	for _, value := range []any{string(encoded), encoded} {
		var scanned ChannelInfo
		require.NoError(t, scanned.Scan(value))
		require.Equal(t, original, scanned)
	}

	var empty ChannelInfo
	require.NoError(t, empty.Scan(nil))
	require.Equal(t, ChannelInfo{}, empty)
	require.Error(t, empty.Scan(1))
}

func TestChannelInfoPostgresSimpleProtocol(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_DSN to run PostgreSQL simple protocol test")
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{PrepareStmt: false})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
	})

	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() {
		tx.Rollback()
	})

	require.NoError(t, tx.Exec(`CREATE TEMP TABLE channel_info_simple_protocol_test (
		id integer PRIMARY KEY,
		channel_info json
	)`).Error)
	require.NoError(t, tx.Exec(`INSERT INTO channel_info_simple_protocol_test (id, channel_info) VALUES (1, '{}')`).Error)

	info := testChannelInfo()
	require.NoError(t, tx.Table("channel_info_simple_protocol_test").Where("id = ?", 1).Update("channel_info", info).Error)

	var stored string
	require.NoError(t, tx.Raw(`SELECT channel_info::text FROM channel_info_simple_protocol_test WHERE id = 1`).Scan(&stored).Error)
	expected, err := common.Marshal(&info)
	require.NoError(t, err)
	require.JSONEq(t, string(expected), stored)
}
