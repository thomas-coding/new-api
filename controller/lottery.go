package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type lotteryTierResponse struct {
	Name        string  `json:"name"`
	Amount      int     `json:"amount"`
	Probability float64 `json:"probability"`
}

type lotteryActivityResponse struct {
	Id                   int                   `json:"id"`
	Scope                string                `json:"scope"`
	OpenMode             string                `json:"open_mode"`
	Status               string                `json:"status"`
	Phase                string                `json:"phase"`
	DrawDate             string                `json:"draw_date"`
	DrawStartsAt         int64                 `json:"draw_starts_at"`
	DrawEndsAt           int64                 `json:"draw_ends_at"`
	AutoActivateAt       int64                 `json:"auto_activate_at"`
	ConsumeStartsAt      int64                 `json:"consume_starts_at"`
	ConsumeEndsAt        int64                 `json:"consume_ends_at"`
	ExpiresAt            int64                 `json:"expires_at"`
	MythBroadcastEnabled bool                  `json:"myth_broadcast_enabled"`
	Tiers                []lotteryTierResponse `json:"tiers"`
}

type lotteryRewardResponse struct {
	Id               int     `json:"id"`
	SourceDrawIndex  int     `json:"source_draw_index"`
	TierName         string  `json:"tier_name"`
	Amount           int     `json:"amount"`
	QuotaTotal       int     `json:"quota_total"`
	QuotaRemaining   int     `json:"quota_remaining"`
	RemainingAmount  float64 `json:"remaining_amount"`
	Status           string  `json:"status"`
	SourceUsername   string  `json:"source_username"`
	OwnerUsername    string  `json:"owner_username"`
	GiftedByUsername string  `json:"gifted_by_username"`
	GiftedToUsername string  `json:"gifted_to_username"`
	GiftCount        int     `json:"gift_count"`
	CreatedAt        int64   `json:"created_at"`
	ActivatedAt      int64   `json:"activated_at"`
	ConsumedAt       int64   `json:"consumed_at"`
	ExpiredAt        int64   `json:"expired_at"`
	GiftedAt         int64   `json:"gifted_at"`
	ConsumeStartsAt  int64   `json:"consume_starts_at"`
	ExpiresAt        int64   `json:"expires_at"`
	CanActivate      bool    `json:"can_activate"`
	CanGift          bool    `json:"can_gift"`
	CanConsume       bool    `json:"can_consume"`
}

type lotteryRewardSummaryResponse struct {
	TotalCount         int     `json:"total_count"`
	TotalAmount        float64 `json:"total_amount"`
	PendingCount       int     `json:"pending_count"`
	PendingAmount      float64 `json:"pending_amount"`
	ActivatedCount     int     `json:"activated_count"`
	ActivatedAmount    float64 `json:"activated_amount"`
	ConsumableCount    int     `json:"consumable_count"`
	ConsumableAmount   float64 `json:"consumable_amount"`
	GiftableCount      int     `json:"giftable_count"`
	ActivatableCount   int     `json:"activatable_count"`
	ReceivedCount      int     `json:"received_count"`
	ReceivedAmount     float64 `json:"received_amount"`
	UsedDrawCount      int     `json:"used_draw_count"`
	RemainingDrawCount int     `json:"remaining_draw_count"`
}

type lotteryGiftRequest struct {
	RewardId       int    `json:"reward_id"`
	TargetUsername string `json:"target_username"`
}

func convertLotteryTiers(tiers []operation_setting.LotteryTierSetting) []lotteryTierResponse {
	resp := make([]lotteryTierResponse, 0, len(tiers))
	for _, tier := range tiers {
		resp = append(resp, lotteryTierResponse{
			Name:        tier.Name,
			Amount:      tier.Amount,
			Probability: tier.Probability,
		})
	}
	return resp
}

