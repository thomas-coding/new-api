package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

const (
	batchPreviewTTLSeconds    = 15 * 60
	batchUsersMaxCount        = 200
	batchRedemptionsMaxCount  = 500
	batchQuotaMaxMatchedUsers = 500
	batchSubscriptionMaxDays  = 365

	batchPasswordModeRandom = "random"
	batchPasswordModeFixed  = "fixed"

	batchScopeAll   = "all"
	batchScopeGroup = "group"

	batchExportFormatTXT = "txt"
	batchExportFormatCSV = "csv"

	batchDelimiterComma = "comma"
	batchDelimiterDash  = "dash"
)

type batchPreviewTokenPayload struct {
	BatchType  string `json:"batch_type"`
	OperatorID int    `json:"operator_id"`
	ExpiresAt  int64  `json:"expires_at"`
	Payload    string `json:"payload"`
}

type BatchOperationContext struct {
	OperatorID       int
	OperatorUsername string
	SourceIP         string
	UserAgent        string
}

type batchUsersPreviewSummary struct {
	RequestedCount int      `json:"requested_count"`
	EffectiveCount int      `json:"effective_count"`
	ConflictCount  int      `json:"conflict_count"`
	TotalQuota     int      `json:"total_quota"`
	Group          string   `json:"group"`
	Samples        []string `json:"samples,omitempty"`
}

type batchRedemptionsPreviewSummary struct {
	RequestedCount int      `json:"requested_count"`
	RedeemType     string   `json:"redeem_type"`
	PlanID         int      `json:"plan_id,omitempty"`
	PlanTitle      string   `json:"plan_title,omitempty"`
	Quota          int      `json:"quota,omitempty"`
	CodeLength     int      `json:"code_length"`
	Prefix         string   `json:"prefix"`
	ExpiredTime    int64    `json:"expired_time"`
	Samples        []string `json:"samples,omitempty"`
}

type batchQuotaPreviewSummary struct {
	MatchedCount    int      `json:"matched_count"`
	TotalQuota      int      `json:"total_quota"`
	ScopeType       string   `json:"scope_type"`
	Group           string   `json:"group,omitempty"`
	IncludeAdmins   bool     `json:"include_admins"`
	SampleUsernames []string `json:"sample_usernames,omitempty"`
}

type batchSubscriptionsExtendPreviewSummary struct {
	TotalUsers                int      `json:"total_users"`
	MatchedUserCount          int      `json:"matched_user_count"`
	MatchedSubscriptionCount  int      `json:"matched_subscription_count"`
	SkippedUserCount          int      `json:"skipped_user_count"`
	ExtensionDays             int      `json:"extension_days"`
	ExtensionSeconds          int64    `json:"extension_seconds"`
	SampleUsernames           []string `json:"sample_usernames,omitempty"`
}

type batchUsersExecuteSummary struct {
	RequestedCount int `json:"requested_count"`
	SuccessCount   int `json:"success_count"`
	FailedCount    int `json:"failed_count"`
	TotalQuota     int `json:"total_quota"`
}

type batchRedemptionsExecuteSummary struct {
	RequestedCount int    `json:"requested_count"`
	SuccessCount   int    `json:"success_count"`
	FailedCount    int    `json:"failed_count"`
	RedeemType     string `json:"redeem_type"`
	PlanID         int    `json:"plan_id,omitempty"`
	PlanTitle      string `json:"plan_title,omitempty"`
	Quota          int    `json:"quota,omitempty"`
}

type batchQuotaExecuteSummary struct {
	MatchedCount int    `json:"matched_count"`
	SuccessCount int    `json:"success_count"`
	FailedCount  int    `json:"failed_count"`
	TotalQuota   int    `json:"total_quota"`
	Reason       string `json:"reason"`
}

type batchSubscriptionsExtendExecuteSummary struct {
	TotalUsers               int   `json:"total_users"`
	MatchedUserCount         int   `json:"matched_user_count"`
	SuccessUserCount         int   `json:"success_user_count"`
	SkippedUserCount         int   `json:"skipped_user_count"`
	FailedUserCount          int   `json:"failed_user_count"`
	UpdatedSubscriptionCount int   `json:"updated_subscription_count"`
	ExtensionDays            int   `json:"extension_days"`
	ExtensionSeconds         int64 `json:"extension_seconds"`
}

type batchSubscriptionTarget struct {
	UserID                  int
	Username                string
	ActiveSubscriptionCount int
}

func PreviewBatchUsers(operatorID int, req dto.BatchUsersPreviewRequest) (*dto.BatchPreviewResponse, error) {
	validated, summary, warnings, err := buildBatchUsersPreview(req)
	if err != nil {
		return nil, err
	}
	payloadBytes, err := common.Marshal(validated)
	if err != nil {
		return nil, err
	}
	token, expiresAt, err := buildPreviewToken(model.AdminBatchTypeUsersCreate, operatorID, string(payloadBytes))
	if err != nil {
		return nil, err
	}
	return &dto.BatchPreviewResponse{
		BatchType:        model.AdminBatchTypeUsersCreate,
		Stage:            "preview",
		PreviewToken:     token,
		PreviewExpiresAt: expiresAt,
		RiskLevel:        previewRiskLevel(summary.EffectiveCount),
		Warnings:         warnings,
		Summary:          summary,
		Samples:          summary.Samples,
		CanExecute:       summary.EffectiveCount > 0,
	}, nil
}

