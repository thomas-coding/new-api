package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// BillingSession — 统一计费会话
// ---------------------------------------------------------------------------

// BillingSession 封装单次请求的预扣费/结算/退款生命周期。
// 实现 relaycommon.BillingSettler 接口。
type BillingSession struct {
	relayInfo        *relaycommon.RelayInfo
	funding          FundingSource
	preConsumedQuota int  // 实际预扣额度（信任用户可能为 0）
	fundingPreUsed   int  // 基础资金来源实际预扣额度
	tokenConsumed    int  // 令牌额度实际扣减量
	lotteryReserved  int  // 乐透券预扣额度
	fundingSettled   bool // funding.Settle 已成功，资金来源已提交
	settled          bool // Settle 全部完成（资金 + 令牌）
	refunded         bool // Refund 已调用
	mu               sync.Mutex
}

// Settle 根据实际消耗额度进行结算。
// 资金来源和令牌额度分两步提交：若资金来源已提交但令牌调整失败，
// 会标记 fundingSettled 防止 Refund 对已提交的资金来源执行退款。
func (s *BillingSession) Settle(actualQuota int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled {
		return nil
	}
	if actualQuota < 0 {
		actualQuota = 0
	}

	plannedLotteryUsed := 0
	if s.lotteryReserved > 0 {
		plannedLotteryUsed = actualQuota
		if plannedLotteryUsed > s.lotteryReserved {
			plannedLotteryUsed = s.lotteryReserved
		}
	}

	basePreConsumed := s.fundingPreUsed
	baseActual := actualQuota - plannedLotteryUsed
	if baseActual < 0 {
		baseActual = 0
	}
	delta := baseActual - basePreConsumed

	fundingAdjusted := false
	// 1) 调整资金来源（仅在尚未提交时执行，防止重复调用）
	if delta != 0 {
		if err := s.funding.Settle(delta); err != nil {
			return s.rollbackAfterSettleFailure(delta, fundingAdjusted, err)
		}
		fundingAdjusted = true
	}

	lotteryUsed := 0
	lotteryRefunded := 0
	if s.lotteryReserved > 0 {
		var err error
		lotteryUsed, lotteryRefunded, err = model.SettleLotteryQuota(s.relayInfo.RequestId, plannedLotteryUsed, model.GetDBTimestamp())
		if err != nil {
			return s.rollbackAfterSettleFailure(delta, fundingAdjusted, err)
		}
	}

	// 2) 调整令牌额度
	var tokenErr error
	tokenDelta := actualQuota - s.preConsumedQuota
	if !s.relayInfo.IsPlayground {
		if tokenDelta > 0 {
			tokenErr = model.DecreaseTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, tokenDelta)
		} else if tokenDelta < 0 {
			tokenErr = model.IncreaseTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, -tokenDelta)
		}
	}
	if tokenErr != nil {
		return s.rollbackAfterSettleFailure(delta, fundingAdjusted, tokenErr)
	}
	// 3) 更新 relayInfo 上的订阅 PostDelta（用于日志）
	if s.funding.Source() == BillingSourceSubscription {
		s.relayInfo.SubscriptionPostDelta += int64(delta)
	}
	s.relayInfo.LotteryPreConsumedQuota = s.lotteryReserved
	s.relayInfo.LotteryConsumedQuota = lotteryUsed
	s.relayInfo.LotteryRefundedQuota = lotteryRefunded
	s.relayInfo.FundingPreConsumedQuota = basePreConsumed
	s.relayInfo.FundingActualQuota = baseActual
	s.fundingSettled = true
	s.settled = true
	return nil
}

// Refund 退还所有预扣费，幂等安全，异步执行。
func (s *BillingSession) Refund(c *gin.Context) {
	s.mu.Lock()
	if s.settled || s.refunded || !s.needsRefundLocked() {
		s.mu.Unlock()
		return
	}
	s.refunded = true
	s.mu.Unlock()

	logger.LogInfo(c, fmt.Sprintf("用户 %d 请求失败, 返还预扣费（token_quota=%s, funding=%s）",
		s.relayInfo.UserId,
		logger.FormatQuota(s.tokenConsumed),
		s.funding.Source(),
	))

	// 复制需要的值到闭包中
	tokenId := s.relayInfo.TokenId
	tokenKey := s.relayInfo.TokenKey
	isPlayground := s.relayInfo.IsPlayground
	tokenConsumed := s.tokenConsumed
	funding := s.funding
	requestId := s.relayInfo.RequestId

	gopool.Go(func() {
		if err := model.RefundLotteryQuotaPreConsume(requestId, model.GetDBTimestamp()); err != nil {
			common.SysLog("error refunding lottery quota: " + err.Error())
		}
		// 1) 退还资金来源
		if err := funding.Refund(); err != nil {
			common.SysLog("error refunding billing source: " + err.Error())
		}
		// 2) 退还令牌额度
		if tokenConsumed > 0 && !isPlayground {
			if err := model.IncreaseTokenQuota(tokenId, tokenKey, tokenConsumed); err != nil {
				common.SysLog("error refunding token quota: " + err.Error())
			}
		}
	})
}

