package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type registrationInviteCodeResponse struct {
	Code                   string  `json:"code"`
	ExpiresAt              int64   `json:"expires_at"`
	UsedAt                 int64   `json:"used_at"`
	RevokedAt              int64   `json:"revoked_at"`
	CanInvite              bool    `json:"can_invite"`
	CanGenerate            bool    `json:"can_generate"`
	IsAdminUnlimited       bool    `json:"is_admin_unlimited"`
	InviteLevel            int     `json:"invite_level"`
	ConsumedAmountUSD      float64 `json:"consumed_amount_usd"`
	NextLevel              int     `json:"next_level"`
	NextLevelThresholdUSD  float64 `json:"next_level_threshold_usd"`
	AmountToNextLevelUSD   float64 `json:"amount_to_next_level_usd"`
	UsedSlots              int     `json:"used_slots"`
	TotalSlots             int     `json:"total_slots"`
	CycleMonths            int     `json:"cycle_months"`
	CycleStartedAt         int64   `json:"cycle_started_at"`
	CycleEndsAt            int64   `json:"cycle_ends_at"`
	NextRefreshAt          int64   `json:"next_refresh_at"`
	NextAvailableAt        int64   `json:"next_available_at"`
	CurrentNewUserQuota    int     `json:"current_new_user_quota"`
	CurrentNewUserQuotaUSD float64 `json:"current_new_user_quota_usd"`
}

type registrationEmailVerificationRequest struct {
	Email string `json:"email"`
}

func SendRegistrationEmailVerification(c *gin.Context) {
	if !common.EmailVerificationEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "邮箱验证未启用",
		})
		return
	}

	var req registrationEmailVerificationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	email := strings.TrimSpace(req.Email)
	if err := validateRegistrationVerificationEmail(email); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	code := common.GenerateVerificationCode(6)
	common.RegisterVerificationCodeWithKey(email, code, common.RegistrationEmailVerificationPurpose)
	subject := fmt.Sprintf("%s邮箱验证邮件", common.SystemName)
	content := fmt.Sprintf("<p>您好，你正在进行%s邮箱注册验证。</p>"+
		"<p>您的验证码为: <strong>%s</strong></p>"+
		"<p>验证码 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.SystemName, code, common.VerificationValidMinutes)
	if err := common.SendEmail(subject, email, content); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func GetSelfRegistrationInviteCode(c *gin.Context) {
	user, ok := getRegistrationInviteIssuer(c)
	if !ok {
		return
	}

	state, err := buildRegistrationInviteCodeResponse(user)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    state,
	})
}

func CreateSelfRegistrationInviteCode(c *gin.Context) {
	user, ok := getRegistrationInviteIssuer(c)
	if !ok {
		return
	}

	state, err := buildRegistrationInviteCodeResponse(user)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if state.Code != "" && !state.CanGenerate {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    state,
		})
		return
	}
	if !state.CanGenerate {
		message := "当前等级未达到邀请条件"
		if state.CanInvite {
			message = "当前周期邀请名额已用完，请等待下个刷新周期"
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": message,
			"data":    state,
		})
		return
	}

	now := common.GetTimestamp()
	var inviteCode *model.RegistrationInviteCode
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := model.RevokeActiveRegistrationInviteCodesByInviterTx(tx, user.Id, now); err != nil {
			return err
		}
		var createErr error
		inviteCode, createErr = model.CreateRegistrationInviteCodeTx(tx, user.Id, state.CycleEndsAt)
		return createErr
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}

	updatedState, err := buildRegistrationInviteCodeResponse(user)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if updatedState.Code == "" && inviteCode != nil {
		updatedState.Code = inviteCode.Code
		updatedState.ExpiresAt = inviteCode.ExpiresAt
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    updatedState,
	})
}

func GetRegistrationInviteTrace(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	trace, err := model.GetRegistrationInviteTrace(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorI18n(c, i18n.MsgUserNotExists)
			return
		}
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    trace,
	})
}

func getRegistrationInviteIssuer(c *gin.Context) (*model.User, bool) {
	if !common.PasswordRegisterOneTimeInviteCodeEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "一次性邀请码功能未启用",
		})
		return nil, false
	}

	userID := c.GetInt("id")
	user, err := model.GetEnabledUserByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorI18n(c, i18n.MsgUserNotExists)
			return nil, false
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "当前用户不可生成邀请码",
		})
		return nil, false
	}
	if user.Role < common.RoleCommonUser {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "当前角色不可生成一次性邀请码",
		})
		return nil, false
	}
	return user, true
}

func buildRegistrationInviteCodeResponse(user *model.User) (*registrationInviteCodeResponse, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}
	now := common.GetTimestamp()
	state, err := model.BuildRegistrationInviteIssuerState(user, now, common.PasswordRegisterOneTimeInviteCodeCycleMonths)
	if err != nil {
		return nil, err
	}
	return registrationInviteCodeResponseFromState(state), nil
}

func registrationInviteCodeResponseFromState(state *model.RegistrationInviteIssuerState) *registrationInviteCodeResponse {
	if state == nil {
		return &registrationInviteCodeResponse{}
	}
	response := &registrationInviteCodeResponse{
		CanInvite:              state.CanInvite,
		CanGenerate:            state.CanGenerate,
		IsAdminUnlimited:       state.IsAdminUnlimited,
		InviteLevel:            state.InviteLevel,
		ConsumedAmountUSD:      state.ConsumedAmountUSD,
		NextLevel:              state.NextLevel,
		NextLevelThresholdUSD:  state.NextLevelThresholdUSD,
		AmountToNextLevelUSD:   state.AmountToNextLevelUSD,
		UsedSlots:              state.UsedSlots,
		TotalSlots:             state.TotalSlots,
		CycleMonths:            state.CycleWindow.Months,
		CycleStartedAt:         state.CycleWindow.StartAt,
		CycleEndsAt:            state.CycleWindow.EndAt,
		NextRefreshAt:          state.CycleWindow.EndAt,
		NextAvailableAt:        state.CycleWindow.EndAt,
		CurrentNewUserQuota:    state.CurrentNewUserQuota,
		CurrentNewUserQuotaUSD: state.CurrentNewUserQuotaUSD,
	}
	if state.ActiveCode != nil {
		response.Code = state.ActiveCode.Code
		response.ExpiresAt = state.ActiveCode.ExpiresAt
		response.UsedAt = state.ActiveCode.UsedAt
		response.RevokedAt = state.ActiveCode.RevokedAt
	}
	return response
}

func validateRegistrationVerificationEmail(email string) error {
	if err := common.Validate.Var(email, "required,email"); err != nil {
		return errors.New("无效的参数")
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return errors.New("无效的邮箱地址")
	}
	localPart := parts[0]
	domainPart := parts[1]

	if common.EmailDomainRestrictionEnabled {
		allowed := false
		for _, domain := range common.EmailDomainWhitelist {
			if domainPart == domain {
				allowed = true
				break
			}
		}
		if !allowed {
			return errors.New("The administrator has enabled the email domain name whitelist, and your email address is not allowed due to special symbols or it's not in the whitelist.")
		}
	}

	if common.EmailAliasRestrictionEnabled {
		containsSpecialSymbols := strings.Contains(localPart, "+") || strings.Contains(localPart, ".")
		if containsSpecialSymbols {
			return errors.New("管理员已启用邮箱地址别名限制，您的邮箱地址由于包含特殊符号而被拒绝。")
		}
	}

	if model.DoesAnyUserUseEmail(email) {
		return errors.New("邮箱地址已被占用")
	}

	return nil
}
