package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// loanAPIResponse 通用响应解码结构，data 按需再断言
type loanAPIResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func setupLoanControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	oldMainDatabaseType := common.MainDatabaseType()
	oldLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to init i18n: %v", err)
	}

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(
		&model.User{},
		&model.Checkin{},
		&model.TokenLoanAccount{},
		&model.TokenLoanRecord{},
		&model.TokenLoanApplication{},
		&model.TokenLoanApplicationMessage{},
	); err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}

	t.Cleanup(func() {
		common.SetDatabaseTypes(oldMainDatabaseType, oldLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

// withControllerLoanSetting 临时整体修改词元贷配置，测试结束后恢复
func withControllerLoanSetting(t *testing.T, mutate func(s *operation_setting.LoanSetting)) {
	t.Helper()
	setting := operation_setting.GetLoanSetting()
	old := *setting
	mutate(setting)
	t.Cleanup(func() { *setting = old })
}

func seedLoanUser(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	username := fmt.Sprintf("loan-ctrl-%d", time.Now().UnixNano())
	user := &model.User{
		Username: username,
		Password: "loan-ctrl-password",
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

// newLoanContext 构造注入用户 id 的测试上下文（替代鉴权中间件），params 为路径参数
func newLoanContext(t *testing.T, method, target string, body any, userId int, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		ctx.Request = httptest.NewRequest(method, target, bytes.NewReader(payload))
		ctx.Request.Header.Set("Content-Type", "application/json")
	} else {
		ctx.Request = httptest.NewRequest(method, target, nil)
	}
	ctx.Set("id", userId)
	ctx.Params = params
	return ctx, recorder
}

func decodeLoanResponse(t *testing.T, recorder *httptest.ResponseRecorder) loanAPIResponse {
	t.Helper()
	var response loanAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return response
}

// TestLoanBorrowTermsGateAndHappyPath 条款门槛 → 同意 → 借款全链路
func TestLoanBorrowTermsGateAndHappyPath(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = true
		s.MaxTotal = 2500000
		s.MinRegisterDays = 0
		s.MaxPerBorrow = 0
	})
	user := seedLoanUser(t, db)

	// 未同意条款直接借款 → 拒绝
	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/borrow",
		map[string]any{"amount_usd": "1.00"}, user.Id, nil)
	BorrowLoan(ctx)
	resp := decodeLoanResponse(t, recorder)
	if resp.Success {
		t.Fatalf("expected borrow to be rejected before agreeing terms")
	}
	if resp.Message != "请先阅读并同意词元贷声明" {
		t.Fatalf("unexpected message: %q", resp.Message)
	}

	// 同意条款（幂等）
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/agree", nil, user.Id, nil)
	AgreeLoanTerms(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("agree failed: %s", resp.Message)
	}
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/agree", nil, user.Id, nil)
	AgreeLoanTerms(ctx)
	if resp = decodeLoanResponse(t, recorder); !resp.Success {
		t.Fatalf("second agree should be idempotent, got: %s", resp.Message)
	}

	// 借款 1.00 USD = 500000 quota
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/borrow",
		map[string]any{"amount_usd": "1.00"}, user.Id, nil)
	BorrowLoan(ctx)
	resp = decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("borrow failed: %s", resp.Message)
	}
	if got := resp.Data["debt"]; got != float64(500000) {
		t.Fatalf("expected debt 500000, got %v", got)
	}
	if got := resp.Data["principal"]; got != float64(500000) {
		t.Fatalf("expected principal 500000, got %v", got)
	}
	if got := resp.Data["terms_agreed"]; got != true {
		t.Fatalf("expected terms_agreed true, got %v", got)
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloaded.Quota != 500000 {
		t.Fatalf("expected user quota 500000, got %d", reloaded.Quota)
	}
}

// TestLoanBorrowDisabled 功能关闭时借款被拒绝
func TestLoanBorrowDisabled(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = false
	})
	user := seedLoanUser(t, db)

	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/borrow",
		map[string]any{"amount_usd": "1.00"}, user.Id, nil)
	BorrowLoan(ctx)
	resp := decodeLoanResponse(t, recorder)
	if resp.Success {
		t.Fatalf("expected borrow to be rejected when disabled")
	}
	if resp.Message != "词元贷功能未启用" {
		t.Fatalf("unexpected message: %q", resp.Message)
	}
}

