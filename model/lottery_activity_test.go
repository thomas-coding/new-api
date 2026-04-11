package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLotteryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:lottery_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	oldDB := DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	t.Cleanup(func() {
		DB = oldDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
	})
	if err := db.AutoMigrate(&LotteryActivity{}, &LotteryReward{}, &LotteryConsumeRecord{}, &User{}); err != nil {
		t.Fatalf("failed to migrate lottery tables: %v", err)
	}
	return db
}

func setLotteryTestSetting(t *testing.T, weeklyDay int) {
	t.Helper()
	current := operation_setting.GetLotterySetting()
	original := *current
	original.Tiers = append([]operation_setting.LotteryTierSetting(nil), current.Tiers...)
	t.Cleanup(func() {
		restored := original
		restored.Tiers = append([]operation_setting.LotteryTierSetting(nil), original.Tiers...)
		*current = restored
	})
	*current = operation_setting.LotterySetting{
		WeeklyDay:            weeklyDay,
		MythBroadcastEnabled: true,
		Tiers: []operation_setting.LotteryTierSetting{
			{Name: "普通", Amount: 3, Probability: 82},
			{Name: "稀有", Amount: 8, Probability: 13},
			{Name: "史诗", Amount: 20, Probability: 4},
			{Name: "传说", Amount: 50, Probability: 0.8},
			{Name: "神话", Amount: 200, Probability: 0.2},
		},
	}
}

func TestOpenImmediateLotteryActivityCreatesWindow(t *testing.T) {
	setupLotteryTestDB(t)

	admin := &User{
		Username: "lottery_admin",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
	}
	if err := DB.Create(admin).Error; err != nil {
		t.Fatalf("failed to seed admin: %v", err)
	}

	setting := operation_setting.GetNormalizedLotterySetting()
	activity, err := OpenImmediateLotteryActivity(admin.Id, LotteryActivityScopePublic, setting)
	if err != nil {
		t.Fatalf("expected activity open success, got err=%v", err)
	}
	if activity == nil {
		t.Fatal("expected created activity")
	}
	if activity.Scope != LotteryActivityScopePublic {
		t.Fatalf("expected scope public, got %s", activity.Scope)
	}
	if activity.DrawStartsAt <= 0 || activity.DrawEndsAt <= activity.DrawStartsAt {
		t.Fatalf("expected valid draw window, got starts=%d ends=%d", activity.DrawStartsAt, activity.DrawEndsAt)
	}
	if activity.ConsumeStartsAt != activity.AutoActivateAt {
		t.Fatalf("expected consume start to match auto activate, got consume_start=%d auto_activate=%d", activity.ConsumeStartsAt, activity.AutoActivateAt)
	}
	if activity.ConsumeEndsAt <= activity.ConsumeStartsAt {
		t.Fatalf("expected valid consume window, got consume_start=%d consume_end=%d", activity.ConsumeStartsAt, activity.ConsumeEndsAt)
	}
}