// NeedsRefund 返回是否存在需要退还的预扣状态。
func (s *BillingSession) NeedsRefund() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.needsRefundLocked()
}

func (s *BillingSession) needsRefundLocked() bool {
	if s.settled || s.refunded || s.fundingSettled {
		// fundingSettled 时资金来源已提交结算，不能再退预扣费
		return false
	}
	if s.tokenConsumed > 0 {
		return true
	}
	if s.lotteryReserved > 0 {
		return true
	}
	// 订阅可能在 tokenConsumed=0 时仍预扣了额度
	if sub, ok := s.funding.(*SubscriptionFunding); ok && sub.preConsumed > 0 {
		return true
	}
	return false
}

func (s *BillingSession) rollbackAfterSettleFailure(delta int, fundingAdjusted bool, cause error) error {
	if cause == nil {
		return nil
	}

	now := model.GetDBTimestamp()
	errMessages := []string{cause.Error()}

	if fundingAdjusted && delta != 0 {
		if err := s.funding.Settle(-delta); err != nil {
			errMessages = append(errMessages, "rollback funding delta failed: "+err.Error())
		}
	}
	if s.lotteryReserved > 0 {
		if err := model.RollbackLotteryQuotaConsumeRecord(s.relayInfo.RequestId, now); err != nil {
			errMessages = append(errMessages, "rollback lottery quota failed: "+err.Error())
		}
	}
	if err := s.funding.Refund(); err != nil {
		errMessages = append(errMessages, "refund funding pre-consume failed: "+err.Error())
	}
	if s.tokenConsumed > 0 && !s.relayInfo.IsPlayground {
		if err := model.IncreaseTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, s.tokenConsumed); err != nil {
			errMessages = append(errMessages, "refund token pre-consume failed: "+err.Error())
		}
	}

	if len(errMessages) == 1 {
		s.resetAfterFailedSettleRollback()
	}
	return errors.New(strings.Join(errMessages, "; "))
}

func (s *BillingSession) resetAfterFailedSettleRollback() {
	s.preConsumedQuota = 0
	s.fundingPreUsed = 0
	s.tokenConsumed = 0
	s.lotteryReserved = 0
	s.fundingSettled = false
	s.refunded = true
	if wallet, ok := s.funding.(*WalletFunding); ok {
		wallet.consumed = 0
	}
	if sub, ok := s.funding.(*SubscriptionFunding); ok {
		sub.preConsumed = 0
		sub.AmountUsedAfter = 0
	}
	s.syncRelayInfo()
}

// GetPreConsumedQuota 返回实际预扣的额度。
func (s *BillingSession) GetPreConsumedQuota() int {
	return s.preConsumedQuota
}

// ---------------------------------------------------------------------------
// PreConsume — 统一预扣费入口（含信任额度旁路）
// ---------------------------------------------------------------------------

