package cron

import (
	"fmt"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/scheduler"
	"github.com/modeltaps/modeltaps/controller"
	"github.com/modeltaps/modeltaps/model"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/go-co-op/gocron/v2"
)

func InitCron() {
	if !config.IsMasterNode {
		logger.SysLog("Cron is disabled on slave node")
		return
	}

	// 添加每日统计任务
	err := scheduler.Manager.AddJob(
		"update_daily_statistics",
		gocron.DailyJob(
			1,
			gocron.NewAtTimes(
				gocron.NewAtTime(0, 0, 30),
			)),
		gocron.NewTask(func() {
			if err := model.UpdateStatistics(model.StatisticsUpdateTypeYesterday); err != nil {
				logger.SysError("Failed to update yesterday's statistics: " + err.Error())
				return
			}
			logger.SysLog("Updated yesterday's statistics")
		}),
	)
	if err != nil {
		logger.SysError("Cron job error: " + err.Error())
		return
	}

	if config.UserInvoiceMonth {
		// 每月一号早上四点生成上个月的账单数据
		err = scheduler.Manager.AddJob(
			"generate_statistics_month",
			gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(4, 0, 0))),
			gocron.NewTask(func() {
				err := model.InsertStatisticsMonth()
				if err != nil {
					logger.SysError("Generate statistics month data error:" + err.Error())
				}
			}),
		)
		if err != nil {
			logger.SysError("Cron job error: " + err.Error())
			return
		}
	}

	// 每小时整点批量重置令牌周期用量（幂等；精确边界由校验路径按配置时区懒重置兜底，
	// cron 仅负责持久化清扫。改为每小时可覆盖任意配置时区的 0 点边界，TK8a）
	err = scheduler.Manager.AddJob(
		"reset_token_period_quota",
		gocron.CronJob("0 * * * *", false),
		gocron.NewTask(func() {
			model.ResetTokenPeriodQuota()
			logger.SysLog("Reset API key period usage")
		}),
	)
	if err != nil {
		logger.SysError("Cron job error: " + err.Error())
		return
	}

	// 每日 00:00 UTC 批量重置组织成员预算（语义保持不变，与令牌周期重置解耦）
	err = scheduler.Manager.AddJob(
		"reset_org_member_budget",
		gocron.CronJob("CRON_TZ=UTC 0 0 * * *", false),
		gocron.NewTask(func() {
			model.ResetOrgMemberBudget()
			logger.SysLog("Reset organization member budgets")
		}),
	)
	if err != nil {
		logger.SysError("Cron job error: " + err.Error())
		return
	}

	// 每日 03:00 清理过期 log_details(完整请求/响应留存,默认保留 30 天,见 T50d)
	err = scheduler.Manager.AddJob(
		"cleanup_log_details",
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(3, 0, 0))),
		gocron.NewTask(func() {
			rows, err := model.CleanupOldLogDetails()
			if err != nil {
				logger.SysError("Failed to clean up expired log_details: " + err.Error())
				return
			}
			logger.SysLog(fmt.Sprintf("Cleaned up expired log_details, deleted %d rows", rows))
		}),
	)
	if err != nil {
		logger.SysError("Cron job error: " + err.Error())
		return
	}

	// 每日 02:00 从 models.dev 同步直连渠道(OpenAI/Anthropic/…)的模型价格，并为所有启用渠道的模型
	// 按 models.dev 全量匹配补齐元信息与价格(可经 catalog_pricing.auto_sync 关闭)
	if viper.GetBool("catalog_pricing.auto_sync") {
		err = scheduler.Manager.AddJob(
			"sync_catalog_pricing",
			gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(2, 0, 0))),
			gocron.NewTask(func() {
				count, err := model.SyncCatalogPricing(viper.GetString("catalog_pricing.url"))
				if err != nil {
					logger.SysError("Failed to sync models.dev catalog pricing: " + err.Error())
					return
				}
				logger.SysLog(fmt.Sprintf("Synced models.dev catalog pricing, %d models", count))
			}),
		)
		if err != nil {
			logger.SysError("Cron job error: " + err.Error())
			return
		}
	}

	// 每日 02:30 同步 OpenRouter 渠道的真实价格(API 自带价格的渠道)
	if viper.GetBool("channel_pricing.auto_sync") {
		err = scheduler.Manager.AddJob(
			"sync_channel_pricing",
			gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(2, 30, 0))),
			gocron.NewTask(func() {
				count, err := controller.SyncAllOpenRouterChannelsPricing()
				if err != nil {
					logger.SysError("Failed to sync OpenRouter channel pricing: " + err.Error())
					return
				}
				if count > 0 {
					logger.SysLog(fmt.Sprintf("Synced OpenRouter channel pricing, %d models", count))
				}
			}),
		)
		if err != nil {
			logger.SysError("Cron job error: " + err.Error())
			return
		}
	}

	// 检测各渠道 models 与上游实时模型列表的差异(上游下架的模型 + 上游新增模型),检出则通知(受 model_drift.auto_check 控制,默认关)
	// 周期由 model_drift.check_interval(Go duration)控制,默认留空即每日 03:30,非法值同样回退
	if viper.GetBool("model_drift.auto_check") {
		err = scheduler.Manager.AddJob(
			"model_drift_check",
			modelDriftJobDefinition(),
			gocron.NewTask(func() {
				summary, err := controller.CheckAndNotifyModelDrift()
				if err != nil {
					logger.SysError("Model drift check failed: " + err.Error())
					return
				}
				logger.SysLog(fmt.Sprintf("Model drift check done: checked %d channels, %d drifted, %d with new upstream models", summary.Checked, summary.Drifted, summary.WithNew))
			}),
		)
		if err != nil {
			logger.SysError("Cron job error: " + err.Error())
			return
		}
	}

	// 每日 03:45 校验「硬编码生图名单判真但 model_info 未标注 image 输出模态」的模型,检出则通知
	// (受 model_info_consistency.auto_check 控制,默认关)
	if viper.GetBool("model_info_consistency.auto_check") {
		err = scheduler.Manager.AddJob(
			"model_info_consistency_check",
			gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(3, 45, 0))),
			gocron.NewTask(func() {
				summary := controller.CheckAndNotifyModelInfoConsistency()
				logger.SysLog(fmt.Sprintf("Model modality consistency check done: checked %d models, %d to fix, %d reverse diffs",
					summary.Checked, len(summary.MissingImage), len(summary.UnexpectedImage)))
			}),
		)
		if err != nil {
			logger.SysError("Cron job error: " + err.Error())
			return
		}
	}

	// 每十分钟更新一次统计数据
	err = scheduler.Manager.AddJob(
		"update_statistics",
		gocron.DurationJob(10*time.Minute),
		gocron.NewTask(func() {
			if err := model.UpdateStatistics(model.StatisticsUpdateTypeToDay); err != nil {
				logger.SysError("Failed to update 10-minute statistics: " + err.Error())
				return
			}
			logger.SysLog("10-minute statistics")
		}),
	)
	if err != nil {
		logger.SysError("Cron job error: " + err.Error())
		return
	}

	// 每天凌晨 3:00 自动清理过期消费日志
	err = scheduler.Manager.AddJob(
		"log_auto_delete",
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(3, 0, 0))),
		gocron.NewTask(func() {
			if !config.LogAutoDeleteEnabled || config.LogAutoDeleteDays <= 0 {
				return
			}
			targetTimestamp := time.Now().AddDate(0, 0, -config.LogAutoDeleteDays).Unix()
			const batchSize = 10000
			var totalDeleted int64
			for {
				affected, err := model.DeleteOldLogBatch(targetTimestamp, batchSize)
				if err != nil {
					logger.SysError(fmt.Sprintf("[cron] Consumption log auto-cleanup failed after deleting %d rows: %v", totalDeleted, err))
					break
				}
				totalDeleted += affected
				if affected == 0 {
					break
				}
			}
			if totalDeleted > 0 {
				logger.SysLog(fmt.Sprintf("[cron] Consumption log auto-cleanup done, deleted %d rows", totalDeleted))
			}
		}),
	)
	if err != nil {
		logger.SysError("Cron job error: " + err.Error())
		return
	}

	// 开启自动更新 并且设置了有效自动更新时间 同时自动更新模式不是system 则会从服务器拉取最新价格表
	autoPriceUpdatesInterval := viper.GetInt("auto_price_updates_interval")
	autoPriceUpdates := viper.GetBool("auto_price_updates")
	autoPriceUpdatesMode := viper.GetString("auto_price_updates_mode")

	if autoPriceUpdates &&
		autoPriceUpdatesInterval > 0 &&
		(autoPriceUpdatesMode == string(model.PriceUpdateModeAdd) ||
			autoPriceUpdatesMode == string(model.PriceUpdateModeOverwrite) ||
			autoPriceUpdatesMode == string(model.PriceUpdateModeUpdate)) {
		// 指定时间周期更新价格表
		err := scheduler.Manager.AddJob(
			"update_pricing_by_service",
			gocron.DurationJob(time.Duration(autoPriceUpdatesInterval)*time.Minute),
			gocron.NewTask(func() {
				err := model.UpdatePriceByPriceService()
				if err != nil {
					logger.SysError("Update Price Error: " + err.Error())
					return
				}
				logger.SysLog("Update Price Done")
			}),
		)
		if err != nil {
			logger.SysError("Cron job error: " + err.Error())
			return
		}
	}

	if err != nil {
		logger.SysError("Cron job error: " + err.Error())
		return
	}
}

// modelDriftJobDefinition 按 model_drift.check_interval 决定漂移检测周期。
// 留空或非法（含非正值）时回退到原每日 03:30 行为。
func modelDriftJobDefinition() gocron.JobDefinition {
	daily := gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(3, 30, 0)))

	raw := strings.TrimSpace(viper.GetString("model_drift.check_interval"))
	if raw == "" {
		return daily
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		logger.SysError("model_drift.check_interval is invalid (" + raw + "), falling back to daily check at 03:30")
		return daily
	}
	return gocron.DurationJob(interval)
}
