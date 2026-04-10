package service

import (
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
