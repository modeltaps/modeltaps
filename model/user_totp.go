package model

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
)

// Two-factor authentication with an authenticator app (TOTP, RFC 6238: HMAC-SHA1, 6 digits,
// 30-second steps, one step of clock drift either way) and ten single-use recovery codes. It is
// optional per user and asked for after a correct password or email code; passkeys and identity
// provider sign-ins are not affected (a passkey is already two factors, an identity provider runs
// its own MFA).
//
// The data lives in its own tables rather than in columns on users: users is read on every
// request and cached, and keeping the secret out of it means no user query or JSON can carry it.
// The secret is encrypted at rest (serializer:encrypted, label "user_totps.secret"); recovery
// codes are kept as SHA-256 hashes only (80 random bits each, so a hash cannot be reversed by
// guessing).
//
// The pending sign-in (password checked, second factor not yet) is kept on the user's row: the
// cookie only holds a random challenge token, the row its hash, expiry and failure count, so the
// state cannot be forged on the client and works across nodes.

const (
	TotpDigits = 6
	TotpPeriod = 30 // seconds
	TotpSkew   = 1  // steps accepted either side of now

	TotpRecoveryCodeCount = 10

	// TotpChallengeTTL is how long the second sign-in step stays open after the password.
	TotpChallengeTTL = 5 * time.Minute
	// TotpChallengeMaxFailures wrong codes end the pending sign-in; the password is asked again.
	TotpChallengeMaxFailures = 5

	// totpSetupTTL is how long a secret shown for enrollment can still be confirmed.
	totpSetupTTL = 15 * time.Minute

	totpSecretBytes       = 20 // 160 bits, the size RFC 4226 recommends
	totpRecoveryCodeBytes = 10 // 80 bits, 16 base32 characters
)

var (
	ErrTotpCodeInvalid      = common.NewAPIError(common.APICodeTotpCodeInvalid, "Two-factor code is incorrect or has already been used", nil)
	ErrTotpChallengeExpired = common.NewAPIError(common.APICodeTotpChallengeExpired, "Two-factor sign-in has expired, please sign in again", nil)
	ErrTotpAlreadyEnabled   = common.NewAPIError(common.APICodeTotpAlreadyEnabled, "Two-factor authentication is already enabled", nil)
	ErrTotpNotEnabled       = common.NewAPIError(common.APICodeTotpNotEnabled, "Two-factor authentication is not enabled", nil)
	ErrTotpSetupRequired    = common.NewAPIError(common.APICodeTotpSetupRequired, "Two-factor setup has expired, please start again", nil)
)

// UserTotp is a user's authenticator secret, from the start of enrollment on. Enabled is false
// until the user confirms a code; only enabled rows are asked for at sign-in.
type UserTotp struct {
	Id     int `json:"-"`
	UserId int `json:"-" gorm:"not null;uniqueIndex:idx_user_totps_user"`
	// Secret is the base32 shared secret. Write it with a struct, never a map (see secret_storage.go).
	Secret      string `json:"-" gorm:"type:text;serializer:encrypted"`
	Enabled     bool   `json:"enabled" gorm:"not null;default:false"`
	CreatedTime int64  `json:"-" gorm:"bigint"`
	EnabledTime int64  `json:"enabled_time" gorm:"bigint"`
	// LastUsedStep is the time step of the last accepted code: a code is accepted only for a later
	// step, so each code works once, even within its 30 seconds and across nodes.
	LastUsedStep int64 `json:"-" gorm:"bigint;not null;default:0"`
	// The pending sign-in: SHA-256 of the cookie's challenge token, its expiry and wrong codes.
	ChallengeHash      string `json:"-" gorm:"type:varchar(64);not null;default:''"`
	ChallengeExpiresAt int64  `json:"-" gorm:"bigint;not null;default:0"`
	ChallengeFailures  int    `json:"-" gorm:"not null;default:0"`
}

// UserTotpRecoveryCode is one single-use recovery code, stored as a hash.
type UserTotpRecoveryCode struct {
	Id          int    `json:"-"`
	UserId      int    `json:"-" gorm:"not null;index"`
	CodeHash    string `json:"-" gorm:"type:varchar(64);not null;index"`
	UsedTime    int64  `json:"-" gorm:"bigint;not null;default:0"`
	CreatedTime int64  `json:"-" gorm:"bigint"`
}

// ---------- RFC 4226 / RFC 6238 ----------

// hotp is the RFC 4226 one-time password of key at counter, with digits digits.
func hotp(key []byte, counter uint64, digits int) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, value%mod)
}

// totpStep is the RFC 6238 time step of t.
func totpStep(t time.Time) int64 {
	return t.Unix() / TotpPeriod
}

var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// decodeTotpSecret reads a base32 secret, ignoring case, spaces and padding.
func decodeTotpSecret(secret string) ([]byte, error) {
	cleaned := strings.ToUpper(strings.NewReplacer(" ", "", "-", "", "=", "").Replace(secret))
	if cleaned == "" {
		return nil, errors.New("empty TOTP secret")
	}
	return totpEncoding.DecodeString(cleaned)
}

