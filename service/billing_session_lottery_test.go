package service

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubFunding struct {
	source          string
	preConsumeErr   error
	settleErr       error
	refundErr       error
	preConsumeCalls int
	settleCalls     []int
	refundCalls     int
}

func (s *stubFunding) Source() string {
	if s.source == "" {
		return BillingSourceWallet
	}
	return s.source
}

func (s *stubFunding) PreConsume(amount int) error {
	s.preConsumeCalls++
	return s.preConsumeErr
}

func (s *stubFunding) Settle(delta int) error {
	s.settleCalls = append(s.settleCalls, delta)
	if s.settleErr != nil {
		return s.settleErr
	}
	return nil
}

func (s *stubFunding) Refund() error {
	s.refundCalls++
	return s.refundErr
}

func seedLotteryActivatedReward(t *testing.T, activityId int, userId int, amountUSD int, now int64) *model.LotteryReward {
	t.Helper()
	reward := &model.LotteryReward{
		ActivityId:      activityId,
		OwnerUserId:     userId,
		SourceUserId:    userId,
		SourceDrawIndex: 1,
		TierName:        "传说",
		Amount:          amountUSD,
		QuotaTotal:      model.GetLotteryQuotaAmountByUSD(amountUSD),
		QuotaRemaining:  model.GetLotteryQuotaAmountByUSD(amountUSD),
		Status:          model.LotteryRewardStatusActivated,
		AutoActivateAt:  now - 3600,
		ConsumeStartsAt: now - 1800,
		ExpiresAt:       now + 3600,
		ActivatedAt:     now - 1800,
	}
	require.NoError(t, model.DB.Create(reward).Error)
	return reward
}

func seedLotteryPendingAutoActivateReward(t *testing.T, activityId int, userId int, amountUSD int, now int64) *model.LotteryReward {
	t.Helper()
	reward := &model.LotteryReward{
		ActivityId:      activityId,
		OwnerUserId:     userId,
		SourceUserId:    userId,
		SourceDrawIndex: 1,
		TierName:        "普通",
		Amount:          amountUSD,
		QuotaTotal:      model.GetLotteryQuotaAmountByUSD(amountUSD),
		QuotaRemaining:  model.GetLotteryQuotaAmountByUSD(amountUSD),
		Status:          model.LotteryRewardStatusPendingActivation,
		AutoActivateAt:  now - 60,
		ConsumeStartsAt: now - 60,
		ExpiresAt:       now + 3600,
	}
	require.NoError(t, model.DB.Create(reward).Error)
	return reward
}

func TestBillingSession_AutoActivatesPendingLotteryRewardBeforeWalletBilling(t *testing.T) {
	truncate(t)

	const userID, tokenID = 43, 43
	initUserQuota := 0
	tokenRemain := int(100 * common.QuotaPerUnit)
	preConsumeQuota := int(20 * common.QuotaPerUnit)
	actualQuota := int(12 * common.QuotaPerUnit)
	now := time.Now().Unix()

	seedUser(t, userID, initUserQuota)
	seedToken(t, tokenID, userID, "sk-lottery-wallet-pending", tokenRemain)
	reward := seedLotteryPendingAutoActivateReward(t, 1, userID, 50, now)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "billing-lottery-wallet-pending-1",
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        "sk-lottery-wallet-pending",
		TokenUnlimited:  false,
		OriginModelName: "gpt-5.4",
		IsPlayground:    true,
		ForcePreConsume: false,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, preConsumeQuota)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	require.NoError(t, model.DB.First(&reward, reward.Id).Error)
	assert.Equal(t, model.LotteryRewardStatusActivated, reward.Status)
	assert.Equal(t, now-60, reward.ActivatedAt)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(30), reward.QuotaRemaining)
	assert.Equal(t, initUserQuota, getUserQuota(t, userID))
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(20), relayInfo.LotteryPreConsumedQuota)
	assert.Equal(t, 0, relayInfo.FundingPreConsumedQuota)

	require.NoError(t, session.Settle(actualQuota))

	require.NoError(t, model.DB.First(&reward, reward.Id).Error)
	assert.Equal(t, model.LotteryRewardStatusActivated, reward.Status)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(38), reward.QuotaRemaining)
	assert.Equal(t, actualQuota, relayInfo.LotteryConsumedQuota)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(8), relayInfo.LotteryRefundedQuota)
	assert.Equal(t, 0, relayInfo.FundingActualQuota)
}

