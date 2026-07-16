package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type redemptionCreateResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Data    []string `json:"data"`
}

func TestAddRegistrationCodeWithoutPaymentCompliance(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupUserSelfControllerTestDB(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	previousConfirmed := paymentSetting.ComplianceConfirmed
	previousTermsVersion := paymentSetting.ComplianceTermsVersion
	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = ""
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = previousConfirmed
		paymentSetting.ComplianceTermsVersion = previousTermsVersion
	})

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/redemption/",
		map[string]any{
			"name":            "Partner Campaign",
			"type":            model.RedemptionTypeRegistration,
			"count":           2,
			"max_redemptions": 3,
		},
		1,
		common.RoleAdminUser,
	)
	AddRedemption(ctx)

	var response redemptionCreateResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Success || len(response.Data) != 2 {
		t.Fatalf("expected two registration codes, got %s", recorder.Body.String())
	}

	var codes []model.Redemption
	if err := db.Order("id ASC").Find(&codes).Error; err != nil {
		t.Fatalf("failed to load registration codes: %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("expected two stored registration codes, got %d", len(codes))
	}
	for _, code := range codes {
		if code.Type != model.RedemptionTypeRegistration || code.MaxRedemptions != 3 || code.Quota != 0 || code.SubscriptionPlanId != 0 {
			t.Fatalf("unexpected registration code payload: %+v", code)
		}
	}
}

func TestAddRegistrationCodeRequiresPositiveRegistrationLimit(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	setupUserSelfControllerTestDB(t)

	ctx, recorder := newSelfJSONContext(
		t,
		http.MethodPost,
		"/api/redemption/",
		map[string]any{
			"name":            "Invalid Campaign",
			"type":            model.RedemptionTypeRegistration,
			"count":           1,
			"max_redemptions": 0,
		},
		1,
		common.RoleAdminUser,
	)
	ctx.Request.Header.Set("Accept-Language", "en")
	AddRedemption(ctx)

	var response redemptionCreateResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Success || response.Message != "Registration limit must be greater than 0" {
		t.Fatalf("expected positive registration limit error, got %s", recorder.Body.String())
	}
}
