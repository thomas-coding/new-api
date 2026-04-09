package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const registrationInviteCodeLength = 16
const registrationInviteEligibleLevel = 3
const registrationInviteLevelThresholdBaseUSD = 100.0
const registrationInviteLevelMultiplier = 5.0

type RegistrationInviteCode struct {
	Id        int    `json:"id"`
	Code      string `json:"code" gorm:"type:varchar(64);uniqueIndex;not null"`
	InviterId int    `json:"inviter_id" gorm:"column:inviter_id;index;not null"`
	InviteeId int    `json:"invitee_id" gorm:"column:invitee_id;index;default:0"`
	ExpiresAt int64  `json:"expires_at" gorm:"column:expires_at;bigint;index;not null"`
	UsedAt    int64  `json:"used_at" gorm:"column:used_at;bigint;default:0"`
	RevokedAt int64  `json:"revoked_at" gorm:"column:revoked_at;bigint;default:0"`
	CreatedAt int64  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (RegistrationInviteCode) TableName() string {
	return "registration_invite_codes"
}

type RegistrationInviteCycleWindow struct {
	StartAt int64 `json:"start_at"`
	EndAt   int64 `json:"end_at"`
	Months  int   `json:"months"`
}

type RegistrationInviteIssuerState struct {
	ActiveCode             *RegistrationInviteCode       `json:"active_code,omitempty"`
	CycleWindow            RegistrationInviteCycleWindow `json:"cycle_window"`
	IsAdminUnlimited       bool                          `json:"is_admin_unlimited"`
	CanInvite              bool                          `json:"can_invite"`
	CanGenerate            bool                          `json:"can_generate"`
	InviteLevel            int                           `json:"invite_level"`
	ConsumedAmountUSD      float64                       `json:"consumed_amount_usd"`
	NextLevel              int                           `json:"next_level"`
	NextLevelThresholdUSD  float64                       `json:"next_level_threshold_usd"`
	AmountToNextLevelUSD   float64                       `json:"amount_to_next_level_usd"`
	UsedSlots              int                           `json:"used_slots"`
	TotalSlots             int                           `json:"total_slots"`
	CurrentNewUserQuota    int                           `json:"current_new_user_quota"`
	CurrentNewUserQuotaUSD float64                       `json:"current_new_user_quota_usd"`
}

func (code *RegistrationInviteCode) IsValidAt(now int64) bool {
	if code == nil {
		return false
	}
	return code.UsedAt == 0 && code.RevokedAt == 0 && code.ExpiresAt > now
}

func (code *RegistrationInviteCode) IsUsableInCycle(now int64, cycleWindow RegistrationInviteCycleWindow) bool {
	if !code.IsValidAt(now) {
		return false
	}
	if cycleWindow.EndAt > 0 && now >= cycleWindow.EndAt {
		return false
	}
	return code.CreatedAt >= cycleWindow.StartAt
}

func NormalizeRegistrationInviteCycleMonths(cycleMonths int) int {
	if cycleMonths < 1 {
		return 1
	}
	return cycleMonths
}

func GetRegistrationInviteCycleWindowByTimestamp(now int64, cycleMonths int) RegistrationInviteCycleWindow {
	normalizedMonths := NormalizeRegistrationInviteCycleMonths(cycleMonths)
	nowTime := time.Unix(now, 0).In(time.Local)
	year, month, _ := nowTime.Date()
	startMonth := ((int(month)-1)/normalizedMonths)*normalizedMonths + 1
	cycleStart := time.Date(year, time.Month(startMonth), 1, 0, 0, 0, 0, nowTime.Location())
	cycleEnd := cycleStart.AddDate(0, normalizedMonths, 0)
	return RegistrationInviteCycleWindow{
		StartAt: cycleStart.Unix(),
		EndAt:   cycleEnd.Unix(),
		Months:  normalizedMonths,
	}
}

func GetRegistrationInviteConsumedAmountUSD(usedQuota int) float64 {
	if usedQuota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(usedQuota) / common.QuotaPerUnit
}

func GetRegistrationInviteLevelThresholdUSD(level int) float64 {
	if level <= 0 {
		return 0
	}
	threshold := registrationInviteLevelThresholdBaseUSD
	for i := 1; i < level; i++ {
		threshold *= registrationInviteLevelMultiplier
	}
	return threshold
}

func GetRegistrationInviteLevelByConsumedAmountUSD(consumedAmountUSD float64) int {
	level := 0
	for nextLevel := 1; nextLevel < 32; nextLevel++ {
		if consumedAmountUSD < GetRegistrationInviteLevelThresholdUSD(nextLevel) {
			break
		}
		level = nextLevel
	}
	return level
}

func GetRegistrationInviteSlotsForLevel(level int) int {
	if level < registrationInviteEligibleLevel {
		return 0
	}
	return level - (registrationInviteEligibleLevel - 1)
}

func GetEnabledUserByID(userID int) (*User, error) {
	if userID <= 0 {
		return nil, errors.New("user id is required")
	}
	var user User
	err := DB.Select("id,username,display_name,role,status,used_quota").First(&user, "id = ?", userID).Error
	if err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, errors.New("user is disabled")
	}
	return &user, nil
}

