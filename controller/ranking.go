package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetTodayUsageRanking(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	ranking, err := model.GetTodayUsageRanking(c.GetInt("id"), limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, ranking)
}