func ExecuteBatchUsers(ctx BatchOperationContext, req dto.BatchUsersExecuteRequest) (*dto.BatchExecuteResponse, error) {
	previewReq := req.BatchUsersPreviewRequest
	validated, previewSummary, _, err := buildBatchUsersPreview(previewReq)
	if err != nil {
		return nil, err
	}
	if err := verifyPreviewToken(model.AdminBatchTypeUsersCreate, ctx.OperatorID, req.PreviewToken, validated); err != nil {
		return nil, err
	}
	batchID := generateBatchID("USR")
	requestPayloadMasked, _ := marshalToJSONString(maskBatchUsersRequest(validated))
	scopeSummary, _ := marshalToJSONString(previewSummary)
	exportMeta, _ := marshalToJSONString(map[string]interface{}{
		"format":    validated.ExportFormat,
		"file_name": validated.FileName,
	})
	job := model.NewAdminBatchJob(batchID, model.AdminBatchTypeUsersCreate, ctx.OperatorID, ctx.OperatorUsername, requestPayloadMasked, scopeSummary, exportMeta, ctx.SourceIP, ctx.UserAgent)
	if err := model.CreateAdminBatchJob(job); err != nil {
		return nil, err
	}

	items := make([]model.AdminBatchJobItem, 0, validated.Count)
	failedItems := make([]dto.BatchFailureItem, 0)
	createdLines := make([]string, 0)
	successCount := 0
	failedCount := 0
	for i, username := range buildPreviewUsernames(validated) {
		exists, err := model.CheckUserExistOrDeleted(username, "")
		if err != nil {
			failedCount++
			failedItems = append(failedItems, dto.BatchFailureItem{TargetKey: username, ReasonCode: "lookup_failed", ReasonMessage: err.Error()})
			items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeUser, username, 0, model.AdminBatchItemResultFailed, "lookup_failed", err.Error(), 0, ""))
			continue
		}
		if exists {
			failedCount++
			failedItems = append(failedItems, dto.BatchFailureItem{TargetKey: username, ReasonCode: "username_conflict", ReasonMessage: "username already exists"})
			items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeUser, username, 0, model.AdminBatchItemResultSkipped, "username_conflict", "username already exists", 0, ""))
			continue
		}
		password, err := buildBatchUserPassword(validated)
		if err != nil {
			return nil, err
		}
		user, token, err := createBatchUser(username, password, validated)
		if err != nil {
			failedCount++
			failedItems = append(failedItems, dto.BatchFailureItem{TargetKey: username, ReasonCode: "create_failed", ReasonMessage: err.Error()})
			items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeUser, username, 0, model.AdminBatchItemResultFailed, "create_failed", err.Error(), 0, ""))
			continue
		}
		successCount++
		line := formatUserExportLine(validated.ExportFormat, validated.ExportDelimiter, user.Username, password, token.Key, user.Group, user.Quota, user.Status)
		createdLines = append(createdLines, line)
		items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeUser, user.Username, user.Id, model.AdminBatchItemResultSuccess, "", "", 0, ""))
		_ = i
	}
	if err := model.CreateAdminBatchJobItems(items); err != nil {
		return nil, err
	}
	summary := batchUsersExecuteSummary{
		RequestedCount: validated.Count,
		SuccessCount:   successCount,
		FailedCount:    failedCount,
		TotalQuota:     successCount * validated.InitialQuota,
	}
	summaryJSON, _ := marshalToJSONString(summary)
	status := summarizeBatchStatus(successCount, failedCount)
	if err := model.UpdateAdminBatchJobResult(job, status, successCount, failedCount, summaryJSON); err != nil {
		return nil, err
	}
	content := strings.Join(createdLines, "\n")
	if content != "" {
		content += "\n"
	}
	return &dto.BatchExecuteResponse{
		BatchID:     batchID,
		BatchType:   model.AdminBatchTypeUsersCreate,
		Stage:       "execute",
		Status:      status,
		Summary:     summary,
		FailedItems: failedItems,
		Export: dto.BatchExportPayload{
			Available:   successCount > 0,
			Format:      validated.ExportFormat,
			Filename:    resolveFileName(validated.FileName, fmt.Sprintf("users-%s", strings.ToLower(batchID)), validated.ExportFormat),
			Content:     content,
			Sensitive:   true,
			OneTimeOnly: true,
		},
	}, nil
}

func PreviewBatchRedemptions(operatorID int, req dto.BatchRedemptionsPreviewRequest) (*dto.BatchPreviewResponse, error) {
	validated, plan, summary, warnings, err := buildBatchRedemptionsPreview(req)
	if err != nil {
		return nil, err
	}
	_ = plan
	payloadBytes, err := common.Marshal(validated)
	if err != nil {
		return nil, err
	}
	token, expiresAt, err := buildPreviewToken(model.AdminBatchTypeRedemptionsCreate, operatorID, string(payloadBytes))
	if err != nil {
		return nil, err
	}
	return &dto.BatchPreviewResponse{
		BatchType:        model.AdminBatchTypeRedemptionsCreate,
		Stage:            "preview",
		PreviewToken:     token,
		PreviewExpiresAt: expiresAt,
		RiskLevel:        previewRiskLevel(validated.Count),
		Warnings:         warnings,
		Summary:          summary,
		Samples:          summary.Samples,
		CanExecute:       validated.Count > 0,
	}, nil
}