func TestEnsureWeeklyPublicLotteryActivityOpensOnce(t *testing.T) {
	setupLotteryTestDB(t)

	nowLocal := time.Date(2026, 4, 14, 12, 0, 0, 0, time.Local)
	weekday := int(nowLocal.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	setLotteryTestSetting(t, weekday)

	activity, err := EnsureWeeklyPublicLotteryActivity(nowLocal.Unix())
	if err != nil {
		t.Fatalf("expected weekly auto open success, got err=%v", err)
	}
	if activity == nil {
		t.Fatal("expected weekly activity to be created")
	}
	if activity.Scope != LotteryActivityScopePublic {
		t.Fatalf("expected weekly activity scope public, got %s", activity.Scope)
	}
	if activity.OpenMode != LotteryActivityOpenModeWeekly {
		t.Fatalf("expected weekly open mode, got %s", activity.OpenMode)
	}
	if activity.CreatedBy != 0 {
		t.Fatalf("expected weekly activity created_by 0 for system open, got %d", activity.CreatedBy)
	}

	second, err := EnsureWeeklyPublicLotteryActivity(nowLocal.Add(2 * time.Hour).Unix())
	if err != nil {
		t.Fatalf("expected second weekly ensure success, got err=%v", err)
	}
	if second != nil {
		t.Fatalf("expected second ensure to skip create, got %#v", second)
	}

	var count int64
	if err := DB.Model(&LotteryActivity{}).
		Where("draw_date = ? AND scope = ? AND open_mode = ?", nowLocal.Format("2006-01-02"), LotteryActivityScopePublic, LotteryActivityOpenModeWeekly).
		Count(&count).Error; err != nil {
		t.Fatalf("failed to count weekly activities: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one weekly activity for the date, got %d", count)
	}
}

func TestEnsureWeeklyPublicLotteryActivitySkipsWhenOtherActivityActive(t *testing.T) {
	setupLotteryTestDB(t)

	nowLocal := time.Date(2026, 4, 15, 10, 0, 0, 0, time.Local)
	weekday := int(nowLocal.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	setLotteryTestSetting(t, weekday)

	adminOnly := &LotteryActivity{
		Scope:           LotteryActivityScopeAdminOnly,
		OpenMode:        LotteryActivityOpenModeImmediate,
		Status:          LotteryActivityStatusActive,
		DrawDate:        nowLocal.Format("2006-01-02"),
		DrawStartsAt:    nowLocal.Unix(),
		DrawEndsAt:      nowLocal.Add(14 * time.Hour).Unix(),
		AutoActivateAt:  nowLocal.Add(14 * time.Hour).Unix(),
		ConsumeStartsAt: nowLocal.Add(14 * time.Hour).Unix(),
		ConsumeEndsAt:   nowLocal.Add(38 * time.Hour).Unix(),
		ExpiresAt:       nowLocal.Add(38 * time.Hour).Unix(),
		ConfigSnapshot:  `{"myth_broadcast_enabled":true,"tiers":[{"name":"普通","amount":3,"probability":100}]}`,
		CreatedBy:       1,
	}
	if err := DB.Create(adminOnly).Error; err != nil {
		t.Fatalf("failed to seed active admin-only activity: %v", err)
	}

	activity, err := EnsureWeeklyPublicLotteryActivity(nowLocal.Unix())
	if err != nil {
		t.Fatalf("expected weekly ensure success, got err=%v", err)
	}
	if activity != nil {
		t.Fatalf("expected weekly ensure to skip while another activity is active, got %#v", activity)
	}

	var count int64
	if err := DB.Model(&LotteryActivity{}).
		Where("draw_date = ? AND scope = ? AND open_mode = ?", nowLocal.Format("2006-01-02"), LotteryActivityScopePublic, LotteryActivityOpenModeWeekly).
		Count(&count).Error; err != nil {
		t.Fatalf("failed to count weekly public activities: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no weekly public activity while admin-only activity is active, got %d", count)
	}
}

func TestEnsureWeeklyPublicLotteryActivityRandomWorkdayUsesStableWeeklyChoice(t *testing.T) {
	setupLotteryTestDB(t)

	weekStart := time.Date(2026, 4, 13, 10, 0, 0, 0, time.Local) // Monday
	setLotteryTestSetting(t, operation_setting.LotteryWeeklyDayRandomWorkday)

	resolvedWeekday := operation_setting.ResolveLotteryWeeklyDay(operation_setting.LotteryWeeklyDayRandomWorkday, weekStart.Unix())
	if resolvedWeekday < 1 || resolvedWeekday > 5 {
		t.Fatalf("expected random workday in [1,5], got %d", resolvedWeekday)
	}

	otherDayOffset := 0
	if resolvedWeekday == 1 {
		otherDayOffset = 1
	}
	otherTime := weekStart.AddDate(0, 0, otherDayOffset)
	second, err := EnsureWeeklyPublicLotteryActivity(otherTime.Unix())
	if err != nil {
		t.Fatalf("expected non-target workday ensure success, got err=%v", err)
	}
	if second != nil {
		t.Fatalf("expected no weekly activity on non-target workday, got %#v", second)
	}

	targetTime := weekStart.AddDate(0, 0, resolvedWeekday-1)
	activity, err := EnsureWeeklyPublicLotteryActivity(targetTime.Unix())
	if err != nil {
		t.Fatalf("expected random workday weekly auto open success, got err=%v", err)
	}
	if activity == nil {
		t.Fatal("expected weekly activity to be created on resolved random workday")
	}
}

func TestGetCurrentLotteryActivityForRoleRespectsAdminOnlyVisibility(t *testing.T) {
	setupLotteryTestDB(t)

	now := time.Now().Unix()
	adminOnly := &LotteryActivity{
		Scope:           LotteryActivityScopeAdminOnly,
		OpenMode:        LotteryActivityOpenModeImmediate,
		Status:          LotteryActivityStatusActive,
		DrawDate:        "2026-04-10",
		DrawStartsAt:    now - 60,
		DrawEndsAt:      now + 60,
		AutoActivateAt:  now + 60,
		ConsumeStartsAt: now + 60,
		ConsumeEndsAt:   now + 86400,
		ExpiresAt:       now + 86400,
		ConfigSnapshot:  `{"myth_broadcast_enabled":true,"tiers":[{"name":"普通","amount":3,"probability":82}]}`,
		CreatedBy:       1,
	}
	if err := DB.Create(adminOnly).Error; err != nil {
		t.Fatalf("failed to seed admin-only activity: %v", err)
	}

	userVisible, err := GetCurrentLotteryActivityForRole(common.RoleCommonUser, now)
	if err != nil {
		t.Fatalf("unexpected error for common user: %v", err)
	}
	if userVisible != nil {
		t.Fatal("expected no visible activity for common user")
	}

	adminVisible, err := GetCurrentLotteryActivityForRole(common.RoleAdminUser, now)
	if err != nil {
		t.Fatalf("unexpected error for admin user: %v", err)
	}
	if adminVisible == nil || adminVisible.Scope != LotteryActivityScopeAdminOnly {
		t.Fatalf("expected admin-only activity for admin, got %#v", adminVisible)
	}
}

func TestDrawGiftAndActivateLotteryRewardFlow(t *testing.T) {
	setupLotteryTestDB(t)

	drawer := &User{
		Username:  "lottery_user",
		AffCode:   "ltu1",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		UsedQuota: int(100 * common.QuotaPerUnit),
	}
	receiver := &User{
		Username: "lottery_receiver",
		AffCode:  "ltu2",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	admin := &User{
		Username: "lottery_admin2",
		AffCode:  "ltu3",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
	}
	for _, user := range []*User{drawer, receiver, admin} {
		if err := DB.Create(user).Error; err != nil {
			t.Fatalf("failed to seed user %s: %v", user.Username, err)
		}
	}

	activity, err := OpenImmediateLotteryActivity(admin.Id, LotteryActivityScopePublic, operation_setting.GetNormalizedLotterySetting())
	if err != nil {
		t.Fatalf("failed to open activity: %v", err)
	}

	now := GetDBTimestamp()
	reward, remainingDrawCount, err := DrawLotteryRewardForUser(drawer, activity, now)
	if err != nil {
		t.Fatalf("failed to draw reward: %v", err)
	}
	if reward == nil {
		t.Fatal("expected reward after draw")
	}
	if remainingDrawCount != 3 {
		t.Fatalf("expected remaining draw count 3 for LV1 first draw, got %d", remainingDrawCount)
	}
	if reward.Status != LotteryRewardStatusPendingActivation {
		t.Fatalf("expected pending reward, got %s", reward.Status)
	}
	expectedQuota := GetLotteryQuotaAmountByUSD(reward.Amount)
	if reward.QuotaTotal != expectedQuota || reward.QuotaRemaining != expectedQuota {
		t.Fatalf("expected full reward quota initialized, got total=%d remaining=%d", reward.QuotaTotal, reward.QuotaRemaining)
	}

	giftedReward, targetUser, err := GiftLotteryRewardByUsername(drawer.Id, activity, reward.Id, receiver.Username, now)
	if err != nil {
		t.Fatalf("failed to gift reward: %v", err)
	}
	if targetUser == nil || targetUser.Id != receiver.Id {
		t.Fatalf("expected receiver user, got %#v", targetUser)
	}
	if giftedReward.OwnerUserId != receiver.Id {
		t.Fatalf("expected owner user id %d, got %d", receiver.Id, giftedReward.OwnerUserId)
	}
	if giftedReward.GiftCount != 1 {
		t.Fatalf("expected gift count 1, got %d", giftedReward.GiftCount)
	}

	receiverRewards, err := ListLotteryRewardsForUserActivity(receiver.Id, activity.Id)
	if err != nil {
		t.Fatalf("failed to load receiver rewards: %v", err)
	}
	if len(receiverRewards) != 1 {
		t.Fatalf("expected receiver to own one reward, got %d", len(receiverRewards))
	}

	activatedCount, err := ActivateAllPendingLotteryRewardsForUser(receiver.Id, activity, now)
	if err != nil {
		t.Fatalf("failed to activate receiver rewards: %v", err)
	}
	if activatedCount != 1 {
		t.Fatalf("expected activated count 1, got %d", activatedCount)
	}

	receiverRewards, err = ListLotteryRewardsForUserActivity(receiver.Id, activity.Id)
	if err != nil {
		t.Fatalf("failed to reload receiver rewards: %v", err)
	}
	if len(receiverRewards) != 1 || receiverRewards[0].Status != LotteryRewardStatusActivated {
		t.Fatalf("expected activated receiver reward, got %#v", receiverRewards)
	}
}

func TestLotteryQuotaPreConsumeSettleAndRefund(t *testing.T) {
	setupLotteryTestDB(t)

	user := &User{
		Username: "lottery_consume_user",
		AffCode:  "ltu4",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	now := time.Now().Unix()
	reward := &LotteryReward{
		ActivityId:      1,
		OwnerUserId:     user.Id,
		SourceUserId:    user.Id,
		SourceDrawIndex: 1,
		TierName:        "传说",
		Amount:          50,
		QuotaTotal:      int(50 * common.QuotaPerUnit),
		QuotaRemaining:  int(50 * common.QuotaPerUnit),
		Status:          LotteryRewardStatusActivated,
		AutoActivateAt:  now - 3600,
		ConsumeStartsAt: now - 1800,
		ExpiresAt:       now + 3600,
		ActivatedAt:     now - 1800,
	}
	if err := DB.Create(reward).Error; err != nil {
		t.Fatalf("failed to seed activated reward: %v", err)
	}

	requestId := "lottery-test-request-1"
	preConsumedQuota, err := PreConsumeLotteryQuota(requestId, user.Id, int(20*common.QuotaPerUnit), now)
	if err != nil {
		t.Fatalf("failed to pre-consume lottery quota: %v", err)
	}
	if preConsumedQuota != int(20*common.QuotaPerUnit) {
		t.Fatalf("expected 20usd pre-consumed, got %d", preConsumedQuota)
	}

	var updatedReward LotteryReward
	if err := DB.First(&updatedReward, reward.Id).Error; err != nil {
		t.Fatalf("failed to load pre-consumed reward: %v", err)
	}
	if updatedReward.QuotaRemaining != int(30*common.QuotaPerUnit) {
		t.Fatalf("expected 30usd remaining after pre-consume, got %d", updatedReward.QuotaRemaining)
	}

	settledQuota, refundedQuota, err := SettleLotteryQuota(requestId, int(12*common.QuotaPerUnit), now)
	if err != nil {
		t.Fatalf("failed to settle lottery quota: %v", err)
	}
	if settledQuota != int(12*common.QuotaPerUnit) || refundedQuota != int(8*common.QuotaPerUnit) {
		t.Fatalf("expected settle=12 refund=8, got settle=%d refund=%d", settledQuota, refundedQuota)
	}

	if err := DB.First(&updatedReward, reward.Id).Error; err != nil {
		t.Fatalf("failed to reload settled reward: %v", err)
	}
	if updatedReward.QuotaRemaining != int(38*common.QuotaPerUnit) {
		t.Fatalf("expected 38usd remaining after settle, got %d", updatedReward.QuotaRemaining)
	}
	if updatedReward.Status != LotteryRewardStatusActivated {
		t.Fatalf("expected reward to stay activated after partial settle, got %s", updatedReward.Status)
	}

	requestId2 := "lottery-test-request-2"
	preConsumedQuota, err = PreConsumeLotteryQuota(requestId2, user.Id, int(5*common.QuotaPerUnit), now)
	if err != nil {
		t.Fatalf("failed to pre-consume second lottery quota: %v", err)
	}
	if preConsumedQuota != int(5*common.QuotaPerUnit) {
		t.Fatalf("expected 5usd second pre-consume, got %d", preConsumedQuota)
	}
	if err := RefundLotteryQuotaPreConsume(requestId2, now); err != nil {
		t.Fatalf("failed to refund second pre-consume: %v", err)
	}

	if err := DB.First(&updatedReward, reward.Id).Error; err != nil {
		t.Fatalf("failed to reload refunded reward: %v", err)
	}
	if updatedReward.QuotaRemaining != int(38*common.QuotaPerUnit) {
		t.Fatalf("expected refunded reward remaining to restore to 38usd, got %d", updatedReward.QuotaRemaining)
	}
}

func TestReconcileLotteryRuntimeStateAutoActivatesAndExpires(t *testing.T) {
	setupLotteryTestDB(t)

	now := time.Now().Unix()
	activity := &LotteryActivity{
		Scope:           LotteryActivityScopePublic,
		OpenMode:        LotteryActivityOpenModeImmediate,
		Status:          LotteryActivityStatusActive,
		DrawDate:        "2026-04-10",
		DrawStartsAt:    now - 3600,
		DrawEndsAt:      now - 1800,
		AutoActivateAt:  now - 1200,
		ConsumeStartsAt: now - 1200,
		ConsumeEndsAt:   now + 1800,
		ExpiresAt:       now + 1800,
		ConfigSnapshot:  `{"myth_broadcast_enabled":true,"tiers":[{"name":"普通","amount":3,"probability":100}]}`,
		CreatedBy:       1,
	}
	if err := DB.Create(activity).Error; err != nil {
		t.Fatalf("failed to seed activity: %v", err)
	}

	reward := &LotteryReward{
		ActivityId:      activity.Id,
		OwnerUserId:     1,
		SourceUserId:    1,
		SourceDrawIndex: 1,
		TierName:        "普通",
		Amount:          3,
		Status:          LotteryRewardStatusPendingActivation,
		AutoActivateAt:  activity.AutoActivateAt,
		ConsumeStartsAt: activity.ConsumeStartsAt,
		ExpiresAt:       activity.ExpiresAt,
	}
	if err := DB.Create(reward).Error; err != nil {
		t.Fatalf("failed to seed reward: %v", err)
	}

	if err := ReconcileLotteryRuntimeState(now); err != nil {
		t.Fatalf("failed to reconcile runtime state: %v", err)
	}

	var updatedReward LotteryReward
	if err := DB.First(&updatedReward, reward.Id).Error; err != nil {
		t.Fatalf("failed to load updated reward: %v", err)
	}
	if updatedReward.Status != LotteryRewardStatusActivated {
		t.Fatalf("expected reward activated after reconcile, got %s", updatedReward.Status)
	}
	if updatedReward.ActivatedAt != activity.AutoActivateAt {
		t.Fatalf("expected activated_at to match auto_activate_at, got %d", updatedReward.ActivatedAt)
	}

	activity.ExpiresAt = now - 10
	activity.ConsumeEndsAt = now - 10
	if err := DB.Save(activity).Error; err != nil {
		t.Fatalf("failed to update activity expiry: %v", err)
	}
	if err := DB.Model(&LotteryReward{}).Where("id = ?", reward.Id).Update("expires_at", now-10).Error; err != nil {
		t.Fatalf("failed to update reward expiry: %v", err)
	}

	if err := ReconcileLotteryRuntimeState(now); err != nil {
		t.Fatalf("failed to reconcile expired runtime state: %v", err)
	}

	if err := DB.First(&updatedReward, reward.Id).Error; err != nil {
		t.Fatalf("failed to reload expired reward: %v", err)
	}
	if updatedReward.Status != LotteryRewardStatusExpired {
		t.Fatalf("expected reward expired after reconcile, got %s", updatedReward.Status)
	}

	var updatedActivity LotteryActivity
	if err := DB.First(&updatedActivity, activity.Id).Error; err != nil {
		t.Fatalf("failed to reload updated activity: %v", err)
	}
	if updatedActivity.Status != LotteryActivityStatusClosed {
		t.Fatalf("expected activity closed after reconcile, got %s", updatedActivity.Status)
	}
}
