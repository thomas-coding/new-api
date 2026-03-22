package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserRegisterControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Token{}); err != nil {
		t.Fatalf("failed to migrate user register tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func newRegisterContext(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func TestRegisterSucceedsWithoutRegistrationCodeWhenDisabled(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordRegisterCodeEnabled = false
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0
	common.UpdatePasswordRegisterCodes("")

	ctx, recorder := newRegisterContext(t, `{"username":"alice","password":"password123"}`)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s body=%s", response.Message, recorder.Body.String())
	}

	var user model.User
	if err := db.Where("username = ?", "alice").First(&user).Error; err != nil {
		t.Fatalf("expected registered user to exist: %v", err)
	}

	var token model.Token
	if err := db.Where("user_id = ? AND name = ?", user.Id, "default").First(&token).Error; err != nil {
		t.Fatalf("expected default token to be created: %v", err)
	}
}

func TestRegisterRejectsMissingRegistrationCodeWhenEnabled(t *testing.T) {
	setupUserRegisterControllerTestDB(t)

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordRegisterCodeEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0
	common.UpdatePasswordRegisterCodes(`["INVITE-OK"]`)

	ctx, recorder := newRegisterContext(t, `{"username":"bob","password":"password123"}`)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected missing registration code to fail, body=%s", recorder.Body.String())
	}
	if response.Message != "user.register_code_required" &&
		!strings.Contains(response.Message, "邀请") &&
		!strings.Contains(strings.ToLower(response.Message), "registration code") {
		t.Fatalf("expected registration code error message, got %q", response.Message)
	}
}

func TestRegisterRejectsInvalidRegistrationCodeWhenEnabled(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordRegisterCodeEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0
	common.UpdatePasswordRegisterCodes(`["INVITE-OK"]`)

	ctx, recorder := newRegisterContext(t, `{"username":"carol","password":"password123","registration_code":"WRONG"}`)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected invalid registration code to fail, body=%s", recorder.Body.String())
	}

	var count int64
	if err := db.Model(&model.User{}).Where("username = ?", "carol").Count(&count).Error; err != nil {
		t.Fatalf("failed to check user count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no user to be created, got count %d", count)
	}
}

func TestRegisterSucceedsWithValidRegistrationCodeWhenEnabled(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordRegisterCodeEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0
	common.UpdatePasswordRegisterCodes(`["INVITE-OK","VIP-ENTRY"]`)

	ctx, recorder := newRegisterContext(t, `{"username":"dave","password":"password123","registration_code":"VIP-ENTRY"}`)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s body=%s", response.Message, recorder.Body.String())
	}

	var user model.User
	if err := db.Where("username = ?", "dave").First(&user).Error; err != nil {
		t.Fatalf("expected registered user to exist: %v", err)
	}
}