// matchTotpStep returns the latest step within TotpSkew of now whose code is code.
func matchTotpStep(secret string, code string, now time.Time) (int64, bool) {
	key, err := decodeTotpSecret(secret)
	if err != nil || len(code) != TotpDigits {
		return 0, false
	}
	current := totpStep(now)
	for step := current + TotpSkew; step >= current-TotpSkew; step-- {
		if step < 0 {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(hotp(key, uint64(step), TotpDigits)), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

// TotpCodeAt is the code an authenticator shows for secret at t.
func TotpCodeAt(secret string, t time.Time) (string, error) {
	key, err := decodeTotpSecret(secret)
	if err != nil {
		return "", err
	}
	return hotp(key, uint64(totpStep(t)), TotpDigits), nil
}

// ---------- codes as typed ----------

// normalizeTotpInput strips the spaces and dashes people type or paste with a code.
func normalizeTotpInput(code string) string {
	return strings.ToUpper(strings.NewReplacer(" ", "", "-", "", "\t", "").Replace(strings.TrimSpace(code)))
}

func isTotpDigits(code string) bool {
	if len(code) != TotpDigits {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func newTotpSecret() (string, error) {
	buf := make([]byte, totpSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return totpEncoding.EncodeToString(buf), nil
}

// newRecoveryCode returns a code shown as XXXX-XXXX-XXXX-XXXX.
func newRecoveryCode() (string, error) {
	buf := make([]byte, totpRecoveryCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	raw := totpEncoding.EncodeToString(buf)
	return raw[0:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:16], nil
}

func recoveryCodeHash(userId int, code string) string {
	sum := sha256.Sum256([]byte("totp-recovery:" + strconv.Itoa(userId) + ":" + normalizeTotpInput(code)))
	return hex.EncodeToString(sum[:])
}

func totpChallengeHash(token string) string {
	sum := sha256.Sum256([]byte("totp-challenge:" + token))
	return hex.EncodeToString(sum[:])
}

// ---------- reads ----------

// GetUserTotp returns the user's row (enabled or in setup), or gorm.ErrRecordNotFound.
func GetUserTotp(userId int) (*UserTotp, error) {
	var row UserTotp
	if err := DB.Where("user_id = ?", userId).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// UserTotpEnabled reports whether sign-in asks the user for a second factor. Callers on the
// sign-in path must fail closed on an error rather than skip the second factor.
func UserTotpEnabled(userId int) (bool, error) {
	var count int64
	err := DB.Model(&UserTotp{}).Where("user_id = ? AND enabled = ?", userId, true).Count(&count).Error
	return count > 0, err
}

// UserTotpRecoveryCodesLeft counts the unused recovery codes.
func UserTotpRecoveryCodesLeft(userId int) (int64, error) {
	var count int64
	err := DB.Model(&UserTotpRecoveryCode{}).Where("user_id = ? AND used_time = 0", userId).Count(&count).Error
	return count, err
}

// ---------- enrollment ----------

// BeginUserTotpSetup draws a new secret for the user to scan and returns it. It replaces a setup
// that was never confirmed and refuses while two-factor authentication is on.
func BeginUserTotpSetup(userId int) (string, error) {
	secret, err := newTotpSecret()
	if err != nil {
		return "", err
	}
	now := utils.GetTimestamp()
	existing, err := GetUserTotp(userId)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if existing == nil {
		return secret, DB.Create(&UserTotp{UserId: userId, Secret: secret, CreatedTime: now}).Error
	}
	if existing.Enabled {
		return "", ErrTotpAlreadyEnabled
	}
	result := DB.Model(&UserTotp{}).Where("id = ? AND enabled = ?", existing.Id, false).
		Select("secret", "created_time", "last_used_step").
		Updates(&UserTotp{Secret: secret, CreatedTime: now, LastUsedStep: 0})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", ErrTotpAlreadyEnabled
	}
	return secret, nil
}

// EnableUserTotp confirms the setup with a code from the app, turns two-factor authentication on
// and returns the recovery codes, which are shown once and stored only as hashes.
func EnableUserTotp(userId int, code string, now time.Time) ([]string, error) {
	row, err := GetUserTotp(userId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTotpSetupRequired
	}
	if err != nil {
		return nil, err
	}
	if row.Enabled {
		return nil, ErrTotpAlreadyEnabled
	}
	if now.Sub(time.Unix(row.CreatedTime, 0)) > totpSetupTTL {
		return nil, ErrTotpSetupRequired
	}
	step, ok := matchTotpStep(row.Secret, normalizeTotpInput(code), now)
	if !ok {
		return nil, ErrTotpCodeInvalid
	}
	var codes []string
	err = DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&UserTotp{}).Where("id = ? AND enabled = ?", row.Id, false).
			Updates(map[string]any{"enabled": true, "enabled_time": now.Unix(), "last_used_step": step})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrTotpAlreadyEnabled
		}
		var err error
		codes, err = replaceRecoveryCodes(tx, userId, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// RegenerateUserTotpRecoveryCodes replaces every recovery code with ten new ones.
func RegenerateUserTotpRecoveryCodes(userId int, now time.Time) ([]string, error) {
	var codes []string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&UserTotp{}).Where("user_id = ? AND enabled = ?", userId, true).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrTotpNotEnabled
		}
		var err error
		codes, err = replaceRecoveryCodes(tx, userId, now)
		return err
	})
	return codes, err
}

func replaceRecoveryCodes(tx *gorm.DB, userId int, now time.Time) ([]string, error) {
	if err := tx.Where("user_id = ?", userId).Delete(&UserTotpRecoveryCode{}).Error; err != nil {
		return nil, err
	}
	codes := make([]string, 0, TotpRecoveryCodeCount)
	rows := make([]UserTotpRecoveryCode, 0, TotpRecoveryCodeCount)
	for i := 0; i < TotpRecoveryCodeCount; i++ {
		code, err := newRecoveryCode()
		if err != nil {
			return nil, err
		}
		codes = append(codes, code)
		rows = append(rows, UserTotpRecoveryCode{UserId: userId, CodeHash: recoveryCodeHash(userId, code), CreatedTime: now.Unix()})
	}
	if err := tx.Create(&rows).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

// DeleteUserTotp turns two-factor authentication off (or drops an unfinished setup) and removes
// the recovery codes. Callers check a code first, except root resetting another user.
func DeleteUserTotp(userId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userId).Delete(&UserTotpRecoveryCode{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userId).Delete(&UserTotp{}).Error
	})
}

// ---------- verification ----------

// VerifyUserTotp checks a code from the app or a recovery code and uses it up: an app code is
// not accepted again (nor any earlier one), a recovery code only once. recovery says which kind
// matched.
func VerifyUserTotp(userId int, code string, now time.Time) (recovery bool, err error) {
	row, err := GetUserTotp(userId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, ErrTotpNotEnabled
	}
	if err != nil {
		return false, err
	}
	if !row.Enabled {
		return false, ErrTotpNotEnabled
	}
	input := normalizeTotpInput(code)
	if input == "" {
		return false, ErrTotpCodeInvalid
	}
	if isTotpDigits(input) {
		step, ok := matchTotpStep(row.Secret, input, now)
		if !ok {
			return false, ErrTotpCodeInvalid
		}
		// The condition makes the check and the use one step across concurrent requests and nodes.
		result := DB.Model(&UserTotp{}).Where("id = ? AND last_used_step < ?", row.Id, step).
			Update("last_used_step", step)
		if result.Error != nil {
			return false, result.Error
		}
		if result.RowsAffected == 0 {
			return false, ErrTotpCodeInvalid
		}
		return false, nil
	}
	result := DB.Model(&UserTotpRecoveryCode{}).
		Where("user_id = ? AND code_hash = ? AND used_time = 0", userId, recoveryCodeHash(userId, input)).
		Update("used_time", now.Unix())
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, ErrTotpCodeInvalid
	}
	return true, nil
}

// ---------- pending sign-in ----------

// StartUserTotpChallenge opens the second sign-in step and returns the token for the cookie. A
// new challenge replaces the previous one.
func StartUserTotpChallenge(userId int, now time.Time) (string, error) {
	token := utils.GetSecureRandomString(43)
	result := DB.Model(&UserTotp{}).Where("user_id = ? AND enabled = ?", userId, true).Updates(map[string]any{
		"challenge_hash":       totpChallengeHash(token),
		"challenge_expires_at": now.Add(TotpChallengeTTL).Unix(),
		"challenge_failures":   0,
	})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", ErrTotpNotEnabled
	}
	return token, nil
}

// CheckUserTotpChallenge reports whether token is the user's open challenge: issued for this
// user, not expired, and with fewer than TotpChallengeMaxFailures wrong codes.
func CheckUserTotpChallenge(userId int, token string, now time.Time) bool {
	if userId <= 0 || token == "" {
		return false
	}
	row, err := GetUserTotp(userId)
	if err != nil || !row.Enabled || row.ChallengeHash == "" {
		return false
	}
	if now.Unix() >= row.ChallengeExpiresAt || row.ChallengeFailures >= TotpChallengeMaxFailures {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(row.ChallengeHash), []byte(totpChallengeHash(token))) == 1
}

// RecordUserTotpChallengeFailure counts a wrong code against the open challenge.
func RecordUserTotpChallengeFailure(userId int) error {
	return DB.Model(&UserTotp{}).Where("user_id = ?", userId).
		Update("challenge_failures", gorm.Expr("challenge_failures + 1")).Error
}

// ClearUserTotpChallenge closes the second sign-in step.
func ClearUserTotpChallenge(userId int) error {
	return DB.Model(&UserTotp{}).Where("user_id = ?", userId).Updates(map[string]any{
		"challenge_hash": "", "challenge_expires_at": 0, "challenge_failures": 0,
	}).Error
}