func TestBillingSession_UsesLotteryQuotaBeforeWallet(t *testing.T) {
	truncate(t)

	const userID, tokenID = 41, 41
	initUserQuota := int(10 * common.QuotaPerUnit)
	tokenRemain := int(100 * common.QuotaPerUnit)
	preConsumeQuota := int(20 * common.QuotaPerUnit)
	actualQuota := int(12 * common.QuotaPerUnit)
	now := time.Now().Unix()

	seedUser(t, userID, initUserQuota)
	seedToken(t, tokenID, userID, "sk-lottery-wallet", tokenRemain)
	seedLotteryActivatedReward(t, 1, userID, 50, now)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "billing-lottery-wallet-1",
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        "sk-lottery-wallet",
		TokenUnlimited:  false,
		OriginModelName: "gpt-5.4",
		IsPlayground:    true,
		ForcePreConsume: false,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, preConsumeQuota)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	var reward model.LotteryReward
	require.NoError(t, model.DB.First(&reward).Error)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(30), reward.QuotaRemaining)
	assert.Equal(t, initUserQuota, getUserQuota(t, userID))
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(20), relayInfo.LotteryPreConsumedQuota)
	assert.Equal(t, 0, relayInfo.FundingPreConsumedQuota)

	require.NoError(t, session.Settle(actualQuota))

	require.NoError(t, model.DB.First(&reward).Error)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(38), reward.QuotaRemaining)
	assert.Equal(t, initUserQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, relayInfo.LotteryConsumedQuota)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(8), relayInfo.LotteryRefundedQuota)
	assert.Equal(t, 0, relayInfo.FundingActualQuota)
}

func TestBillingSession_UsesLotteryQuotaBeforeSubscriptionAndKeepsSettleAnchor(t *testing.T) {
	truncate(t)

	const userID, tokenID = 42, 42
	const subscriptionID, planID = 7, 7
	initUserQuota := 0
	tokenRemain := int(100 * common.QuotaPerUnit)
	preConsumeQuota := int(20 * common.QuotaPerUnit)
	actualQuota := int(25 * common.QuotaPerUnit)
	now := time.Now().Unix()

	seedUser(t, userID, initUserQuota)
	seedToken(t, tokenID, userID, "sk-lottery-subscription", tokenRemain)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:            planID,
		Title:         "大乐透订阅测试",
		PriceAmount:   9.9,
		Currency:      "USD",
		DurationUnit:  "month",
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   int64(100 * common.QuotaPerUnit),
	}).Error)
	seedSubscription(t, subscriptionID, userID, int64(100*common.QuotaPerUnit), 0)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).
		Where("id = ?", subscriptionID).
		Update("plan_id", planID).Error)
	seedLotteryActivatedReward(t, 1, userID, 50, now)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "billing-lottery-subscription-1",
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        "sk-lottery-subscription",
		TokenUnlimited:  false,
		OriginModelName: "gpt-5.4",
		IsPlayground:    true,
		ForcePreConsume: false,
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_only",
		},
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, preConsumeQuota)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	var reward model.LotteryReward
	require.NoError(t, model.DB.First(&reward).Error)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(30), reward.QuotaRemaining)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(20), relayInfo.LotteryPreConsumedQuota)
	assert.Equal(t, 1, relayInfo.FundingPreConsumedQuota)
	assert.Equal(t, int64(1), getSubscriptionUsed(t, subscriptionID))

	require.NoError(t, session.Settle(actualQuota))

	require.NoError(t, model.DB.First(&reward).Error)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(30), reward.QuotaRemaining)
	assert.Equal(t, int64(5*common.QuotaPerUnit), getSubscriptionUsed(t, subscriptionID))
	assert.Equal(t, preConsumeQuota, relayInfo.LotteryConsumedQuota)
	assert.Equal(t, 0, relayInfo.LotteryRefundedQuota)
	assert.Equal(t, int(5*common.QuotaPerUnit), relayInfo.FundingActualQuota)
	assert.Equal(t, int64(relayInfo.FundingActualQuota-relayInfo.FundingPreConsumedQuota), relayInfo.SubscriptionPostDelta)
}

func TestBillingSession_UsesLotteryQuotaBeforeTrustedWalletBilling(t *testing.T) {
	truncate(t)

	const userID, tokenID = 44, 44
	initUserQuota := int(20 * common.QuotaPerUnit)
	preConsumeQuota := int(20 * common.QuotaPerUnit)
	actualQuota := int(12 * common.QuotaPerUnit)
	now := time.Now().Unix()

	seedUser(t, userID, initUserQuota)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             tokenID,
		UserId:         userID,
		Key:            "sk-lottery-wallet-trusted",
		Name:           "trusted_wallet_token",
		Status:         common.TokenStatusEnabled,
		UnlimitedQuota: true,
		RemainQuota:    0,
	}).Error)
	seedLotteryActivatedReward(t, 1, userID, 50, now)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "billing-lottery-wallet-trusted-1",
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        "sk-lottery-wallet-trusted",
		TokenUnlimited:  true,
		OriginModelName: "gpt-5.4",
		IsPlayground:    true,
		ForcePreConsume: false,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, preConsumeQuota)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, 0, session.GetPreConsumedQuota())

	var reward model.LotteryReward
	require.NoError(t, model.DB.First(&reward).Error)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(30), reward.QuotaRemaining)
	assert.Equal(t, initUserQuota, getUserQuota(t, userID))
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(20), relayInfo.LotteryPreConsumedQuota)
	assert.Equal(t, 0, relayInfo.FundingPreConsumedQuota)

	require.NoError(t, session.Settle(actualQuota))

	require.NoError(t, model.DB.First(&reward).Error)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(38), reward.QuotaRemaining)
	assert.Equal(t, initUserQuota, getUserQuota(t, userID))
	assert.Equal(t, actualQuota, relayInfo.LotteryConsumedQuota)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(8), relayInfo.LotteryRefundedQuota)
	assert.Equal(t, 0, relayInfo.FundingActualQuota)
}

