package model

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupRegistrationInviteModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(&User{}, &RegistrationInviteCode{}); err != nil {
		t.Fatalf("failed to migrate registration invite test tables: %v", err)
	}

	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.UsingMySQL = oldUsingMySQL
		common.UsingSQLite = oldUsingSQLite
		DB = oldDB
		LOG_DB = oldLogDB

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestGetRegistrationInviteTraceIncludesSoftDeletedUsers(t *testing.T) {
	db := setupRegistrationInviteModelTestDB(t)

	inviter := &User{
		Username:    "inviter",
		Password:    "hash",
		DisplayName: "inviter",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "AFF3",
	}
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("failed to seed inviter: %v", err)
	}

	invitee := &User{
		Username:    "invitee",
		Password:    "hash",
		DisplayName: "invitee",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		InviterId:   inviter.Id,
		AffCode:     "AFF4",
	}
	if err := db.Create(invitee).Error; err != nil {
		t.Fatalf("failed to seed invitee: %v", err)
	}

	inviteCode := &RegistrationInviteCode{
		Code:      "TRACECODE123456",
		InviterId: inviter.Id,
		InviteeId: invitee.Id,
		ExpiresAt: common.GetTimestamp() + 3600,
		UsedAt:    common.GetTimestamp(),
	}
	if err := db.Create(inviteCode).Error; err != nil {
		t.Fatalf("failed to seed invite code: %v", err)
	}

	if err := db.Delete(inviter).Error; err != nil {
		t.Fatalf("failed to soft-delete inviter: %v", err)
	}

	trace, err := GetRegistrationInviteTrace(invitee.Id)
	if err != nil {
		t.Fatalf("expected trace lookup to succeed: %v", err)
	}
	if trace.Inviter == nil {
		t.Fatal("expected inviter to be returned in trace")
	}
	if !trace.Inviter.Deleted {
		t.Fatal("expected inviter to be marked as deleted in trace")
	}
	if trace.InviteCode == nil || trace.InviteCode.Code != inviteCode.Code {
		t.Fatalf("expected invite code %q in trace", inviteCode.Code)
	}
}

func TestGetRegistrationInviteCycleWindowByTimestamp(t *testing.T) {
	location := time.Local
	now := time.Date(2026, time.May, 19, 12, 30, 0, 0, location).Unix()

	monthly := GetRegistrationInviteCycleWindowByTimestamp(now, 1)
	if monthly.StartAt != time.Date(2026, time.May, 1, 0, 0, 0, 0, location).Unix() {
		t.Fatalf("unexpected monthly cycle start: %d", monthly.StartAt)
	}
	if monthly.EndAt != time.Date(2026, time.June, 1, 0, 0, 0, 0, location).Unix() {
		t.Fatalf("unexpected monthly cycle end: %d", monthly.EndAt)
	}

	quarterly := GetRegistrationInviteCycleWindowByTimestamp(now, 3)
	if quarterly.StartAt != time.Date(2026, time.April, 1, 0, 0, 0, 0, location).Unix() {
		t.Fatalf("unexpected quarterly cycle start: %d", quarterly.StartAt)
	}
	if quarterly.EndAt != time.Date(2026, time.July, 1, 0, 0, 0, 0, location).Unix() {
		t.Fatalf("unexpected quarterly cycle end: %d", quarterly.EndAt)
	}
}

func TestBuildRegistrationInviteIssuerStateUsesLevelAndCycleSlots(t *testing.T) {
	db := setupRegistrationInviteModelTestDB(t)

	now := common.GetTimestamp()
	cycleWindow := GetRegistrationInviteCycleWindowByTimestamp(now, 1)
	user := &User{
		Username:    "lv4",
		Password:    "hash",
		DisplayName: "lv4",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "AFF-LV4-STATE",
		UsedQuota:   int(12500 * common.QuotaPerUnit),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed invite issuer: %v", err)
	}

	usedCode := &RegistrationInviteCode{
		Code:      "USEDSTATE123456",
		InviterId: user.Id,
		InviteeId: 1001,
		ExpiresAt: cycleWindow.EndAt,
		UsedAt:    cycleWindow.StartAt + 10,
	}
	if err := db.Create(usedCode).Error; err != nil {
		t.Fatalf("failed to seed used invite code: %v", err)
	}

	legacyActiveCode := &RegistrationInviteCode{
		Code:      "LEGACYACTIVE1234",
		InviterId: user.Id,
		ExpiresAt: cycleWindow.EndAt + 86400,
		CreatedAt: cycleWindow.StartAt - 10,
		UpdatedAt: cycleWindow.StartAt - 10,
	}
	if err := db.Create(legacyActiveCode).Error; err != nil {
		t.Fatalf("failed to seed legacy active invite code: %v", err)
	}

	state, err := BuildRegistrationInviteIssuerState(user, now, 1)
	if err != nil {
		t.Fatalf("expected state build to succeed: %v", err)
	}
	if state.InviteLevel != 4 {
		t.Fatalf("expected invite level 4, got %d", state.InviteLevel)
	}
	if state.TotalSlots != 2 || state.UsedSlots != 1 {
		t.Fatalf("expected used/total slots 1/2, got %d/%d", state.UsedSlots, state.TotalSlots)
	}
	if !state.CanInvite || !state.CanGenerate {
		t.Fatalf("expected lv4 issuer with one remaining slot to be able to generate, got can_invite=%v can_generate=%v", state.CanInvite, state.CanGenerate)
	}
	if state.ActiveCode != nil {
		t.Fatalf("expected cross-cycle legacy active code to be ignored, got %q", state.ActiveCode.Code)
	}
}
