package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type registrationInviteCodeAPIResponse struct {
	Code                   string  `json:"code"`
	ExpiresAt              int64   `json:"expires_at"`
	CanInvite              bool    `json:"can_invite"`
	CanGenerate            bool    `json:"can_generate"`
	IsAdminUnlimited       bool    `json:"is_admin_unlimited"`
	InviteLevel            int     `json:"invite_level"`
	UsedSlots              int     `json:"used_slots"`
	TotalSlots             int     `json:"total_slots"`
	CycleMonths            int     `json:"cycle_months"`
	CycleStartedAt         int64   `json:"cycle_started_at"`
	CycleEndsAt            int64   `json:"cycle_ends_at"`
	NextRefreshAt          int64   `json:"next_refresh_at"`
	CurrentNewUserQuotaUSD float64 `json:"current_new_user_quota_usd"`
}

func TestCreateSelfRegistrationInviteCodeRejectsCommonUserBelowLV3(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	common.PasswordRegisterOneTimeInviteCodeEnabled = true
	common.PasswordRegisterOneTimeInviteCodeCycleMonths = 1

	user := &model.User{
		Username:    "lv2_user",
		Password:    "hash",
		DisplayName: "lv2_user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "AFF-LV2",
		UsedQuota:   int(500 * common.QuotaPerUnit),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed issuer: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/registration-invite-code", nil, user.Id)
	CreateSelfRegistrationInviteCode(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected lv2 user to be rejected, body=%s", recorder.Body.String())
	}

	var data registrationInviteCodeAPIResponse
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode invite code response: %v", err)
	}
	if data.CanInvite || data.CanGenerate {
		t.Fatalf("expected lv2 user to have no invite permission, got can_invite=%v can_generate=%v", data.CanInvite, data.CanGenerate)
	}
	if data.TotalSlots != 0 {
		t.Fatalf("expected lv2 user total slots to be 0, got %d", data.TotalSlots)
	}

	var count int64
	if err := db.Model(&model.RegistrationInviteCode{}).Where("inviter_id = ?", user.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count invite codes: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no invite code record to be created, got %d", count)
	}
}

func TestCreateSelfRegistrationInviteCodeReturnsExistingValidCode(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	common.PasswordRegisterOneTimeInviteCodeEnabled = true
	common.PasswordRegisterOneTimeInviteCodeCycleMonths = 1

	user := &model.User{
		Username:    "issuer",
		Password:    "hash",
		DisplayName: "issuer",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "AFF2",
		UsedQuota:   int(2500 * common.QuotaPerUnit),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed issuer: %v", err)
	}

	cycleWindow := model.GetRegistrationInviteCycleWindowByTimestamp(common.GetTimestamp(), common.PasswordRegisterOneTimeInviteCodeCycleMonths)
	inviteCode := &model.RegistrationInviteCode{
		Code:      "EXISTINGCODE1234",
		InviterId: user.Id,
		ExpiresAt: cycleWindow.EndAt,
	}
	if err := db.Create(inviteCode).Error; err != nil {
		t.Fatalf("failed to seed invite code: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/registration-invite-code", nil, user.Id)
	CreateSelfRegistrationInviteCode(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected existing valid code to be returned, body=%s", recorder.Body.String())
	}

	var data registrationInviteCodeAPIResponse
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode invite code response: %v", err)
	}
	if data.Code != inviteCode.Code {
		t.Fatalf("expected code %q, got %q", inviteCode.Code, data.Code)
	}
	if data.CanGenerate {
		t.Fatal("expected common user with an active code to be unable to generate another code immediately")
	}

	var count int64
	if err := db.Model(&model.RegistrationInviteCode{}).Where("inviter_id = ?", user.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count invite codes: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one invite code record, got %d", count)
	}
}

