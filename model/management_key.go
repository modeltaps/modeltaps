package model

import (
	"errors"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
)

// Management keys (AGENT-4) call the management API and MCP for their user, never model APIs.
// They live in their own table rather than in tokens: a tokens row is a billing entity the relay
// accepts, so a management key there would be one missed `type` filter away from calling models.
// They reuse the API key machinery instead: random secret, only the hash and a hint stored, the
// key shown once.
//
// Scopes:
//   - read: balance, usage, logs and the API key list (keys masked);
//   - write: read plus creating, updating, disabling and deleting API keys;
//   - legacy: a user's old system access token, moved here by migration 202611170201. It keeps
//     its old full access for one release as a deprecated path and is then removed.
//
// What each scope may call is the allow-list in middleware/management_key.go; the key always acts
// as its user with the user's current role, so it never exceeds it (organization roles included).
const (
	ManagementKeyScopeRead   = "read"
	ManagementKeyScopeWrite  = "write"
	ManagementKeyScopeLegacy = "legacy"
)

const (
	// ManagementKeyDefaultLifetime is the expiry of a management key created without one.
	ManagementKeyDefaultLifetime = 90 * 24 * time.Hour
	// ManagementKeyMaxLifetime is the longest expiry a management key may have: one year, plus a
	// day so a "1 year" picked in the console is not refused for clock skew or a leap year.
	ManagementKeyMaxLifetime = 366 * 24 * time.Hour
	// managementKeyTouchInterval throttles accessed_time writes to one per key per minute.
	managementKeyTouchInterval = 60
	// LegacyAccessTokenName is the name of a migrated or newly generated legacy access token.
	LegacyAccessTokenName = "Access token (deprecated)"
	// LegacyAccessTokenMaxLifetime caps a legacy access token (SEC-71): a new one expires this
	// long after it is issued, and one stored without an expiry (-1, every migrated token) stops
	// working this long after it was created.
	LegacyAccessTokenMaxLifetime = ManagementKeyMaxLifetime
)

var (
	ErrManagementKeyInvalid = errors.New("invalid management key")
	ErrManagementKeyExpired = errors.New("management key has expired")
)

