package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	newapii18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type openAITokenAuthErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func setupTokenAuthDisabledReasonTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false

	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	originalDB := model.DB
	originalLOGDB := model.LOG_DB

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.IsMasterNode = false
	common.SQLitePath = dsn
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	if err := os.Setenv("SQL_DSN", "local"); err != nil {
		t.Fatalf("failed to set SQL_DSN: %v", err)
	}
	if err := model.InitDB(); err != nil {
		t.Fatalf("failed to initialize sqlite db: %v", err)
	}
	db := model.DB
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Token{}); err != nil {
		t.Fatalf("failed to migrate token auth test db: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if hadSQLDSN {
			_ = os.Setenv("SQL_DSN", originalSQLDSN)
		} else {
			_ = os.Unsetenv("SQL_DSN")
		}
		model.DB = originalDB
		model.LOG_DB = originalLOGDB
	})

	return db
}

func seedDisabledTokenAuthUser(t *testing.T, db *gorm.DB, username string, disableReason string) *model.User {
	t.Helper()

	user := &model.User{
		Username:      username,
		Password:      "password123",
		Role:          common.RoleCommonUser,
		Status:        common.UserStatusDisabled,
		Group:         "default",
		Quota:         100,
		DisableReason: disableReason,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create disabled user: %v", err)
	}
	return user
}

func seedTokenAuthToken(t *testing.T, db *gorm.DB, userID int, key string) *model.Token {
	t.Helper()

	token := &model.Token{
		UserId:         userID,
		Name:           "disabled-user-token",
		Key:            key,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return token
}

func performDisabledTokenAuthRequest(t *testing.T, tokenKey string) openAITokenAuthErrorResponse {
	t.Helper()

	router := gin.New()
	router.GET("/v1/models", TokenAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer sk-"+tokenKey)
	request.Header.Set("Accept-Language", "en")

	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP %d, got %d: %s", http.StatusForbidden, recorder.Code, recorder.Body.String())
	}

	var response openAITokenAuthErrorResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode token auth error response: %v", err)
	}
	return response
}

func TestTokenAuthDisabledUserMessageUsesReasonWhenPresent(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupTokenAuthDisabledReasonTestDB(t)
	user := seedDisabledTokenAuthUser(t, db, "token-disabled-reason-user", "policy violation")
	token := seedTokenAuthToken(t, db, user.Id, "tokendisabledreason")

	response := performDisabledTokenAuthRequest(t, token.Key)

	if !strings.Contains(response.Error.Message, "This user has been disabled: policy violation") {
		t.Fatalf("expected disabled reason in error message, got %q", response.Error.Message)
	}
	if strings.Contains(response.Error.Message, "User has been banned") {
		t.Fatalf("expected custom disabled reason instead of default message, got %q", response.Error.Message)
	}
	if response.Error.Type != "new_api_error" {
		t.Fatalf("expected new_api_error type, got %q", response.Error.Type)
	}
}

func TestTokenAuthDisabledUserMessageUsesDefaultWhenReasonEmpty(t *testing.T) {
	if err := newapii18n.Init(); err != nil {
		t.Fatalf("failed to initialize i18n: %v", err)
	}
	db := setupTokenAuthDisabledReasonTestDB(t)
	user := seedDisabledTokenAuthUser(t, db, "token-disabled-empty-reason-user", "   ")
	token := seedTokenAuthToken(t, db, user.Id, "tokendisableddefault")

	response := performDisabledTokenAuthRequest(t, token.Key)

	if !strings.Contains(response.Error.Message, "User has been banned") {
		t.Fatalf("expected default banned message, got %q", response.Error.Message)
	}
	if strings.Contains(response.Error.Message, "This user has been disabled:") {
		t.Fatalf("expected no disabled reason message when reason is empty, got %q", response.Error.Message)
	}
}
