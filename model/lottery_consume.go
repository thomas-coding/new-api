package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	LotteryConsumeRecordStatusPreConsumed = "pre_consumed"
	LotteryConsumeRecordStatusSettled     = "settled"
	LotteryConsumeRecordStatusRefunded    = "refunded"
)

type LotteryConsumeAllocation struct {
	RewardId int `json:"reward_id"`
	Quota    int `json:"quota"`
}

type LotteryConsumeRecord struct {
	Id            int    `json:"id"`
	RequestId     string `json:"request_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	UserId        int    `json:"user_id" gorm:"index;not null"`
	TotalQuota    int    `json:"total_quota" gorm:"type:int;not null;default:0"`
	SettledQuota  int    `json:"settled_quota" gorm:"type:int;not null;default:0"`
	RefundedQuota int    `json:"refunded_quota" gorm:"type:int;not null;default:0"`
	Status        string `json:"status" gorm:"type:varchar(32);index;not null;default:'pre_consumed'"`
	Allocations   string `json:"allocations" gorm:"type:text;not null"`
	SettledAt     int64  `json:"settled_at" gorm:"bigint;default:0"`
	RefundedAt    int64  `json:"refunded_at" gorm:"bigint;default:0"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint"`
}

func (LotteryConsumeRecord) TableName() string {
	return "lottery_consume_records"
}

func (r *LotteryConsumeRecord) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *LotteryConsumeRecord) BeforeUpdate(_ *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func (r *LotteryConsumeRecord) GetAllocations() ([]LotteryConsumeAllocation, error) {
	if r == nil || r.Allocations == "" {
		return []LotteryConsumeAllocation{}, nil
	}
	var allocations []LotteryConsumeAllocation
	if err := common.UnmarshalJsonStr(r.Allocations, &allocations); err != nil {
		return nil, err
	}
	return allocations, nil
}

func GetAvailableLotteryQuotaForUser(userId int, now int64) (int, error) {
	if userId <= 0 {
		return 0, nil
	}
	if now <= 0 {
		now = GetDBTimestamp()
	}
	if err := ReconcileLotteryRuntimeState(now); err != nil {
		return 0, err
	}
	var rewards []LotteryReward
	if err := DB.Model(&LotteryReward{}).
		Where("owner_user_id = ? AND status = ? AND consume_starts_at <= ? AND expires_at > ? AND quota_remaining > ?", userId, LotteryRewardStatusActivated, now, now, 0).
		Select("quota_remaining").
		Find(&rewards).Error; err != nil {
		return 0, err
	}
	total := 0
	for _, reward := range rewards {
		total += reward.QuotaRemaining
	}
	return total, nil
}

func PreConsumeLotteryQuota(requestId string, userId int, amount int, now int64) (int, error) {
	if requestId == "" || userId <= 0 || amount <= 0 {
		return 0, nil
	}
	if now <= 0 {
		now = GetDBTimestamp()
	}

	reservedQuota := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := reconcileLotteryRuntimeStateTx(tx, now); err != nil {
			return err
		}

		var existing LotteryConsumeRecord
		err := tx.Where("request_id = ?", requestId).First(&existing).Error
		if err == nil {
			reservedQuota = existing.TotalQuota
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var rewards []LotteryReward
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("owner_user_id = ? AND status = ? AND consume_starts_at <= ? AND expires_at > ? AND quota_remaining > ?", userId, LotteryRewardStatusActivated, now, now, 0).
			Order("expires_at asc, id asc").
			Find(&rewards).Error; err != nil {
			return err
		}

		allocations := make([]LotteryConsumeAllocation, 0)
		remainingNeed := amount
		for _, reward := range rewards {
			if remainingNeed <= 0 {
				break
			}
			if reward.QuotaRemaining <= 0 {
				continue
			}
			quotaToReserve := reward.QuotaRemaining
			if quotaToReserve > remainingNeed {
				quotaToReserve = remainingNeed
			}
			if quotaToReserve <= 0 {
				continue
			}

			newQuotaRemaining := reward.QuotaRemaining - quotaToReserve
			newStatus := LotteryRewardStatusActivated
			consumedAt := int64(0)
			if newQuotaRemaining <= 0 {
				newQuotaRemaining = 0
				newStatus = LotteryRewardStatusConsumed
				consumedAt = now
			}

			result := tx.Model(&LotteryReward{}).
				Where("id = ? AND status = ? AND quota_remaining = ?", reward.Id, LotteryRewardStatusActivated, reward.QuotaRemaining).
				Updates(map[string]any{
					"quota_remaining": newQuotaRemaining,
					"status":          newStatus,
					"consumed_at":     consumedAt,
					"updated_at":      now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrLotteryConsumeConflict
			}

			allocations = append(allocations, LotteryConsumeAllocation{
				RewardId: reward.Id,
				Quota:    quotaToReserve,
			})
			reservedQuota += quotaToReserve
			remainingNeed -= quotaToReserve
		}

		if reservedQuota <= 0 {
			return nil
		}

		allocationBytes, err := common.Marshal(allocations)
		if err != nil {
			return err
		}

		record := &LotteryConsumeRecord{
			RequestId:   requestId,
			UserId:      userId,
			TotalQuota:  reservedQuota,
			Status:      LotteryConsumeRecordStatusPreConsumed,
			Allocations: string(allocationBytes),
		}
		return tx.Create(record).Error
	})
	if err != nil {
		return 0, err
	}
	return reservedQuota, nil
}

func RefundLotteryQuotaPreConsume(requestId string, now int64) error {
	if requestId == "" {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record LotteryConsumeRecord
		err := tx.Where("request_id = ?", requestId).First(&record).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if record.Status == LotteryConsumeRecordStatusRefunded {
			return nil
		}
		if record.Status == LotteryConsumeRecordStatusSettled {
			return nil
		}

		allocations, err := record.GetAllocations()
		if err != nil {
			return err
		}
		for i := len(allocations) - 1; i >= 0; i-- {
			if err := refundLotteryAllocationTx(tx, allocations[i].RewardId, allocations[i].Quota, now); err != nil {
				return err
			}
		}
		return tx.Model(&LotteryConsumeRecord{}).
			Where("id = ? AND status = ?", record.Id, LotteryConsumeRecordStatusPreConsumed).
			Updates(map[string]any{
				"status":         LotteryConsumeRecordStatusRefunded,
				"refunded_quota": record.TotalQuota,
				"refunded_at":    now,
				"updated_at":     now,
			}).Error
	})
}

