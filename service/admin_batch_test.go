package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBatchUsersPreviewAndExecute(t *testing.T) {
	truncate(t)
	req := dto.BatchUsersPreviewRequest{
		Count:           2,
		UsernamePrefix:  "demo",
		StartNumber:     1,
		NumberWidth:     4,
		InitialQuota:    123,
		Group:           "default",
		Status:          common.UserStatusEnabled,
		PasswordMode:    "fixed",
		FixedPassword:   "Passw0rd!",
		ExportFormat:    "txt",
		ExportDelimiter: "comma",
		FileName:        "users-batch",
	}

	preview, err := PreviewBatchUsers(1, req)
	require.NoError(t, err)
	require.NotEmpty(t, preview.PreviewToken)

	resp, err := ExecuteBatchUsers(BatchOperationContext{
		OperatorID:       1,
		OperatorUsername: "root",
		SourceIP:         "127.0.0.1",
		UserAgent:        "test",
	}, dto.BatchUsersExecuteRequest{
		BatchUsersPreviewRequest: req,
		PreviewToken:             preview.PreviewToken,
	})
	require.NoError(t, err)
	require.Equal(t, model.AdminBatchStatusCompleted, resp.Status)
	require.Contains(t, resp.Export.Content, "demo0001,Passw0rd!")
	require.NotContains(t, resp.Export.Content, "demo0001,Passw0rd!,")

	var userCount int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("username IN ?", []string{"demo0001", "demo0002"}).Count(&userCount).Error)
	require.Equal(t, int64(2), userCount)
	var tokenCount int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id IN (SELECT id FROM users WHERE username IN ?)", []string{"demo0001", "demo0002"}).Count(&tokenCount).Error)
	require.Equal(t, int64(2), tokenCount)

	job, err := model.GetAdminBatchJobByBatchID(resp.BatchID)
	require.NoError(t, err)
	require.Equal(t, 2, job.SuccessCount)
	items, err := model.ListAdminBatchJobItems(job.Id)
	require.NoError(t, err)
	require.Len(t, items, 2)
}