func ExecuteBatchRedemptions(ctx BatchOperationContext, req dto.BatchRedemptionsExecuteRequest) (*dto.BatchExecuteResponse, error) {
	validated, plan, previewSummary, _, err := buildBatchRedemptionsPreview(req.BatchRedemptionsPreviewRequest)
	if err != nil {
		return nil, err
	}
	if err := verifyPreviewToken(model.AdminBatchTypeRedemptionsCreate, ctx.OperatorID, req.PreviewToken, validated); err != nil {
		return nil, err
	}
	batchID := generateBatchID("RED")
	requestPayloadMasked, _ := marshalToJSONString(validated)
	scopeSummary, _ := marshalToJSONString(previewSummary)
	exportMeta, _ := marshalToJSONString(map[string]interface{}{
		"format":    validated.ExportFormat,
		"file_name": validated.FileName,
	})
	job := model.NewAdminBatchJob(batchID, model.AdminBatchTypeRedemptionsCreate, ctx.OperatorID, ctx.OperatorUsername, requestPayloadMasked, scopeSummary, exportMeta, ctx.SourceIP, ctx.UserAgent)
	if err := model.CreateAdminBatchJob(job); err != nil {
		return nil, err
	}

	items := make([]model.AdminBatchJobItem, 0, validated.Count)
	failedItems := make([]dto.BatchFailureItem, 0)
	lines := make([]string, 0, validated.Count)
	successCount := 0
	failedCount := 0
	for i := 0; i < validated.Count; i++ {
		key, err := generateUniqueRedemptionKey(validated.Prefix, validated.CodeLength)
		if err != nil {
			failedCount++
			failedItems = append(failedItems, dto.BatchFailureItem{TargetKey: fmt.Sprintf("item-%d", i+1), ReasonCode: "code_generation_failed", ReasonMessage: err.Error()})
			items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeRedemption, fmt.Sprintf("item-%d", i+1), 0, model.AdminBatchItemResultFailed, "code_generation_failed", err.Error(), 0, ""))
			continue
		}
		redemption := &model.Redemption{
			Key:                key,
			Status:             validated.Status,
			Name:               buildRedemptionName(validated, plan),
			RedeemType:         validated.RedeemType,
			Quota:              validated.Quota,
			SubscriptionPlanId: validated.PlanID,
			CreatedTime:        time.Now().Unix(),
			ExpiredTime:        validated.ExpiredTime,
		}
		if err := redemption.Insert(); err != nil {
			failedCount++
			failedItems = append(failedItems, dto.BatchFailureItem{TargetKey: key, ReasonCode: "create_failed", ReasonMessage: err.Error()})
			items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeRedemption, key, 0, model.AdminBatchItemResultFailed, "create_failed", err.Error(), 0, maskSensitiveCode(key)))
			continue
		}
		successCount++
		lines = append(lines, formatRedemptionExportLine(
			validated.ExportFormat,
			key,
			validated.RedeemType,
			validated.Quota,
			subscriptionPlanTitle(plan),
			validated.ExpiredTime,
			batchID,
		))
		items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeRedemption, key, redemption.Id, model.AdminBatchItemResultSuccess, "", "", 0, maskSensitiveCode(key)))
	}
	if err := model.CreateAdminBatchJobItems(items); err != nil {
		return nil, err
	}
	summary := batchRedemptionsExecuteSummary{
		RequestedCount: validated.Count,
		SuccessCount:   successCount,
		FailedCount:    failedCount,
		RedeemType:     validated.RedeemType,
		PlanID:         validated.PlanID,
		PlanTitle:      subscriptionPlanTitle(plan),
		Quota:          validated.Quota,
	}
	summaryJSON, _ := marshalToJSONString(summary)
	status := summarizeBatchStatus(successCount, failedCount)
	if err := model.UpdateAdminBatchJobResult(job, status, successCount, failedCount, summaryJSON); err != nil {
		return nil, err
	}
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	return &dto.BatchExecuteResponse{
		BatchID:     batchID,
		BatchType:   model.AdminBatchTypeRedemptionsCreate,
		Stage:       "execute",
		Status:      status,
		Summary:     summary,
		FailedItems: failedItems,
		Export: dto.BatchExportPayload{
			Available:   successCount > 0,
			Format:      validated.ExportFormat,
			Filename:    resolveFileName(validated.FileName, fmt.Sprintf("redeem-codes-%s", strings.ToLower(batchID)), validated.ExportFormat),
			Content:     content,
			Sensitive:   true,
			OneTimeOnly: true,
		},
	}, nil
}

func PreviewBatchQuota(operatorID int, req dto.BatchQuotaPreviewRequest) (*dto.BatchPreviewResponse, error) {
	validated, users, summary, warnings, err := buildBatchQuotaPreview(req)
	if err != nil {
		return nil, err
	}
	_ = users
	payloadBytes, err := common.Marshal(validated)
	if err != nil {
		return nil, err
	}
	token, expiresAt, err := buildPreviewToken(model.AdminBatchTypeQuotaGrant, operatorID, string(payloadBytes))
	if err != nil {
		return nil, err
	}
	return &dto.BatchPreviewResponse{
		BatchType:        model.AdminBatchTypeQuotaGrant,
		Stage:            "preview",
		PreviewToken:     token,
		PreviewExpiresAt: expiresAt,
		RiskLevel:        previewRiskLevel(summary.MatchedCount),
		Warnings:         warnings,
		Summary:          summary,
		Samples:          summary.SampleUsernames,
		CanExecute:       summary.MatchedCount > 0,
	}, nil
}

func ExecuteBatchQuota(ctx BatchOperationContext, req dto.BatchQuotaExecuteRequest) (*dto.BatchExecuteResponse, error) {
	validated, users, previewSummary, _, err := buildBatchQuotaPreview(req.BatchQuotaPreviewRequest)
	if err != nil {
		return nil, err
	}
	if err := verifyPreviewToken(model.AdminBatchTypeQuotaGrant, ctx.OperatorID, req.PreviewToken, validated); err != nil {
		return nil, err
	}
	batchID := generateBatchID("QTA")
	requestPayloadMasked, _ := marshalToJSONString(validated)
	scopeSummary, _ := marshalToJSONString(previewSummary)
	job := model.NewAdminBatchJob(batchID, model.AdminBatchTypeQuotaGrant, ctx.OperatorID, ctx.OperatorUsername, requestPayloadMasked, scopeSummary, "", ctx.SourceIP, ctx.UserAgent)
	if err := model.CreateAdminBatchJob(job); err != nil {
		return nil, err
	}
	items := make([]model.AdminBatchJobItem, 0, len(users))
	failedItems := make([]dto.BatchFailureItem, 0)
	successCount := 0
	failedCount := 0
	for _, user := range users {
		if user == nil {
			continue
		}
		err := model.DeltaUpdateUserQuota(user.Id, validated.QuotaDelta)
		if err != nil {
			failedCount++
			failedItems = append(failedItems, dto.BatchFailureItem{TargetKey: user.Username, TargetID: user.Id, ReasonCode: "quota_update_failed", ReasonMessage: err.Error(), DeltaQuota: validated.QuotaDelta})
			items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeQuota, user.Username, user.Id, model.AdminBatchItemResultFailed, "quota_update_failed", err.Error(), validated.QuotaDelta, ""))
			continue
		}
		successCount++
		items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeQuota, user.Username, user.Id, model.AdminBatchItemResultSuccess, "", "", validated.QuotaDelta, ""))
	}
	if err := model.CreateAdminBatchJobItems(items); err != nil {
		return nil, err
	}
	summary := batchQuotaExecuteSummary{
		MatchedCount: len(users),
		SuccessCount: successCount,
		FailedCount:  failedCount,
		TotalQuota:   successCount * validated.QuotaDelta,
		Reason:       validated.Reason,
	}
	summaryJSON, _ := marshalToJSONString(summary)
	status := summarizeBatchStatus(successCount, failedCount)
	if err := model.UpdateAdminBatchJobResult(job, status, successCount, failedCount, summaryJSON); err != nil {
		return nil, err
	}
	failedLines := make([]string, 0, len(failedItems))
	for _, item := range failedItems {
		failedLines = append(failedLines, fmt.Sprintf("%s,failed,%s", item.TargetKey, item.ReasonMessage))
	}
	content := strings.Join(failedLines, "\n")
	if content != "" {
		content += "\n"
	}
	return &dto.BatchExecuteResponse{
		BatchID:     batchID,
		BatchType:   model.AdminBatchTypeQuotaGrant,
		Stage:       "execute",
		Status:      status,
		Summary:     summary,
		FailedItems: failedItems,
		Export: dto.BatchExportPayload{
			Available:   len(failedLines) > 0,
			Format:      batchExportFormatCSV,
			Filename:    fmt.Sprintf("quota-failures-%s.csv", strings.ToLower(batchID)),
			Content:     content,
			Sensitive:   false,
			OneTimeOnly: false,
		},
	}, nil
}