// TestLoanGetStatusNoAccount 无贷用户的只读投影全零，且 GET 不落盘建行
func TestLoanGetStatusNoAccount(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.MaxTotal = 2500000
		s.TermsEnabled = true
	})
	user := seedLoanUser(t, db)

	ctx, recorder := newLoanContext(t, http.MethodGet, "/api/user/loan/status", nil, user.Id, nil)
	GetLoanStatus(ctx)
	resp := decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("status failed: %s", resp.Message)
	}
	for _, field := range []string{"principal", "interest", "debt", "total_borrowed", "total_repaid"} {
		if got := resp.Data[field]; got != float64(0) {
			t.Fatalf("expected %s 0, got %v", field, got)
		}
	}
	if got := resp.Data["available"]; got != float64(2500000) {
		t.Fatalf("expected available 2500000, got %v", got)
	}
	if got := resp.Data["terms_agreed"]; got != false {
		t.Fatalf("expected terms_agreed false, got %v", got)
	}

	// GET 不得创建账户行
	var count int64
	if err := db.Model(&model.TokenLoanAccount{}).Where("user_id = ?", user.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count accounts: %v", err)
	}
	if count != 0 {
		t.Fatalf("GET status must not persist an account row")
	}
}

// TestLoanRecordsPagination 台账分页（id 倒序 + 总数）
func TestLoanRecordsPagination(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	user := seedLoanUser(t, db)
	for i := 0; i < 3; i++ {
		record := &model.TokenLoanRecord{
			UserId:        user.Id,
			Type:          "borrow",
			Amount:        int64(1000 * (i + 1)),
			PrincipalPart: int64(1000 * (i + 1)),
			DebtAfter:     int64(1000 * (i + 1)),
			Source:        "manual",
			CreatedAt:     time.Now().Unix(),
		}
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("failed to seed record: %v", err)
		}
	}

	// 第一页 2 条，总数 3，最新在前
	ctx, recorder := newLoanContext(t, http.MethodGet, "/api/user/loan/records?p=1&page_size=2", nil, user.Id, nil)
	GetLoanRecords(ctx)
	page1 := decodeLoanResponse(t, recorder)
	if !page1.Success {
		t.Fatalf("records failed: %s", page1.Message)
	}
	if got := page1.Data["total"]; got != float64(3) {
		t.Fatalf("expected total 3, got %v", got)
	}
	items, ok := page1.Data["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("expected 2 items on page 1, got %v", page1.Data["items"])
	}
	first := items[0].(map[string]any)
	if got := first["amount"]; got != float64(3000) {
		t.Fatalf("expected newest record first (amount 3000), got %v", got)
	}

	// 第二页剩 1 条
	ctx, recorder = newLoanContext(t, http.MethodGet, "/api/user/loan/records?p=2&page_size=2", nil, user.Id, nil)
	GetLoanRecords(ctx)
	page2 := decodeLoanResponse(t, recorder)
	items, ok = page2.Data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 item on page 2, got %v", page2.Data["items"])
	}
}

// TestCreateLoanApplicationValidation 建单参数与开关校验
func TestCreateLoanApplicationValidation(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	user := seedLoanUser(t, db)

	post := func(body map[string]any) loanAPIResponse {
		ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/applications", body, user.Id, nil)
		CreateLoanApplication(ctx)
		return decodeLoanResponse(t, recorder)
	}

	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.AiEnabled = false
		s.TermsEnabled = true
		s.AiModels = nil
	})

	// topic 白名单校验最靠前
	resp := post(map[string]any{"topic": "hack", "content": "hello"})
	if resp.Success || resp.Message != "无效的申请类型" {
		t.Fatalf("expected invalid topic rejection, got: %+v", resp)
	}

	// AI 业务员未启用
	resp = post(map[string]any{"topic": "credit", "content": "hello"})
	if resp.Success || resp.Message != "AI 业务员功能未启用" {
		t.Fatalf("expected officer disabled rejection, got: %+v", resp)
	}

	// 条款未同意
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.AiEnabled = true
		s.TermsEnabled = true
		s.AiModels = nil
	})
	resp = post(map[string]any{"topic": "credit", "content": "hello"})
	if resp.Success || resp.Message != "请先阅读并同意词元贷声明" {
		t.Fatalf("expected terms required rejection, got: %+v", resp)
	}

	// 同意条款后，未配置模型
	if err := model.AgreeLoanTerms(user.Id); err != nil {
		t.Fatalf("failed to agree terms: %v", err)
	}
	resp = post(map[string]any{"topic": "credit", "content": "hello"})
	if resp.Success || resp.Message != "未配置可用的 AI 业务员模型" {
		t.Fatalf("expected no model rejection, got: %+v", resp)
	}
}