func buildLotteryActivityResponse(activity *model.LotteryActivity, now int64) (*lotteryActivityResponse, error) {
	if activity == nil {
		return nil, nil
	}
	snapshot, err := activity.GetConfigSnapshot()
	if err != nil {
		return nil, err
	}
	resp := &lotteryActivityResponse{
		Id:              activity.Id,
		Scope:           activity.Scope,
		OpenMode:        activity.OpenMode,
		Status:          activity.Status,
		Phase:           activity.GetPhase(now),
		DrawDate:        activity.DrawDate,
		DrawStartsAt:    activity.DrawStartsAt,
		DrawEndsAt:      activity.DrawEndsAt,
		AutoActivateAt:  activity.AutoActivateAt,
		ConsumeStartsAt: activity.ConsumeStartsAt,
		ConsumeEndsAt:   activity.ConsumeEndsAt,
		ExpiresAt:       activity.ExpiresAt,
	}
	if snapshot != nil {
		resp.MythBroadcastEnabled = snapshot.MythBroadcastEnabled
		resp.Tiers = convertLotteryTiers(snapshot.Tiers)
	}
	return resp, nil
}

func buildLotteryRewardPool(activity *model.LotteryActivity, fallback operation_setting.LotterySetting) []lotteryTierResponse {
	if activity != nil {
		if snapshot, err := activity.GetConfigSnapshot(); err == nil && snapshot != nil && len(snapshot.Tiers) > 0 {
			return convertLotteryTiers(snapshot.Tiers)
		}
	}
	return convertLotteryTiers(fallback.Tiers)
}

func buildLotteryRewardResponses(rewards []model.LotteryReward, now int64) []lotteryRewardResponse {
	usernames := make(map[int]string)
	for _, reward := range rewards {
		for _, userId := range []int{reward.SourceUserId, reward.OwnerUserId, reward.GiftedByUserId, reward.GiftedToUserId} {
			if userId <= 0 {
				continue
			}
			if _, ok := usernames[userId]; ok {
				continue
			}
			username, err := model.GetUsernameById(userId, false)
			if err != nil {
				usernames[userId] = ""
				continue
			}
			usernames[userId] = username
		}
	}

	resp := make([]lotteryRewardResponse, 0, len(rewards))
	for _, reward := range rewards {
		resp = append(resp, lotteryRewardResponse{
			Id:               reward.Id,
			SourceDrawIndex:  reward.SourceDrawIndex,
			TierName:         reward.TierName,
			Amount:           reward.Amount,
			QuotaTotal:       reward.QuotaTotal,
			QuotaRemaining:   reward.QuotaRemaining,
			RemainingAmount:  reward.RemainingAmountUSD(),
			Status:           reward.Status,
			SourceUsername:   usernames[reward.SourceUserId],
			OwnerUsername:    usernames[reward.OwnerUserId],
			GiftedByUsername: usernames[reward.GiftedByUserId],
			GiftedToUsername: usernames[reward.GiftedToUserId],
			GiftCount:        reward.GiftCount,
			CreatedAt:        reward.CreatedAt,
			ActivatedAt:      reward.ActivatedAt,
			ConsumedAt:       reward.ConsumedAt,
			ExpiredAt:        reward.ExpiredAt,
			GiftedAt:         reward.GiftedAt,
			ConsumeStartsAt:  reward.ConsumeStartsAt,
			ExpiresAt:        reward.ExpiresAt,
			CanActivate:      reward.CanActivateAt(now),
			CanGift:          reward.CanGiftAt(now),
			CanConsume:       reward.CanConsumeAt(now),
		})
	}
	return resp
}

func buildLotteryRewardSummary(rewards []model.LotteryReward, usedDrawCount int, drawCount int, now int64) lotteryRewardSummaryResponse {
	summary := lotteryRewardSummaryResponse{
		UsedDrawCount:      usedDrawCount,
		RemainingDrawCount: drawCount - usedDrawCount,
	}
	if summary.RemainingDrawCount < 0 {
		summary.RemainingDrawCount = 0
	}
	for _, reward := range rewards {
		summary.TotalCount++
		summary.TotalAmount += reward.RemainingAmountUSD()

		if reward.SourceUserId != reward.OwnerUserId {
			summary.ReceivedCount++
			summary.ReceivedAmount += reward.RemainingAmountUSD()
		}
		if reward.CanActivateAt(now) {
			summary.ActivatableCount++
		}
		if reward.CanGiftAt(now) {
			summary.GiftableCount++
		}
		if reward.CanConsumeAt(now) {
			summary.ConsumableCount++
			summary.ConsumableAmount += reward.RemainingAmountUSD()
		}

		switch reward.Status {
		case model.LotteryRewardStatusPendingActivation:
			summary.PendingCount++
			summary.PendingAmount += reward.RemainingAmountUSD()
		case model.LotteryRewardStatusActivated:
			summary.ActivatedCount++
			summary.ActivatedAmount += reward.RemainingAmountUSD()
		}
	}
	return summary
}

