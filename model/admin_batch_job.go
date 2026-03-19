package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	AdminBatchTypeUsersCreate       = "users_create"
	AdminBatchTypeRedemptionsCreate = "redemptions_create"
	AdminBatchTypeQuotaGrant        = "quota_grant"

	AdminBatchStatusPreviewed     = "previewed"
	AdminBatchStatusRunning       = "running"
	AdminBatchStatusCompleted     = "completed"
	AdminBatchStatusPartialFailed = "partial_failed"
	AdminBatchStatusFailed        = "failed"

	AdminBatchItemTypeUser       = "user"
	AdminBatchItemTypeRedemption = "redemption"
	AdminBatchItemTypeQuota      = "quota_target"

	AdminBatchItemResultSuccess = "success"
	AdminBatchItemResultFailed  = "failed"
	AdminBatchItemResultSkipped = "skipped"
)

type AdminBatchJob struct {
	Id                   int            `json:"id"`
	BatchID              string         `json:"batch_id" gorm:"type:varchar(64);uniqueIndex"`
	BatchType            string         `json:"batch_type" gorm:"type:varchar(32);index"`
	Status               string         `json:"status" gorm:"type:varchar(32);index"`
	OperatorUserID       int            `json:"operator_user_id" gorm:"index"`
	OperatorUsername     string         `json:"operator_username" gorm:"type:varchar(64)"`
	RequestPayloadMasked string         `json:"request_payload_masked" gorm:"type:text"`
	ScopeSummaryJSON     string         `json:"scope_summary_json" gorm:"type:text"`
	SummaryJSON          string         `json:"summary_json" gorm:"type:text"`
	ExportMetaJSON       string         `json:"export_meta_json" gorm:"type:text"`
	SuccessCount         int            `json:"success_count" gorm:"default:0"`
	FailedCount          int            `json:"failed_count" gorm:"default:0"`
	SourceIP             string         `json:"source_ip" gorm:"type:varchar(64)"`
	UserAgent            string         `json:"user_agent" gorm:"type:varchar(255)"`
	Reversible           bool           `json:"reversible" gorm:"default:false"`
	RevertedFromBatchID  string         `json:"reverted_from_batch_id" gorm:"type:varchar(64);index"`
	RevertedByUserID     int            `json:"reverted_by_user_id"`
	RevertedAt           int64          `json:"reverted_at" gorm:"bigint"`
	CreatedAt            int64          `json:"created_at" gorm:"bigint;index"`
	StartedAt            int64          `json:"started_at" gorm:"bigint"`
	FinishedAt           int64          `json:"finished_at" gorm:"bigint"`
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

type AdminBatchJobItem struct {
	Id                     int            `json:"id"`
	BatchJobID             int            `json:"batch_job_id" gorm:"index"`
	ItemType               string         `json:"item_type" gorm:"type:varchar(32);index"`
	TargetKey              string         `json:"target_key" gorm:"type:varchar(128);index"`
	TargetID               int            `json:"target_id" gorm:"index"`
	Result                 string         `json:"result" gorm:"type:varchar(16);index"`
	ReasonCode             string         `json:"reason_code" gorm:"type:varchar(64)"`
	ReasonMessage          string         `json:"reason_message" gorm:"type:varchar(255)"`
	DeltaQuota             int            `json:"delta_quota"`
	SensitivePayloadMasked string         `json:"sensitive_payload_masked" gorm:"type:text"`
	CreatedAt              int64          `json:"created_at" gorm:"bigint;index"`
	DeletedAt              gorm.DeletedAt `gorm:"index"`
}

func NewAdminBatchJob(batchID string, batchType string, operatorUserID int, operatorUsername string, requestPayloadMasked string, scopeSummaryJSON string, exportMetaJSON string, sourceIP string, userAgent string) *AdminBatchJob {
	now := time.Now().Unix()
	return &AdminBatchJob{
		BatchID:              batchID,
		BatchType:            batchType,
		Status:               AdminBatchStatusRunning,
		OperatorUserID:       operatorUserID,
		OperatorUsername:     operatorUsername,
		RequestPayloadMasked: requestPayloadMasked,
		ScopeSummaryJSON:     scopeSummaryJSON,
		ExportMetaJSON:       exportMetaJSON,
		SourceIP:             sourceIP,
		UserAgent:            userAgent,
		CreatedAt:            now,
		StartedAt:            now,
	}
}

func CreateAdminBatchJob(job *AdminBatchJob) error {
	if job == nil {
		return nil
	}
	return DB.Create(job).Error
}

func UpdateAdminBatchJobResult(job *AdminBatchJob, status string, successCount int, failedCount int, summaryJSON string) error {
	if job == nil {
		return nil
	}
	job.Status = status
	job.SuccessCount = successCount
	job.FailedCount = failedCount
	job.SummaryJSON = summaryJSON
	job.FinishedAt = time.Now().Unix()
	return DB.Model(job).Updates(map[string]interface{}{
		"status":        job.Status,
		"success_count": job.SuccessCount,
		"failed_count":  job.FailedCount,
		"summary_json":  job.SummaryJSON,
		"finished_at":   job.FinishedAt,
	}).Error
}

func CreateAdminBatchJobItems(items []AdminBatchJobItem) error {
	if len(items) == 0 {
		return nil
	}
	return DB.Create(&items).Error
}

func ListAdminBatchJobs(limit int) ([]AdminBatchJob, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	var jobs []AdminBatchJob
	var total int64
	if err := DB.Model(&AdminBatchJob{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := DB.Order("created_at desc").Limit(limit).Find(&jobs).Error; err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}

func GetAdminBatchJobByBatchID(batchID string) (*AdminBatchJob, error) {
	var job AdminBatchJob
	if err := DB.Where("batch_id = ?", batchID).First(&job).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func ListAdminBatchJobItems(batchJobID int) ([]AdminBatchJobItem, error) {
	var items []AdminBatchJobItem
	if err := DB.Where("batch_job_id = ?", batchJobID).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