func PreviewBatchSubscriptionsExtend(operatorID int, req dto.BatchSubscriptionsExtendPreviewRequest) (*dto.BatchPreviewResponse, error) {
	validated, targets, summary, warnings, err := buildBatchSubscriptionsExtendPreview(req)
	if err != nil {
		return nil, err
	}
	_ = targets
	payloadBytes, err := common.Marshal(validated)
	if err != nil {
		return nil, err
	}
	token, expiresAt, err := buildPreviewToken(model.AdminBatchTypeSubscriptionsExtend, operatorID, string(payloadBytes))
	if err != nil {
		return nil, err
	}
	return &dto.BatchPreviewResponse{
		BatchType:        model.AdminBatchTypeSubscriptionsExtend,
		Stage:            "preview",
		PreviewToken:     token,
		PreviewExpiresAt: expiresAt,
		RiskLevel:        previewRiskLevel(summary.MatchedUserCount),
		Warnings:         warnings,
		Summary:          summary,
		Samples:          summary.SampleUsernames,
		CanExecute:       summary.MatchedSubscriptionCount > 0,
	}, nil
}

func ExecuteBatchSubscriptionsExtend(ctx BatchOperationContext, req dto.BatchSubscriptionsExtendExecuteRequest) (*dto.BatchExecuteResponse, error) {
	validated, targets, previewSummary, _, err := buildBatchSubscriptionsExtendPreview(req.BatchSubscriptionsExtendPreviewRequest)
	if err != nil {
		return nil, err
	}
	if err := verifyPreviewToken(model.AdminBatchTypeSubscriptionsExtend, ctx.OperatorID, req.PreviewToken, validated); err != nil {
		return nil, err
	}

	batchID := generateBatchID("SUB")
	requestPayloadMasked, _ := marshalToJSONString(validated)
	scopeSummary, _ := marshalToJSONString(previewSummary)
	job := model.NewAdminBatchJob(batchID, model.AdminBatchTypeSubscriptionsExtend, ctx.OperatorID, ctx.OperatorUsername, requestPayloadMasked, scopeSummary, "", ctx.SourceIP, ctx.UserAgent)
	if err := model.CreateAdminBatchJob(job); err != nil {
		return nil, err
	}

	items := make([]model.AdminBatchJobItem, 0, len(targets))
	failedItems := make([]dto.BatchFailureItem, 0)
	failedLines := make([]string, 0)
	successUserCount := 0
	failedUserCount := 0
	updatedSubscriptionCount := 0
	extensionSeconds := int64(validated.Days) * 24 * 3600

	for _, target := range targets {
		updatedCount, err := model.ExtendAllActiveSubscriptionsForUser(target.UserID, extensionSeconds)
		if err != nil {
			failedUserCount++
			failedItems = append(failedItems, dto.BatchFailureItem{
				TargetKey:     target.Username,
				TargetID:      target.UserID,
				ReasonCode:    "subscription_extend_failed",
				ReasonMessage: err.Error(),
			})
			failedLines = append(failedLines, fmt.Sprintf("%s,failed,%s", target.Username, err.Error()))
			items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeSubscription, target.Username, target.UserID, model.AdminBatchItemResultFailed, "subscription_extend_failed", err.Error(), 0, ""))
			continue
		}
		if updatedCount <= 0 {
			items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeSubscription, target.Username, target.UserID, model.AdminBatchItemResultSkipped, "no_active_subscription", "no active subscription at execution time", 0, ""))
			continue
		}

		successUserCount++
		updatedSubscriptionCount += updatedCount
		items = append(items, newBatchItem(job.Id, model.AdminBatchItemTypeSubscription, target.Username, target.UserID, model.AdminBatchItemResultSuccess, "", "", 0, ""))
	}

	if err := model.CreateAdminBatchJobItems(items); err != nil {
		return nil, err
	}

	skippedUserCount := previewSummary.TotalUsers - successUserCount - failedUserCount
	if skippedUserCount < 0 {
		skippedUserCount = 0
	}
	summary := batchSubscriptionsExtendExecuteSummary{
		TotalUsers:               previewSummary.TotalUsers,
		MatchedUserCount:         previewSummary.MatchedUserCount,
		SuccessUserCount:         successUserCount,
		SkippedUserCount:         skippedUserCount,
		FailedUserCount:          failedUserCount,
		UpdatedSubscriptionCount: updatedSubscriptionCount,
		ExtensionDays:            validated.Days,
		ExtensionSeconds:         extensionSeconds,
	}
	summaryJSON, _ := marshalToJSONString(summary)
	status := summarizeBatchStatus(successUserCount, failedUserCount)
	if successUserCount == 0 && failedUserCount == 0 {
		status = model.AdminBatchStatusCompleted
	}
	if err := model.UpdateAdminBatchJobResult(job, status, successUserCount, failedUserCount, summaryJSON); err != nil {
		return nil, err
	}

	content := strings.Join(failedLines, "\n")
	if content != "" {
		content += "\n"
	}
	return &dto.BatchExecuteResponse{
		BatchID:     batchID,
		BatchType:   model.AdminBatchTypeSubscriptionsExtend,
		Stage:       "execute",
		Status:      status,
		Summary:     summary,
		FailedItems: failedItems,
		Export: dto.BatchExportPayload{
			Available:   len(failedLines) > 0,
			Format:      batchExportFormatCSV,
			Filename:    fmt.Sprintf("subscription-extend-failures-%s.csv", strings.ToLower(batchID)),
			Content:     content,
			Sensitive:   false,
			OneTimeOnly: false,
		},
	}, nil
}

