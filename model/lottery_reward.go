package model

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

const (
	LotteryRewardStatusPendingActivation = "pending_activation"
	LotteryRewardStatusActivated         = "activated"
	LotteryRewardStatusConsumed          = "consumed"
	LotteryRewardStatusExpired           = "expired"
)

var (
	ErrLotteryActivityNotOpen     = errors.New("lottery activity is not open")
	ErrLotteryActivityNotVisible  = errors.New("lottery activity is not visible")
	ErrLotteryUserNotEligible     = errors.New("lottery user is not eligible")
	ErrLotteryDrawLimitReached    = errors.New("lottery draw limit reached")
	ErrLotteryRewardNotFound      = errors.New("lottery reward not found")
	ErrLotteryRewardNotPending    = errors.New("lottery reward is not pending")
	ErrLotteryRewardAlreadyGifted = errors.New("lottery reward already gifted")
	ErrLotteryGiftTargetNotFound  = errors.New("lottery gift target not found")
	ErrLotteryGiftTargetInvalid   = errors.New("lottery gift target invalid")
	ErrLotteryGiftTargetSelf      = errors.New("lottery gift target is self")
	ErrLotteryConfigInvalid       = errors.New("lottery config invalid")
	ErrLotteryConsumeConflict     = errors.New("lottery consume conflict")
)