type ManagementKey struct {
	Id     int    `json:"id"`
	UserId int    `json:"user_id" gorm:"index"`
	Name   string `json:"name" gorm:"type:varchar(64);default:''"`
	Scope  string `json:"scope" gorm:"type:varchar(16);index"`
	// KeyHash is common.HashManagementKey of the key; the key itself is never stored.
	KeyHash string `json:"-" gorm:"type:char(64);uniqueIndex"`
	// KeyHint is the masked key the console lists; it is not a secret.
	KeyHint string `json:"key_hint" gorm:"type:varchar(32);default:''"`
	// Key holds the plaintext key in memory only, right after it was generated, so the request
	// that made it can return it once.
	Key          string `json:"-" gorm:"-"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	AccessedTime int64  `json:"accessed_time" gorm:"bigint;default:0"`
	// ExpiredTime is required for read and write keys; only legacy access tokens have -1 (never).
	ExpiredTime int64 `json:"expired_time" gorm:"bigint"`
}

// BeforeCreate gives a new management key its secret, unless the caller already set Key (legacy
// access tokens, tests), and stores only the hash and the hint.
func (key *ManagementKey) BeforeCreate(tx *gorm.DB) error {
	if key.Key == "" {
		secret, err := common.NewManagementKey()
		if err != nil {
			return err
		}
		key.Key = secret
	}
	key.Key = common.NormalizeManagementKey(key.Key)
	key.KeyHash = common.HashManagementKey(key.Key)
	key.KeyHint = common.ManagementKeyHint(key.Key)
	if key.CreatedTime == 0 {
		key.CreatedTime = utils.GetTimestamp()
	}
	return nil
}

// IsValidManagementKeyScope reports whether scope can be given to a new management key.
func IsValidManagementKeyScope(scope string) bool {
	return scope == ManagementKeyScopeRead || scope == ManagementKeyScopeWrite
}

// ResolveManagementKeyExpiry checks the expiry (Unix seconds) asked for a new management key:
// 0 means the default lifetime, "never" (-1) and past times are refused, and so is anything
// beyond ManagementKeyMaxLifetime.
func ResolveManagementKeyExpiry(requested int64, now time.Time) (int64, error) {
	if requested == 0 {
		return now.Add(ManagementKeyDefaultLifetime).Unix(), nil
	}
	if requested < 0 || requested <= now.Unix() {
		return 0, common.NewAPIError(common.APICodeManagementKeyExpiryRequired,
			"a management key needs an expiry time in the future", nil)
	}
	if requested > now.Add(ManagementKeyMaxLifetime).Unix() {
		return 0, common.NewAPIError(common.APICodeManagementKeyExpiryTooLong,
			"a management key can be valid for at most one year", nil)
	}
	return requested, nil
}

// CreateManagementKey creates a read or write management key for the user. The returned key
// carries the plaintext in Key, to be shown once.
func CreateManagementKey(userId int, name string, scope string, expiredTime int64) (*ManagementKey, error) {
	if !IsValidManagementKeyScope(scope) {
		return nil, common.NewAPIError(common.APICodeManagementKeyScopeInvalid, "invalid management key scope", nil)
	}
	expiry, err := ResolveManagementKeyExpiry(expiredTime, time.Now())
	if err != nil {
		return nil, err
	}
	key := &ManagementKey{UserId: userId, Name: name, Scope: scope, ExpiredTime: expiry}
	if err := DB.Create(key).Error; err != nil {
		return nil, err
	}
	return key, nil
}

// effectiveExpiry is when the key stops working (Unix seconds): its ExpiredTime, except that a
// legacy access token stored without an expiry is capped at LegacyAccessTokenMaxLifetime after
// it was created.
func (key *ManagementKey) effectiveExpiry() int64 {
	if key.ExpiredTime == -1 && key.Scope == ManagementKeyScopeLegacy {
		return key.CreatedTime + int64(LegacyAccessTokenMaxLifetime/time.Second)
	}
	return key.ExpiredTime
}

// ListManagementKeys returns the user's management keys, newest first. A legacy access token
// stored without an expiry is listed with its capped expiry, so the console shows when it stops.
func ListManagementKeys(userId int) ([]*ManagementKey, error) {
	keys := make([]*ManagementKey, 0)
	err := DB.Where("user_id = ?", userId).Order("id DESC").Find(&keys).Error
	for _, key := range keys {
		key.ExpiredTime = key.effectiveExpiry()
	}
	return keys, err
}

// DeleteManagementKey revokes one of the user's management keys at once.
func DeleteManagementKey(id int, userId int) error {
	result := DB.Where("id = ? AND user_id = ?", id, userId).Delete(&ManagementKey{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.NewAPIError(common.APICodeManagementKeyNotFound, "management key not found", nil)
	}
	return nil
}

// ValidateManagementKey finds the management key (or legacy access token) a request presented and
// the user it acts for, read fresh from the database so a disabled, demoted or deleted user takes
// effect at once. Organization shadow accounts never authenticate this way.
func ValidateManagementKey(credential string) (*ManagementKey, *User, error) {
	credential = common.NormalizeManagementKey(credential)
	if credential == "" {
		return nil, nil, ErrManagementKeyInvalid
	}
	var key ManagementKey
	if err := DB.Where("key_hash = ?", common.HashManagementKey(credential)).First(&key).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.SysError("failed to look up a management key: " + err.Error())
		}
		return nil, nil, ErrManagementKeyInvalid
	}
	now := utils.GetTimestamp()
	if key.ExpiredTime == -1 && key.Scope != ManagementKeyScopeLegacy {
		// Only legacy access tokens may be stored without an expiry; anything else is a corrupt row.
		return nil, nil, ErrManagementKeyInvalid
	}
	if key.effectiveExpiry() <= now {
		return nil, nil, ErrManagementKeyExpired
	}
	user := &User{}
	if ExcludeShadowUsers(DB.Omit("password").Where("id = ?", key.UserId)).First(user).Error != nil {
		return nil, nil, ErrManagementKeyInvalid
	}
	if now-key.AccessedTime >= managementKeyTouchInterval {
		DB.Model(&ManagementKey{}).Where("id = ?", key.Id).UpdateColumn("accessed_time", now)
		key.AccessedTime = now
	}
	return &key, user, nil
}

// ValidateAccessToken finds the user of a legacy access token (deprecated, AGENT-4). Management
// keys are not accepted: the only remaining caller binds a Telegram account, which a management
// key must not do.
func ValidateAccessToken(token string) *User {
	key, user, err := ValidateManagementKey(token)
	if err != nil || key.Scope != ManagementKeyScopeLegacy {
		return nil
	}
	return user
}

// IssueLegacyAccessToken replaces the user's legacy access token with a new one and returns it
// once (deprecated: kept for one release so existing integrations can rotate). Only its hash is
// stored, and it expires after LegacyAccessTokenMaxLifetime.
func IssueLegacyAccessToken(userId int) (string, error) {
	token := utils.GetUUID()
	expiry := time.Now().Add(LegacyAccessTokenMaxLifetime).Unix()
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND scope = ?", userId, ManagementKeyScopeLegacy).Delete(&ManagementKey{}).Error; err != nil {
			return err
		}
		return tx.Create(&ManagementKey{
			UserId:      userId,
			Name:        LegacyAccessTokenName,
			Scope:       ManagementKeyScopeLegacy,
			Key:         token,
			ExpiredTime: expiry,
		}).Error
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// RevokeLegacyAccessToken deletes the user's legacy access token, if there is one (SEC-71).
// Changing or resetting the password and signing out other devices call it: the token is not
// tied to a session and has the user's full access, so whoever copied it from a stolen session
// would otherwise keep that access after the user secured the account.
//
// Read and write management keys are deliberately kept: the user creates them one by one for
// automation, they expire within a year, they cannot change the password, sign-in methods,
// sessions or management keys, and the console lists each with its last use, so a user who
// suspects a stolen session reviews and deletes them there instead of having every integration
// break on a routine password change.
func RevokeLegacyAccessToken(userId int) error {
	return DB.Where("user_id = ? AND scope = ?", userId, ManagementKeyScopeLegacy).Delete(&ManagementKey{}).Error
}
