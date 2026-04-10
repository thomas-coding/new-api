package controller

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type lotterySelfStatePayload struct {
	PanelVisible bool `json:"panel_visible"`
	Eligible     bool `json:"eligible"`
	Activity     *struct {
		ID    int    `json:"id"`
		Scope string `json:"scope"`
	} `json:"activity"`
	Rewards []struct {
		ID            int    `json:"id"`
		Status        string `json:"status"`
		OwnerUsername string `json:"owner_username"`
	} `json:"rewards"`
	RewardSummary struct {
		ActivatableCount int `json:"activatable_count"`
	} `json:"reward_summary"`
}

type lotteryDrawPayload struct {
	Reward struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	} `json:"reward"`
	RemainingDrawCount int `json:"remaining_draw_count"`
}

type lotteryGiftPayload struct {
	Reward struct {
		ID            int    `json:"id"`
		OwnerUsername string `json:"owner_username"`
	} `json:"reward"`
	TargetUsername string `json:"target_username"`
}

type lotteryActivatePayload struct {
	ActivatedCount int64 `json:"activated_count"`
}

func setupLotteryControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	if err := db.AutoMigrate(&model.User{}, &model.LotteryActivity{}, &model.LotteryReward{}, &model.LotteryConsumeRecord{}); err != nil {
		t.Fatalf("failed to migrate lottery controller tables: %v", err)
	}

	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.RedisEnabled = oldRedisEnabled
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedLotteryControllerUser(t *testing.T, db *gorm.DB, id int, username string, role int, usedQuota int) *model.User {
	t.Helper()
	user := &model.User{
		Id:        id,
		Username:  username,
		Password:  "password123",
		Role:      role,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   fmt.Sprintf("aff_%d", id),
		UsedQuota: usedQuota,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user %s: %v", username, err)
	}
	return user
}

func TestLotteryControllerDrawGiftActivateFlow(t *testing.T) {
	db := setupLotteryControllerTestDB(t)

	drawer := seedLotteryControllerUser(t, db, 11, "lottery_drawer", common.RoleCommonUser, int(100*common.QuotaPerUnit))
	receiver := seedLotteryControllerUser(t, db, 12, "lottery_receiver", common.RoleCommonUser, 0)
	admin := seedLotteryControllerUser(t, db, 13, "lottery_admin", common.RoleAdminUser, 0)

	activity, err := model.OpenImmediateLotteryActivity(admin.Id, model.LotteryActivityScopePublic, operation_setting.GetNormalizedLotterySetting())
	if err != nil {
		t.Fatalf("failed to open lottery activity: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/lottery/self/state", nil, drawer.Id)
	GetLotterySelfState(ctx)

	stateResp := decodeAPIResponse(t, recorder)
	if !stateResp.Success {
		t.Fatalf("expected self state success, got message=%s body=%s", stateResp.Message, recorder.Body.String())
	}
	var state lotterySelfStatePayload
	if err := common.Unmarshal(stateResp.Data, &state); err != nil {
		t.Fatalf("failed to decode lottery self state: %v", err)
	}
	if !state.PanelVisible || !state.Eligible || state.Activity == nil || state.Activity.ID != activity.Id {
		t.Fatalf("unexpected self state payload: %+v", state)
	}

	drawCtx, drawRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/lottery/self/draw", nil, drawer.Id)
	DrawLotteryReward(drawCtx)
	drawResp := decodeAPIResponse(t, drawRecorder)
	if !drawResp.Success {
		t.Fatalf("expected draw success, got message=%s body=%s", drawResp.Message, drawRecorder.Body.String())
	}
	var drawData lotteryDrawPayload
	if err := common.Unmarshal(drawResp.Data, &drawData); err != nil {
		t.Fatalf("failed to decode draw payload: %v", err)
	}
	if drawData.Reward.ID <= 0 || drawData.Reward.Status != model.LotteryRewardStatusPendingActivation {
		t.Fatalf("unexpected draw payload: %+v", drawData)
	}

	giftCtx, giftRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/lottery/self/gift", map[string]any{
		"reward_id":       drawData.Reward.ID,
		"target_username": receiver.Username,
	}, drawer.Id)
	GiftLotteryReward(giftCtx)
	giftResp := decodeAPIResponse(t, giftRecorder)
	if !giftResp.Success {
		t.Fatalf("expected gift success, got message=%s body=%s", giftResp.Message, giftRecorder.Body.String())
	}
	var giftData lotteryGiftPayload
	if err := common.Unmarshal(giftResp.Data, &giftData); err != nil {
		t.Fatalf("failed to decode gift payload: %v", err)
	}
	if giftData.TargetUsername != receiver.Username || giftData.Reward.OwnerUsername != receiver.Username {
		t.Fatalf("unexpected gift payload: %+v", giftData)
	}

	activateCtx, activateRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/lottery/self/activate", nil, receiver.Id)
	ActivateLotteryRewards(activateCtx)
	activateResp := decodeAPIResponse(t, activateRecorder)
	if !activateResp.Success {
		t.Fatalf("expected activate success, got message=%s body=%s", activateResp.Message, activateRecorder.Body.String())
	}
	var activateData lotteryActivatePayload
	if err := common.Unmarshal(activateResp.Data, &activateData); err != nil {
		t.Fatalf("failed to decode activate payload: %v", err)
	}
	if activateData.ActivatedCount != 1 {
		t.Fatalf("expected one activated reward, got %+v", activateData)
	}

	receiverStateCtx, receiverStateRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/lottery/self/state", nil, receiver.Id)
	GetLotterySelfState(receiverStateCtx)
	receiverStateResp := decodeAPIResponse(t, receiverStateRecorder)
	if !receiverStateResp.Success {
		t.Fatalf("expected receiver self state success, got message=%s body=%s", receiverStateResp.Message, receiverStateRecorder.Body.String())
	}
	var receiverState lotterySelfStatePayload
	if err := common.Unmarshal(receiverStateResp.Data, &receiverState); err != nil {
		t.Fatalf("failed to decode receiver state payload: %v", err)
	}
	if len(receiverState.Rewards) != 1 || receiverState.Rewards[0].Status != model.LotteryRewardStatusActivated {
		t.Fatalf("unexpected receiver rewards: %+v", receiverState.Rewards)
	}
}