func ListAdminBatchJobs() (*dto.AdminBatchJobListResponse, error) {
	jobs, total, err := model.ListAdminBatchJobs(10)
	if err != nil {
		return nil, err
	}
	items := make([]dto.AdminBatchJobListItem, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, dto.AdminBatchJobListItem{
			BatchID:          job.BatchID,
			BatchType:        job.BatchType,
			Status:           job.Status,
			OperatorUsername: job.OperatorUsername,
			SuccessCount:     job.SuccessCount,
			FailedCount:      job.FailedCount,
			CreatedAt:        job.CreatedAt,
			FinishedAt:       job.FinishedAt,
		})
	}
	return &dto.AdminBatchJobListResponse{Items: items, Total: total}, nil
}

func GetAdminBatchJob(batchID string) (*dto.AdminBatchJobDetailResponse, error) {
	job, err := model.GetAdminBatchJobByBatchID(strings.TrimSpace(batchID))
	if err != nil {
		return nil, err
	}
	items, err := model.ListAdminBatchJobItems(job.Id)
	if err != nil {
		return nil, err
	}
	requestPayloadMasked := decodeJSONAny(job.RequestPayloadMasked)
	scopeSummary := decodeJSONAny(job.ScopeSummaryJSON)
	summary := decodeJSONAny(job.SummaryJSON)
	failureSummary := make(map[string]int)
	failedItems := make([]dto.BatchFailureItem, 0)
	for _, item := range items {
		if item.Result == model.AdminBatchItemResultSuccess {
			continue
		}
		failureSummary[item.ReasonCode]++
		failedItems = append(failedItems, dto.BatchFailureItem{
			TargetKey:     item.TargetKey,
			TargetID:      item.TargetID,
			ReasonCode:    item.ReasonCode,
			ReasonMessage: item.ReasonMessage,
			DeltaQuota:    item.DeltaQuota,
			SensitiveMask: item.SensitivePayloadMasked,
		})
	}
	return &dto.AdminBatchJobDetailResponse{
		BatchID:              job.BatchID,
		BatchType:            job.BatchType,
		Status:               job.Status,
		OperatorUsername:     job.OperatorUsername,
		CreatedAt:            job.CreatedAt,
		StartedAt:            job.StartedAt,
		FinishedAt:           job.FinishedAt,
		RequestPayloadMasked: requestPayloadMasked,
		ScopeSummary:         scopeSummary,
		Summary:              summary,
		FailureSummary:       failureSummary,
		FailedItems:          failedItems,
	}, nil
}

func buildBatchUsersPreview(req dto.BatchUsersPreviewRequest) (dto.BatchUsersPreviewRequest, batchUsersPreviewSummary, []string, error) {
	validated := req
	validated.Count = clampInt(req.Count, 1, batchUsersMaxCount)
	validated.StartNumber = maxInt(req.StartNumber, 1)
	validated.NumberWidth = clampInt(req.NumberWidth, 1, 8)
	validated.InitialQuota = maxInt(req.InitialQuota, 0)
	validated.UsernamePrefix = strings.TrimSpace(req.UsernamePrefix)
	validated.Group = strings.TrimSpace(req.Group)
	validated.PasswordMode = normalizePasswordMode(req.PasswordMode)
	validated.ExportFormat = normalizeExportFormat(req.ExportFormat)
	validated.ExportDelimiter = normalizeExportDelimiter(req.ExportDelimiter)
	validated.FileName = strings.TrimSpace(req.FileName)
	if validated.UsernamePrefix == "" {
		return validated, batchUsersPreviewSummary{}, nil, errors.New("username prefix is required")
	}
	if len(validated.UsernamePrefix)+validated.NumberWidth > model.UserNameMaxLength {
		return validated, batchUsersPreviewSummary{}, nil, fmt.Errorf("username exceeds max length %d", model.UserNameMaxLength)
	}
	if err := validateGroup(validated.Group); err != nil {
		return validated, batchUsersPreviewSummary{}, nil, err
	}
	if validated.Status != common.UserStatusEnabled && validated.Status != common.UserStatusDisabled {
		validated.Status = common.UserStatusEnabled
	}
	if validated.PasswordMode == batchPasswordModeFixed {
		if len(validated.FixedPassword) < 8 || len(validated.FixedPassword) > 20 {
			return validated, batchUsersPreviewSummary{}, nil, errors.New("fixed password must be 8-20 characters")
		}
	} else {
		validated.RandomPasswordLength = clampInt(req.RandomPasswordLength, 8, 20)
	}
	usernames := buildPreviewUsernames(validated)
	conflictCount := 0
	effectiveCount := 0
	for _, username := range usernames {
		exists, err := model.CheckUserExistOrDeleted(username, "")
		if err != nil {
			return validated, batchUsersPreviewSummary{}, nil, err
		}
		if exists {
			conflictCount++
			continue
		}
		effectiveCount++
	}
	samples := usernames
	if len(samples) > 10 {
		samples = samples[:10]
	}
	warnings := []string{"敏感内容仅会在执行成功后的当前结果中返回一次"}
	if conflictCount > 0 {
		warnings = append(warnings, fmt.Sprintf("检测到 %d 个用户名冲突，执行时会跳过这些账号", conflictCount))
	}
	return validated, batchUsersPreviewSummary{
		RequestedCount: validated.Count,
		EffectiveCount: effectiveCount,
		ConflictCount:  conflictCount,
		TotalQuota:     effectiveCount * validated.InitialQuota,
		Group:          validated.Group,
		Samples:        samples,
	}, warnings, nil
}