// preConsume 执行预扣费：信任检查 -> 令牌预扣 -> 乐透券预占 -> 资金来源预扣。
// 任一步骤失败时原子回滚已完成的步骤。
func (s *BillingSession) preConsume(c *gin.Context, quota int) *types.NewAPIError {
	requestQuota := quota
	effectiveQuota := requestQuota

	// ---- 信任额度旁路 ----
	if s.shouldTrust(c) {
		effectiveQuota = 0
		logger.LogInfo(c, fmt.Sprintf("用户 %d 额度充足, 信任旁路基础预扣费 (funding=%s)", s.relayInfo.UserId, s.funding.Source()))
	} else if effectiveQuota > 0 {
		logger.LogInfo(c, fmt.Sprintf("用户 %d 需要预扣费 %s (funding=%s)", s.relayInfo.UserId, logger.FormatQuota(effectiveQuota), s.funding.Source()))
	}

	// ---- 1) 预扣令牌额度 ----
	if effectiveQuota > 0 {
		if err := PreConsumeTokenQuota(s.relayInfo, effectiveQuota); err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		s.tokenConsumed = effectiveQuota
	}

	fundingQuota := effectiveQuota
	if requestQuota > 0 && s.canUseLotteryQuota() {
		lotteryReserved, err := model.PreConsumeLotteryQuota(s.relayInfo.RequestId, s.relayInfo.UserId, requestQuota, model.GetDBTimestamp())
		if err != nil {
			if s.tokenConsumed > 0 && !s.relayInfo.IsPlayground {
				if rollbackErr := model.IncreaseTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, s.tokenConsumed); rollbackErr != nil {
					common.SysLog(fmt.Sprintf("error rolling back token quota after lottery pre-consume failure (userId=%d, tokenId=%d, amount=%d, lotteryErr=%s): %s",
						s.relayInfo.UserId, s.relayInfo.TokenId, s.tokenConsumed, err.Error(), rollbackErr.Error()))
				}
				s.tokenConsumed = 0
			}
			return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
		}
		s.lotteryReserved = lotteryReserved
		fundingQuota -= lotteryReserved
		if fundingQuota < 0 {
			fundingQuota = 0
		}
	}
	if fundingQuota <= 0 && requestQuota > 0 && s.funding.Source() == BillingSourceSubscription {
		// Keep a tiny subscription pre-consume anchor so later positive deltas can settle on the same subscription.
		fundingQuota = 1
	}

	// ---- 2) 预扣资金来源 ----
	if err := s.funding.PreConsume(fundingQuota); err != nil {
		if s.lotteryReserved > 0 {
			if refundErr := model.RefundLotteryQuotaPreConsume(s.relayInfo.RequestId, model.GetDBTimestamp()); refundErr != nil {
				common.SysLog("error refunding lottery quota after funding pre-consume failure: " + refundErr.Error())
			}
			s.lotteryReserved = 0
		}
		// 预扣费失败，回滚令牌额度
		if s.tokenConsumed > 0 && !s.relayInfo.IsPlayground {
			if rollbackErr := model.IncreaseTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, s.tokenConsumed); rollbackErr != nil {
				common.SysLog(fmt.Sprintf("error rolling back token quota (userId=%d, tokenId=%d, amount=%d, fundingErr=%s): %s",
					s.relayInfo.UserId, s.relayInfo.TokenId, s.tokenConsumed, err.Error(), rollbackErr.Error()))
			}
			s.tokenConsumed = 0
		}
		// TODO: model 层应定义哨兵错误（如 ErrNoActiveSubscription），用 errors.Is 替代字符串匹配
		errMsg := err.Error()
		if strings.Contains(errMsg, "no active subscription") || strings.Contains(errMsg, "subscription quota insufficient") {
			return types.NewErrorWithStatusCode(fmt.Errorf("订阅额度不足或未配置订阅: %s", errMsg), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
	}

	s.preConsumedQuota = effectiveQuota
	s.fundingPreUsed = fundingQuota

	// ---- 同步 RelayInfo 兼容字段 ----
	s.syncRelayInfo()

	return nil
}

func (s *BillingSession) canUseLotteryQuota() bool {
	if s == nil || s.relayInfo == nil {
		return false
	}
	if s.relayInfo.ForcePreConsume {
		return false
	}
	if s.relayInfo.TaskRelayInfo != nil {
		return false
	}
	return true
}

// shouldTrust 统一信任额度检查，适用于钱包和订阅。
func (s *BillingSession) shouldTrust(c *gin.Context) bool {
	// 异步任务（ForcePreConsume=true）必须预扣全额，不允许信任旁路
	if s.relayInfo.ForcePreConsume {
		return false
	}

	trustQuota := common.GetTrustQuota()
	if trustQuota <= 0 {
		return false
	}

	// 检查令牌是否充足
	tokenTrusted := s.relayInfo.TokenUnlimited
	if !tokenTrusted {
		tokenQuota := c.GetInt("token_quota")
		tokenTrusted = tokenQuota > trustQuota
	}
	if !tokenTrusted {
		return false
	}

	switch s.funding.Source() {
	case BillingSourceWallet:
		return s.relayInfo.UserQuota > trustQuota
	case BillingSourceSubscription:
		// 订阅不能启用信任旁路。原因：
		// 1. PreConsumeUserSubscription 要求 amount>0 来创建预扣记录并锁定订阅
		// 2. SubscriptionFunding.PreConsume 忽略参数，始终用 s.amount 预扣
		// 3. 若信任旁路将 effectiveQuota 设为 0，会导致 preConsumedQuota 与实际订阅预扣不一致
		return false
	default:
		return false
	}
}