func getLotteryActivityForUserRole(role int, now int64) (*model.LotteryActivity, error) {
	if err := model.ReconcileLotteryRuntimeState(now); err != nil {
		return nil, err
	}
	if _, err := model.EnsureWeeklyPublicLotteryActivity(now); err != nil {
		return nil, err
	}
	return model.GetCurrentLotteryActivityForRole(role, now)
}

func buildLotterySelfStatePayload(user *model.User) (gin.H, error) {
	now := model.GetDBTimestamp()
	setting := operation_setting.GetNormalizedLotterySetting()
	level, drawCount, consumedAmountUSD, eligible, adminOverride := model.GetLotteryProfileFromUser(user)
	activity, err := getLotteryActivityForUserRole(user.Role, now)
	if err != nil {
		return nil, err
	}

	activityResp, err := buildLotteryActivityResponse(activity, now)
	if err != nil {
		return nil, err
	}

	rewards := make([]model.LotteryReward, 0)
	rewardResponses := make([]lotteryRewardResponse, 0)
	usedDrawCount := 0
	panelVisible := false
	if activity != nil {
		rewards, err = model.ListLotteryRewardsForUserActivity(user.Id, activity.Id)
		if err != nil {
			return nil, err
		}
		usedDrawCount64, err := model.CountLotteryRewardDrawsForUser(activity.Id, user.Id)
		if err != nil {
			return nil, err
		}
		usedDrawCount = int(usedDrawCount64)
		rewardResponses = buildLotteryRewardResponses(rewards, now)
		panelVisible = activity.IsVisibleToRole(user.Role) && (eligible || len(rewards) > 0)
	}

	rewardSummary := buildLotteryRewardSummary(rewards, usedDrawCount, drawCount, now)
	canParticipate := activity != nil &&
		panelVisible &&
		eligible &&
		activity.IsDrawOpenAt(now) &&
		rewardSummary.RemainingDrawCount > 0

	return gin.H{
		"panel_visible":             panelVisible,
		"eligible_level_required":   1,
		"current_level":             level,
		"draw_count":                drawCount,
		"used_draw_count":           rewardSummary.UsedDrawCount,
		"remaining_draw_count":      rewardSummary.RemainingDrawCount,
		"can_participate":           canParticipate,
		"eligible":                  eligible,
		"admin_override":            adminOverride,
		"consumed_amount_usd":       consumedAmountUSD,
		"weekly_day":                setting.WeeklyDay,
		"myth_broadcast_enabled":    setting.MythBroadcastEnabled,
		"reward_pool":               buildLotteryRewardPool(activity, setting),
		"activity":                  activityResp,
		"reward_summary":            rewardSummary,
		"rewards":                   rewardResponses,
		"server_now":                now,
		"current_demo_route_exists": true,
	}, nil
}

func GetLotterySelfState(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	payload, err := buildLotterySelfStatePayload(user)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, payload)
}

func DrawLotteryReward(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	now := model.GetDBTimestamp()
	activity, err := getLotteryActivityForUserRole(user.Role, now)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if activity == nil || !activity.IsVisibleToRole(user.Role) {
		common.ApiErrorMsg(c, "当前没有可参与的大乐透活动")
		return
	}

	reward, remainingDrawCount, err := model.DrawLotteryRewardForUser(user, activity, now)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrLotteryUserNotEligible):
			common.ApiErrorMsg(c, "当前等级不足，LV1 起可参与大乐透")
		case errors.Is(err, model.ErrLotteryDrawLimitReached):
			common.ApiErrorMsg(c, "本场大乐透可抽次数已用完")
		case errors.Is(err, model.ErrLotteryActivityNotOpen):
			common.ApiErrorMsg(c, "当前不在大乐透抽奖时间")
		default:
			common.ApiError(c, err)
		}
		return
	}

	common.ApiSuccess(c, gin.H{
		"reward":               buildLotteryRewardResponses([]model.LotteryReward{*reward}, now)[0],
		"remaining_draw_count": remainingDrawCount,
	})
}