func buildBatchRedemptionsPreview(req dto.BatchRedemptionsPreviewRequest) (dto.BatchRedemptionsPreviewRequest, *model.SubscriptionPlan, batchRedemptionsPreviewSummary, []string, error) {
	validated := req
	validated.Count = clampInt(req.Count, 1, batchRedemptionsMaxCount)
	validated.RedeemType = normalizeBatchRedemptionType(req.RedeemType)
	validated.Quota = maxInt(req.Quota, 0)
	validated.CodeLength = clampInt(req.CodeLength, 6, 24)
	validated.Prefix = strings.ToUpper(strings.TrimSpace(req.Prefix))
	validated.Status = normalizeRedemptionStatus(req.Status)
	validated.ExportFormat = normalizeExportFormat(req.ExportFormat)
	validated.FileName = strings.TrimSpace(req.FileName)
	if len(validated.Prefix)+validated.CodeLength > 32 {
		return validated, nil, batchRedemptionsPreviewSummary{}, nil, errors.New("code length with prefix must be <= 32")
	}
	var (
		plan *model.SubscriptionPlan
		err  error
	)
	if validated.RedeemType == model.RedemptionTypeSubscription {
		if validated.PlanID <= 0 {
			return validated, nil, batchRedemptionsPreviewSummary{}, nil, errors.New("plan_id is required")
		}
		plan, err = model.GetSubscriptionPlanById(validated.PlanID)
		if err != nil {
			return validated, nil, batchRedemptionsPreviewSummary{}, nil, err
		}
		validated.Quota = 0
	} else {
		validated.PlanID = 0
		if validated.Quota <= 0 {
			return validated, nil, batchRedemptionsPreviewSummary{}, nil, errors.New("quota must be positive")
		}
	}
	samples := make([]string, 0, minInt(validated.Count, 10))
	for i := 0; i < minInt(validated.Count, 10); i++ {
		samples = append(samples, buildRandomCodePreview(validated.Prefix, validated.CodeLength))
	}
	warnings := []string{"完整兑换码仅在执行成功后的当前结果页可下载"}
	if validated.ExpiredTime > 0 {
		warnings = append(warnings, "请确认过期时间后再执行，生成后不会自动延长")
	}
	return validated, plan, batchRedemptionsPreviewSummary{
		RequestedCount: validated.Count,
		RedeemType:     validated.RedeemType,
		PlanID:         validated.PlanID,
		PlanTitle:      subscriptionPlanTitle(plan),
		Quota:          validated.Quota,
		CodeLength:     validated.CodeLength,
		Prefix:         validated.Prefix,
		ExpiredTime:    validated.ExpiredTime,
		Samples:        samples,
	}, warnings, nil
}

func buildBatchQuotaPreview(req dto.BatchQuotaPreviewRequest) (dto.BatchQuotaPreviewRequest, []*model.User, batchQuotaPreviewSummary, []string, error) {
	validated := req
	validated.ScopeType = normalizeQuotaScope(req.ScopeType)
	validated.Group = strings.TrimSpace(req.Group)
	validated.QuotaDelta = maxInt(req.QuotaDelta, 0)
	validated.Reason = strings.TrimSpace(req.Reason)
	if validated.QuotaDelta <= 0 {
		return validated, nil, batchQuotaPreviewSummary{}, nil, errors.New("quota_delta must be positive")
	}
	if validated.Reason == "" {
		return validated, nil, batchQuotaPreviewSummary{}, nil, errors.New("reason is required")
	}
	if validated.ScopeType == batchScopeGroup {
		if err := validateGroup(validated.Group); err != nil {
			return validated, nil, batchQuotaPreviewSummary{}, nil, err
		}
	}
	users, err := loadBatchQuotaUsers(validated)
	if err != nil {
		return validated, nil, batchQuotaPreviewSummary{}, nil, err
	}
	if len(users) > batchQuotaMaxMatchedUsers {
		return validated, nil, batchQuotaPreviewSummary{}, nil, fmt.Errorf("matched users exceed max limit %d", batchQuotaMaxMatchedUsers)
	}
	samples := make([]string, 0, minInt(len(users), 10))
	for _, user := range users {
		if len(samples) >= 10 {
			break
		}
		samples = append(samples, user.Username)
	}
	warnings := []string{"该操作会立即生效，不支持一键回滚"}
	if !validated.IncludeAdmins {
		warnings = append(warnings, "默认已排除管理员账号")
	}
	return validated, users, batchQuotaPreviewSummary{
		MatchedCount:    len(users),
		TotalQuota:      len(users) * validated.QuotaDelta,
		ScopeType:       validated.ScopeType,
		Group:           validated.Group,
		IncludeAdmins:   validated.IncludeAdmins,
		SampleUsernames: samples,
	}, warnings, nil
}

func buildBatchSubscriptionsExtendPreview(req dto.BatchSubscriptionsExtendPreviewRequest) (dto.BatchSubscriptionsExtendPreviewRequest, []batchSubscriptionTarget, batchSubscriptionsExtendPreviewSummary, []string, error) {
	validated := req
	validated.Days = req.Days
	if validated.Days <= 0 || validated.Days > batchSubscriptionMaxDays {
		return validated, nil, batchSubscriptionsExtendPreviewSummary{}, nil, fmt.Errorf("days must be between 1 and %d", batchSubscriptionMaxDays)
	}

	totalUsers, targets, matchedSubscriptionCount, err := loadBatchSubscriptionTargets()
	if err != nil {
		return validated, nil, batchSubscriptionsExtendPreviewSummary{}, nil, err
	}

	samples := make([]string, 0, minInt(len(targets), 10))
	for _, target := range targets {
		if len(samples) >= 10 {
			break
		}
		samples = append(samples, target.Username)
	}

	warnings := []string{
		"该操作会立即生效，不支持一键回滚。",
		"只会延期当前仍生效中的订阅；没有生效订阅的用户会被跳过。",
		"命中的用户如果有多条生效订阅，会全部一起延期。",
	}

	warnings = []string{
		"This operation takes effect immediately and cannot be rolled back in one click.",
		"Only currently active subscriptions will be extended; users without active subscriptions will be skipped.",
		"If a matched user has multiple active subscriptions, all of them will be extended together.",
	}

	return validated, targets, batchSubscriptionsExtendPreviewSummary{
		TotalUsers:               int(totalUsers),
		MatchedUserCount:         len(targets),
		MatchedSubscriptionCount: matchedSubscriptionCount,
		SkippedUserCount:         int(totalUsers) - len(targets),
		ExtensionDays:            validated.Days,
		ExtensionSeconds:         int64(validated.Days) * 24 * 3600,
		SampleUsernames:          samples,
	}, warnings, nil
}