func GetLatestRegistrationInviteCodeByInviter(inviterID int) (*RegistrationInviteCode, error) {
	if inviterID <= 0 {
		return nil, errors.New("inviter id is required")
	}
	var inviteCode RegistrationInviteCode
	err := DB.Where("inviter_id = ?", inviterID).Order("created_at desc").Order("id desc").First(&inviteCode).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inviteCode, nil
}

func GetLatestValidRegistrationInviteCodeByInviter(inviterID int, now int64) (*RegistrationInviteCode, error) {
	if inviterID <= 0 {
		return nil, errors.New("inviter id is required")
	}
	var inviteCode RegistrationInviteCode
	err := DB.Where(
		"inviter_id = ? AND invitee_id = 0 AND used_at = 0 AND revoked_at = 0 AND expires_at > ?",
		inviterID,
		now,
	).Order("created_at desc").Order("id desc").First(&inviteCode).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inviteCode, nil
}

func CountConsumedRegistrationInviteCodesByInviterInWindow(inviterID int, startAt int64, endAt int64) (int64, error) {
	if inviterID <= 0 {
		return 0, errors.New("inviter id is required")
	}
	var count int64
	err := DB.Model(&RegistrationInviteCode{}).
		Where("inviter_id = ? AND invitee_id > 0 AND used_at >= ? AND used_at < ?", inviterID, startAt, endAt).
		Count(&count).Error
	return count, err
}

func GetActiveRegistrationInviteCodeByCode(code string, now int64) (*RegistrationInviteCode, error) {
	if code == "" {
		return nil, errors.New("invite code is required")
	}
	var inviteCode RegistrationInviteCode
	err := DB.Where(
		"code = ? AND invitee_id = 0 AND used_at = 0 AND revoked_at = 0 AND expires_at > ?",
		code,
		now,
	).First(&inviteCode).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inviteCode, nil
}

func CreateRegistrationInviteCode(inviterID int, expiresAt int64) (*RegistrationInviteCode, error) {
	return CreateRegistrationInviteCodeTx(DB, inviterID, expiresAt)
}

func CreateRegistrationInviteCodeTx(tx *gorm.DB, inviterID int, expiresAt int64) (*RegistrationInviteCode, error) {
	if tx == nil {
		tx = DB
	}
	if inviterID <= 0 {
		return nil, errors.New("inviter id is required")
	}
	if expiresAt <= common.GetTimestamp() {
		return nil, errors.New("expires_at must be in the future")
	}

	for i := 0; i < 5; i++ {
		codeValue, err := common.GenerateRandomCharsKey(registrationInviteCodeLength)
		if err != nil {
			return nil, err
		}

		var count int64
		if err := tx.Model(&RegistrationInviteCode{}).Where("code = ?", codeValue).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			continue
		}

		inviteCode := &RegistrationInviteCode{
			Code:      codeValue,
			InviterId: inviterID,
			ExpiresAt: expiresAt,
		}
		if err := tx.Create(inviteCode).Error; err != nil {
			return nil, err
		}
		return inviteCode, nil
	}

	return nil, fmt.Errorf("failed to generate a unique registration invite code")
}

func RevokeActiveRegistrationInviteCodesByInviterTx(tx *gorm.DB, inviterID int, now int64) error {
	if tx == nil {
		tx = DB
	}
	if inviterID <= 0 {
		return errors.New("inviter id is required")
	}
	return tx.Model(&RegistrationInviteCode{}).
		Where("inviter_id = ? AND invitee_id = 0 AND used_at = 0 AND revoked_at = 0 AND expires_at > ?", inviterID, now).
		Updates(map[string]any{
			"revoked_at": now,
			"updated_at": now,
		}).Error
}

