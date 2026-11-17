// Path: controller/midjourney.go
package controller

import (
	"bytes"
	"context"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	provider "github.com/modeltaps/modeltaps/providers/midjourney"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	taskActive int32 = 0
	lock       sync.Mutex
	cond       = sync.NewCond(&lock)
)

func InitMidjourneyTask() {
	common.SafeGoroutine(func() {
		midjourneyTask()
	})

	ActivateUpdateMidjourneyTaskBulk()
}

func midjourneyTask() {
	for {
		lock.Lock()
		for atomic.LoadInt32(&taskActive) == 0 {
			cond.Wait() // 等待激活信号
		}
		lock.Unlock()
		UpdateMidjourneyTaskBulk()
	}
}

func ActivateUpdateMidjourneyTaskBulk() {
	if atomic.LoadInt32(&taskActive) == 1 {
		return
	}

	lock.Lock()
	atomic.StoreInt32(&taskActive, 1)
	cond.Signal() // 通知等待的任务
	lock.Unlock()
}

func DeactivateMidjourneyTaskBulk() {
	if atomic.LoadInt32(&taskActive) == 0 {
		return
	}

	lock.Lock()
	atomic.StoreInt32(&taskActive, 0)
	lock.Unlock()
}

func UpdateMidjourneyTaskBulk() {
	ctx := context.WithValue(context.Background(), logger.RequestIdKey, "MidjourneyTask")
	for {
		logger.LogInfo(ctx, "running")

		tasks := model.GetAllUnFinishTasks()

		// 如果没有未完成的任务，则等待
		if len(tasks) == 0 {
			DeactivateMidjourneyTaskBulk()
			logger.LogInfo(ctx, "no tasks, waiting...")
			return
		}

		logger.LogWarn(ctx, fmt.Sprintf("unfinished tasks detected: %v", len(tasks)))
		taskChannelM := make(map[int][]string)
		taskM := make(map[string]*model.Midjourney)
		nullTaskIds := make([]int, 0)
		for _, task := range tasks {
			if task.MjId == "" {
				// 统计失败的未完成任务
				nullTaskIds = append(nullTaskIds, task.Id)
				continue
			}
			taskM[task.MjId] = task
			taskChannelM[task.ChannelId] = append(taskChannelM[task.ChannelId], task.MjId)
		}
		if len(nullTaskIds) > 0 {
			err := model.MjBulkUpdateByTaskIds(nullTaskIds, map[string]any{
				"status":   "FAILURE",
				"progress": "100%",
			})
			if err != nil {
				logger.LogError(ctx, fmt.Sprintf("Fix null mj_id task error: %v", err))
			} else {
				logger.LogInfo(ctx, fmt.Sprintf("Fix null mj_id task success: %v", nullTaskIds))
			}
		}
		if len(taskChannelM) == 0 {
			continue
		}

		for channelId, taskIds := range taskChannelM {
			logger.LogWarn(ctx, fmt.Sprintf("channel #%d unfinished tasks: %d", channelId, len(taskIds)))
			if len(taskIds) == 0 {
				continue
			}
			midjourneyChannel := model.ChannelGroup.GetChannel(channelId)
			if midjourneyChannel == nil {
				err := model.MjBulkUpdate(taskIds, map[string]any{
					"fail_reason": fmt.Sprintf("Failed to get channel info, please contact the admin. Channel ID: %d", channelId),
					"status":      "FAILURE",
					"progress":    "100%",
				})
				logger.LogError(ctx, fmt.Sprintf("UpdateMidjourneyTask error: %v", err))
				continue
			}

			err := MjTaskHandler(midjourneyChannel, taskIds, taskM)
			if err != nil {
				logger.LogError(ctx, fmt.Sprintf("MjTaskHandler error: %v", err))
			}
		}
		time.Sleep(time.Duration(15) * time.Second)
	}
}