func ActivateLotteryRewards(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	now := model.GetDBTimestamp()
	activity, err := getLotteryActivityForUserRole(user.Role, now)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if activity == nil || !activity.IsVisibleToRole(user.Role) {
		common.ApiErrorMsg(c, "当前没有可操作的大乐透活动")
		return
	}

	activatedCount, err := model.ActivateAllPendingLotteryRewardsForUser(user.Id, activity, now)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"activated_count": activatedCount,
	})
}

func GiftLotteryReward(c *gin.Context) {
	var req lotteryGiftRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.RewardId <= 0 || req.TargetUsername == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	now := model.GetDBTimestamp()
	activity, err := getLotteryActivityForUserRole(user.Role, now)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if activity == nil || !activity.IsVisibleToRole(user.Role) {
		common.ApiErrorMsg(c, "当前没有可操作的大乐透活动")
		return
	}

	reward, targetUser, err := model.GiftLotteryRewardByUsername(user.Id, activity, req.RewardId, req.TargetUsername, now)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrLotteryGiftTargetNotFound):
			common.ApiErrorMsg(c, "目标用户名不存在或不可用")
		case errors.Is(err, model.ErrLotteryGiftTargetSelf):
			common.ApiErrorMsg(c, "不能把乐透券赠送给自己")
		case errors.Is(err, model.ErrLotteryRewardAlreadyGifted):
			common.ApiErrorMsg(c, "这张乐透券已经赠送过，不能再次转赠")
		case errors.Is(err, model.ErrLotteryRewardNotPending):
			common.ApiErrorMsg(c, "只有未激活的乐透券才能赠送")
		case errors.Is(err, model.ErrLotteryRewardNotFound):
			common.ApiErrorMsg(c, "未找到这张乐透券")
		case errors.Is(err, model.ErrLotteryGiftTargetInvalid):
			common.ApiErrorMsg(c, "当前活动下不能赠送给该用户")
		default:
			common.ApiError(c, err)
		}
		return
	}

	common.ApiSuccess(c, gin.H{
		"reward":          buildLotteryRewardResponses([]model.LotteryReward{*reward}, now)[0],
		"target_username": targetUser.Username,
	})
}

func GetLotteryAdminActive(c *gin.Context) {
	now := model.GetDBTimestamp()
	if err := model.ReconcileLotteryRuntimeState(now); err != nil {
		common.ApiError(c, err)
		return
	}

	setting := operation_setting.GetNormalizedLotterySetting()
	activity, err := model.GetAnyActiveLotteryActivity(now)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	activityResp, err := buildLotteryActivityResponse(activity, now)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"activity":               activityResp,
		"weekly_day":             setting.WeeklyDay,
		"myth_broadcast_enabled": setting.MythBroadcastEnabled,
		"reward_pool":            buildLotteryRewardPool(activity, setting),
	})
}

func AdminOpenPublicLotteryActivity(c *gin.Context) {
	adminOpenLotteryActivity(c, model.LotteryActivityScopePublic)
}

func AdminOpenAdminTestLotteryActivity(c *gin.Context) {
	adminOpenLotteryActivity(c, model.LotteryActivityScopeAdminOnly)
}

func adminOpenLotteryActivity(c *gin.Context, scope string) {
	setting := operation_setting.GetNormalizedLotterySetting()
	activity, err := model.OpenImmediateLotteryActivity(c.GetInt("id"), scope, setting)
	if err != nil {
		if errors.Is(err, model.ErrLotteryActivityAlreadyActive) {
			common.ApiErrorMsg(c, "当前已有进行中的大乐透活动")
			return
		}
		common.ApiError(c, err)
		return
	}
	activityResp, err := buildLotteryActivityResponse(activity, model.GetDBTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"activity": activityResp,
	})
}