// TestCreateLoanApplicationHappyPath 建单 + 首轮 AI 回复（注入假模型调用）
func TestCreateLoanApplicationHappyPath(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	withControllerLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.AiEnabled = true
		s.TermsEnabled = true
		s.AiModels = []operation_setting.AiModelConfig{{Model: "loan-test-model", ContextWindow: 8192}}
		s.AiMaxOutput = 256
		s.AiMaxRounds = 10
		s.AiMaxActiveApplications = 1
		s.AiDailyLimit = 3
		s.AiPrompt = "你是 AI 业务员"
	})
	user := seedLoanUser(t, db)
	if err := model.AgreeLoanTerms(user.Id); err != nil {
		t.Fatalf("failed to agree terms: %v", err)
	}

	// controller 包 init 接线的是真实上游直调，测试中替换为假实现，结束后恢复
	service.RegisterLoanOfficerModelCaller(func(userId int, modelName string, messages []dto.Message, maxOutputTokens int) (string, error) {
		return "好的，我们聊聊", nil
	})
	t.Cleanup(func() {
		service.RegisterLoanOfficerModelCaller(callLoanOfficerUpstream)
	})

	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/applications",
		map[string]any{"topic": "credit", "content": "想提额"}, user.Id, nil)
	CreateLoanApplication(ctx)
	resp := decodeLoanResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("create application failed: %s", resp.Message)
	}
	if got := resp.Data["reply"]; got != "好的，我们聊聊" {
		t.Fatalf("unexpected reply: %v", got)
	}
	if got := resp.Data["closed"]; got != false {
		t.Fatalf("expected closed false, got %v", got)
	}
	app, ok := resp.Data["application"].(map[string]any)
	if !ok {
		t.Fatalf("missing application in response: %v", resp.Data)
	}
	// model_used / decision 为内部审计字段，不得经 API 透出
	for _, k := range []string{"model_used", "decision"} {
		if _, exists := app[k]; exists {
			t.Fatalf("internal field %s should not be exposed in response: %v", k, app)
		}
	}

	// 首轮消息已落库（user + assistant）
	appId := int(app["id"].(float64))
	msgs, err := model.GetLoanApplicationMessages(appId)
	if err != nil {
		t.Fatalf("failed to load messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after first round, got %d", len(msgs))
	}
}

// TestRateLoanApplicationRejected 未结案评分与非法评分被拒绝
func TestRateLoanApplicationRejected(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	user := seedLoanUser(t, db)
	app := &model.TokenLoanApplication{
		UserId:    user.Id,
		Topic:     "credit",
		Status:    model.LoanAppStatusOpen,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	if err := db.Create(app).Error; err != nil {
		t.Fatalf("failed to seed application: %v", err)
	}
	params := gin.Params{{Key: "id", Value: strconv.Itoa(app.Id)}}

	// open 工单评分 → 拒绝
	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/applications/:id/rate",
		map[string]any{"rating": 5}, user.Id, params)
	RateLoanApplication(ctx)
	resp := decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "该申请已评分或尚未结案" {
		t.Fatalf("expected already rated / not closed rejection, got: %+v", resp)
	}

	// 评分越界 → 拒绝
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/applications/:id/rate",
		map[string]any{"rating": 0}, user.Id, params)
	RateLoanApplication(ctx)
	resp = decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "评分需在 1 到 5 之间" {
		t.Fatalf("expected invalid rating rejection, got: %+v", resp)
	}

	// 他人工单 → 不存在
	other := seedLoanUser(t, db)
	ctx, recorder = newLoanContext(t, http.MethodPost, "/api/user/loan/applications/:id/rate",
		map[string]any{"rating": 5}, other.Id, params)
	RateLoanApplication(ctx)
	resp = decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "申请不存在" {
		t.Fatalf("expected not found rejection, got: %+v", resp)
	}
}

// TestReplyLoanApplicationClosed 已结案工单拒绝继续回复
func TestReplyLoanApplicationClosed(t *testing.T) {
	db := setupLoanControllerTestDB(t)
	user := seedLoanUser(t, db)
	app := &model.TokenLoanApplication{
		UserId:    user.Id,
		Topic:     "credit",
		Status:    model.LoanAppStatusClosed,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	if err := db.Create(app).Error; err != nil {
		t.Fatalf("failed to seed application: %v", err)
	}

	ctx, recorder := newLoanContext(t, http.MethodPost, "/api/user/loan/applications/:id/reply",
		map[string]any{"content": "还在吗"}, user.Id,
		gin.Params{{Key: "id", Value: strconv.Itoa(app.Id)}})
	ReplyLoanApplication(ctx)
	resp := decodeLoanResponse(t, recorder)
	if resp.Success || resp.Message != "该申请已结案，无法继续回复" {
		t.Fatalf("expected not open rejection, got: %+v", resp)
	}
}
