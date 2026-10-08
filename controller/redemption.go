package controller

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetRedemptionsList(c *gin.Context) {
	var params model.GenericParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	redemptions, err := model.GetRedemptionsList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    redemptions,
	})
}

func GetRedemption(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	redemption, err := model.GetRedemptionById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    redemption,
	})
}

func AddRedemption(c *gin.Context) {
	redemption := model.Redemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if len(redemption.Name) == 0 || len(redemption.Name) > 20 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Redemption code name must be 1-20 characters long",
		})
		return
	}
	if redemption.Count <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Redemption code count must be greater than 0",
		})
		return
	}
	if redemption.Count > 100 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Cannot generate more than 100 redemption codes at once",
		})
		return
	}
	if redemption.Quota <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Redemption code quota must be greater than 0",
		})
		return
	}
	if !model.IsValidRedemptionScope(redemption.Scope) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid redemption code purpose",
		})
		return
	}
	if redemption.Scope == "" {
		redemption.Scope = model.RedemptionScopeAny
	}
	keys := make([]string, 0, redemption.Count)
	redemptions := make([]*model.Redemption, 0, redemption.Count)
	for i := 0; i < redemption.Count; i++ {
		key := utils.GetUUID()
		redemptions = append(redemptions, &model.Redemption{
			UserId:      c.GetInt("id"),
			Name:        redemption.Name,
			Key:         key,
			CreatedTime: utils.GetTimestamp(),
			Quota:       redemption.Quota,
			Scope:       redemption.Scope,
			ExpiredTime: redemption.ExpiredTime,
		})
		keys = append(keys, key)
	}
	// 批量创建使用单事务：全部成功才提交，任一失败整体回滚，
	// 不留部分已创建码，返回值与实际落库一致。
	err = model.BatchInsertRedemptions(redemptions)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    []string{},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    keys,
	})
}

func DeleteRedemption(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	err := model.DeleteRedemptionById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateRedemption(c *gin.Context) {
	statusOnly := c.Query("status_only")
	redemption := model.Redemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	cleanRedemption, err := model.GetRedemptionById(redemption.Id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if statusOnly != "" {
		cleanRedemption.Status = redemption.Status
	} else {
		if !model.IsValidRedemptionScope(redemption.Scope) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Invalid redemption code purpose",
			})
			return
		}
		// If you add more fields, please also update redemption.Update()
		cleanRedemption.Name = redemption.Name
		cleanRedemption.Quota = redemption.Quota
		if redemption.Scope == "" {
			cleanRedemption.Scope = model.RedemptionScopeAny
		} else {
			cleanRedemption.Scope = redemption.Scope
		}
		cleanRedemption.ExpiredTime = redemption.ExpiredTime
	}
	err = cleanRedemption.Update()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    cleanRedemption,
	})
}
