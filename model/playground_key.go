package model

import (
	"errors"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
)

// The console playground calls the relay (/v1/...) from the browser, which needs a bearer key.
// Since API keys can no longer be read back (SEC-47), the signed-in user gets a short-lived
// playground key instead (POST /api/token/playground): a random secret stored only as a hash,
// valid for PlaygroundKeyTTL, that authenticates as the user's personal playground API key
// (PlaygroundTokenName). It spends and is limited exactly like that key; deleting the playground
// key or disabling the user stops every playground key at once.

// PlaygroundTokenName is the name of the personal API key the playground spends on. The console
// recognizes it by name and does not offer to edit it; users cannot take sys_ names.
const PlaygroundTokenName = "sys_playground"

// PlaygroundKeyTTL is how long a playground key stays valid.
const PlaygroundKeyTTL = time.Hour

// maxLivePlaygroundKeys caps the live playground keys of one user (each console tab takes one);
// issuing more drops the oldest.
const maxLivePlaygroundKeys = 20

type PlaygroundKey struct {
	Id        int    `json:"id"`
	UserId    int    `json:"user_id" gorm:"index"`
	TokenId   int    `json:"token_id" gorm:"index"`
	KeyHash   string `json:"-" gorm:"type:char(64);uniqueIndex"`
	ExpiresAt int64  `json:"expires_at" gorm:"bigint;index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;autoCreateTime"`
}

// EnsurePlaygroundToken returns the user's playground API key, creating it on first use.
func EnsurePlaygroundToken(userId int) (*Token, error) {
	token, err := GetTokenByName(PlaygroundTokenName, userId)
	if err == nil {
		return token, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := utils.GetTimestamp()
	token = &Token{
		UserId:         userId,
		Name:           PlaygroundTokenName,
		CreatedTime:    now,
		AccessedTime:   now,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	if err := token.Insert(); err != nil {
		return nil, err
	}
	// Its own key is random and never shown: the playground only uses playground keys.
	token.Key = ""
	return token, nil
}

// IssuePlaygroundKey creates a playground key for the user and returns it (without "sk-") with
// its expiry (Unix seconds).
func IssuePlaygroundKey(userId int) (string, int64, error) {
	token, err := EnsurePlaygroundToken(userId)
	if err != nil {
		return "", 0, err
	}
	now := time.Now()
	if err := DB.Where("expires_at <= ?", now.Unix()).Delete(&PlaygroundKey{}).Error; err != nil {
		logger.SysError("failed to delete expired playground keys: " + err.Error())
	}
	var stale []int
	if err := DB.Model(&PlaygroundKey{}).Where("user_id = ?", userId).Order("id DESC").
		Offset(maxLivePlaygroundKeys-1).Pluck("id", &stale).Error; err == nil && len(stale) > 0 {
		DB.Where("id IN ?", stale).Delete(&PlaygroundKey{})
	}

	key, err := common.NewPlaygroundKey()
	if err != nil {
		return "", 0, err
	}
	row := &PlaygroundKey{
		UserId:    userId,
		TokenId:   token.Id,
		KeyHash:   common.HashUserTokenKey(key),
		ExpiresAt: now.Add(PlaygroundKeyTTL).Unix(),
	}
	if err := DB.Create(row).Error; err != nil {
		return "", 0, err
	}
	return key, row.ExpiresAt, nil
}

// getPlaygroundTokenModel resolves a playground key to the user's playground API key.
func getPlaygroundTokenModel(key string) (*Token, error) {
	var row PlaygroundKey
	if err := DB.Where("key_hash = ?", common.HashUserTokenKey(key)).First(&row).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.SysError("failed to look up playground key: " + err.Error())
		}
		return nil, ErrTokenInvalid
	}
	if row.ExpiresAt <= time.Now().Unix() {
		return nil, ErrTokenExpired
	}
	token, err := GetTokenById(row.TokenId)
	if err != nil || token.UserId != row.UserId || token.Name != PlaygroundTokenName {
		return nil, ErrTokenInvalid
	}
	if userEnabled, err := CacheIsUserEnabled(token.UserId); err != nil || !userEnabled {
		return nil, ErrTokenInvalid
	}
	return token, nil
}

// deletePlaygroundKeysOf drops the playground keys that spend on this API key.
func deletePlaygroundKeysOf(token *Token) {
	if token == nil || token.Name != PlaygroundTokenName || !DB.Migrator().HasTable(&PlaygroundKey{}) {
		return
	}
	if err := DB.Where("token_id = ?", token.Id).Delete(&PlaygroundKey{}).Error; err != nil {
		logger.SysError("failed to delete playground keys: " + err.Error())
	}
}
