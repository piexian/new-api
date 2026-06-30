package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const userSettingRecordIpLogKey = "record_ip_log"

func parseUserSettingMap(raw string) map[string]any {
	settingMap := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return settingMap
	}
	if err := common.Unmarshal([]byte(raw), &settingMap); err != nil {
		common.SysLog("failed to unmarshal user setting: " + err.Error())
		return map[string]any{}
	}
	if settingMap == nil {
		return map[string]any{}
	}
	return settingMap
}

func enableRecordIpLogInSettingJSON(raw string) (string, bool, error) {
	settingMap := parseUserSettingMap(raw)
	if value, ok := settingMap[userSettingRecordIpLogKey].(bool); ok && value {
		return raw, false, nil
	}
	settingMap[userSettingRecordIpLogKey] = true
	settingBytes, err := common.Marshal(settingMap)
	if err != nil {
		return "", false, err
	}
	return string(settingBytes), true, nil
}

func UserSettingJSONForResponse(raw string) string {
	if !common.ForceRecordIpLogEnabled || strings.TrimSpace(raw) == "" {
		return raw
	}
	settingMap := parseUserSettingMap(raw)
	delete(settingMap, userSettingRecordIpLogKey)
	settingBytes, err := common.Marshal(settingMap)
	if err != nil {
		common.SysLog("failed to marshal user setting: " + err.Error())
		return raw
	}
	return string(settingBytes)
}

func ShouldRecordRequestLogIP(userId int) bool {
	if common.ForceRecordIpLogEnabled {
		return true
	}
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		return settingMap.RecordIpLog
	}
	return false
}

func updateRecordIpLogUserSettingCaches(changedSettings map[int]string) {
	for userId, setting := range changedSettings {
		if err := updateUserSettingCache(userId, setting); err != nil {
			common.SysLog("failed to update user setting cache: " + err.Error())
		}
	}
}

func forceEnableRecordIpLogForAllUsersTx(tx *gorm.DB) (map[int]string, error) {
	changedSettings := map[int]string{}
	var users []User
	err := tx.Model(&User{}).Select("id", "setting").FindInBatches(&users, 100, func(batchTx *gorm.DB, batch int) error {
		for i := range users {
			nextSetting, changed, err := enableRecordIpLogInSettingJSON(users[i].Setting)
			if err != nil {
				return err
			}
			if !changed {
				continue
			}
			if err := batchTx.Model(&User{}).Where("id = ?", users[i].Id).Update("setting", nextSetting).Error; err != nil {
				return err
			}
			changedSettings[users[i].Id] = nextSetting
		}
		return nil
	}).Error
	return changedSettings, err
}

func ForceEnableRecordIpLogForAllUsers() error {
	var changedSettings map[int]string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		changedSettings, err = forceEnableRecordIpLogForAllUsersTx(tx)
		return err
	})
	if err != nil {
		return err
	}
	updateRecordIpLogUserSettingCaches(changedSettings)
	return nil
}
