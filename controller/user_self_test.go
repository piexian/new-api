package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type selfAPIResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	Data    selfResponseData `json:"data"`
}

type selfResponseData struct {
	ID                      int    `json:"id"`
	Username                string `json:"username"`
	HasPassword             bool   `json:"has_password"`
	UsernameChangeLimit     int    `json:"username_change_limit"`
	UsernameChangeCount     int    `json:"username_change_count"`
	UsernameChangeRemaining int    `json:"username_change_remaining"`
	UsernameChangeResetAt   int64  `json:"username_change_reset_at"`
}

func setupUserSelfControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.TurnstileCheckEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedSelfUser(t *testing.T, db *gorm.DB, username string, password string) *model.User {
	t.Helper()

	user := &model.User{
		Username: username,
		Password: password,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  username + "-aff",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func newSelfContext(t *testing.T, userID int, role int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	ctx.Set("id", userID)
	ctx.Set("role", role)
	return ctx, recorder
}

func newSelfJSONContext(
	t *testing.T,
	method string,
	target string,
	body any,
	userID int,
	role int,
) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	payload, err := common.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userID)
	ctx.Set("role", role)
	return ctx, recorder
}

func decodeSelfResponse(t *testing.T, recorder *httptest.ResponseRecorder) selfAPIResponse {
	t.Helper()

	var response selfAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode self response: %v", err)
	}
	return response
}

func TestGetSelfReportsHasPasswordWhenPasswordExists(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	hashedPassword, err := common.Password2Hash("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := seedSelfUser(t, db, "self-user-with-password", hashedPassword)

	ctx, recorder := newSelfContext(t, user.Id, user.Role)
	GetSelf(ctx)

	response := decodeSelfResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if !response.Data.HasPassword {
		t.Fatalf("expected has_password=true, got false: %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), hashedPassword) {
		t.Fatalf("response leaked hashed password: %s", recorder.Body.String())
	}
}

func TestGetSelfReportsHasPasswordFalseWhenPasswordEmpty(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	user := seedSelfUser(t, db, "self-user-without-password", "")

	ctx, recorder := newSelfContext(t, user.Id, user.Role)
	GetSelf(ctx)

	response := decodeSelfResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if response.Data.HasPassword {
		t.Fatalf("expected has_password=false, got true: %s", recorder.Body.String())
	}
}

func TestUpdateSelfUsernameDoesNotRequireOriginalPassword(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	hashedPassword, err := common.Password2Hash("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := seedSelfUser(t, db, "old-username", hashedPassword)

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPut,
		"/api/user/self",
		map[string]any{"username": "new-username"},
		user.Id,
		user.Role,
	)
	UpdateSelf(ctx)

	response := decodeSelfResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var updated model.User
	if err := db.First(&updated, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updated.Username != "new-username" {
		t.Fatalf("expected username to be updated, got %q", updated.Username)
	}

	var logs []model.Log
	if err := db.Where("user_id = ?", user.Id).Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("failed to query logs: %v", err)
	}
	if len(logs) == 0 {
		t.Fatalf("expected username update log to be recorded")
	}
	lastLog := logs[len(logs)-1]
	if lastLog.Type != model.LogTypeSystem {
		t.Fatalf("expected system log type, got %d", lastLog.Type)
	}
	if !strings.Contains(lastLog.Content, "用户自助修改用户名: old-username -> new-username") {
		t.Fatalf("unexpected username update log content: %q", lastLog.Content)
	}
}

func TestUpdateUserReturnsFriendlyErrorWhenUsernameTaken(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	existingUser := seedSelfUser(t, db, "existing-username", "")
	targetUser := seedSelfUser(t, db, "target-username", "")

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPut,
		"/api/user",
		map[string]any{
			"id":       targetUser.Id,
			"username": existingUser.Username,
			"role":     common.RoleCommonUser,
		},
		999,
		common.RoleAdminUser,
	)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	UpdateUser(ctx)

	response := decodeSelfResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected duplicate username failure, got success: %s", recorder.Body.String())
	}
	if response.Message != "用户名已被占用" {
		t.Fatalf("expected friendly duplicate username message, got %q", response.Message)
	}

	var updated model.User
	if err := db.First(&updated, targetUser.Id).Error; err != nil {
		t.Fatalf("failed to reload target user: %v", err)
	}
	if updated.Username != "target-username" {
		t.Fatalf("expected username to remain unchanged, got %q", updated.Username)
	}
}

func TestUpdateUserReturnsFriendlyErrorWhenUsernameTooLong(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	targetUser := seedSelfUser(t, db, "target-username", "")

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPut,
		"/api/user",
		map[string]any{
			"id":       targetUser.Id,
			"username": strings.Repeat("a", model.UserNameMaxLength+1),
			"role":     common.RoleCommonUser,
		},
		999,
		common.RoleAdminUser,
	)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	UpdateUser(ctx)

	response := decodeSelfResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected long username failure, got success: %s", recorder.Body.String())
	}
	if response.Message != "用户名长度不能超过20个字符" {
		t.Fatalf("expected friendly long username message, got %q", response.Message)
	}

	var updated model.User
	if err := db.First(&updated, targetUser.Id).Error; err != nil {
		t.Fatalf("failed to reload target user: %v", err)
	}
	if updated.Username != "target-username" {
		t.Fatalf("expected username to remain unchanged, got %q", updated.Username)
	}
}

