package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
)

type manageUserResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Role          int    `json:"role"`
		Status        int    `json:"status"`
		DisableReason string `json:"disable_reason"`
		DisabledUntil int64  `json:"disabled_until"`
	} `json:"data"`
	Message string `json:"message"`
}

func TestManageUserTemporaryDisableAndEnable(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	user := seedSelfUser(t, db, "temporary-disable-user", "")
	before := common.GetTimestamp()

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/user/manage",
		map[string]any{
			"id":               user.Id,
			"action":           "disable",
			"disable_reason":   "temporary review",
			"duration_minutes": 1,
		},
		999,
		common.RoleRootUser,
	)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder.Body.Bytes())
	if !response.Success {
		t.Fatalf("expected temporary disable success, got message: %s", response.Message)
	}
	if response.Data.DisabledUntil < before+60 || response.Data.DisabledUntil > common.GetTimestamp()+60 {
		t.Fatalf("unexpected disabled_until: %d", response.Data.DisabledUntil)
	}

	var disabled model.User
	if err := db.First(&disabled, user.Id).Error; err != nil {
		t.Fatalf("failed to reload temporarily disabled user: %v", err)
	}
	if disabled.DisabledUntil != response.Data.DisabledUntil {
		t.Fatalf("database disabled_until %d does not match response %d", disabled.DisabledUntil, response.Data.DisabledUntil)
	}

	ctx, recorder = newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/user/manage",
		map[string]any{"id": user.Id, "action": "enable"},
		999,
		common.RoleRootUser,
	)
	ManageUser(ctx)

	response = decodeManageUserResponse(t, recorder.Body.Bytes())
	if !response.Success || response.Data.DisabledUntil != 0 {
		t.Fatalf("expected enable to clear disabled_until, got %#v", response)
	}
}

func TestValidateAndFillRestoresExpiredTemporaryDisable(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	hashedPassword, err := common.Password2Hash("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := seedSelfUser(t, db, "expired-temporary-user", hashedPassword)
	if err := db.Model(&model.User{}).Where("id = ?", user.Id).Updates(map[string]any{
		"status":         common.UserStatusDisabled,
		"disable_reason": "temporary review",
		"disabled_until": common.GetTimestamp() - 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed expired disable: %v", err)
	}

	candidate := model.User{Username: user.Username, Password: "password123"}
	if err := candidate.ValidateAndFill(); err != nil {
		t.Fatalf("expected expired temporary ban validation success, got %v", err)
	}
	var restored model.User
	if err := db.First(&restored, user.Id).Error; err != nil {
		t.Fatalf("failed to reload restored user: %v", err)
	}
	if restored.Status != common.UserStatusEnabled || restored.DisabledUntil != 0 || restored.DisableReason != "" {
		t.Fatalf("unexpected restored user state: status=%d until=%d reason=%q", restored.Status, restored.DisabledUntil, restored.DisableReason)
	}
}

func decodeManageUserResponse(t *testing.T, body []byte) manageUserResponse {
	t.Helper()

	var response manageUserResponse
	if err := common.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to decode manage user response: %v", err)
	}
	return response
}

func TestManageUserDisableReasonRoundTrip(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	user := seedSelfUser(t, db, "disable-reason-user", "")

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/user/manage",
		map[string]any{
			"id":             user.Id,
			"action":         "disable",
			"disable_reason": "  policy violation  ",
		},
		999,
		common.RoleRootUser,
	)
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder.Body.Bytes())
	if !response.Success {
		t.Fatalf("expected disable success, got message: %s", response.Message)
	}
	if response.Data.Status != common.UserStatusDisabled {
		t.Fatalf("expected disabled status in response, got %d", response.Data.Status)
	}
	if response.Data.DisableReason != "policy violation" {
		t.Fatalf("expected trimmed disable reason in response, got %q", response.Data.DisableReason)
	}

	var disabled model.User
	if err := db.First(&disabled, user.Id).Error; err != nil {
		t.Fatalf("failed to reload disabled user: %v", err)
	}
	if disabled.Status != common.UserStatusDisabled {
		t.Fatalf("expected disabled status in database, got %d", disabled.Status)
	}
	if disabled.DisableReason != "policy violation" {
		t.Fatalf("expected trimmed disable reason in database, got %q", disabled.DisableReason)
	}

	ctx, recorder = newSelfJSONContext(
		t,
		http.MethodPut,
		"/api/user",
		map[string]any{
			"id":             user.Id,
			"username":       user.Username,
			"display_name":   user.DisplayName,
			"role":           user.Role,
			"group":          user.Group,
			"disable_reason": "manual review",
		},
		999,
		common.RoleRootUser,
	)
	UpdateUser(ctx)

	editResponse := decodeSelfResponse(t, recorder)
	if !editResponse.Success {
		t.Fatalf("expected update success, got message: %s", editResponse.Message)
	}
	var edited model.User
	if err := db.First(&edited, user.Id).Error; err != nil {
		t.Fatalf("failed to reload edited user: %v", err)
	}
	if edited.DisableReason != "manual review" {
		t.Fatalf("expected edited disable reason, got %q", edited.DisableReason)
	}

	ctx, recorder = newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/user/manage",
		map[string]any{
			"id":     user.Id,
			"action": "enable",
		},
		999,
		common.RoleRootUser,
	)
	ManageUser(ctx)

	enableResponse := decodeManageUserResponse(t, recorder.Body.Bytes())
	if !enableResponse.Success {
		t.Fatalf("expected enable success, got message: %s", enableResponse.Message)
	}
	if enableResponse.Data.Status != common.UserStatusEnabled {
		t.Fatalf("expected enabled status in response, got %d", enableResponse.Data.Status)
	}
	if enableResponse.Data.DisableReason != "" {
		t.Fatalf("expected empty disable reason in response after enable, got %q", enableResponse.Data.DisableReason)
	}

	var enabled model.User
	if err := db.First(&enabled, user.Id).Error; err != nil {
		t.Fatalf("failed to reload enabled user: %v", err)
	}
	if enabled.Status != common.UserStatusEnabled {
		t.Fatalf("expected enabled status in database, got %d", enabled.Status)
	}
	if enabled.DisableReason != "" {
		t.Fatalf("expected empty disable reason in database after enable, got %q", enabled.DisableReason)
	}
}

