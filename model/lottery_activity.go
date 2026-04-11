package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

const (
	LotteryActivityScopePublic    = "public"
	LotteryActivityScopeAdminOnly = "admin_only"

	LotteryActivityOpenModeImmediate = "immediate"
	LotteryActivityOpenModeWeekly    = "weekly"

	LotteryActivityStatusActive = "active"
	LotteryActivityStatusClosed = "closed"
)

var ErrLotteryActivityAlreadyActive = errors.New("lottery activity already active")

type LotteryActivity struct {
	Id                  int    `json:"id"`
	Scope               string `json:"scope" gorm:"type:varchar(32);index;not null;default:'public'"`
	OpenMode            string `json:"open_mode" gorm:"type:varchar(32);not null;default:'immediate'"`
	Status              string `json:"status" gorm:"type:varchar(32);index;not null;default:'active'"`
	DrawDate            string `json:"draw_date" gorm:"type:varchar(10);index;not null"`
	DrawStartsAt        int64  `json:"draw_starts_at" gorm:"bigint;not null"`
	DrawEndsAt          int64  `json:"draw_ends_at" gorm:"bigint;index;not null"`
	AutoActivateAt      int64  `json:"auto_activate_at" gorm:"bigint;not null"`
	ConsumeStartsAt     int64  `json:"consume_starts_at" gorm:"bigint;not null"`
	ConsumeEndsAt       int64  `json:"consume_ends_at" gorm:"bigint;index;not null"`
	ExpiresAt           int64  `json:"expires_at" gorm:"bigint;index;not null"`
	ConfigSnapshot      string `json:"config_snapshot" gorm:"type:text;not null"`
	CreatedBy           int    `json:"created_by" gorm:"index;not null"`
	LastAutoActivatedAt int64  `json:"last_auto_activated_at" gorm:"bigint;default:0"`
	ClosedAt            int64  `json:"closed_at" gorm:"bigint;default:0"`
	CreatedAt           int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt           int64  `json:"updated_at" gorm:"bigint"`
}

type LotteryActivitySnapshot struct {
	MythBroadcastEnabled bool                                   `json:"myth_broadcast_enabled"`
	Tiers                []operation_setting.LotteryTierSetting `json:"tiers"`
}

func (LotteryActivity) TableName() string {
	return "lottery_activities"
}

func (a *LotteryActivity) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	a.CreatedAt = now
	a.UpdatedAt = now
	return nil
}

func (a *LotteryActivity) BeforeUpdate(_ *gorm.DB) error {
	a.UpdatedAt = common.GetTimestamp()
	return nil
}

func NormalizeLotteryActivityScope(scope string) string {
	switch strings.TrimSpace(scope) {
	case LotteryActivityScopeAdminOnly:
		return LotteryActivityScopeAdminOnly
	default:
		return LotteryActivityScopePublic
	}
}

func NormalizeLotteryActivityOpenMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case LotteryActivityOpenModeWeekly:
		return LotteryActivityOpenModeWeekly
	default:
		return LotteryActivityOpenModeImmediate
	}
}

func (a *LotteryActivity) IsVisibleToRole(role int) bool {
	if a == nil {
		return false
	}
	if NormalizeLotteryActivityScope(a.Scope) == LotteryActivityScopeAdminOnly {
		return role >= common.RoleAdminUser
	}
	return true
}

func (a *LotteryActivity) IsOngoing(now int64) bool {
	if a == nil {
		return false
	}
	return a.Status == LotteryActivityStatusActive && a.ExpiresAt > now
}

func (a *LotteryActivity) IsDrawOpenAt(now int64) bool {
	if a == nil {
		return false
	}
	return a.Status == LotteryActivityStatusActive && now >= a.DrawStartsAt && now < a.DrawEndsAt
}

func (a *LotteryActivity) IsConsumeOpenAt(now int64) bool {
	if a == nil {
		return false
	}
	return a.Status == LotteryActivityStatusActive && now >= a.ConsumeStartsAt && now < a.ConsumeEndsAt
}

func (a *LotteryActivity) GetPhase(now int64) string {
	if a == nil {
		return "none"
	}
	switch {
	case now < a.DrawStartsAt:
		return "upcoming"
	case now < a.DrawEndsAt:
		return "draw"
	case now < a.ConsumeStartsAt:
		return "waiting_consume"
	case now < a.ConsumeEndsAt:
		return "consume"
	default:
		return "expired"
	}
}

