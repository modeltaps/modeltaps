package model

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/redis"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// 周期性配额重置（OpenRouter 风格 limit_reset）：
// period_used 在 TokenSetting JSON 中另记，重置只清 period_used，不动累计 used_quota。
// 周期边界按站点配置的时区(QuotaResetTimezone,默认 UTC)的 0 点计算，weekly 周起点
// 由 QuotaResetWeekStart(默认 monday)决定。无 quota_reset 字段时零行为变化。

const (
	TokenQuotaResetPeriodDaily   = "daily"
	TokenQuotaResetPeriodWeekly  = "weekly"
	TokenQuotaResetPeriodMonthly = "monthly"
)

type QuotaResetSetting struct {
	Period string `json:"period"` // daily | weekly | monthly
	Limit  int    `json:"limit"`
}

// tokenInPeriodMode 判定令牌是否处于周期模式（OpenRouter limit+limit_reset 语义）：
// 配置了 quota_reset 且 limit>0。该模式下额度按周期重置，remain_quota 不再作为约束，
// 仅受周期上限（tokenPeriodQuotaExceeded）约束。
func tokenInPeriodMode(setting *TokenSetting) bool {
	return setting.QuotaReset != nil && setting.QuotaReset.Limit > 0
}

// 周期边界时区缓存:LoadLocation 有开销,按配置的时区名缓存 *time.Location,
// 仅在时区名变化时重新加载。无效时区名回退 UTC 并对该名字记一次 SysError。
var (
	resetLocMu     sync.Mutex
	resetLoc       = time.UTC
	resetLocName   = "UTC"
	resetLocErrFor string // 已记过 SysError 的无效时区名,避免重复刷屏
)

// resetLocation 返回当前配置时区对应的 *time.Location(带缓存)。
func resetLocation() *time.Location {
	name := strings.TrimSpace(config.QuotaResetTimezone)
	if name == "" {
		name = "UTC"
	}
	resetLocMu.Lock()
	defer resetLocMu.Unlock()
	if name == resetLocName {
		return resetLoc
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		if resetLocErrFor != name {
			logger.SysError(fmt.Sprintf("invalid period reset time zone %q, falling back to UTC: %s", name, err.Error()))
			resetLocErrFor = name
		}
		resetLoc = time.UTC
		resetLocName = name
		return resetLoc
	}
	resetLoc = loc
	resetLocName = name
	resetLocErrFor = ""
	return resetLoc
}

// weekStartOffset 返回 wd 距离本周起点的天数(依 QuotaResetWeekStart)。
// monday(默认):周一为起点;sunday:周日为起点。
func weekStartOffset(wd time.Weekday) int {
	if strings.EqualFold(strings.TrimSpace(config.QuotaResetWeekStart), "sunday") {
		return int(wd) // Sunday=0
	}
	return (int(wd) + 6) % 7 // monday 起点
}

// currentPeriodStart 返回 now 所在周期的起点(按配置时区)。未知 period 返回零值。
func currentPeriodStart(period string, now time.Time) time.Time {
	loc := resetLocation()
	t := now.In(loc)
	switch period {
	case TokenQuotaResetPeriodDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	case TokenQuotaResetPeriodWeekly:
		d := t.AddDate(0, 0, -weekStartOffset(t.Weekday()))
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	case TokenQuotaResetPeriodMonthly:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	default:
		return time.Time{}
	}
}

// tokenPeriodQuotaExceeded 判断周期限额是否已用尽。
// PeriodStart 早于当前周期起点视为已跨期（等效 period_used=0），
// 因此边界语义精确到配置时区的 0 点，不依赖 cron 触发时刻。
func tokenPeriodQuotaExceeded(setting *TokenSetting, periodUsed int, periodStart int64, now time.Time) bool {
	rs := setting.QuotaReset
	if rs == nil || rs.Limit <= 0 {
		return false
	}
	ps := currentPeriodStart(rs.Period, now)
	if ps.IsZero() {
		return false
	}
	return periodStart >= ps.Unix() && periodUsed >= rs.Limit
}

// PeriodConfigChanged 周期配置(period)是否发生变化。计数现为 Token 专用列:
// 普通更新不触碰计数列故自动保留;仅 period 变化时调用方需清零(ResetTokenPeriodCounters)。
func PeriodConfigChanged(newSetting *TokenSetting, oldSetting *TokenSetting) bool {
	oldPeriod, newPeriod := "", ""
	if oldSetting.QuotaReset != nil {
		oldPeriod = oldSetting.QuotaReset.Period
	}
	if newSetting.QuotaReset != nil {
		newPeriod = newSetting.QuotaReset.Period
	}
	return oldPeriod != newPeriod
}

