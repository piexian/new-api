package model

import (
	"database/sql/driver"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func testTaskPrivateData() TaskPrivateData {
	return TaskPrivateData{
		UpstreamTaskID: "upstream-123",
		ResultURL:      "https://example.com/video.mp4",
		BillingSource:  "wallet",
		TokenId:        7,
		BillingContext: &TaskBillingContext{
			OriginModelName: "agnes-video-v2.0",
			ModelPrice:      0.01,
			OtherRatios:     map[string]float64{"seconds": 5},
		},
	}
}

func mustValue(t *testing.T, valuer driver.Valuer) driver.Value {
	t.Helper()
	value, err := valuer.Value()
	require.NoError(t, err)
	return value
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := common.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestTaskJSONFieldsValueReturnJSONText(t *testing.T) {
	properties := Properties{Input: "prompt", UpstreamModelName: "agnes-video-v2.0"}
	privateData := testTaskPrivateData()
	data := JSONValue(`{"task_id":"upstream-123","status":"SUBMITTED"}`)

	cases := map[string]struct {
		got      driver.Value
		expected []byte
	}{
		"properties":   {mustValue(t, properties), mustMarshal(t, properties)},
		"private_data": {mustValue(t, privateData), mustMarshal(t, privateData)},
		"data":         {mustValue(t, data), []byte(data)},
	}
	for name, c := range cases {
		jsonText, ok := c.got.(string)
		require.True(t, ok, "%s must use text values so PostgreSQL simple protocol does not encode JSON as bytea", name)
		require.JSONEq(t, string(c.expected), jsonText, name)
	}
}

func TestTaskJSONFieldsZeroValueWritesNull(t *testing.T) {
	valuers := map[string]driver.Valuer{
		"properties":   Properties{},
		"private_data": TaskPrivateData{},
		"data":         JSONValue(nil),
	}
	for name, valuer := range valuers {
		value, err := valuer.Value()
		require.NoError(t, err, name)
		require.Nil(t, value, "%s zero value must map to database NULL", name)
	}
}

func TestTaskJSONFieldsScanSupportsDatabaseJSONTypes(t *testing.T) {
	properties := Properties{Input: "prompt", UpstreamModelName: "agnes-video-v2.0"}
	privateData := testTaskPrivateData()
	data := JSONValue(`{"task_id":"upstream-123"}`)

	var scannedProperties Properties
	require.NoError(t, scannedProperties.Scan(string(mustMarshal(t, properties))))
	require.Equal(t, properties, scannedProperties)
	var scannedPropertiesBytes Properties
	require.NoError(t, scannedPropertiesBytes.Scan(mustMarshal(t, properties)))
	require.Equal(t, properties, scannedPropertiesBytes)

	var fromString TaskPrivateData
	require.NoError(t, fromString.Scan(string(mustMarshal(t, privateData))))
	require.Equal(t, privateData, fromString)
	var fromBytes TaskPrivateData
	require.NoError(t, fromBytes.Scan(mustMarshal(t, privateData)))
	require.Equal(t, privateData, fromBytes)

	var scannedData JSONValue
	require.NoError(t, scannedData.Scan(string(data)))
	require.JSONEq(t, string(data), string(scannedData))
	var scannedDataBytes JSONValue
	require.NoError(t, scannedDataBytes.Scan([]byte(data)))
	require.JSONEq(t, string(data), string(scannedDataBytes))

	var emptyProperties Properties
	require.NoError(t, emptyProperties.Scan(nil))
	require.Equal(t, Properties{}, emptyProperties)
	var emptyPrivate TaskPrivateData
	require.NoError(t, emptyPrivate.Scan(nil))
	require.Equal(t, TaskPrivateData{}, emptyPrivate)

	require.Error(t, (&Properties{}).Scan(1))
	require.Error(t, (&TaskPrivateData{}).Scan(1))
}

// TestTaskJSONFieldsPostgresSimpleProtocol 复现生产写入路径：
// pgx 简单协议 + json 列，验证 Task 的三个 JSON 字段可以完整写入并读回。
// 与 0aa3069e1（fix(db): 修复 PostgreSQL 渠道 JSON 更新失败）覆盖的
// ChannelInfo 属于同一类问题。
func TestTaskJSONFieldsPostgresSimpleProtocol(t *testing.T) {
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

	require.NoError(t, tx.Exec(`CREATE TEMP TABLE task_json_simple_protocol_test (
		id integer PRIMARY KEY,
		updated_at bigint,
		properties json,
		private_data json,
		data json
	)`).Error)
	require.NoError(t, tx.Exec(`INSERT INTO task_json_simple_protocol_test (id) VALUES (1)`).Error)

	task := Task{
		TaskID:      "task_test",
		Properties:  Properties{Input: "prompt", UpstreamModelName: "agnes-video-v2.0"},
		PrivateData: testTaskPrivateData(),
		Data:        JSONValue(`{"status":"SUBMITTED"}`),
	}
	require.NoError(t, tx.Table("task_json_simple_protocol_test").
		Where("id = ?", 1).
		Select("properties", "private_data", "data").
		Updates(&task).Error)

	var stored struct {
		Properties  string
		PrivateData string
		Data        string
	}
	require.NoError(t, tx.Raw(`SELECT properties::text, private_data::text, data::text
		FROM task_json_simple_protocol_test WHERE id = 1`).Scan(&stored).Error)
	require.JSONEq(t, string(mustMarshal(t, task.Properties)), stored.Properties)
	require.JSONEq(t, string(mustMarshal(t, task.PrivateData)), stored.PrivateData)
	require.JSONEq(t, string(task.Data), stored.Data)

	var roundTrip struct {
		Properties  Properties
		PrivateData TaskPrivateData
	}
	require.NoError(t, tx.Raw(`SELECT properties, private_data
		FROM task_json_simple_protocol_test WHERE id = 1`).Scan(&roundTrip).Error)
	require.Equal(t, task.Properties, roundTrip.Properties)
	require.Equal(t, task.PrivateData, roundTrip.PrivateData)
}