func TestLotteryControllerAdminOnlyActivityHiddenFromCommonUser(t *testing.T) {
	db := setupLotteryControllerTestDB(t)

	commonUser := seedLotteryControllerUser(t, db, 21, "lottery_common", common.RoleCommonUser, int(100*common.QuotaPerUnit))
	admin := seedLotteryControllerUser(t, db, 22, "lottery_admin_only", common.RoleAdminUser, 0)

	if _, err := model.OpenImmediateLotteryActivity(admin.Id, model.LotteryActivityScopeAdminOnly, operation_setting.GetNormalizedLotterySetting()); err != nil {
		t.Fatalf("failed to open admin-only activity: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/lottery/self/state", nil, commonUser.Id)
	GetLotterySelfState(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected self state success, got message=%s body=%s", resp.Message, recorder.Body.String())
	}
	var state lotterySelfStatePayload
	if err := common.Unmarshal(resp.Data, &state); err != nil {
		t.Fatalf("failed to decode admin-only hidden state: %v", err)
	}
	if state.PanelVisible {
		t.Fatalf("expected panel to stay hidden for common user, got %+v", state)
	}
	if state.Activity != nil {
		t.Fatalf("expected no visible activity for common user, got %+v", state.Activity)
	}
}

func TestLotteryControllerWeeklyOpenAppearsOnSelfState(t *testing.T) {
	db := setupLotteryControllerTestDB(t)
	_ = db

	current := operation_setting.GetLotterySetting()
	original := *current
	original.Tiers = append([]operation_setting.LotteryTierSetting(nil), current.Tiers...)
	t.Cleanup(func() {
		restored := original
		restored.Tiers = append([]operation_setting.LotteryTierSetting(nil), original.Tiers...)
		*current = restored
	})

	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	*current = operation_setting.LotterySetting{
		WeeklyDay:            weekday,
		MythBroadcastEnabled: true,
		Tiers:                append([]operation_setting.LotteryTierSetting(nil), operation_setting.GetNormalizedLotterySetting().Tiers...),
	}

	user := seedLotteryControllerUser(t, model.DB, 31, "weekly_user", common.RoleCommonUser, int(100*common.QuotaPerUnit))

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/lottery/self/state", nil, user.Id)
	GetLotterySelfState(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected weekly self state success, got message=%s body=%s", resp.Message, recorder.Body.String())
	}
	var state lotterySelfStatePayload
	if err := common.Unmarshal(resp.Data, &state); err != nil {
		t.Fatalf("failed to decode weekly self state: %v", err)
	}
	if state.Activity == nil || state.Activity.Scope != model.LotteryActivityScopePublic {
		t.Fatalf("expected weekly public activity to appear, got %+v", state.Activity)
	}
}
