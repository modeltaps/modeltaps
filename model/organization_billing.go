package model

import (
	"errors"
	"fmt"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
)

// TransferQuotaToOrganization Owner/Admin 将个人积分单向转入组织池(规格 §3.3 / a5)。
// 事务内完成:个人条件扣减(防并发透支)→ 影子账户入账 → 双侧 Log 记录;提交后清理双方配额缓存。
func TransferQuotaToOrganization(org *Organization, fromUserId int, quota int) error {
	if org == nil || org.ShadowUserId == 0 {
		return errors.New("organization shadow account not found")
	}
	if fromUserId == 0 {
		return errors.New("user id must not be empty")
	}
	if quota <= 0 {
		return errors.New("transfer amount must be greater than 0")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 条件更新:余额不足时影响行数为 0,保证并发下不透支
		result := tx.Model(&User{}).
			Where("id = ? AND quota >= ?", fromUserId, quota).
			Update("quota", gorm.Expr("quota - ?", quota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("insufficient personal balance")
		}
		if err := IncreaseUserQuotaWithTx(tx, org.ShadowUserId, quota); err != nil {
			return err
		}
		var username string
		if err := tx.Model(&User{}).Where("id = ?", fromUserId).Select("username").Find(&username).Error; err != nil {
			username, _ = CacheGetUsername(fromUserId)
		}
		now := utils.GetTimestamp()
		logs := []*Log{
			{
				UserId:    fromUserId,
				Username:  username,
				Quota:     quota,
				CreatedAt: now,
				Type:      LogTypeManage,
				Content:   fmt.Sprintf("Transferred personal balance to organization %s: %s", org.Name, common.LogQuota(quota)),
			},
			{
				UserId:    org.ShadowUserId,
				Username:  "org-" + org.Slug,
				Quota:     quota,
				CreatedAt: now,
				Type:      LogTypeManage,
				Content:   fmt.Sprintf("Received personal balance from member %s: %s", username, common.LogQuota(quota)),
			},
		}
		return tx.Create(&logs).Error
	})
	if err != nil {
		return err
	}
	if config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserQuotaCacheKey, fromUserId))
		redis.RedisDel(fmt.Sprintf(UserQuotaCacheKey, org.ShadowUserId))
	}
	return nil
}
