package service

import (
	"testing"

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
	require.Contains(t, resp.Export.Content, "demo0001,Passw0rd!,")

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
