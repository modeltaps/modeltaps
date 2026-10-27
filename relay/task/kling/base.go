package kling

import (
	"encoding/json"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	KlingProvider "github.com/modeltaps/modeltaps/providers/kling"

	"github.com/gin-gonic/gin"
)

func StringError(c *gin.Context, httpCode int, code, message string) {
	err := &types.TaskResponse[any]{
		Code:    code,
		Message: utils.MaskSensitiveInfo(message), // 可能是上游原文，回写客户端前脱敏
	}

	c.JSON(httpCode, err)
}

func TaskModel2Dto(task *model.Task) *KlingProvider.KlingResponse[*KlingProvider.KlingTaskData] {
	data := &KlingProvider.KlingResponse[*KlingProvider.KlingTaskData]{}
	json.Unmarshal(task.Data, data)

	return data
}
