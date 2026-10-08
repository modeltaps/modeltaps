package model

import (
	"errors"
	"fmt"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 兑换码限定用途：空值等同 RedemptionScopeAny(兼容存量数据)
const (
	RedemptionScopeAny      = "any"      // 不限：个人与组织均可兑换
	RedemptionScopePersonal = "personal" // 仅个人账户
	RedemptionScopeOrg      = "org"      // 仅组织池(影子记账账户)
)

// IsValidRedemptionScope 校验用途取值，空值按「不限」处理。
func IsValidRedemptionScope(scope string) bool {
	switch scope {
	case "", RedemptionScopeAny, RedemptionScopePersonal, RedemptionScopeOrg:
		return true
	}
	return false
}

type Redemption struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id"`
	Key          string `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int    `json:"status" gorm:"default:1"`
	Name         string `json:"name" gorm:"index"`
	Quota        int    `json:"quota" gorm:"default:100"`
	Scope        string `json:"scope" gorm:"type:varchar(16);default:'any'"` // any/personal/org
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	RedeemedTime int64  `json:"redeemed_time" gorm:"bigint"`
	ExpiredTime  int64  `json:"expired_time" gorm:"bigint;default:0"` // 过期时间戳，-1 或 0 表示永不过期
	Count        int    `json:"count" gorm:"-:all"`                   // only for api request
}

var allowedRedemptionslOrderFields = map[string]bool{
	"id":            true,
	"name":          true,
	"status":        true,
	"quota":         true,
	"created_time":  true,
	"redeemed_time": true,
	"expired_time":  true,
}

func GetRedemptionsList(params *GenericParams) (*DataResult[Redemption], error) {
	var redemptions []*Redemption
	db := DB
	if params.Keyword != "" {
		db = db.Where("id = ? or name LIKE ?", utils.String2Int(params.Keyword), params.Keyword+"%")
	}

	return PaginateAndOrder[Redemption](db, &params.PaginationParams, &redemptions, allowedRedemptionslOrderFields)
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func Redeem(key string, userId int, ip string) (quota int, err error) {
	if key == "" {
		return 0, errors.New("redemption code not provided")
	}
	if userId == 0 {
		return 0, errors.New("invalid user id")
	}
	redemption := &Redemption{}

	keyCol := "`key`"
	if common.UsingPostgreSQL {
		keyCol = `"key"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		// 行锁读取(MySQL/PostgreSQL 生效;SQLite 不支持行锁,由下方原子守卫兜底)
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(keyCol+" = ?", key).First(redemption).Error
		if err != nil {
			return errors.New("invalid redemption code")
		}
		if redemption.Status != config.RedemptionCodeStatusEnabled {
			return errors.New("this redemption code has already been used")
		}
		if redemption.ExpiredTime > 0 && redemption.ExpiredTime < utils.GetTimestamp() {
			return errors.New("this redemption code has expired")
		}
		// 用途校验放在标记 used 之前，被拒绝时兑换码保持可用
		if redemption.Scope == RedemptionScopePersonal || redemption.Scope == RedemptionScopeOrg {
			var userType int
			if err := tx.Model(&User{}).Where("id = ?", userId).Select("type").Scan(&userType).Error; err != nil {
				return err
			}
			isOrgAccount := userType == config.UserTypeOrgShadow
			if redemption.Scope == RedemptionScopeOrg && !isOrgAccount {
				return errors.New("this redemption code can only be used by organization accounts")
			}
			if redemption.Scope == RedemptionScopePersonal && isOrgAccount {
				return errors.New("this redemption code can only be used by personal accounts")
			}
		}
		// 原子守卫:仅当仍为 enabled 时置为 used,RowsAffected==1 才算抢到(跨库双保险)
		redeemedTime := utils.GetTimestamp()
		result := tx.Model(&Redemption{}).
			Where(keyCol+" = ? AND status = ?", key, config.RedemptionCodeStatusEnabled).
			Updates(map[string]interface{}{
				"status":        config.RedemptionCodeStatusUsed,
				"redeemed_time": redeemedTime,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("this redemption code has already been used")
		}
		// 抢到后再加额度,保证标记 used 与加额度在同一事务内一致
		err = tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", redemption.Quota)).Error
		if err != nil {
			return err
		}
		redemption.RedeemedTime = redeemedTime
		redemption.Status = config.RedemptionCodeStatusUsed
		return nil
	})
	if err != nil {
		return 0, errors.New("redemption failed: " + err.Error())
	}

	// Try to upgrade user group based on cumulative recharge amount
	// (the transaction above has committed the redeemed quota to users.quota)
	err = CheckAndUpgradeUserGroup(userId)
	if err != nil {
		logger.SysError("failed to check and upgrade user group: " + err.Error())
	}

	RecordQuotaLog(userId, LogTypeTopup, redemption.Quota, ip, fmt.Sprintf("Top-up of %s via redemption code", common.LogQuota(redemption.Quota)))

	// 处理邀请人充值返利
	err = ProcessInviterReward(userId, redemption.Quota, ip)
	if err != nil {
		logger.SysError("failed to process inviter reward for redemption: " + err.Error())
	}

	return redemption.Quota, nil
}

func (redemption *Redemption) Insert() error {
	var err error
	err = DB.Create(redemption).Error
	return err
}

// BatchInsertRedemptions 在单个事务中批量创建兑换码：全部成功才提交，
// 任一失败整体回滚，不会留下部分已创建的兑换码。
func BatchInsertRedemptions(redemptions []*Redemption) error {
	if len(redemptions) == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&redemptions).Error
	})
}

func (redemption *Redemption) SelectUpdate() error {
	// This can update zero values
	return DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (redemption *Redemption) Update() error {
	var err error
	err = DB.Model(redemption).Select("name", "status", "quota", "scope", "redeemed_time", "expired_time").Updates(redemption).Error
	return err
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id is empty")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

type RedemptionStatistics struct {
	Count  int64 `json:"count"`
	Quota  int64 `json:"quota"`
	Status int   `json:"status"`
}

func GetStatisticsRedemption() (redemptionStatistics []*RedemptionStatistics, err error) {
	err = DB.Model(&Redemption{}).Select("status", "count(*) as count", "sum(quota) as quota").Where("status != ?", 2).Group("status").Scan(&redemptionStatistics).Error
	return redemptionStatistics, err
}

type RedemptionStatisticsGroup struct {
	Date      string `json:"date"`
	Quota     int64  `json:"quota"`
	UserCount int64  `json:"user_count"`
}

func GetStatisticsRedemptionByPeriod(startTimestamp, endTimestamp int64) (redemptionStatistics []*RedemptionStatisticsGroup, err error) {
	groupSelect := getTimestampGroupsSelect("redeemed_time", "day", "date")

	err = DB.Raw(`
		SELECT `+groupSelect+`,
		sum(quota) as quota,
		count(distinct user_id) as user_count
		FROM redemptions
		WHERE status=3
		AND redeemed_time BETWEEN ? AND ?
		GROUP BY date
		ORDER BY date
	`, startTimestamp, endTimestamp).Scan(&redemptionStatistics).Error

	return redemptionStatistics, err
}