func TestManageUserRejectsLongDisableReason(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	user := seedSelfUser(t, db, "long-disable-reason-user", "")

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/user/manage",
		map[string]any{
			"id":             user.Id,
			"action":         "disable",
			"disable_reason": strings.Repeat("禁", model.DisableReasonMaxLength+1),
		},
		999,
		common.RoleRootUser,
	)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	ManageUser(ctx)

	response := decodeManageUserResponse(t, recorder.Body.Bytes())
	if response.Success {
		t.Fatalf("expected long disable reason failure, got success: %s", recorder.Body.String())
	}
	if !strings.Contains(response.Message, "disable_reason max 5000") {
		t.Fatalf("expected max length error, got %q", response.Message)
	}

	var unchanged model.User
	if err := db.First(&unchanged, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if unchanged.Status != common.UserStatusEnabled {
		t.Fatalf("expected user to remain enabled, got %d", unchanged.Status)
	}
	if unchanged.DisableReason != "" {
		t.Fatalf("expected disable reason to remain empty, got %q", unchanged.DisableReason)
	}
}

func TestLoginDisabledUserShowsReasonAfterPasswordValidation(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	hashedPassword, err := common.Password2Hash("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := seedSelfUser(t, db, "login-disabled-user", hashedPassword)
	if err := db.Model(&model.User{}).Where("id = ?", user.Id).Updates(map[string]any{
		"status":         common.UserStatusDisabled,
		"disable_reason": "billing abuse",
	}).Error; err != nil {
		t.Fatalf("failed to disable user: %v", err)
	}

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/user/login",
		map[string]any{
			"username": user.Username,
			"password": "password123",
		},
		0,
		0,
	)
	ctx.Request.Header.Set("Accept-Language", "en")
	Login(ctx)

	response := decodeSelfResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected disabled login failure, got success: %s", recorder.Body.String())
	}
	if response.Message != "This user has been disabled: billing abuse" {
		t.Fatalf("expected disabled reason message, got %q", response.Message)
	}

	ctx, recorder = newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/user/login",
		map[string]any{
			"username": user.Username,
			"password": "wrong-password",
		},
		0,
		0,
	)
	ctx.Request.Header.Set("Accept-Language", "en")
	Login(ctx)

	response = decodeSelfResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected invalid password failure, got success: %s", recorder.Body.String())
	}
	if strings.Contains(response.Message, "billing abuse") {
		t.Fatalf("invalid password path must not expose disable reason, got %q", response.Message)
	}
}