func RollbackLotteryQuotaConsumeRecord(requestId string, now int64) error {
	if requestId == "" {
		return nil
	}
	if now <= 0 {
		now = GetDBTimestamp()
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record LotteryConsumeRecord
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where("request_id = ?", requestId).First(&record).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if record.Status == LotteryConsumeRecordStatusRefunded {
			return nil
		}

		refundQuota := 0
		switch record.Status {
		case LotteryConsumeRecordStatusPreConsumed:
			refundQuota = record.TotalQuota
		case LotteryConsumeRecordStatusSettled:
			refundQuota = record.SettledQuota
		default:
			return nil
		}

		if refundQuota > 0 {
			allocations, err := record.GetAllocations()
			if err != nil {
				return err
			}
			remainingRefund := refundQuota
			for i := len(allocations) - 1; i >= 0 && remainingRefund > 0; i-- {
				refundPart := allocations[i].Quota
				if refundPart > remainingRefund {
					refundPart = remainingRefund
				}
				if refundPart <= 0 {
					continue
				}
				if err := refundLotteryAllocationTx(tx, allocations[i].RewardId, refundPart, now); err != nil {
					return err
				}
				remainingRefund -= refundPart
			}
			if remainingRefund != 0 {
				return ErrLotteryConsumeConflict
			}
		}

		return tx.Model(&LotteryConsumeRecord{}).
			Where("id = ? AND status = ?", record.Id, record.Status).
			Updates(map[string]any{
				"status":         LotteryConsumeRecordStatusRefunded,
				"settled_quota":  0,
				"refunded_quota": record.TotalQuota,
				"settled_at":     0,
				"refunded_at":    now,
				"updated_at":     now,
			}).Error
	})
}

func SettleLotteryQuota(requestId string, actualQuota int, now int64) (int, int, error) {
	if requestId == "" {
		return 0, 0, nil
	}

	settledQuota := 0
	refundedQuota := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var record LotteryConsumeRecord
		err := tx.Where("request_id = ?", requestId).First(&record).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if record.Status == LotteryConsumeRecordStatusRefunded || record.Status == LotteryConsumeRecordStatusSettled {
			settledQuota = record.SettledQuota
			refundedQuota = record.RefundedQuota
			return nil
		}

		if actualQuota < 0 {
			actualQuota = 0
		}
		settledQuota = actualQuota
		if settledQuota > record.TotalQuota {
			settledQuota = record.TotalQuota
		}
		refundedQuota = record.TotalQuota - settledQuota

		if refundedQuota > 0 {
			allocations, err := record.GetAllocations()
			if err != nil {
				return err
			}
			remainingRefund := refundedQuota
			for i := len(allocations) - 1; i >= 0 && remainingRefund > 0; i-- {
				refundQuota := allocations[i].Quota
				if refundQuota > remainingRefund {
					refundQuota = remainingRefund
				}
				if refundQuota <= 0 {
					continue
				}
				if err := refundLotteryAllocationTx(tx, allocations[i].RewardId, refundQuota, now); err != nil {
					return err
				}
				remainingRefund -= refundQuota
			}
			if remainingRefund != 0 {
				return ErrLotteryConsumeConflict
			}
		}

		return tx.Model(&LotteryConsumeRecord{}).
			Where("id = ? AND status = ?", record.Id, LotteryConsumeRecordStatusPreConsumed).
			Updates(map[string]any{
				"status":         LotteryConsumeRecordStatusSettled,
				"settled_quota":  settledQuota,
				"refunded_quota": refundedQuota,
				"settled_at":     now,
				"refunded_at":    now,
				"updated_at":     now,
			}).Error
	})
	if err != nil {
		return 0, 0, err
	}
	return settledQuota, refundedQuota, nil
}

func refundLotteryAllocationTx(tx *gorm.DB, rewardId int, quota int, now int64) error {
	if tx == nil || rewardId <= 0 || quota <= 0 {
		return nil
	}

	var reward LotteryReward
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", rewardId).First(&reward).Error; err != nil {
		return err
	}

	newQuotaRemaining := reward.QuotaRemaining + quota
	if newQuotaRemaining > reward.QuotaTotal && reward.QuotaTotal > 0 {
		newQuotaRemaining = reward.QuotaTotal
	}
	newStatus := reward.Status
	consumedAt := reward.ConsumedAt
	if reward.ExpiresAt <= now {
		newStatus = LotteryRewardStatusExpired
	} else if reward.ConsumeStartsAt <= now {
		newStatus = LotteryRewardStatusActivated
		consumedAt = 0
	}

	return tx.Model(&LotteryReward{}).
		Where("id = ?", reward.Id).
		Updates(map[string]any{
			"quota_remaining": newQuotaRemaining,
			"status":          newStatus,
			"consumed_at":     consumedAt,
			"updated_at":      now,
		}).Error
}