func ConsumeRegistrationInviteCodeTx(tx *gorm.DB, inviteCodeID int, inviteeID int, now int64) error {
	if tx == nil {
		tx = DB
	}
	if inviteCodeID <= 0 || inviteeID <= 0 {
		return errors.New("invite code id and invitee id are required")
	}

	result := tx.Model(&RegistrationInviteCode{}).
		Where("id = ? AND invitee_id = 0 AND used_at = 0 AND revoked_at = 0 AND expires_at > ?", inviteCodeID, now).
		Updates(map[string]any{
			"invitee_id": inviteeID,
			"used_at":    now,
			"updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("registration invite code is no longer available")
	}
	return nil
}

func BuildRegistrationInviteIssuerState(user *User, now int64, cycleMonths int) (*RegistrationInviteIssuerState, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}

	cycleWindow := GetRegistrationInviteCycleWindowByTimestamp(now, cycleMonths)
	consumedAmountUSD := GetRegistrationInviteConsumedAmountUSD(user.UsedQuota)
	inviteLevel := GetRegistrationInviteLevelByConsumedAmountUSD(consumedAmountUSD)
	nextLevel := inviteLevel + 1
	nextLevelThresholdUSD := GetRegistrationInviteLevelThresholdUSD(nextLevel)
	amountToNextLevelUSD := nextLevelThresholdUSD - consumedAmountUSD
	if amountToNextLevelUSD < 0 {
		amountToNextLevelUSD = 0
	}

	activeCode, err := GetLatestValidRegistrationInviteCodeByInviter(user.Id, now)
	if err != nil {
		return nil, err
	}
	if activeCode != nil && !activeCode.IsUsableInCycle(now, cycleWindow) {
		activeCode = nil
	}

	state := &RegistrationInviteIssuerState{
		ActiveCode:             activeCode,
		CycleWindow:            cycleWindow,
		IsAdminUnlimited:       user.Role > common.RoleCommonUser,
		InviteLevel:            inviteLevel,
		ConsumedAmountUSD:      consumedAmountUSD,
		NextLevel:              nextLevel,
		NextLevelThresholdUSD:  nextLevelThresholdUSD,
		AmountToNextLevelUSD:   amountToNextLevelUSD,
		CurrentNewUserQuota:    common.QuotaForNewUser,
		CurrentNewUserQuotaUSD: GetRegistrationInviteConsumedAmountUSD(common.QuotaForNewUser),
	}

	if state.IsAdminUnlimited {
		state.CanInvite = true
		state.CanGenerate = true
		return state, nil
	}

	state.TotalSlots = GetRegistrationInviteSlotsForLevel(inviteLevel)
	state.CanInvite = state.TotalSlots > 0
	if !state.CanInvite {
		return state, nil
	}

	usedSlots, err := CountConsumedRegistrationInviteCodesByInviterInWindow(user.Id, cycleWindow.StartAt, cycleWindow.EndAt)
	if err != nil {
		return nil, err
	}
	state.UsedSlots = int(usedSlots)
	state.CanGenerate = state.ActiveCode == nil && state.UsedSlots < state.TotalSlots
	return state, nil
}

type RegistrationInviteTraceUser struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        int    `json:"role"`
	Status      int    `json:"status"`
	Deleted     bool   `json:"deleted"`
}

type RegistrationInviteTrace struct {
	Invitee    *RegistrationInviteTraceUser `json:"invitee,omitempty"`
	Inviter    *RegistrationInviteTraceUser `json:"inviter,omitempty"`
	InviteCode *RegistrationInviteCode      `json:"invite_code,omitempty"`
}

func GetRegistrationInviteTrace(inviteeID int) (*RegistrationInviteTrace, error) {
	if inviteeID <= 0 {
		return nil, errors.New("invitee id is required")
	}

	invitee, err := getRegistrationInviteTraceUser(inviteeID)
	if err != nil {
		return nil, err
	}

	trace := &RegistrationInviteTrace{
		Invitee: invitee,
	}

	var inviteCode RegistrationInviteCode
	if err := DB.Where("invitee_id = ?", inviteeID).Order("used_at desc").Order("id desc").First(&inviteCode).Error; err == nil {
		trace.InviteCode = &inviteCode
	}

	if trace.InviteCode != nil {
		inviter, err := getRegistrationInviteTraceUser(trace.InviteCode.InviterId)
		if err == nil {
			trace.Inviter = inviter
		}
		return trace, nil
	}

	if trace.Invitee != nil && trace.Invitee.Id > 0 {
		var inviteeUser User
		if err := DB.Unscoped().Select("id,inviter_id").First(&inviteeUser, "id = ?", inviteeID).Error; err == nil && inviteeUser.InviterId > 0 {
			inviter, inviterErr := getRegistrationInviteTraceUser(inviteeUser.InviterId)
			if inviterErr == nil {
				trace.Inviter = inviter
			}
		}
	}

	return trace, nil
}

func getRegistrationInviteTraceUser(userID int) (*RegistrationInviteTraceUser, error) {
	var user User
	err := DB.Unscoped().Select("id,username,display_name,role,status,deleted_at").First(&user, "id = ?", userID).Error
	if err != nil {
		return nil, err
	}
	return &RegistrationInviteTraceUser{
		Id:          user.Id,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Status:      user.Status,
		Deleted:     user.DeletedAt.Valid,
	}, nil
}
