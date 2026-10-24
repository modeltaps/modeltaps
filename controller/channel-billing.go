package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	providersBase "github.com/modeltaps/modeltaps/providers/base"

	"github.com/gin-gonic/gin"
)

type OpenAISubscriptionResponse struct {
	Object             string  `json:"object"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
	SoftLimitUSD       float64 `json:"soft_limit_usd"`
	HardLimitUSD       float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD float64 `json:"system_hard_limit_usd"`
	AccessUntil        int64   `json:"access_until"`
}

type OpenAIUsageDailyCost struct {
	Timestamp float64 `json:"timestamp"`
	LineItems []struct {
		Name string  `json:"name"`
		Cost float64 `json:"cost"`
	}
}

type OpenAICreditGrants struct {
	Object         string  `json:"object"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
	TotalAvailable float64 `json:"total_available"`
}

type OpenAIUsageResponse struct {
	Object string `json:"object"`
	//DailyCosts []OpenAIUsageDailyCost `json:"daily_costs"`
	TotalUsage float64 `json:"total_usage"` // unit: 0.01 dollar
}

// errBalanceNotSupported 标记「该渠道类型没有余额查询」：批量刷新静默跳过，单渠道刷新给出明确提示。
var errBalanceNotSupported = providersBase.ErrBalanceNotSupported

func updateChannelBalance(channel *model.Channel) (float64, error) {
	req, err := http.NewRequest("POST", "/balance", nil)
	if err != nil {
		return 0, err
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	req.Header.Set("Content-Type", "application/json")

	provider := providers.GetProvider(channel, c)
	if provider == nil {
		return 0, errors.New("provider not found")
	}

	balanceProvider, ok := provider.(providersBase.BalanceInterface)
	if !ok {
		return 0, errBalanceNotSupported
	}

	balance, err := balanceProvider.Balance()
	if err != nil {
		return 0, err
	}
	checkLowBalance(channel, balance)
	return balance, nil
}

// checkLowBalance 在余额低于渠道阈值时发一次通知；余额回到阈值之上后清零标记，下次再跌破才会再发。
// 阈值为 0 表示该渠道不提醒。
func checkLowBalance(channel *model.Channel, balance float64) {
	if channel.BalanceThreshold <= 0 {
		return
	}
	if balance >= channel.BalanceThreshold {
		if channel.BalanceAlertedTime != 0 {
			_ = channel.UpdateBalanceAlertedTime(0)
		}
		return
	}
	if channel.BalanceAlertedTime != 0 {
		return
	}
	if err := channel.UpdateBalanceAlertedTime(time.Now().Unix()); err != nil {
		return
	}
	subject := fmt.Sprintf("Channel %s balance is low", channel.Name)
	content := fmt.Sprintf("Channel: %s (ID %d)\nBalance: $%.2f\nThreshold: $%.2f\nChecked at: %s",
		channel.Name, channel.Id, balance, channel.BalanceThreshold, time.Now().Format(time.RFC3339))
	notify.SendKind(notify.KindLowBalance, subject, content)
}

func UpdateChannelBalance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	channel, err := model.GetChannelById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	balance, err := updateChannelBalance(channel)
	if err != nil {
		if errors.Is(err, errBalanceNotSupported) {
			c.JSON(http.StatusOK, gin.H{
				"success":   false,
				"supported": false,
				"message":   "balance query is not supported for this channel type",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"balance": balance,
	})
}

// updateAllChannelsBalance 刷新所有启用渠道的余额：不支持余额查询的类型跳过，查询失败只记日志；
// 余额 ≤ 0 视为额度用尽，自动禁用渠道。低余额提醒在 updateChannelBalance 内处理。
func updateAllChannelsBalance() error {
	channels, err := model.GetAllChannels()
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if channel.Status != config.ChannelStatusEnabled {
			continue
		}
		balance, err := updateChannelBalance(channel)
		if err != nil {
			if !errors.Is(err, errBalanceNotSupported) {
				logger.SysError(fmt.Sprintf("update balance failed for channel %s (ID %d): %s", channel.Name, channel.Id, err.Error()))
			}
			continue
		}
		// err is nil & balance <= 0 means quota is used up
		if balance <= 0 {
			DisableChannel(channel.Id, channel.Name, "", "Insufficient balance", true)
		}
		time.Sleep(config.RequestInterval)
	}
	return nil
}

// UpdateAllChannelsBalanceJob 供 cron 调用的批量刷新入口。
func UpdateAllChannelsBalanceJob() {
	if err := updateAllChannelsBalance(); err != nil {
		logger.SysError("update all channels balance failed: " + err.Error())
		return
	}
	logger.SysLog("update all channels balance done")
}

func UpdateAllChannelsBalance(c *gin.Context) {
	// TODO: make it async
	err := updateAllChannelsBalance()
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

// AutomaticallyUpdateChannels 按 channel.update_frequency（分钟）定期刷新所有渠道余额；0 表示关闭。
func AutomaticallyUpdateChannels(frequency int) {
	if frequency <= 0 {
		return
	}

	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		logger.SysLog("updating all channels balance")
		UpdateAllChannelsBalanceJob()
	}
}