func TestUpdateSelfReturnsFriendlyErrorWhenUsernameTooLong(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	user := seedSelfUser(t, db, "self-user", "")

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPut,
		"/api/user/self",
		map[string]any{
			"username": strings.Repeat("a", model.UserNameMaxLength+1),
		},
		user.Id,
		user.Role,
	)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	UpdateSelf(ctx)

	response := decodeSelfResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected long username failure, got success: %s", recorder.Body.String())
	}
	if response.Message != "用户名长度不能超过20个字符" {
		t.Fatalf("expected friendly long username message, got %q", response.Message)
	}

	var updated model.User
	if err := db.First(&updated, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updated.Username != "self-user" {
		t.Fatalf("expected username to remain unchanged, got %q", updated.Username)
	}
}

func TestGetSelfReturnsUsernameChangeQuota(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	user := seedSelfUser(t, db, "quota-user", "")
	setting := dto.UserSetting{
		UsernameChangeWindowStart: common.GetTimestamp() - 3600,
		UsernameChangeCount:       1,
	}
	user.SetSetting(setting)
	if err := db.Model(&model.User{}).Where("id = ?", user.Id).Update("setting", user.Setting).Error; err != nil {
		t.Fatalf("failed to update user setting: %v", err)
	}

	ctx, recorder := newSelfContext(t, user.Id, user.Role)
	GetSelf(ctx)

	response := decodeSelfResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if response.Data.UsernameChangeLimit != usernameChangeLimit {
		t.Fatalf("expected username change limit %d, got %d", usernameChangeLimit, response.Data.UsernameChangeLimit)
	}
	if response.Data.UsernameChangeCount != 1 {
		t.Fatalf("expected username change count 1, got %d", response.Data.UsernameChangeCount)
	}
	if response.Data.UsernameChangeRemaining != 2 {
		t.Fatalf("expected username change remaining 2, got %d", response.Data.UsernameChangeRemaining)
	}
	if response.Data.UsernameChangeResetAt <= common.GetTimestamp() {
		t.Fatalf("expected future reset time, got %d", response.Data.UsernameChangeResetAt)
	}
}

func TestUpdateSelfUsernameRespectsYearlyLimit(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	user := seedSelfUser(t, db, "limited-user", "")
	setting := dto.UserSetting{
		UsernameChangeWindowStart: common.GetTimestamp() - 3600,
		UsernameChangeCount:       usernameChangeLimit,
	}
	user.SetSetting(setting)
	if err := db.Model(&model.User{}).Where("id = ?", user.Id).Update("setting", user.Setting).Error; err != nil {
		t.Fatalf("failed to update user setting: %v", err)
	}

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPut,
		"/api/user/self",
		map[string]any{"username": "blocked-username"},
		user.Id,
		user.Role,
	)
	UpdateSelf(ctx)

	response := decodeSelfResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected rate limit failure, got success: %s", recorder.Body.String())
	}
	if !strings.Contains(response.Message, "用户名一年内最多只能修改3次") {
		t.Fatalf("expected yearly limit error, got %q", response.Message)
	}

	var updated model.User
	if err := db.First(&updated, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updated.Username != "limited-user" {
		t.Fatalf("expected username to remain unchanged, got %q", updated.Username)
	}
}

func TestUpdateSelfUsernameResetsAfterWindowExpires(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	user := seedSelfUser(t, db, "expired-user", "")
	setting := dto.UserSetting{
		UsernameChangeWindowStart: common.GetTimestamp() - usernameChangeWindowSeconds - 10,
		UsernameChangeCount:       usernameChangeLimit,
	}
	user.SetSetting(setting)
	if err := db.Model(&model.User{}).Where("id = ?", user.Id).Update("setting", user.Setting).Error; err != nil {
		t.Fatalf("failed to update user setting: %v", err)
	}

	beforeUpdate := common.GetTimestamp()
	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPut,
		"/api/user/self",
		map[string]any{"username": "fresh-username"},
		user.Id,
		user.Role,
	)
	UpdateSelf(ctx)

	response := decodeSelfResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var updated model.User
	if err := db.First(&updated, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updated.Username != "fresh-username" {
		t.Fatalf("expected username to be updated, got %q", updated.Username)
	}

	updatedSetting := updated.GetSetting()
	if updatedSetting.UsernameChangeCount != 1 {
		t.Fatalf("expected reset username change count 1, got %d", updatedSetting.UsernameChangeCount)
	}
	if updatedSetting.UsernameChangeWindowStart < beforeUpdate {
		t.Fatalf("expected refreshed window start >= %d, got %d", beforeUpdate, updatedSetting.UsernameChangeWindowStart)
	}
}

func TestUpdateSelfPasswordRecordsSystemLog(t *testing.T) {
	db := setupUserSelfControllerTestDB(t)
	hashedPassword, err := common.Password2Hash("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := seedSelfUser(t, db, "password-user", hashedPassword)

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPut,
		"/api/user/self",
		map[string]any{
			"original_password": "password123",
			"password":          "newpassword123",
		},
		user.Id,
		user.Role,
	)
	UpdateSelf(ctx)

	response := decodeSelfResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var logs []model.Log
	if err := db.Where("user_id = ?", user.Id).Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("failed to query logs: %v", err)
	}
	if len(logs) == 0 {
		t.Fatalf("expected password update log to be recorded")
	}
	lastLog := logs[len(logs)-1]
	if lastLog.Type != model.LogTypeSystem {
		t.Fatalf("expected system log type, got %d", lastLog.Type)
	}
	if lastLog.Content != "用户自助修改密码" {
		t.Fatalf("unexpected password update log content: %q", lastLog.Content)
	}
	if strings.Contains(lastLog.Content, "newpassword123") {
		t.Fatalf("password should never appear in log content: %q", lastLog.Content)
	}
}