func MjTaskHandler(midjourneyChannel *model.Channel, taskIds []string, taskM map[string]*model.Midjourney) error {
	requestUrl := fmt.Sprintf("%s/mj/task/list-by-condition", *midjourneyChannel.BaseURL)

	body, _ := json.Marshal(map[string]any{
		"ids": taskIds,
	})
	req, err := http.NewRequest("POST", requestUrl, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("get task error: %v", err)
	}
	// 设置超时时间
	timeout := time.Second * 5
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// 使用带有超时的 context 创建新的请求
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("mj-api-secret", midjourneyChannel.Key)
	resp, err := requester.GetHTTPClient(midjourneyChannel.GetProxy()).Do(req)
	if err != nil {
		return fmt.Errorf("get task do req error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get task status code: %d", resp.StatusCode)
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("get task parse body error: %v", err)
	}
	var responseItems []provider.MidjourneyDto
	err = json.Unmarshal(responseBody, &responseItems)
	if err != nil {
		return fmt.Errorf("get task parse body error2: %v, body: %s", err, string(responseBody))
	}

	for _, responseItem := range responseItems {
		task := taskM[responseItem.MjId]
		if task == nil {
			continue
		}

		if mjTaskTimedOut(task, responseItem, time.Now().UnixMilli()) {
			responseItem.FailReason = "Upstream task timed out (over 1 hour)"
			responseItem.Status = "FAILURE"
			// The task is refunded, so it must not keep a result the user could still fetch.
			responseItem.ImageUrl = ""
		}

		if !checkMjTaskNeedUpdate(task, responseItem) {
			continue
		}
		task.Code = 1
		task.Progress = responseItem.Progress
		task.PromptEn = responseItem.PromptEn
		task.State = responseItem.State
		task.SubmitTime = responseItem.SubmitTime
		task.StartTime = responseItem.StartTime
		task.FinishTime = responseItem.FinishTime
		task.ImageUrl = responseItem.ImageUrl
		task.Status = responseItem.Status
		task.FailReason = responseItem.FailReason
		if responseItem.Properties != nil {
			propertiesStr, _ := json.Marshal(responseItem.Properties)
			task.Properties = string(propertiesStr)
		}
		if responseItem.Buttons != nil {
			buttonStr, _ := json.Marshal(responseItem.Buttons)
			task.Buttons = string(buttonStr)
		}

		failed := (task.Progress != "100%" && responseItem.FailReason != "") || (task.Progress == "100%" && task.Status == "FAILURE")
		if failed {
			logger.LogError(ctx, task.MjId+" build failed, "+task.FailReason)
			task.Progress = "100%"
		}
		// Save first: the task leaves the unfinished set only once it is stored at 100%, so a
		// failed save must not refund, or the next round would refund it again.
		err = task.Update()
		if err != nil {
			logger.LogError(ctx, "UpdateMidjourneyTask task error: "+err.Error())
			continue
		}
		if failed {
			refundMjTask(ctx, task)
		}
	}

	return nil
}

// mjTaskTimeoutMs is how long a task may stay unfinished upstream before it is failed and refunded.
const mjTaskTimeoutMs = 3600 * 1000

// mjTaskTimedOut reports whether a task the upstream has not finished ran past the timeout. A task
// the upstream reports as finished (SUCCESS, FAILURE or 100%) is never timed out, so a late success
// is not refunded while its image stays available (SEC-76).
func mjTaskTimedOut(task *model.Midjourney, item provider.MidjourneyDto, nowMs int64) bool {
	if task.Progress == "100%" || item.Progress == "100%" || item.Status == "SUCCESS" || item.Status == "FAILURE" {
		return false
	}
	return nowMs-task.SubmitTime > mjTaskTimeoutMs
}

// refundMjTask returns the quota a failed task was charged. task.Quota is the amount its
// submission was actually charged (0 for actions that are never billed), so the refund never
// exceeds the charge (SEC-63).
func refundMjTask(ctx context.Context, task *model.Midjourney) {
	quota := task.Quota
	if quota <= 0 {
		return
	}
	if err := model.IncreaseUserQuota(task.UserId, quota); err != nil {
		logger.LogError(ctx, "fail to increase user quota: "+err.Error())
		return
	}
	if err := model.CacheUpdateUserQuota(task.UserId); err != nil {
		logger.LogError(ctx, "error update user quota cache: "+err.Error())
	}
	logContent := fmt.Sprintf("Image generation failed %s, refunded %s", task.MjId, common.LogQuota(quota))
	model.RecordLog(task.UserId, model.LogTypeSystem, logContent)
}

func checkMjTaskNeedUpdate(oldTask *model.Midjourney, newTask provider.MidjourneyDto) bool {
	if oldTask.Code != 1 {
		return true
	}
	if oldTask.Progress != newTask.Progress {
		return true
	}
	if oldTask.PromptEn != newTask.PromptEn {
		return true
	}
	if oldTask.State != newTask.State {
		return true
	}
	if oldTask.SubmitTime != newTask.SubmitTime {
		return true
	}
	if oldTask.StartTime != newTask.StartTime {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if oldTask.ImageUrl != newTask.ImageUrl {
		return true
	}
	if oldTask.Status != newTask.Status {
		return true
	}
	if oldTask.FailReason != newTask.FailReason {
		return true
	}
	if oldTask.Progress != "100%" && newTask.FailReason != "" {
		return true
	}

	return false
}

func GetAllMidjourney(c *gin.Context) {
	var params model.MJTaskQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	midjourneys, err := model.GetAllMJTasks(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    midjourneys,
	})
}

func GetUserMidjourney(c *gin.Context) {
	userId := c.GetInt("id")
	tokenId := c.GetInt("token_id")
	var params model.MJTaskQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if tokenId > 0 {
		params.TokenID = tokenId
	}

	midjourneys, err := model.GetAllUserMJTask(userId, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    midjourneys,
	})
}