func loadBatchQuotaUsers(req dto.BatchQuotaPreviewRequest) ([]*model.User, error) {
	query := model.DB.Model(&model.User{})
	if req.ScopeType == batchScopeGroup {
		query = query.Where(&model.User{Group: req.Group})
	}
	if !req.IncludeAdmins {
		query = query.Where("role < ?", common.RoleAdminUser)
	}
	var users []*model.User
	if err := query.Order("id asc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func loadBatchSubscriptionTargets() (int64, []batchSubscriptionTarget, int, error) {
	var totalUsers int64
	if err := model.DB.Model(&model.User{}).Count(&totalUsers).Error; err != nil {
		return 0, nil, 0, err
	}

	now := model.GetDBTimestamp()
	rows := make([]batchSubscriptionTarget, 0)
	err := model.DB.Table("user_subscriptions AS us").
		Select("users.id AS user_id, users.username AS username, COUNT(us.id) AS active_subscription_count").
		Joins("JOIN users ON users.id = us.user_id").
		Where("us.status = ? AND us.end_time > ?", "active", now).
		Group("users.id, users.username").
		Order("users.id ASC").
		Scan(&rows).Error
	if err != nil {
		return 0, nil, 0, err
	}

	totalSubscriptions := 0
	for _, row := range rows {
		totalSubscriptions += row.ActiveSubscriptionCount
	}
	return totalUsers, rows, totalSubscriptions, nil
}

func validateGroup(group string) error {
	if group == "" {
		return errors.New("group is required")
	}
	groups := ratio_setting.GetGroupRatioCopy()
	if _, ok := groups[group]; !ok {
		return fmt.Errorf("group %s does not exist", group)
	}
	return nil
}

func createBatchUser(username string, password string, req dto.BatchUsersPreviewRequest) (*model.User, *model.Token, error) {
	hashedPassword, err := common.Password2Hash(password)
	if err != nil {
		return nil, nil, err
	}
	setting := dto.UserSetting{SidebarModules: model.GenerateDefaultSidebarConfigForRole(common.RoleCommonUser)}
	user := &model.User{
		Username:    username,
		Password:    hashedPassword,
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      req.Status,
		Quota:       req.InitialQuota,
		Group:       req.Group,
		AffCode:     common.GetUUID(),
		Remark:      "",
	}
	user.SetSetting(setting)
	var userToken *model.Token
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		token, err := model.CreateDefaultTokenForUserTx(tx, user.Id)
		if err != nil {
			return err
		}
		userToken = token
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return user, userToken, nil
}

func buildBatchUserPassword(req dto.BatchUsersPreviewRequest) (string, error) {
	if req.PasswordMode == batchPasswordModeFixed {
		return req.FixedPassword, nil
	}
	return generateRandomPassword(req.RandomPasswordLength)
}

func generateRandomPassword(length int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%"
	if length <= 0 {
		length = 12
	}
	bytes := make([]byte, length)
	for i := range bytes {
		n, err := rand.Int(rand.Reader, bigInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		bytes[i] = alphabet[n.Int64()]
	}
	return string(bytes), nil
}

func buildPreviewUsernames(req dto.BatchUsersPreviewRequest) []string {
	usernames := make([]string, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		current := req.StartNumber + i
		usernames = append(usernames, fmt.Sprintf("%s%0*d", req.UsernamePrefix, req.NumberWidth, current))
	}
	return usernames
}

func generateUniqueRedemptionKey(prefix string, length int) (string, error) {
	for i := 0; i < 8; i++ {
		candidate := buildRandomCodePreview(prefix, length)
		var count int64
		if err := model.DB.Model(&model.Redemption{}).Where(commonKeyColOrDefault()+" = ?", candidate).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", errors.New("failed to generate unique redemption key")
}

func buildRandomCodePreview(prefix string, length int) string {
	randomPart := strings.ToUpper(common.GetRandomString(length))
	return prefix + randomPart
}

func buildRedemptionName(req dto.BatchRedemptionsPreviewRequest, plan *model.SubscriptionPlan) string {
	if strings.TrimSpace(req.Name) != "" {
		return strings.TrimSpace(req.Name)
	}
	if req.RedeemType == model.RedemptionTypeSubscription {
		if plan != nil && strings.TrimSpace(plan.Title) != "" {
			return strings.TrimSpace(plan.Title)
		}
		return "订阅套餐兑换码"
	}
	if req.Quota > 0 {
		return fmt.Sprintf("%s兑换码", logger.FormatQuota(req.Quota))
	}
	return "额度兑换码"
}

func maskBatchUsersRequest(req dto.BatchUsersPreviewRequest) map[string]interface{} {
	return map[string]interface{}{
		"count":                  req.Count,
		"username_prefix":        req.UsernamePrefix,
		"start_number":           req.StartNumber,
		"number_width":           req.NumberWidth,
		"initial_quota":          req.InitialQuota,
		"group":                  req.Group,
		"status":                 req.Status,
		"password_mode":          req.PasswordMode,
		"random_password_length": req.RandomPasswordLength,
		"fixed_password":         maskPasswordField(req.FixedPassword),
		"export_format":          req.ExportFormat,
		"export_delimiter":       req.ExportDelimiter,
		"file_name":              req.FileName,
	}
}

func formatUserExportLine(format string, delimiter string, username string, password string, token string, group string, quota int, status int) string {
	if format == batchExportFormatCSV {
		return fmt.Sprintf("%s,%s,%s,%s,%d,%d", username, password, token, group, quota, status)
	}
	_ = delimiter
	return fmt.Sprintf("%s,%s", username, password)
}

func formatRedemptionExportLine(format string, code string, redeemType string, quota int, planTitle string, expiredTime int64, batchID string) string {
	if format == batchExportFormatCSV {
		target := planTitle
		if redeemType == model.RedemptionTypeQuota {
			target = fmt.Sprintf("%d", quota)
		}
		return fmt.Sprintf("%s,%s,%s,%d,%s", code, redeemType, target, expiredTime, batchID)
	}
	return code
}

func maskSensitiveCode(code string) string {
	trimmed := strings.TrimSpace(code)
	if len(trimmed) <= 8 {
		return trimmed
	}
	return trimmed[:4] + "****" + trimmed[len(trimmed)-4:]
}

func buildPreviewToken(batchType string, operatorID int, payload string) (string, int64, error) {
	payloadStruct := batchPreviewTokenPayload{
		BatchType:  batchType,
		OperatorID: operatorID,
		ExpiresAt:  time.Now().Unix() + batchPreviewTTLSeconds,
		Payload:    payload,
	}
	raw, err := common.Marshal(payloadStruct)
	if err != nil {
		return "", 0, err
	}
	mac := hmac.New(sha256.New, []byte(common.SessionSecret))
	_, _ = mac.Write(raw)
	signature := hex.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString(raw) + "." + signature, payloadStruct.ExpiresAt, nil
}

func verifyPreviewToken(batchType string, operatorID int, token string, payload interface{}) error {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return errors.New("invalid preview token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(common.SessionSecret))
	_, _ = mac.Write(raw)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return errors.New("preview token signature mismatch")
	}
	var payloadStruct batchPreviewTokenPayload
	if err := common.Unmarshal(raw, &payloadStruct); err != nil {
		return err
	}
	if payloadStruct.BatchType != batchType || payloadStruct.OperatorID != operatorID {
		return errors.New("preview token is not valid for current operator")
	}
	if payloadStruct.ExpiresAt < time.Now().Unix() {
		return errors.New("preview token expired")
	}
	payloadBytes, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	if payloadStruct.Payload != string(payloadBytes) {
		return errors.New("preview token payload mismatch")
	}
	return nil
}

func summarizeBatchStatus(successCount int, failedCount int) string {
	if successCount > 0 && failedCount > 0 {
		return model.AdminBatchStatusPartialFailed
	}
	if successCount > 0 {
		return model.AdminBatchStatusCompleted
	}
	return model.AdminBatchStatusFailed
}

func previewRiskLevel(count int) string {
	if count >= 100 {
		return "high"
	}
	if count >= 20 {
		return "medium"
	}
	return "low"
}

func generateBatchID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ToUpper(common.GetRandomString(12)))
}

func newBatchItem(batchJobID int, itemType string, targetKey string, targetID int, result string, reasonCode string, reasonMessage string, deltaQuota int, sensitiveMask string) model.AdminBatchJobItem {
	return model.AdminBatchJobItem{
		BatchJobID:             batchJobID,
		ItemType:               itemType,
		TargetKey:              targetKey,
		TargetID:               targetID,
		Result:                 result,
		ReasonCode:             reasonCode,
		ReasonMessage:          reasonMessage,
		DeltaQuota:             deltaQuota,
		SensitivePayloadMasked: sensitiveMask,
		CreatedAt:              time.Now().Unix(),
	}
}

func decodeJSONAny(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]interface{}{}
	}
	var value interface{}
	if err := common.UnmarshalJsonStr(trimmed, &value); err != nil {
		return map[string]interface{}{"raw": trimmed}
	}
	return value
}