func TestCreateSelfRegistrationInviteCodeAllowsLV4UserWithRemainingCycleSlot(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	common.PasswordRegisterOneTimeInviteCodeEnabled = true
	common.PasswordRegisterOneTimeInviteCodeCycleMonths = 1
	common.QuotaForNewUser = int(20 * common.QuotaPerUnit)

	now := common.GetTimestamp()
	cycleWindow := model.GetRegistrationInviteCycleWindowByTimestamp(now, common.PasswordRegisterOneTimeInviteCodeCycleMonths)
	user := &model.User{
		Username:    "lv4_user",
		Password:    "hash",
		DisplayName: "lv4_user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "AFF-LV4",
		UsedQuota:   int(12500 * common.QuotaPerUnit),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed lv4 issuer: %v", err)
	}

	usedInvite := &model.RegistrationInviteCode{
		Code:      "USEDCODE12345678",
		InviterId: user.Id,
		InviteeId: 999,
		ExpiresAt: cycleWindow.EndAt,
		UsedAt:    cycleWindow.StartAt + 3600,
	}
	if err := db.Create(usedInvite).Error; err != nil {
		t.Fatalf("failed to seed used invite code: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/registration-invite-code", nil, user.Id)
	CreateSelfRegistrationInviteCode(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected lv4 user to generate next code successfully, body=%s", recorder.Body.String())
	}

	var data registrationInviteCodeAPIResponse
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode invite code response: %v", err)
	}
	if data.Code == "" {
		t.Fatal("expected a new invite code to be returned")
	}
	if data.InviteLevel != 4 {
		t.Fatalf("expected invite level 4, got %d", data.InviteLevel)
	}
	if data.UsedSlots != 1 || data.TotalSlots != 2 {
		t.Fatalf("expected used/total slots to be 1/2, got %d/%d", data.UsedSlots, data.TotalSlots)
	}
	if data.CycleMonths != 1 {
		t.Fatalf("expected cycle months 1, got %d", data.CycleMonths)
	}
	if data.CurrentNewUserQuotaUSD != 20 {
		t.Fatalf("expected current new user quota usd 20, got %v", data.CurrentNewUserQuotaUSD)
	}
	if data.ExpiresAt != cycleWindow.EndAt || data.NextRefreshAt != cycleWindow.EndAt {
		t.Fatalf("expected new code and refresh time to end at cycle end %d, got expires_at=%d next_refresh_at=%d", cycleWindow.EndAt, data.ExpiresAt, data.NextRefreshAt)
	}
}

func TestAdminCanCreateRegistrationInviteCodeWithoutWaiting(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	common.PasswordRegisterOneTimeInviteCodeEnabled = true
	common.PasswordRegisterOneTimeInviteCodeCycleMonths = 3

	admin := &model.User{
		Username:    "admin_issuer",
		Password:    "hash",
		DisplayName: "admin_issuer",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "AFF5",
	}
	if err := db.Create(admin).Error; err != nil {
		t.Fatalf("failed to seed admin issuer: %v", err)
	}

	existingCode := &model.RegistrationInviteCode{
		Code:      "ADMINCODE1234567",
		InviterId: admin.Id,
		ExpiresAt: common.GetTimestamp() + 3600,
	}
	if err := db.Create(existingCode).Error; err != nil {
		t.Fatalf("failed to seed existing admin invite code: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/registration-invite-code", nil, admin.Id)
	CreateSelfRegistrationInviteCode(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected admin invite code creation to succeed, body=%s", recorder.Body.String())
	}

	var data registrationInviteCodeAPIResponse
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode invite code response: %v", err)
	}
	if data.Code == "" {
		t.Fatal("expected a new admin invite code to be returned")
	}
	if data.Code == existingCode.Code {
		t.Fatalf("expected a newly created code different from %q", existingCode.Code)
	}
	if !data.CanGenerate || !data.IsAdminUnlimited {
		t.Fatal("expected admin response to remain immediately generatable and unlimited")
	}

	var count int64
	if err := db.Model(&model.RegistrationInviteCode{}).Where("inviter_id = ?", admin.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count admin invite codes: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected two admin invite code records, got %d", count)
	}

	var revokedExisting model.RegistrationInviteCode
	if err := db.First(&revokedExisting, existingCode.Id).Error; err != nil {
		t.Fatalf("failed to reload old admin code: %v", err)
	}
	if revokedExisting.RevokedAt == 0 {
		t.Fatal("expected old admin code to be revoked when generating a new one")
	}
}