func TestBatchRedemptionsPreviewAndExecute(t *testing.T) {
	truncate(t)
	plan := &model.SubscriptionPlan{Id: 1, Title: "周卡", PriceAmount: 9.9, Currency: "CNY", DurationUnit: "day", DurationValue: 7, TotalAmount: 1000}
	require.NoError(t, model.DB.Create(plan).Error)

	req := dto.BatchRedemptionsPreviewRequest{
		PlanID:       plan.Id,
		Count:        2,
		CodeLength:   8,
		Prefix:       "WK-",
		Status:       common.RedemptionCodeStatusEnabled,
		ExportFormat: "txt",
		FileName:     "codes",
	}

	preview, err := PreviewBatchRedemptions(1, req)
	require.NoError(t, err)
	require.NotEmpty(t, preview.PreviewToken)

	resp, err := ExecuteBatchRedemptions(BatchOperationContext{OperatorID: 1, OperatorUsername: "root"}, dto.BatchRedemptionsExecuteRequest{
		BatchRedemptionsPreviewRequest: req,
		PreviewToken:                   preview.PreviewToken,
	})
	require.NoError(t, err)
	require.Equal(t, model.AdminBatchStatusCompleted, resp.Status)

	var count int64
	require.NoError(t, model.DB.Model(&model.Redemption{}).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestBatchQuotaRedemptionsPreviewAndExecute(t *testing.T) {
	truncate(t)
	req := dto.BatchRedemptionsPreviewRequest{
		RedeemType:   model.RedemptionTypeQuota,
		Quota:        250,
		Count:        2,
		CodeLength:   8,
		Prefix:       "QT-",
		Status:       common.RedemptionCodeStatusEnabled,
		ExportFormat: "txt",
		FileName:     "quota-codes",
	}

	preview, err := PreviewBatchRedemptions(1, req)
	require.NoError(t, err)
	require.NotEmpty(t, preview.PreviewToken)

	resp, err := ExecuteBatchRedemptions(BatchOperationContext{OperatorID: 1, OperatorUsername: "root"}, dto.BatchRedemptionsExecuteRequest{
		BatchRedemptionsPreviewRequest: req,
		PreviewToken:                   preview.PreviewToken,
	})
	require.NoError(t, err)
	require.Equal(t, model.AdminBatchStatusCompleted, resp.Status)

	var redemptions []model.Redemption
	require.NoError(t, model.DB.Order("id asc").Find(&redemptions).Error)
	require.Len(t, redemptions, 2)
	for _, redemption := range redemptions {
		require.Equal(t, model.RedemptionTypeQuota, redemption.RedeemType)
		require.Equal(t, req.Quota, redemption.Quota)
		require.Equal(t, 0, redemption.SubscriptionPlanId)
	}
}

func TestBatchQuotaPreviewExcludesAdmins(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.Create(&model.User{Username: "root", Password: "x", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "root0001"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Username: "demo001", Password: "x", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "demo0001"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Username: "demo002", Password: "x", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "demo0002"}).Error)

	preview, err := PreviewBatchQuota(1, dto.BatchQuotaPreviewRequest{
		ScopeType:     "all",
		IncludeAdmins: false,
		QuotaDelta:    50,
		Reason:        "campaign",
	})
	require.NoError(t, err)
	summary, ok := preview.Summary.(batchQuotaPreviewSummary)
	require.True(t, ok)
	require.Equal(t, 2, summary.MatchedCount)
	resp, err := ExecuteBatchQuota(BatchOperationContext{OperatorID: 1, OperatorUsername: "root"}, dto.BatchQuotaExecuteRequest{
		BatchQuotaPreviewRequest: dto.BatchQuotaPreviewRequest{
			ScopeType:     "all",
			IncludeAdmins: false,
			QuotaDelta:    50,
			Reason:        "campaign",
		},
		PreviewToken: preview.PreviewToken,
	})
	require.NoError(t, err)
	require.Equal(t, model.AdminBatchStatusCompleted, resp.Status)

	var root model.User
	require.NoError(t, model.DB.Where("username = ?", "root").First(&root).Error)
	require.Equal(t, 0, root.Quota)
}

func TestExtendAllActiveSubscriptionsForUser_RecomputesNextReset(t *testing.T) {
	truncate(t)

	require.NoError(t, model.DB.Create(&model.User{
		Id:       101,
		Username: "sub-user",
		Password: "x",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "subuser001",
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:                       201,
		Title:                    "Hourly",
		PriceAmount:              1,
		Currency:                 "USD",
		DurationUnit:             model.SubscriptionDurationCustom,
		CustomSeconds:            7200,
		QuotaResetPeriod:         model.SubscriptionResetCustom,
		QuotaResetCustomSeconds:  3600,
	}).Error)

	now := model.GetDBTimestamp()
	active := &model.UserSubscription{
		Id:            301,
		UserId:        101,
		PlanId:        201,
		Status:        "active",
		StartTime:     now - 7200,
		EndTime:       now + 1800,
		LastResetTime: 0,
		NextResetTime: 0,
	}
	expired := &model.UserSubscription{
		Id:        302,
		UserId:    101,
		PlanId:    201,
		Status:    "expired",
		StartTime: now - 7200,
		EndTime:   now - 10,
	}
	require.NoError(t, model.DB.Create(active).Error)
	require.NoError(t, model.DB.Create(expired).Error)

	updatedCount, err := model.ExtendAllActiveSubscriptionsForUser(101, 7200)
	require.NoError(t, err)
	require.Equal(t, 1, updatedCount)

	var reloadedActive model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", active.Id).First(&reloadedActive).Error)
	require.Equal(t, active.EndTime+7200, reloadedActive.EndTime)
	require.NotZero(t, reloadedActive.NextResetTime)
	require.Greater(t, reloadedActive.NextResetTime, now)
	require.LessOrEqual(t, reloadedActive.NextResetTime, reloadedActive.EndTime)
	require.GreaterOrEqual(t, reloadedActive.LastResetTime, now)

	var reloadedExpired model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", expired.Id).First(&reloadedExpired).Error)
	require.Equal(t, expired.EndTime, reloadedExpired.EndTime)
}

func TestBatchSubscriptionsExtendPreviewAndExecute(t *testing.T) {
	truncate(t)

	users := []*model.User{
		{Id: 1, Username: "user001", Password: "x", Status: common.UserStatusEnabled, Group: "default", AffCode: "user001"},
		{Id: 2, Username: "user002", Password: "x", Status: common.UserStatusEnabled, Group: "default", AffCode: "user002"},
		{Id: 3, Username: "user003", Password: "x", Status: common.UserStatusEnabled, Group: "default", AffCode: "user003"},
	}
	for _, user := range users {
		require.NoError(t, model.DB.Create(user).Error)
	}

	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:                       1001,
		Title:                    "Plan A",
		PriceAmount:              1,
		Currency:                 "USD",
		DurationUnit:             model.SubscriptionDurationCustom,
		CustomSeconds:            7200,
		QuotaResetPeriod:         model.SubscriptionResetCustom,
		QuotaResetCustomSeconds:  3600,
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:           1002,
		Title:        "Plan B",
		PriceAmount:  1,
		Currency:     "USD",
		DurationUnit: model.SubscriptionDurationDay,
		DurationValue: 1,
	}).Error)

	now := time.Now().Unix()
	activeAEnd := now + 1800
	activeBEnd := now + 3600
	activeCEnd := now + 5400
	expiredEnd := now - 60

	subs := []*model.UserSubscription{
		{Id: 1, UserId: 1, PlanId: 1001, Status: "active", StartTime: now - 7200, EndTime: activeAEnd, LastResetTime: 0, NextResetTime: 0},
		{Id: 2, UserId: 1, PlanId: 1002, Status: "active", StartTime: now - 3600, EndTime: activeBEnd},
		{Id: 3, UserId: 1, PlanId: 1002, Status: "expired", StartTime: now - 7200, EndTime: expiredEnd},
		{Id: 4, UserId: 2, PlanId: 1002, Status: "active", StartTime: now - 1800, EndTime: activeCEnd},
		{Id: 5, UserId: 2, PlanId: 1002, Status: "cancelled", StartTime: now - 1800, EndTime: activeCEnd},
	}
	for _, sub := range subs {
		require.NoError(t, model.DB.Create(sub).Error)
	}

	req := dto.BatchSubscriptionsExtendPreviewRequest{Days: 1}
	preview, err := PreviewBatchSubscriptionsExtend(1, req)
	require.NoError(t, err)
	require.NotEmpty(t, preview.PreviewToken)
	require.True(t, preview.CanExecute)

	previewSummary, ok := preview.Summary.(batchSubscriptionsExtendPreviewSummary)
	require.True(t, ok)
	require.Equal(t, 3, previewSummary.TotalUsers)
	require.Equal(t, 2, previewSummary.MatchedUserCount)
	require.Equal(t, 3, previewSummary.MatchedSubscriptionCount)
	require.Equal(t, 1, previewSummary.SkippedUserCount)
	require.Equal(t, 1, previewSummary.ExtensionDays)

	resp, err := ExecuteBatchSubscriptionsExtend(BatchOperationContext{
		OperatorID:       1,
		OperatorUsername: "root",
		SourceIP:         "127.0.0.1",
		UserAgent:        "test",
	}, dto.BatchSubscriptionsExtendExecuteRequest{
		BatchSubscriptionsExtendPreviewRequest: req,
		PreviewToken:                           preview.PreviewToken,
	})
	require.NoError(t, err)
	require.Equal(t, model.AdminBatchStatusCompleted, resp.Status)

	execSummary, ok := resp.Summary.(batchSubscriptionsExtendExecuteSummary)
	require.True(t, ok)
	require.Equal(t, 3, execSummary.TotalUsers)
	require.Equal(t, 2, execSummary.MatchedUserCount)
	require.Equal(t, 2, execSummary.SuccessUserCount)
	require.Equal(t, 1, execSummary.SkippedUserCount)
	require.Equal(t, 0, execSummary.FailedUserCount)
	require.Equal(t, 3, execSummary.UpdatedSubscriptionCount)

	var activeA model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 1).First(&activeA).Error)
	require.Equal(t, activeAEnd+86400, activeA.EndTime)
	require.NotZero(t, activeA.NextResetTime)

	var activeB model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 2).First(&activeB).Error)
	require.Equal(t, activeBEnd+86400, activeB.EndTime)

	var expired model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3).First(&expired).Error)
	require.Equal(t, expiredEnd, expired.EndTime)

	var activeC model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 4).First(&activeC).Error)
	require.Equal(t, activeCEnd+86400, activeC.EndTime)

	var cancelled model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 5).First(&cancelled).Error)
	require.Equal(t, activeCEnd, cancelled.EndTime)

	job, err := model.GetAdminBatchJobByBatchID(resp.BatchID)
	require.NoError(t, err)
	require.Equal(t, model.AdminBatchTypeSubscriptionsExtend, job.BatchType)
	require.Equal(t, 2, job.SuccessCount)
	require.Equal(t, 0, job.FailedCount)
	items, err := model.ListAdminBatchJobItems(job.Id)
	require.NoError(t, err)
	require.Len(t, items, 2)
}

func TestBatchSubscriptionsExtendPreviewWithoutActiveSubscriptions(t *testing.T) {
	truncate(t)

	require.NoError(t, model.DB.Create(&model.User{
		Id:       11,
		Username: "user011",
		Password: "x",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "user011",
	}).Error)

	preview, err := PreviewBatchSubscriptionsExtend(1, dto.BatchSubscriptionsExtendPreviewRequest{Days: 1})
	require.NoError(t, err)
	require.False(t, preview.CanExecute)

	summary, ok := preview.Summary.(batchSubscriptionsExtendPreviewSummary)
	require.True(t, ok)
	require.Equal(t, 1, summary.TotalUsers)
	require.Equal(t, 0, summary.MatchedUserCount)
	require.Equal(t, 0, summary.MatchedSubscriptionCount)
	require.Equal(t, 1, summary.SkippedUserCount)
}