// AccrueTokenPeriodUsed 累计周期用量（delta 可为负，用于回退预扣）。
// 仅在 token 配置了 quota_reset 时由调用方触发，未配置时不产生任何写入。
func AccrueTokenPeriodUsed(id int, delta int) error {
	if delta == 0 {
		return nil
	}
	if config.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeTokenPeriodUsed, id, delta)
		return nil
	}
	return accrueTokenPeriodUsed(id, delta)
}

func accrueTokenPeriodUsed(id int, delta int) error {
	// 仅读取周期配置以算当前周期起点(非竞态值);period_used 由下方单条原子语句变更
	token, err := GetTokenById(id)
	if err != nil {
		return err
	}
	rs := token.Setting.Data().QuotaReset
	if rs == nil {
		return nil
	}
	// 未知周期用 -1 使懒重置分支恒不触发,仅做自增
	psUnix := int64(-1)
	if ps := currentPeriodStart(rs.Period, time.Now()); !ps.IsZero() {
		psUnix = ps.Unix()
	}
	// 单条原子 UPDATE:懒重置(跨期清零并推进起点)+ 自增 + 钳制≥0。同一行并发 UPDATE 由 DB
	// 行级串行,每条语句基于当前已提交值求值,故无读-改-写丢更新(对照 token 额度 gorm.Expr)。
	res := DB.Model(&Token{}).Where("id = ?", id).UpdateColumns(map[string]interface{}{
		"period_used": gorm.Expr(
			"CASE WHEN period_start < ? THEN (CASE WHEN ? > 0 THEN ? ELSE 0 END) "+
				"ELSE (CASE WHEN period_used + ? < 0 THEN 0 ELSE period_used + ? END) END",
			psUnix, delta, delta, delta, delta),
		"period_start": gorm.Expr("CASE WHEN period_start < ? THEN ? ELSE period_start END", psUnix, psUnix),
	})
	// period_used 参与令牌校验，写入后必须清缓存（token.go:330-336 模式）
	if res.Error == nil && config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserTokensKey, token.Key))
	}
	return res.Error
}

// ResetTokenPeriodCounters 把令牌周期计数列清零(周期配置变化时由更新路径调用)。
func ResetTokenPeriodCounters(id int) error {
	token, err := GetTokenById(id)
	if err != nil {
		return err
	}
	err = DB.Model(&Token{}).Where("id = ?", id).
		UpdateColumns(map[string]interface{}{"period_used": 0, "period_start": 0}).Error
	if err == nil && config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserTokensKey, token.Key))
	}
	return err
}

func batchAccrueTokenPeriodUsed(store map[int]int) {
	for id, delta := range store {
		if err := accrueTokenPeriodUsed(id, delta); err != nil {
			logger.SysError(fmt.Sprintf("batch accrue token period used failed (id=%d): %s", id, err.Error()))
		}
	}
}

// ResetTokenPeriodQuota 批量把已跨期 token 的 period_used 落库清零（幂等）。
// 精确的按配置时区 0 点边界由校验/累计路径的懒重置保证，cron(每小时整点)只负责持久化清扫。
func ResetTokenPeriodQuota() {
	now := time.Now()
	var tokens []*Token
	result := DB.Where("setting IS NOT NULL").FindInBatches(&tokens, 500, func(tx *gorm.DB, batch int) error {
		for _, token := range tokens {
			rs := token.Setting.Data().QuotaReset
			if rs == nil {
				continue
			}
			ps := currentPeriodStart(rs.Period, now)
			if ps.IsZero() || token.PeriodStart >= ps.Unix() {
				continue
			}
			// 跨期清零并推进起点(专用列);period_start < ? 守卫保证幂等
			if err := tx.Model(&Token{}).Where("id = ? AND period_start < ?", token.Id, ps.Unix()).
				UpdateColumns(map[string]interface{}{"period_used": 0, "period_start": ps.Unix()}).Error; err != nil {
				logger.SysError(fmt.Sprintf("reset token period quota failed (id=%d): %s", token.Id, err.Error()))
				continue
			}
			if config.RedisEnabled {
				redis.RedisDel(fmt.Sprintf(UserTokensKey, token.Key))
			}
		}
		return nil
	})
	if result.Error != nil {
		logger.SysError("reset token period quota scan failed: " + result.Error.Error())
	}
}