func marshalToJSONString(v interface{}) (string, error) {
	bytes, err := common.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func normalizePasswordMode(mode string) string {
	if strings.TrimSpace(mode) == batchPasswordModeFixed {
		return batchPasswordModeFixed
	}
	return batchPasswordModeRandom
}

func normalizeExportFormat(format string) string {
	if strings.TrimSpace(strings.ToLower(format)) == batchExportFormatCSV {
		return batchExportFormatCSV
	}
	return batchExportFormatTXT
}

func normalizeExportDelimiter(delimiter string) string {
	if strings.TrimSpace(strings.ToLower(delimiter)) == batchDelimiterDash {
		return batchDelimiterDash
	}
	return batchDelimiterComma
}

func normalizeQuotaScope(scope string) string {
	if strings.TrimSpace(scope) == batchScopeGroup {
		return batchScopeGroup
	}
	return batchScopeAll
}

func normalizeRedemptionStatus(status int) int {
	if status == common.RedemptionCodeStatusDisabled {
		return common.RedemptionCodeStatusDisabled
	}
	return common.RedemptionCodeStatusEnabled
}

func normalizeBatchRedemptionType(redeemType string) string {
	switch strings.TrimSpace(redeemType) {
	case model.RedemptionTypeQuota:
		return model.RedemptionTypeQuota
	default:
		return model.RedemptionTypeSubscription
	}
}

func subscriptionPlanTitle(plan *model.SubscriptionPlan) string {
	if plan == nil {
		return ""
	}
	return strings.TrimSpace(plan.Title)
}

func maskPasswordField(password string) string {
	if password == "" {
		return ""
	}
	return fmt.Sprintf("len:%d", len(password))
}

func resolveFileName(value string, fallback string, format string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = fallback
	}
	trimmed = strings.ReplaceAll(trimmed, " ", "-")
	if !strings.HasSuffix(strings.ToLower(trimmed), "."+format) {
		trimmed += "." + format
	}
	return trimmed
}

func clampInt(v int, min int, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func bigInt(v int64) *big.Int {
	return big.NewInt(v)
}

func commonKeyColOrDefault() string {
	if common.UsingPostgreSQL {
		return `"key"`
	}
	return "`key`"
}
