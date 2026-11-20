package model

import (
	"errors"
	"strings"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/gorm"
)

// UserGroupReferences counts what names a user group: accounts (organization shadow accounts
// apart), API keys by their group and by their backup group, and channels that list the group
// by name (a channel on ChannelGroupAll covers whatever groups exist and does not count).
type UserGroupReferences struct {
	Users          int64 `json:"users"`
	ShadowAccounts int64 `json:"shadow_accounts"`
	Tokens         int64 `json:"tokens"`
	BackupTokens   int64 `json:"backup_tokens"`
	Channels       int64 `json:"channels"`
}

// Any reports whether anything names the group.
func (r UserGroupReferences) Any() bool {
	return r.Users+r.ShadowAccounts+r.Tokens+r.BackupTokens+r.Channels > 0
}

// groupColumn is the group column quoted for the database in use ("group" is a reserved word).
func groupColumn() string {
	if common.UsingPostgreSQL {
		return `"group"`
	}
	return "`group`"
}

// ErrUserGroupSuccessor is a successor group that cannot take the deleted group's references.
var ErrUserGroupSuccessor = errors.New("the successor group must be another existing group")

// channelsListingGroup are the channels whose group list names symbol, soft-deleted ones included
// so a restored channel does not come back with a group that is gone.
func channelsListingGroup(tx *gorm.DB, symbol string) ([]*Channel, error) {
	var candidates []*Channel
	if err := tx.Unscoped().Select("id", "group").Where(groupColumn()+" LIKE ?", "%"+symbol+"%").Find(&candidates).Error; err != nil {
		return nil, err
	}
	var out []*Channel
	for _, ch := range candidates {
		if ChannelGroupIsAll(ch.Group) {
			continue
		}
		for _, g := range splitChannelGroup(ch.Group) {
			if g == symbol {
				out = append(out, ch)
				break
			}
		}
	}
	return out, nil
}

// CountUserGroupReferences counts what names the group (soft-deleted rows left out).
func CountUserGroupReferences(symbol string) (UserGroupReferences, error) {
	var r UserGroupReferences
	if err := ExcludeShadowUsers(DB.Model(&User{}).Where(groupColumn()+" = ?", symbol)).Count(&r.Users).Error; err != nil {
		return r, err
	}
	var all int64
	if err := DB.Model(&User{}).Where(groupColumn()+" = ?", symbol).Count(&all).Error; err != nil {
		return r, err
	}
	r.ShadowAccounts = all - r.Users
	if err := DB.Model(&Token{}).Where(groupColumn()+" = ?", symbol).Count(&r.Tokens).Error; err != nil {
		return r, err
	}
	if err := DB.Model(&Token{}).Where("backup_group = ?", symbol).Count(&r.BackupTokens).Error; err != nil {
		return r, err
	}
	var channels []*Channel
	if err := DB.Select("id", "group").Where(groupColumn()+" LIKE ?", "%"+symbol+"%").Find(&channels).Error; err != nil {
		return r, err
	}
	for _, ch := range channels {
		if ChannelGroupIsAll(ch.Group) {
			continue
		}
		for _, g := range splitChannelGroup(ch.Group) {
			if g == symbol {
				r.Channels++
				break
			}
		}
	}
	return r, nil
}

// DeleteUserGroupMoving deletes the group and, in the same transaction, moves what names it to
// successor: accounts and API keys take the successor group, a backup group becomes the successor
// (or none, where that is the key's group), and channels drop the group from their list (a list
// left empty becomes the successor). successor may be empty only when nothing names the group.
// The group caches, the routing table and the caches of the accounts and keys moved are rebuilt.
func DeleteUserGroupMoving(group *UserGroup, successor string) error {
	if group == nil || group.Symbol == "" {
		return errors.New("user group not found")
	}
	symbol := group.Symbol
	successor = strings.TrimSpace(successor)
	if successor != "" {
		var n int64
		if err := DB.Model(&UserGroup{}).Where("symbol = ?", successor).Count(&n).Error; err != nil {
			return err
		}
		if successor == symbol || n == 0 {
			return ErrUserGroupSuccessor
		}
	}

	var userIds []int
	var tokenHashes []string
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Model(&User{}).Where(groupColumn()+" = ?", symbol).Pluck("id", &userIds).Error; err != nil {
			return err
		}
		var tokenIds []int
		tokens := tx.Unscoped().Model(&Token{}).Where(groupColumn()+" = ? OR backup_group = ?", symbol, symbol)
		if err := tokens.Pluck("id", &tokenIds).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Model(&Token{}).Where("id IN ?", append(tokenIds, 0)).Pluck("key_hash", &tokenHashes).Error; err != nil {
			return err
		}
		channels, err := channelsListingGroup(tx, symbol)
		if err != nil {
			return err
		}
		if successor == "" && (len(userIds) > 0 || len(tokenIds) > 0 || len(channels) > 0) {
			return ErrUserGroupSuccessor
		}

		if len(userIds) > 0 {
			if err := tx.Unscoped().Model(&User{}).Where("id IN ?", userIds).Update("group", successor).Error; err != nil {
				return err
			}
		}
		if len(tokenIds) > 0 {
			moved := tx.Unscoped().Model(&Token{}).Where("id IN ?", tokenIds)
			if err := moved.Session(&gorm.Session{}).Where(groupColumn()+" = ?", symbol).Update("group", successor).Error; err != nil {
				return err
			}
			if err := moved.Session(&gorm.Session{}).Where("backup_group = ?", symbol).Update("backup_group", successor).Error; err != nil {
				return err
			}
			// A backup group that is now the key's own group backs up nothing.
			if err := moved.Session(&gorm.Session{}).Where("backup_group = "+groupColumn()).Update("backup_group", "").Error; err != nil {
				return err
			}
		}
		for _, ch := range channels {
			var kept []string
			for _, g := range splitChannelGroup(ch.Group) {
				if g != symbol {
					kept = append(kept, g)
				}
			}
			if len(kept) == 0 {
				kept = []string{successor}
			}
			if err := tx.Unscoped().Model(&Channel{}).Where("id = ?", ch.Id).Update("group", strings.Join(kept, ",")).Error; err != nil {
				return err
			}
		}
		return tx.Delete(group).Error
	})
	if err != nil {
		return err
	}

	GlobalUserGroupRatio.Load()
	ChannelGroup.Load()
	for _, id := range userIds {
		ClearUserGroupAndTokensCache(id)
	}
	for _, hash := range tokenHashes {
		invalidateTokenCache(hash)
	}
	return nil
}
