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
	if err := db.AutoMigrate(&model.RegistrationInviteCode{}); err != nil {
		t.Fatalf("failed to migrate user register tables: %v", err)
	}

	common.ResetVerificationStateForTest()

	t.Cleanup(func() {
		common.ResetVerificationStateForTest()
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
	common.PasswordRegisterOneTimeInviteCodeEnabled = false
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
	common.PasswordRegisterOneTimeInviteCodeEnabled = false
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
	common.PasswordRegisterOneTimeInviteCodeEnabled = false
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
	common.PasswordRegisterOneTimeInviteCodeEnabled = false
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

func TestRegisterRequiresRegistrationVerificationPurpose(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordRegisterCodeEnabled = false
	common.PasswordRegisterOneTimeInviteCodeEnabled = false
	common.EmailVerificationEnabled = true
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0

	email := "eve@example.com"
	common.RegisterVerificationCodeWithKey(email, "111111", common.EmailVerificationPurpose)

	ctx, recorder := newRegisterContext(t, `{"username":"eve","password":"password123","email":"eve@example.com","verification_code":"111111"}`)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected register request with bind-email verification purpose to fail, body=%s", recorder.Body.String())
	}

	common.RegisterVerificationCodeWithKey(email, "222222", common.RegistrationEmailVerificationPurpose)

	ctx, recorder = newRegisterContext(t, `{"username":"eve","password":"password123","email":"eve@example.com","verification_code":"222222"}`)
	Register(ctx)

	response = decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected register request with registration verification purpose to succeed, body=%s", recorder.Body.String())
	}

	var user model.User
	if err := db.Where("username = ?", "eve").First(&user).Error; err != nil {
		t.Fatalf("expected registered user to exist: %v", err)
	}
}

func TestRegisterConsumesOneTimeInviteCodeAndSetsInviter(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	inviter := &model.User{
		Username:    "inviter",
		Password:    "hash",
		DisplayName: "inviter",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "AFF1",
	}
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("failed to seed inviter: %v", err)
	}

	inviteCode := &model.RegistrationInviteCode{
		Code:      "ONETIME123456",
		InviterId: inviter.Id,
		ExpiresAt: common.GetTimestamp() + 3600,
	}
	if err := db.Create(inviteCode).Error; err != nil {
		t.Fatalf("failed to seed registration invite code: %v", err)
	}

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordRegisterCodeEnabled = false
	common.PasswordRegisterOneTimeInviteCodeEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser = int(20 * common.QuotaPerUnit)
	common.QuotaForInvitee = 111
	common.QuotaForInviter = 222

	ctx, recorder := newRegisterContext(t, `{"username":"frank","password":"password123","registration_code":"ONETIME123456"}`)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected one-time invite registration to succeed, body=%s", recorder.Body.String())
	}

	var invitee model.User
	if err := db.Where("username = ?", "frank").First(&invitee).Error; err != nil {
		t.Fatalf("expected invitee to be created: %v", err)
	}
	if invitee.InviterId != inviter.Id {
		t.Fatalf("expected inviter_id %d, got %d", inviter.Id, invitee.InviterId)
	}
	if invitee.Quota != common.QuotaForNewUser {
		t.Fatalf("expected only new-user initial quota %d to be granted, got %d", common.QuotaForNewUser, invitee.Quota)
	}

	var consumedCode model.RegistrationInviteCode
	if err := db.First(&consumedCode, inviteCode.Id).Error; err != nil {
		t.Fatalf("expected invite code to remain queryable: %v", err)
	}
	if consumedCode.InviteeId != invitee.Id {
		t.Fatalf("expected invitee_id %d, got %d", invitee.Id, consumedCode.InviteeId)
	}
	if consumedCode.UsedAt == 0 {
		t.Fatal("expected invite code to be marked as used")
	}

	var token model.Token
	if err := db.Where("user_id = ? AND name = ?", invitee.Id, "default").First(&token).Error; err != nil {
		t.Fatalf("expected default token to be created: %v", err)
	}

	var inviterAfter model.User
	if err := db.First(&inviterAfter, inviter.Id).Error; err != nil {
		t.Fatalf("expected inviter to remain queryable: %v", err)
	}
	if inviterAfter.AffCount != 0 {
		t.Fatalf("expected no inviter count reward, got %d", inviterAfter.AffCount)
	}
	if inviterAfter.AffQuota != 0 {
		t.Fatalf("expected no inviter quota reward, got %d", inviterAfter.AffQuota)
	}
}

func TestRegisterRejectsLegacyStaticCodeWhenOneTimeModeEnabled(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordRegisterCodeEnabled = true
	common.PasswordRegisterOneTimeInviteCodeEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0
	common.UpdatePasswordRegisterCodes(`["LEGACY-CODE"]`)

	ctx, recorder := newRegisterContext(t, `{"username":"grace","password":"password123","registration_code":"LEGACY-CODE"}`)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected legacy static code to be rejected when one-time mode is enabled, body=%s", recorder.Body.String())
	}

	var count int64
	if err := db.Model(&model.User{}).Where("username = ?", "grace").Count(&count).Error; err != nil {
		t.Fatalf("failed to check user count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no user to be created, got count %d", count)
	}
}
