package dto

type BatchUsersPreviewRequest struct {
	Count                int    `json:"count"`
	UsernamePrefix       string `json:"username_prefix"`
	StartNumber          int    `json:"start_number"`
	NumberWidth          int    `json:"number_width"`
	InitialQuota         int    `json:"initial_quota"`
	Group                string `json:"group"`
	Status               int    `json:"status"`
	PasswordMode         string `json:"password_mode"`
	FixedPassword        string `json:"fixed_password,omitempty"`
	RandomPasswordLength int    `json:"random_password_length"`
	ExportFormat         string `json:"export_format"`
	ExportDelimiter      string `json:"export_delimiter"`
	FileName             string `json:"file_name"`
}

type BatchUsersExecuteRequest struct {
	BatchUsersPreviewRequest
	PreviewToken string `json:"preview_token"`
}

type BatchRedemptionsPreviewRequest struct {
	PlanID       int    `json:"plan_id"`
	Count        int    `json:"count"`
	CodeLength   int    `json:"code_length"`
	Prefix       string `json:"prefix"`
	ExpiredTime  int64  `json:"expired_time"`
	Status       int    `json:"status"`
	Name         string `json:"name"`
	ExportFormat string `json:"export_format"`
	FileName     string `json:"file_name"`
	Description  string `json:"description"`
}

type BatchRedemptionsExecuteRequest struct {
	BatchRedemptionsPreviewRequest
	PreviewToken string `json:"preview_token"`
}

type BatchQuotaPreviewRequest struct {
	ScopeType     string `json:"scope_type"`
	Group         string `json:"group"`
	IncludeAdmins bool   `json:"include_admins"`
	QuotaDelta    int    `json:"quota_delta"`
	Reason        string `json:"reason"`
}

type BatchQuotaExecuteRequest struct {
	BatchQuotaPreviewRequest
	PreviewToken string `json:"preview_token"`
}

type BatchPreviewResponse struct {
	BatchType        string      `json:"batch_type"`
	Stage            string      `json:"stage"`
	PreviewToken     string      `json:"preview_token"`
	PreviewExpiresAt int64       `json:"preview_expires_at"`
	RiskLevel        string      `json:"risk_level"`
	Warnings         []string    `json:"warnings"`
	Summary          interface{} `json:"summary"`
	Samples          interface{} `json:"samples"`
	CanExecute       bool        `json:"can_execute"`
	ConfirmationText string      `json:"confirmation_text,omitempty"`
}

type BatchFailureItem struct {
	TargetKey     string `json:"target_key"`
	TargetID      int    `json:"target_id,omitempty"`
	ReasonCode    string `json:"reason_code"`
	ReasonMessage string `json:"reason_message"`
	DeltaQuota    int    `json:"delta_quota,omitempty"`
	SensitiveMask string `json:"sensitive_mask,omitempty"`
}

type BatchExportPayload struct {
	Available   bool   `json:"available"`
	Format      string `json:"format"`
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	Sensitive   bool   `json:"sensitive"`
	OneTimeOnly bool   `json:"one_time_only"`
}

type BatchExecuteResponse struct {
	BatchID     string             `json:"batch_id"`
	BatchType   string             `json:"batch_type"`
	Stage       string             `json:"stage"`
	Status      string             `json:"status"`
	Summary     interface{}        `json:"summary"`
	FailedItems []BatchFailureItem `json:"failed_items"`
	Export      BatchExportPayload `json:"export"`
}

type AdminBatchJobListItem struct {
	BatchID          string `json:"batch_id"`
	BatchType        string `json:"batch_type"`
	Status           string `json:"status"`
	OperatorUsername string `json:"operator_username"`
	SuccessCount     int    `json:"success_count"`
	FailedCount      int    `json:"failed_count"`
	CreatedAt        int64  `json:"created_at"`
	FinishedAt       int64  `json:"finished_at"`
}

type AdminBatchJobListResponse struct {
	Items []AdminBatchJobListItem `json:"items"`
	Total int64                   `json:"total"`
}

type AdminBatchJobDetailResponse struct {
	BatchID              string             `json:"batch_id"`
	BatchType            string             `json:"batch_type"`
	Status               string             `json:"status"`
	OperatorUsername     string             `json:"operator_username"`
	CreatedAt            int64              `json:"created_at"`
	StartedAt            int64              `json:"started_at"`
	FinishedAt           int64              `json:"finished_at"`
	RequestPayloadMasked interface{}        `json:"request_payload_masked"`
	ScopeSummary         interface{}        `json:"scope_summary"`
	Summary              interface{}        `json:"summary"`
	FailureSummary       map[string]int     `json:"failure_summary"`
	FailedItems          []BatchFailureItem `json:"failed_items"`
}