func TestBillingSession_TrustedWalletFallsBackToWalletAfterLottery(t *testing.T) {
	truncate(t)

	const userID, tokenID = 45, 45
	initUserQuota := int(20 * common.QuotaPerUnit)
	preConsumeQuota := int(20 * common.QuotaPerUnit)
	actualQuota := int(25 * common.QuotaPerUnit)
	now := time.Now().Unix()

	seedUser(t, userID, initUserQuota)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             tokenID,
		UserId:         userID,
		Key:            "sk-lottery-wallet-trusted-overage",
		Name:           "trusted_wallet_token_overage",
		Status:         common.TokenStatusEnabled,
		UnlimitedQuota: true,
		RemainQuota:    0,
	}).Error)
	seedLotteryActivatedReward(t, 1, userID, 20, now)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "billing-lottery-wallet-trusted-2",
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        "sk-lottery-wallet-trusted-overage",
		TokenUnlimited:  true,
		OriginModelName: "gpt-5.4",
		IsPlayground:    true,
		ForcePreConsume: false,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
	}

	session, apiErr := NewBillingSession(ctx, relayInfo, preConsumeQuota)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, 0, session.GetPreConsumedQuota())
	assert.Equal(t, initUserQuota, getUserQuota(t, userID))
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(20), relayInfo.LotteryPreConsumedQuota)
	assert.Equal(t, 0, relayInfo.FundingPreConsumedQuota)

	require.NoError(t, session.Settle(actualQuota))

	var reward model.LotteryReward
	require.NoError(t, model.DB.First(&reward).Error)
	assert.Equal(t, 0, reward.QuotaRemaining)
	assert.Equal(t, model.LotteryRewardStatusConsumed, reward.Status)
	assert.Equal(t, initUserQuota-int(5*common.QuotaPerUnit), getUserQuota(t, userID))
	assert.Equal(t, preConsumeQuota, relayInfo.LotteryConsumedQuota)
	assert.Equal(t, 0, relayInfo.LotteryRefundedQuota)
	assert.Equal(t, int(5*common.QuotaPerUnit), relayInfo.FundingActualQuota)
}

func TestBillingSession_TrustedLotterySettleFailureRollsBackReservedReward(t *testing.T) {
	truncate(t)

	const userID = 46
	preConsumeQuota := int(20 * common.QuotaPerUnit)
	actualQuota := int(25 * common.QuotaPerUnit)
	initUserQuota := int(20 * common.QuotaPerUnit)
	now := time.Now().Unix()

	seedUser(t, userID, initUserQuota)
	reward := seedLotteryActivatedReward(t, 1, userID, 20, now)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "billing-lottery-wallet-trusted-settle-fail-1",
		UserId:          userID,
		TokenUnlimited:  true,
		OriginModelName: "gpt-5.4",
		IsPlayground:    true,
		ForcePreConsume: false,
		UserQuota:       initUserQuota,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
	}
	funding := &stubFunding{
		source:    BillingSourceWallet,
		settleErr: errors.New("forced settle failure"),
	}
	session := &BillingSession{
		relayInfo: relayInfo,
		funding:   funding,
	}

	apiErr := session.preConsume(ctx, preConsumeQuota)
	require.Nil(t, apiErr)
	require.NoError(t, model.DB.First(&reward, reward.Id).Error)
	assert.Equal(t, 0, reward.QuotaRemaining)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(20), relayInfo.LotteryPreConsumedQuota)

	err := session.Settle(actualQuota)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced settle failure")

	require.NoError(t, model.DB.First(&reward, reward.Id).Error)
	assert.Equal(t, model.GetLotteryQuotaAmountByUSD(20), reward.QuotaRemaining)
	assert.Equal(t, model.LotteryRewardStatusActivated, reward.Status)

	var record model.LotteryConsumeRecord
	require.NoError(t, model.DB.Where("request_id = ?", relayInfo.RequestId).First(&record).Error)
	assert.Equal(t, model.LotteryConsumeRecordStatusRefunded, record.Status)
	assert.Equal(t, 0, record.SettledQuota)
	assert.Equal(t, preConsumeQuota, record.RefundedQuota)
	assert.False(t, session.NeedsRefund())
	assert.Equal(t, 1, funding.refundCalls)
	assert.Equal(t, []int{int(5 * common.QuotaPerUnit)}, funding.settleCalls)
}
