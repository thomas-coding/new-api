package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func PreviewBatchUsers(c *gin.Context) {
	var req dto.BatchUsersPreviewRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	resp, err := service.PreviewBatchUsers(c.GetInt("id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func ExecuteBatchUsers(c *gin.Context) {
	var req dto.BatchUsersExecuteRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	resp, err := service.ExecuteBatchUsers(serviceBatchContext(c), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func PreviewBatchRedemptions(c *gin.Context) {
	var req dto.BatchRedemptionsPreviewRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	resp, err := service.PreviewBatchRedemptions(c.GetInt("id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func ExecuteBatchRedemptions(c *gin.Context) {
	var req dto.BatchRedemptionsExecuteRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	resp, err := service.ExecuteBatchRedemptions(serviceBatchContext(c), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func PreviewBatchQuota(c *gin.Context) {
	var req dto.BatchQuotaPreviewRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	resp, err := service.PreviewBatchQuota(c.GetInt("id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func ExecuteBatchQuota(c *gin.Context) {
	var req dto.BatchQuotaExecuteRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	resp, err := service.ExecuteBatchQuota(serviceBatchContext(c), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func ListAdminBatchJobs(c *gin.Context) {
	resp, err := service.ListAdminBatchJobs()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func GetAdminBatchJob(c *gin.Context) {
	resp, err := service.GetAdminBatchJob(c.Param("batch_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func serviceBatchContext(c *gin.Context) service.BatchOperationContext {
	return service.BatchOperationContext{
		OperatorID:       c.GetInt("id"),
		OperatorUsername: c.GetString("username"),
		SourceIP:         c.ClientIP(),
		UserAgent:        c.Request.UserAgent(),
	}
}
