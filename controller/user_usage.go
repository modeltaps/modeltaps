package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// 个人用量聚合查询的最大时间跨度（天）
const MaxUserUsageQueryDays = 370

// checkUserUsageTimeRange 校验聚合查询时间范围，失败时写入错误响应并返回 true
func checkUserUsageTimeRange(c *gin.Context, startTimestamp, endTimestamp int64) bool {
	var err error
	if startTimestamp > 0 && endTimestamp > 0 {
		maxDuration := time.Duration(MaxUserUsageQueryDays) * 24 * time.Hour
		if endTimestamp < startTimestamp {
			err = fmt.Errorf("end time must not be earlier than start time")
		} else if time.Unix(endTimestamp, 0).Sub(time.Unix(startTimestamp, 0)) > maxDuration {
			err = fmt.Errorf("query time range must not exceed %d days", MaxUserUsageQueryDays)
		}
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return true
	}
	return false
}

// GetUserUsageAnalytics 个人用量聚合(按令牌/模型/时间维度,形态与 GetOrgUsageAnalytics 同构)
func GetUserUsageAnalytics(c *gin.Context) {
	var params model.LogsListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if checkUserUsageTimeRange(c, params.StartTimestamp, params.EndTimestamp) {
		return
	}
	groupBy := c.DefaultQuery("group_by", model.UserUsageGroupDate)
	userId := c.GetInt("id")
	statistics, err := model.GetUserUsageStatistics(userId, groupBy, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    statistics,
	})
}