func (a *LotteryActivity) GetConfigSnapshot() (*LotteryActivitySnapshot, error) {
	if a == nil || a.ConfigSnapshot == "" {
		return nil, nil
	}
	var snapshot LotteryActivitySnapshot
	if err := common.UnmarshalJsonStr(a.ConfigSnapshot, &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func buildLotteryActivitySnapshot(setting operation_setting.LotterySetting) (*LotteryActivitySnapshot, error) {
	snapshot := &LotteryActivitySnapshot{
		MythBroadcastEnabled: setting.MythBroadcastEnabled,
		Tiers:                append([]operation_setting.LotteryTierSetting(nil), setting.Tiers...),
	}
	return snapshot, nil
}

func buildImmediateLotteryWindow(now int64) (string, int64, int64, int64, int64, int64, int64) {
	localNow := time.Unix(now, 0).In(time.Local)
	year, month, day := localNow.Date()
	dayStart := time.Date(year, month, day, 0, 0, 0, 0, localNow.Location())
	nextDayStart := dayStart.AddDate(0, 0, 1)
	thirdDayStart := dayStart.AddDate(0, 0, 2)
	return dayStart.Format("2006-01-02"), now, nextDayStart.Unix(), nextDayStart.Unix(), nextDayStart.Unix(), thirdDayStart.Unix(), thirdDayStart.Unix()
}

func getLotteryLocalDateInfo(now int64) (time.Time, string, int) {
	localNow := time.Unix(now, 0).In(time.Local)
	weekday := int(localNow.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return localNow, localNow.Format("2006-01-02"), weekday
}

func createLotteryActivityTx(tx *gorm.DB, createdBy int, scope string, openMode string, now int64, setting operation_setting.LotterySetting) (*LotteryActivity, error) {
	scope = NormalizeLotteryActivityScope(scope)
	openMode = NormalizeLotteryActivityOpenMode(openMode)

	drawDate, drawStartsAt, drawEndsAt, autoActivateAt, consumeStartsAt, consumeEndsAt, expiresAt := buildImmediateLotteryWindow(now)
	snapshot, err := buildLotteryActivitySnapshot(setting)
	if err != nil {
		return nil, err
	}
	snapshotBytes, err := common.Marshal(snapshot)
	if err != nil {
		return nil, err
	}

	activity := &LotteryActivity{
		Scope:           scope,
		OpenMode:        openMode,
		Status:          LotteryActivityStatusActive,
		DrawDate:        drawDate,
		DrawStartsAt:    drawStartsAt,
		DrawEndsAt:      drawEndsAt,
		AutoActivateAt:  autoActivateAt,
		ConsumeStartsAt: consumeStartsAt,
		ConsumeEndsAt:   consumeEndsAt,
		ExpiresAt:       expiresAt,
		ConfigSnapshot:  string(snapshotBytes),
		CreatedBy:       createdBy,
	}
	if err := tx.Create(activity).Error; err != nil {
		return nil, err
	}
	return activity, nil
}

func GetAnyActiveLotteryActivity(now int64) (*LotteryActivity, error) {
	var activity LotteryActivity
	err := DB.Where("status = ? AND expires_at > ?", LotteryActivityStatusActive, now).
		Order("created_at desc, id desc").
		First(&activity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &activity, nil
}

func GetCurrentLotteryActivityForRole(role int, now int64) (*LotteryActivity, error) {
	var activities []LotteryActivity
	if err := DB.Where("status = ? AND expires_at > ?", LotteryActivityStatusActive, now).
		Order("created_at desc, id desc").
		Find(&activities).Error; err != nil {
		return nil, err
	}
	for i := range activities {
		if activities[i].IsVisibleToRole(role) {
			activity := activities[i]
			return &activity, nil
		}
	}
	return nil, nil
}

func OpenImmediateLotteryActivity(createdBy int, scope string, setting operation_setting.LotterySetting) (*LotteryActivity, error) {
	if createdBy <= 0 {
		return nil, errors.New("invalid createdBy")
	}
	now := GetDBTimestamp()
	var created *LotteryActivity
	err := DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&LotteryActivity{}).
			Where("status = ? AND expires_at > ?", LotteryActivityStatusActive, now).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrLotteryActivityAlreadyActive
		}
		activity, err := createLotteryActivityTx(tx, createdBy, scope, LotteryActivityOpenModeImmediate, now, setting)
		if err != nil {
			return err
		}
		created = activity
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func EnsureWeeklyPublicLotteryActivity(now int64) (*LotteryActivity, error) {
	if now <= 0 {
		now = GetDBTimestamp()
	}
	setting := operation_setting.GetNormalizedLotterySetting()
	if setting.WeeklyDay <= 0 {
		return nil, nil
	}

	_, drawDate, weekday := getLotteryLocalDateInfo(now)
	targetWeekday := operation_setting.ResolveLotteryWeeklyDay(setting.WeeklyDay, now)
	if weekday != targetWeekday {
		return nil, nil
	}

	var created *LotteryActivity
	err := DB.Transaction(func(tx *gorm.DB) error {
		var activeCount int64
		if err := tx.Model(&LotteryActivity{}).
			Where("status = ? AND expires_at > ?", LotteryActivityStatusActive, now).
			Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount > 0 {
			return nil
		}

		var existingCount int64
		if err := tx.Model(&LotteryActivity{}).
			Where("draw_date = ? AND scope = ? AND open_mode = ?", drawDate, LotteryActivityScopePublic, LotteryActivityOpenModeWeekly).
			Count(&existingCount).Error; err != nil {
			return err
		}
		if existingCount > 0 {
			return nil
		}

		activity, err := createLotteryActivityTx(tx, 0, LotteryActivityScopePublic, LotteryActivityOpenModeWeekly, now, setting)
		if err != nil {
			return err
		}
		created = activity
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func GetLotteryEligibilityFromUser(user *User) (int, int, float64) {
	if user == nil {
		return 0, 0, 0
	}
	consumedAmountUSD := GetRegistrationInviteConsumedAmountUSD(user.UsedQuota)
	level := GetRegistrationInviteLevelByConsumedAmountUSD(consumedAmountUSD)
	return level, CalculateLotteryDrawCountForLevel(level), consumedAmountUSD
}

func CalculateLotteryDrawCountForLevel(level int) int {
	if level < 1 {
		return 0
	}
	return level*2 + 2
}