// syncRelayInfo 将 BillingSession 的状态同步到 RelayInfo 的兼容字段上。
func (s *BillingSession) syncRelayInfo() {
	info := s.relayInfo
	info.FinalPreConsumedQuota = s.preConsumedQuota
	info.BillingSource = s.funding.Source()
	info.LotteryPreConsumedQuota = s.lotteryReserved
	info.LotteryConsumedQuota = 0
	info.LotteryRefundedQuota = 0
	info.FundingPreConsumedQuota = s.fundingPreUsed
	info.FundingActualQuota = 0

	if sub, ok := s.funding.(*SubscriptionFunding); ok {
		info.SubscriptionId = sub.subscriptionId
		info.SubscriptionPreConsumed = sub.preConsumed
		info.SubscriptionPostDelta = 0
		info.SubscriptionAmountTotal = sub.AmountTotal
		info.SubscriptionAmountUsedAfterPreConsume = sub.AmountUsedAfter
		info.SubscriptionPlanId = sub.PlanId
		info.SubscriptionPlanTitle = sub.PlanTitle
	} else {
		info.SubscriptionId = 0
		info.SubscriptionPreConsumed = 0
	}
}

// ---------------------------------------------------------------------------
// NewBillingSession 工厂 — 根据计费偏好创建会话并处理回退
// ---------------------------------------------------------------------------

// NewBillingSession 根据用户计费偏好创建 BillingSession，处理 subscription_first / wallet_first 的回退。
func NewBillingSession(c *gin.Context, relayInfo *relaycommon.RelayInfo, preConsumedQuota int) (*BillingSession, *types.NewAPIError) {
	if relayInfo == nil {
		return nil, types.NewError(fmt.Errorf("relayInfo is nil"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	pref := common.NormalizeBillingPreference(relayInfo.UserSetting.BillingPreference)

	// 钱包路径需要先检查用户额度
	tryWallet := func() (*BillingSession, *types.NewAPIError) {
		userQuota, err := model.GetUserQuota(relayInfo.UserId, false)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		lotteryQuota := 0
		if relayInfo.TaskRelayInfo == nil {
			lotteryQuota, err = model.GetAvailableLotteryQuotaForUser(relayInfo.UserId, model.GetDBTimestamp())
			if err != nil {
				return nil, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
			}
		}
		if userQuota <= 0 && lotteryQuota <= 0 {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("用户额度不足, 剩余额度: %s", logger.FormatQuota(userQuota)),
				types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		requiredWalletQuota := preConsumedQuota - lotteryQuota
		if requiredWalletQuota < 0 {
			requiredWalletQuota = 0
		}
		if userQuota-requiredWalletQuota < 0 {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("预扣费额度失败, 用户剩余额度: %s, 需要预扣费额度: %s", logger.FormatQuota(userQuota), logger.FormatQuota(requiredWalletQuota)),
				types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		relayInfo.UserQuota = userQuota

		session := &BillingSession{
			relayInfo: relayInfo,
			funding:   &WalletFunding{userId: relayInfo.UserId},
		}
		if apiErr := session.preConsume(c, preConsumedQuota); apiErr != nil {
			return nil, apiErr
		}
		return session, nil
	}

	trySubscription := func() (*BillingSession, *types.NewAPIError) {
		subConsume := int64(preConsumedQuota)
		if subConsume <= 0 {
			subConsume = 1
		}
		session := &BillingSession{
			relayInfo: relayInfo,
			funding: &SubscriptionFunding{
				requestId: relayInfo.RequestId,
				userId:    relayInfo.UserId,
				modelName: relayInfo.OriginModelName,
				amount:    subConsume,
			},
		}
		// 必须传 subConsume 而非 preConsumedQuota，保证 SubscriptionFunding.amount、
		// preConsume 参数和 FinalPreConsumedQuota 三者一致，避免订阅多扣费。
		if apiErr := session.preConsume(c, int(subConsume)); apiErr != nil {
			return nil, apiErr
		}
		return session, nil
	}

	switch pref {
	case "subscription_only":
		return trySubscription()
	case "wallet_only":
		return tryWallet()
	case "wallet_first":
		session, err := tryWallet()
		if err != nil {
			if err.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
				return trySubscription()
			}
			return nil, err
		}
		return session, nil
	case "subscription_first":
		fallthrough
	default:
		hasSub, subCheckErr := model.HasActiveUserSubscription(relayInfo.UserId)
		if subCheckErr != nil {
			return nil, types.NewError(subCheckErr, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		if !hasSub {
			return tryWallet()
		}
		session, apiErr := trySubscription()
		if apiErr != nil {
			if apiErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
				return tryWallet()
			}
			return nil, apiErr
		}
		return session, nil
	}
}