type LotteryReward struct {
	Id              int    `json:"id"`
	ActivityId      int    `json:"activity_id" gorm:"uniqueIndex:idx_lottery_rewards_draw_seq;index;not null"`
	OwnerUserId     int    `json:"owner_user_id" gorm:"index;not null"`
	SourceUserId    int    `json:"source_user_id" gorm:"uniqueIndex:idx_lottery_rewards_draw_seq;index;not null"`
	SourceDrawIndex int    `json:"source_draw_index" gorm:"uniqueIndex:idx_lottery_rewards_draw_seq;not null"`
	TierName        string `json:"tier_name" gorm:"type:varchar(32);not null"`
	Amount          int    `json:"amount" gorm:"not null"`
	QuotaTotal      int    `json:"quota_total" gorm:"type:int;not null;default:0"`
	QuotaRemaining  int    `json:"quota_remaining" gorm:"type:int;not null;default:0"`
	Status          string `json:"status" gorm:"type:varchar(32);index;not null;default:'pending_activation'"`
	AutoActivateAt  int64  `json:"auto_activate_at" gorm:"bigint;index;not null"`
	ConsumeStartsAt int64  `json:"consume_starts_at" gorm:"bigint;index;not null"`
	ExpiresAt       int64  `json:"expires_at" gorm:"bigint;index;not null"`
	ActivatedAt     int64  `json:"activated_at" gorm:"bigint;default:0"`
	ConsumedAt      int64  `json:"consumed_at" gorm:"bigint;default:0"`
	ExpiredAt       int64  `json:"expired_at" gorm:"bigint;default:0"`
	GiftCount       int    `json:"gift_count" gorm:"type:int;default:0"`
	GiftedAt        int64  `json:"gifted_at" gorm:"bigint;default:0"`
	GiftedByUserId  int    `json:"gifted_by_user_id" gorm:"index;default:0"`
	GiftedToUserId  int    `json:"gifted_to_user_id" gorm:"index;default:0"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
}

func (LotteryReward) TableName() string {
	return "lottery_rewards"
}

func (r *LotteryReward) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *LotteryReward) BeforeUpdate(_ *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func (r *LotteryReward) CanActivateAt(now int64) bool {
	if r == nil {
		return false
	}
	return r.Status == LotteryRewardStatusPendingActivation && now < r.AutoActivateAt
}

func (r *LotteryReward) CanGiftAt(now int64) bool {
	if r == nil {
		return false
	}
	return r.Status == LotteryRewardStatusPendingActivation && r.GiftCount == 0 && now < r.AutoActivateAt
}

func (r *LotteryReward) CanConsumeAt(now int64) bool {
	if r == nil {
		return false
	}
	return r.Status == LotteryRewardStatusActivated && r.QuotaRemaining > 0 && now >= r.ConsumeStartsAt && now < r.ExpiresAt
}

func (r *LotteryReward) FaceAmountUSD() float64 {
	if r == nil {
		return 0
	}
	return float64(r.Amount)
}

func (r *LotteryReward) RemainingAmountUSD() float64 {
	if r == nil {
		return 0
	}
	return GetLotteryAmountUSDByQuota(r.QuotaRemaining)
}

func GetLotteryQuotaAmountByUSD(amount int) int {
	if amount <= 0 {
		return 0
	}
	return int(float64(amount) * common.QuotaPerUnit)
}

func GetLotteryAmountUSDByQuota(quota int) float64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}

func GetLotteryProfileFromUser(user *User) (int, int, float64, bool, bool) {
	if user == nil {
		return 0, 0, 0, false, false
	}
	consumedAmountUSD := GetRegistrationInviteConsumedAmountUSD(user.UsedQuota)
	level := GetRegistrationInviteLevelByConsumedAmountUSD(consumedAmountUSD)
	drawCount := CalculateLotteryDrawCountForLevel(level)
	eligible := level >= 1
	adminOverride := false
	if user.Role >= common.RoleAdminUser && drawCount < CalculateLotteryDrawCountForLevel(1) {
		drawCount = CalculateLotteryDrawCountForLevel(1)
		eligible = true
		adminOverride = true
	}
	return level, drawCount, consumedAmountUSD, eligible, adminOverride
}

func GetLotteryRewardCountForUserActivity(ownerUserId int, activityId int) (int64, error) {
	if ownerUserId <= 0 || activityId <= 0 {
		return 0, nil
	}
	var count int64
	err := DB.Model(&LotteryReward{}).
		Where("owner_user_id = ? AND activity_id = ?", ownerUserId, activityId).
		Count(&count).Error
	return count, err
}

func CountLotteryRewardDrawsForUser(activityId int, sourceUserId int) (int64, error) {
	if activityId <= 0 || sourceUserId <= 0 {
		return 0, nil
	}
	var count int64
	err := DB.Model(&LotteryReward{}).
		Where("activity_id = ? AND source_user_id = ?", activityId, sourceUserId).
		Count(&count).Error
	return count, err
}

func ListLotteryRewardsForUserActivity(ownerUserId int, activityId int) ([]LotteryReward, error) {
	if ownerUserId <= 0 || activityId <= 0 {
		return []LotteryReward{}, nil
	}
	var rewards []LotteryReward
	err := DB.Where("owner_user_id = ? AND activity_id = ?", ownerUserId, activityId).
		Order("created_at desc, id desc").
		Find(&rewards).Error
	if err != nil {
		return nil, err
	}
	return rewards, nil
}

func ReconcileLotteryRuntimeState(now int64) error {
	if now <= 0 {
		now = GetDBTimestamp()
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		return reconcileLotteryRuntimeStateTx(tx, now)
	})
}

func reconcileLotteryRuntimeStateTx(tx *gorm.DB, now int64) error {
	if tx == nil {
		return nil
	}
	if err := tx.Model(&LotteryReward{}).
		Where("status = ? AND auto_activate_at <= ? AND expires_at > ?", LotteryRewardStatusPendingActivation, now, now).
		Updates(map[string]any{
			"status":       LotteryRewardStatusActivated,
			"activated_at": gorm.Expr("auto_activate_at"),
			"updated_at":   now,
		}).Error; err != nil {
		return err
	}

	if err := tx.Model(&LotteryReward{}).
		Where("status IN ? AND expires_at <= ?", []string{LotteryRewardStatusPendingActivation, LotteryRewardStatusActivated}, now).
		Updates(map[string]any{
			"status":     LotteryRewardStatusExpired,
			"expired_at": now,
			"updated_at": now,
		}).Error; err != nil {
		return err
	}

	if err := tx.Model(&LotteryActivity{}).
		Where("status = ? AND expires_at <= ?", LotteryActivityStatusActive, now).
		Updates(map[string]any{
			"status":     LotteryActivityStatusClosed,
			"closed_at":  now,
			"updated_at": now,
		}).Error; err != nil {
		return err
	}
	return nil
}

func DrawLotteryRewardForUser(user *User, activity *LotteryActivity, now int64) (*LotteryReward, int, error) {
	if user == nil {
		return nil, 0, ErrLotteryUserNotEligible
	}
	if activity == nil {
		return nil, 0, ErrLotteryActivityNotOpen
	}
	if !activity.IsVisibleToRole(user.Role) {
		return nil, 0, ErrLotteryActivityNotVisible
	}
	if !activity.IsDrawOpenAt(now) {
		return nil, 0, ErrLotteryActivityNotOpen
	}

	_, drawCount, _, eligible, _ := GetLotteryProfileFromUser(user)
	if !eligible {
		return nil, 0, ErrLotteryUserNotEligible
	}

	snapshot, err := activity.GetConfigSnapshot()
	if err != nil {
		return nil, 0, err
	}
	tier, err := pickLotteryTier(snapshot)
	if err != nil {
		return nil, 0, err
	}

	var reward *LotteryReward
	remainingDrawCount := 0
	err = DB.Transaction(func(tx *gorm.DB) error {
		var usedCount int64
		if err := tx.Model(&LotteryReward{}).
			Where("activity_id = ? AND source_user_id = ?", activity.Id, user.Id).
			Count(&usedCount).Error; err != nil {
			return err
		}
		if int(usedCount) >= drawCount {
			return ErrLotteryDrawLimitReached
		}

		record := &LotteryReward{
			ActivityId:      activity.Id,
			OwnerUserId:     user.Id,
			SourceUserId:    user.Id,
			SourceDrawIndex: int(usedCount) + 1,
			TierName:        tier.Name,
			Amount:          tier.Amount,
			QuotaTotal:      GetLotteryQuotaAmountByUSD(tier.Amount),
			QuotaRemaining:  GetLotteryQuotaAmountByUSD(tier.Amount),
			Status:          LotteryRewardStatusPendingActivation,
			AutoActivateAt:  activity.AutoActivateAt,
			ConsumeStartsAt: activity.ConsumeStartsAt,
			ExpiresAt:       activity.ExpiresAt,
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		reward = record
		remainingDrawCount = drawCount - record.SourceDrawIndex
		if remainingDrawCount < 0 {
			remainingDrawCount = 0
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return reward, remainingDrawCount, nil
}

func ActivateAllPendingLotteryRewardsForUser(ownerUserId int, activity *LotteryActivity, now int64) (int64, error) {
	if ownerUserId <= 0 || activity == nil {
		return 0, ErrLotteryActivityNotOpen
	}
	result := DB.Model(&LotteryReward{}).
		Where("owner_user_id = ? AND activity_id = ? AND status = ? AND auto_activate_at > ?", ownerUserId, activity.Id, LotteryRewardStatusPendingActivation, now).
		Updates(map[string]any{
			"status":       LotteryRewardStatusActivated,
			"activated_at": now,
			"updated_at":   now,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func GiftLotteryRewardByUsername(fromUserId int, activity *LotteryActivity, rewardId int, targetUsername string, now int64) (*LotteryReward, *User, error) {
	if fromUserId <= 0 || activity == nil || rewardId <= 0 {
		return nil, nil, ErrLotteryGiftTargetInvalid
	}
	targetUsername = strings.TrimSpace(targetUsername)
	if targetUsername == "" {
		return nil, nil, ErrLotteryGiftTargetInvalid
	}

	var reward *LotteryReward
	var targetUser *User
	err := DB.Transaction(func(tx *gorm.DB) error {
		var target User
		if err := tx.Where("username = ? AND status = ?", targetUsername, common.UserStatusEnabled).
			First(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrLotteryGiftTargetNotFound
			}
			return err
		}
		if target.Id == fromUserId {
			return ErrLotteryGiftTargetSelf
		}
		if NormalizeLotteryActivityScope(activity.Scope) == LotteryActivityScopeAdminOnly && target.Role < common.RoleAdminUser {
			return ErrLotteryGiftTargetInvalid
		}

		var record LotteryReward
		if err := tx.Where("id = ? AND activity_id = ? AND owner_user_id = ?", rewardId, activity.Id, fromUserId).
			First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrLotteryRewardNotFound
			}
			return err
		}
		if record.Status != LotteryRewardStatusPendingActivation {
			return ErrLotteryRewardNotPending
		}
		if record.GiftCount > 0 {
			return ErrLotteryRewardAlreadyGifted
		}
		if now >= record.AutoActivateAt {
			return ErrLotteryRewardNotPending
		}

		result := tx.Model(&LotteryReward{}).
			Where("id = ? AND owner_user_id = ? AND status = ? AND gift_count = 0", record.Id, fromUserId, LotteryRewardStatusPendingActivation).
			Updates(map[string]any{
				"owner_user_id":     target.Id,
				"gift_count":        gorm.Expr("gift_count + 1"),
				"gifted_at":         now,
				"gifted_by_user_id": fromUserId,
				"gifted_to_user_id": target.Id,
				"updated_at":        now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrLotteryRewardNotPending
		}
		if err := tx.First(&record, "id = ?", record.Id).Error; err != nil {
			return err
		}

		reward = &record
		targetCopy := target
		targetUser = &targetCopy
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return reward, targetUser, nil
}

func pickLotteryTier(snapshot *LotteryActivitySnapshot) (*operation_setting.LotteryTierSetting, error) {
	if snapshot == nil || len(snapshot.Tiers) == 0 {
		return nil, ErrLotteryConfigInvalid
	}
	total := 0.0
	available := make([]operation_setting.LotteryTierSetting, 0, len(snapshot.Tiers))
	for _, tier := range snapshot.Tiers {
		if tier.Amount <= 0 || tier.Probability <= 0 {
			continue
		}
		total += tier.Probability
		available = append(available, tier)
	}
	if total <= 0 || len(available) == 0 {
		return nil, ErrLotteryConfigInvalid
	}

	roll, err := cryptoRandomFloat64()
	if err != nil {
		return nil, err
	}
	target := roll * total
	cursor := 0.0
	for _, tier := range available {
		cursor += tier.Probability
		if target < cursor {
			chosen := tier
			return &chosen, nil
		}
	}
	chosen := available[len(available)-1]
	return &chosen, nil
}

func cryptoRandomFloat64() (float64, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}
	value := binary.BigEndian.Uint64(buf[:]) >> 11
	return float64(value) / float64(uint64(1)<<53), nil
}
