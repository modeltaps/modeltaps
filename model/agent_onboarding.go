package model

import (
	"crypto/subtle"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"

	"gorm.io/gorm"
)

// Handing an API key to an agent without it passing through the user (AGENT-7): pairing codes for
// a key the user created, and the device authorization flow (RFC 8628) for an agent that has none.
//
// Neither path ever stores the plaintext key, not even for the ten minutes a pairing code lives.
// Keys are hash-only (SEC-47), so there is nothing to read back; instead the key material is
// issued at redemption: redeeming a pairing code gives its key a fresh secret and returns that,
// and a device flow approval creates the key while the successful poll gives it the secret it
// returns. The secret shown when the key was created stops working once an agent claims the key,
// so afterwards only the agent holds a working copy.
//
// Both paths only reach the tokens table, so they only ever yield API (call) keys: management
// keys live in their own table and cannot be paired or issued.

const (
	// AgentPairingCodeLifetime is how long a pairing code can be redeemed.
	AgentPairingCodeLifetime = 10 * time.Minute
	// AgentPairingMaxFailures wrong secrets void a pairing code, as wrong guesses void an email
	// verification code (SEC-43, common.VerificationMaxFailures).
	AgentPairingMaxFailures = common.VerificationMaxFailures

	// AgentDeviceCodeLifetime is how long a device authorization waits for the user and the agent.
	AgentDeviceCodeLifetime = 15 * time.Minute
	// AgentDevicePollInterval is the polling interval an agent starts with (RFC 8628 default 5s);
	// each slow_down adds AgentDeviceSlowDownStep, up to AgentDeviceMaxInterval.
	AgentDevicePollInterval = 5
	AgentDeviceSlowDownStep = 5
	AgentDeviceMaxInterval  = 60

	// agentRecordRetention is how long expired pairing codes and device authorizations are kept
	// before the next new one sweeps them. Who claimed a key stays in agent_key_claims.
	agentRecordRetention = 24 * time.Hour
)

const (
	AgentPairingStatusPending  = "pending"
	AgentPairingStatusRedeemed = "redeemed"
	AgentPairingStatusVoid     = "void"

	AgentDeviceStatusPending  = "pending"
	AgentDeviceStatusApproved = "approved"
	AgentDeviceStatusDenied   = "denied"
	AgentDeviceStatusClaimed  = "claimed"

	AgentClaimViaPairingCode = "pairing_code"
	AgentClaimViaDeviceFlow  = "device_flow"
)

var (
	ErrPairingCodeInvalid    = errors.New("invalid pairing code")
	ErrPairingCodeExpired    = errors.New("the pairing code has expired; generate a new one in the API key's details")
	ErrPairingCodeUsed       = errors.New("the pairing code was already used; generate a new one in the API key's details")
	ErrPairingKeyUnavailable = errors.New("the API key of this pairing code is disabled, expired or deleted")

	ErrDeviceAuthPending     = errors.New("the user has not approved the request yet")
	ErrDeviceSlowDown        = errors.New("polling too often; increase the interval by 5 seconds")
	ErrDeviceAccessDenied    = errors.New("the user denied the request")
	ErrDeviceExpired         = errors.New("the device code has expired; start a new authorization")
	ErrDeviceInvalidGrant    = errors.New("invalid or already used device code")
	ErrDeviceUserCodeInvalid = errors.New("this code is invalid, expired or was already answered")
)

// agentNow is the clock of this file; tests move it.
var agentNow = func() int64 { return time.Now().Unix() }

// AgentPairingCode lets an agent claim one API key once, within AgentPairingCodeLifetime.
type AgentPairingCode struct {
	Id      int `json:"id"`
	TokenId int `json:"token_id" gorm:"index"`
	// Lookup is the public part of the code (common.ParsePairingCode); SecretHash the hash of the
	// rest. The code itself is only in the response that created it.
	Lookup      string `json:"-" gorm:"type:varchar(16);uniqueIndex"`
	SecretHash  string `json:"-" gorm:"type:char(64)"`
	CreatedBy   int    `json:"created_by" gorm:"default:0"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	ExpiresAt   int64  `json:"expires_at" gorm:"bigint;index"`
	Failures    int    `json:"failures" gorm:"default:0"`
	Status      string `json:"status" gorm:"type:varchar(16);default:'pending'"`
	// RedeemedTime is when an agent claimed the key; who did is in AgentKeyClaim.
	RedeemedTime int64 `json:"redeemed_time" gorm:"bigint;default:0"`
}

// AgentKeyClaim records an agent receiving an API key: when, from where and through which path.
// The key's details show the latest one ("claimed by an agent") until the key is reset.
type AgentKeyClaim struct {
	Id          int    `json:"-"`
	TokenId     int    `json:"-" gorm:"index"`
	Via         string `json:"via" gorm:"type:varchar(16)"`
	ClientName  string `json:"client_name" gorm:"type:varchar(64);default:''"`
	Ip          string `json:"ip" gorm:"type:varchar(64);default:''"`
	UserAgent   string `json:"user_agent" gorm:"type:varchar(255);default:''"`
	CreatedTime int64  `json:"claimed_at" gorm:"bigint"`
	// Superseded is set when the key is reset: the agent's copy no longer works.
	Superseded bool `json:"-" gorm:"default:false"`
}

// AgentDeviceAuthorization is one run of the device authorization flow.
type AgentDeviceAuthorization struct {
	Id int `json:"-"`
	// DeviceCodeHash is common.HashAgentSecret of the device code the agent polls with.
	DeviceCodeHash string `json:"-" gorm:"type:char(64);uniqueIndex"`
	// UserCode is the normalized user code (common.NormalizeDeviceUserCode).
	UserCode         string `json:"user_code" gorm:"type:varchar(16);index"`
	ClientName       string `json:"client_name" gorm:"type:varchar(64);default:''"`
	RequestIp        string `json:"request_ip" gorm:"type:varchar(64);default:''"`
	RequestUserAgent string `json:"request_user_agent" gorm:"type:varchar(255);default:''"`
	CreatedTime      int64  `json:"created_time" gorm:"bigint"`
	ExpiresAt        int64  `json:"expires_at" gorm:"bigint;index"`
	Interval         int    `json:"-" gorm:"default:5"`
	LastPolledAt     int64  `json:"-" gorm:"bigint;default:0"`
	Status           string `json:"status" gorm:"type:varchar(16);default:'pending'"`
	// Set when the user answers: who, in which organization (0 = personal), and the key created.
	DecidedBy   int   `json:"-" gorm:"default:0"`
	OrgId       int   `json:"-" gorm:"default:0"`
	TokenId     int   `json:"-" gorm:"default:0"`
	DecidedTime int64 `json:"-" gorm:"bigint;default:0"`
}

// truncateRunes keeps at most n characters of s (request headers and client names are untrusted).
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

// ---------------------------------------------------------------- pairing codes

// CreateAgentPairingCode issues a pairing code for the API key tokenId, replacing any pending one
// (a key has at most one live code). The code is returned here only.
func CreateAgentPairingCode(tokenId, createdBy int) (string, int64, error) {
	now := agentNow()
	expiresAt := now + int64(AgentPairingCodeLifetime/time.Second)
	var code string
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := voidAgentPairingCodesTx(tx, tokenId); err != nil {
			return err
		}
		for attempt := 0; ; attempt++ {
			full, lookup, secret, err := common.NewPairingCode()
			if err != nil {
				return err
			}
			row := &AgentPairingCode{
				TokenId:     tokenId,
				Lookup:      lookup,
				SecretHash:  common.HashAgentSecret(secret),
				CreatedBy:   createdBy,
				CreatedTime: now,
				ExpiresAt:   expiresAt,
				Status:      AgentPairingStatusPending,
			}
			// A lookup collision (47 bits) is practically impossible; try again rather than fail.
			if err := tx.Create(row).Error; err != nil {
				if attempt < 3 {
					continue
				}
				return err
			}
			code = full
			return nil
		}
	})
	if err != nil {
		return "", 0, err
	}
	DB.Where("expires_at < ?", now-int64(agentRecordRetention/time.Second)).Delete(&AgentPairingCode{})
	return code, expiresAt, nil
}

func voidAgentPairingCodesTx(tx *gorm.DB, tokenId int) error {
	return tx.Model(&AgentPairingCode{}).
		Where("token_id = ? AND status = ?", tokenId, AgentPairingStatusPending).
		Update("status", AgentPairingStatusVoid).Error
}

// RedeemAgentPairingCode claims the API key of a pairing code: it gives the key a new secret and
// returns the key with it (token.Key, without "sk-"). A wrong secret counts against the code; at
// AgentPairingMaxFailures the code is void. ip and userAgent are recorded as the claim.
func RedeemAgentPairingCode(code, ip, userAgent string) (*Token, error) {
	lookup, secret, ok := common.ParsePairingCode(code)
	if !ok {
		return nil, ErrPairingCodeInvalid
	}
	var row AgentPairingCode
	if err := DB.Where("lookup = ?", lookup).First(&row).Error; err != nil {
		return nil, ErrPairingCodeInvalid
	}
	if subtle.ConstantTimeCompare([]byte(common.HashAgentSecret(secret)), []byte(row.SecretHash)) != 1 {
		recordAgentPairingFailure(row.Id)
		return nil, ErrPairingCodeInvalid
	}
	now := agentNow()
	switch {
	case row.Status == AgentPairingStatusRedeemed:
		return nil, ErrPairingCodeUsed
	case row.Status != AgentPairingStatusPending:
		return nil, ErrPairingCodeInvalid
	case row.ExpiresAt <= now:
		return nil, ErrPairingCodeExpired
	}
	token, err := GetTokenById(row.TokenId)
	if err != nil || !tokenClaimable(token, now) {
		return nil, ErrPairingKeyUnavailable
	}

	var oldHash string
	err = DB.Transaction(func(tx *gorm.DB) error {
		// The conditional update is the single-use guard: of two concurrent redemptions only one
		// moves the code out of pending.
		res := tx.Model(&AgentPairingCode{}).
			Where("id = ? AND status = ? AND expires_at > ?", row.Id, AgentPairingStatusPending, now).
			Updates(map[string]any{"status": AgentPairingStatusRedeemed, "redeemed_time": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrPairingCodeUsed
		}
		var err error
		oldHash, err = claimTokenTx(tx, token, AgentKeyClaim{Via: AgentClaimViaPairingCode, Ip: ip, UserAgent: userAgent}, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	invalidateTokenCache(oldHash)
	return token, nil
}

// recordAgentPairingFailure counts a wrong secret and voids the code at the limit.
func recordAgentPairingFailure(id int) {
	if err := DB.Model(&AgentPairingCode{}).Where("id = ? AND status = ?", id, AgentPairingStatusPending).
		Update("failures", gorm.Expr("failures + 1")).Error; err != nil {
		logger.SysError("failed to count a pairing code failure: " + err.Error())
		return
	}
	if err := DB.Model(&AgentPairingCode{}).
		Where("id = ? AND status = ? AND failures >= ?", id, AgentPairingStatusPending, AgentPairingMaxFailures).
		Update("status", AgentPairingStatusVoid).Error; err != nil {
		logger.SysError("failed to void a pairing code: " + err.Error())
	}
}

// tokenClaimable reports whether an agent may receive this key: it must be enabled and unexpired.
func tokenClaimable(token *Token, now int64) bool {
	if token.Status != config.TokenStatusEnabled {
		return false
	}
	return token.ExpiredTime == -1 || token.ExpiredTime > now
}

// claimTokenTx gives the key a new secret (left in token.Key) and records the claim, superseding
// earlier ones. It returns the previous key hash, whose cache entry the caller drops after commit.
func claimTokenTx(tx *gorm.DB, token *Token, claim AgentKeyClaim, now int64) (string, error) {
	key, err := common.NewUserTokenKey()
	if err != nil {
		return "", err
	}
	hash := common.HashUserTokenKey(key)
	hint := common.UserTokenKeyHint(key)
	res := tx.Model(&Token{}).Where("id = ?", token.Id).UpdateColumns(map[string]any{"key_hash": hash, "key_hint": hint})
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "", ErrPairingKeyUnavailable
	}
	if err := supersedeAgentClaimsTx(tx, token.Id); err != nil {
		return "", err
	}
	claim.TokenId = token.Id
	claim.ClientName = truncateRunes(claim.ClientName, 64)
	claim.Ip = truncateRunes(claim.Ip, 64)
	claim.UserAgent = truncateRunes(claim.UserAgent, 255)
	claim.CreatedTime = now
	if err := tx.Create(&claim).Error; err != nil {
		return "", err
	}
	oldHash := token.KeyHash
	token.KeyHash = hash
	token.KeyHint = hint
	token.Key = key
	return oldHash, nil
}

func supersedeAgentClaimsTx(tx *gorm.DB, tokenId int) error {
	return tx.Model(&AgentKeyClaim{}).Where("token_id = ? AND superseded = ?", tokenId, false).
		Update("superseded", true).Error
}

// forgetAgentHandoverTx runs when a key is reset: its pending pairing code is void (it would hand
// out a working key again) and its claim no longer describes who holds the key.
func forgetAgentHandoverTx(tx *gorm.DB, tokenId int) error {
	if err := voidAgentPairingCodesTx(tx, tokenId); err != nil {
		return err
	}
	return supersedeAgentClaimsTx(tx, tokenId)
}

// AttachAgentInfo fills the agent fields of a key's details: the latest claim, and until when its
// pending pairing code can be redeemed.
func AttachAgentInfo(token *Token) {
	if token == nil || token.Id == 0 {
		return
	}
	var claim AgentKeyClaim
	err := DB.Where("token_id = ? AND superseded = ?", token.Id, false).Order("id desc").Limit(1).Find(&claim).Error
	if err != nil {
		logger.SysError("failed to read the agent claim of an API key: " + err.Error())
	} else if claim.Id > 0 {
		token.AgentClaim = &claim
	}
	var code AgentPairingCode
	err = DB.Where("token_id = ? AND status = ? AND expires_at > ?", token.Id, AgentPairingStatusPending, agentNow()).
		Order("id desc").Limit(1).Find(&code).Error
	if err == nil && code.Id > 0 {
		token.PairingExpiresAt = code.ExpiresAt
	}
}

// ---------------------------------------------------------------- device authorization

// CreateAgentDeviceAuthorization starts a device authorization for an agent. It returns the device
// code (only here) and the stored row, which carries the user code.
func CreateAgentDeviceAuthorization(clientName, ip, userAgent string) (string, *AgentDeviceAuthorization, error) {
	now := agentNow()
	deviceCode, err := common.NewDeviceCode()
	if err != nil {
		return "", nil, err
	}
	row := &AgentDeviceAuthorization{
		DeviceCodeHash:   common.HashAgentSecret(deviceCode),
		ClientName:       truncateRunes(clientName, 64),
		RequestIp:        truncateRunes(ip, 64),
		RequestUserAgent: truncateRunes(userAgent, 255),
		CreatedTime:      now,
		ExpiresAt:        now + int64(AgentDeviceCodeLifetime/time.Second),
		Interval:         AgentDevicePollInterval,
		Status:           AgentDeviceStatusPending,
	}
	// A user code must name one live authorization; with 20^8 codes a clash is rare, try again.
	for attempt := 0; ; attempt++ {
		userCode, err := common.NewDeviceUserCode()
		if err != nil {
			return "", nil, err
		}
		var live int64
		if err := DB.Model(&AgentDeviceAuthorization{}).Where("user_code = ? AND expires_at > ?", userCode, now).Count(&live).Error; err != nil {
			return "", nil, err
		}
		if live == 0 || attempt >= 3 {
			row.UserCode = userCode
			break
		}
	}
	if err := DB.Create(row).Error; err != nil {
		return "", nil, err
	}
	DB.Where("expires_at < ?", now-int64(agentRecordRetention/time.Second)).Delete(&AgentDeviceAuthorization{})
	return deviceCode, row, nil
}

// GetPendingAgentDeviceAuthorization finds the live, unanswered authorization of a user code.
func GetPendingAgentDeviceAuthorization(userCode string) (*AgentDeviceAuthorization, error) {
	code := common.NormalizeDeviceUserCode(userCode)
	if code == "" {
		return nil, ErrDeviceUserCodeInvalid
	}
	var row AgentDeviceAuthorization
	err := DB.Where("user_code = ? AND status = ? AND expires_at > ?", code, AgentDeviceStatusPending, agentNow()).
		Order("id desc").First(&row).Error
	if err != nil {
		return nil, ErrDeviceUserCodeInvalid
	}
	return &row, nil
}

// AgentDeviceStatusExpired is not stored: AnswerStatus reports it for an approval whose agent did
// not come for the key in time.
const AgentDeviceStatusExpired = "expired"

// AnswerStatus is the state of an answered authorization as the confirmation page shows it.
func (row *AgentDeviceAuthorization) AnswerStatus() string {
	if row.Status == AgentDeviceStatusApproved && row.ExpiresAt <= agentNow() {
		return AgentDeviceStatusExpired
	}
	return row.Status
}

// GetAnsweredAgentDeviceAuthorization finds the newest authorization of a user code that decidedBy
// answered, for the confirmation page to follow it after the answer, and the claim of its key once
// the agent received it (nil before).
func GetAnsweredAgentDeviceAuthorization(userCode string, decidedBy int) (*AgentDeviceAuthorization, *AgentKeyClaim, error) {
	code := common.NormalizeDeviceUserCode(userCode)
	if code == "" || decidedBy == 0 {
		return nil, nil, ErrDeviceUserCodeInvalid
	}
	var row AgentDeviceAuthorization
	err := DB.Where("user_code = ? AND decided_by = ? AND status <> ?", code, decidedBy, AgentDeviceStatusPending).
		Order("id desc").First(&row).Error
	if err != nil {
		return nil, nil, ErrDeviceUserCodeInvalid
	}
	if row.Status != AgentDeviceStatusClaimed || row.TokenId == 0 {
		return &row, nil, nil
	}
	var claim AgentKeyClaim
	if err := DB.Where("token_id = ? AND via = ?", row.TokenId, AgentClaimViaDeviceFlow).Order("id desc").Limit(1).Find(&claim).Error; err != nil {
		return nil, nil, err
	}
	if claim.Id == 0 {
		return &row, nil, nil
	}
	return &row, &claim, nil
}

// ApproveAgentDeviceAuthorization answers a pending authorization with yes: it creates the API key
// (token, prepared by the caller with the user's choices) and binds it to the authorization. The
// key's secret is only issued to the agent's next poll, so the plaintext never exists before.
func ApproveAgentDeviceAuthorization(row *AgentDeviceAuthorization, decidedBy, orgId int, token *Token) error {
	now := agentNow()
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(token).Error; err != nil {
			return err
		}
		// Whatever secret BeforeCreate generated is dropped here: nobody has seen it.
		token.Key = ""
		res := tx.Model(&AgentDeviceAuthorization{}).
			Where("id = ? AND status = ? AND expires_at > ?", row.Id, AgentDeviceStatusPending, now).
			Updates(map[string]any{
				"status":       AgentDeviceStatusApproved,
				"decided_by":   decidedBy,
				"org_id":       orgId,
				"token_id":     token.Id,
				"decided_time": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrDeviceUserCodeInvalid
		}
		return nil
	})
}

// DenyAgentDeviceAuthorization answers a pending authorization with no.
func DenyAgentDeviceAuthorization(row *AgentDeviceAuthorization, decidedBy int) error {
	now := agentNow()
	res := DB.Model(&AgentDeviceAuthorization{}).
		Where("id = ? AND status = ? AND expires_at > ?", row.Id, AgentDeviceStatusPending, now).
		Updates(map[string]any{"status": AgentDeviceStatusDenied, "decided_by": decidedBy, "decided_time": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDeviceUserCodeInvalid
	}
	return nil
}

// PollAgentDeviceAuthorization is the agent's poll with its device code. Until the user answers
// it returns ErrDeviceAuthPending (ErrDeviceSlowDown when polled faster than the interval, which
// then grows); after a yes it returns the new API key with its secret (token.Key, without "sk-"),
// once. The returned row says which organization (if any) the key belongs to.
func PollAgentDeviceAuthorization(deviceCode, ip, userAgent string) (*Token, *AgentDeviceAuthorization, error) {
	if deviceCode == "" {
		return nil, nil, ErrDeviceInvalidGrant
	}
	var row AgentDeviceAuthorization
	if err := DB.Where("device_code_hash = ?", common.HashAgentSecret(deviceCode)).First(&row).Error; err != nil {
		return nil, nil, ErrDeviceInvalidGrant
	}
	now := agentNow()
	if row.Status == AgentDeviceStatusClaimed {
		return nil, nil, ErrDeviceInvalidGrant
	}
	if row.ExpiresAt <= now {
		return nil, nil, ErrDeviceExpired
	}
	if row.Status == AgentDeviceStatusDenied {
		return nil, nil, ErrDeviceAccessDenied
	}

	interval := row.Interval
	if interval <= 0 {
		interval = AgentDevicePollInterval
	}
	tooSoon := row.LastPolledAt > 0 && now-row.LastPolledAt < int64(interval)
	updates := map[string]any{"last_polled_at": now}
	if tooSoon {
		updates["interval"] = min(interval+AgentDeviceSlowDownStep, AgentDeviceMaxInterval)
	}
	if err := DB.Model(&AgentDeviceAuthorization{}).Where("id = ?", row.Id).Updates(updates).Error; err != nil {
		return nil, nil, err
	}
	if tooSoon {
		return nil, nil, ErrDeviceSlowDown
	}
	if row.Status != AgentDeviceStatusApproved {
		return nil, nil, ErrDeviceAuthPending
	}

	token, err := GetTokenById(row.TokenId)
	if err != nil || !tokenClaimable(token, now) {
		return nil, nil, ErrDeviceAccessDenied
	}
	var oldHash string
	err = DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&AgentDeviceAuthorization{}).
			Where("id = ? AND status = ?", row.Id, AgentDeviceStatusApproved).
			Update("status", AgentDeviceStatusClaimed)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrDeviceInvalidGrant
		}
		var err error
		oldHash, err = claimTokenTx(tx, token, AgentKeyClaim{
			Via: AgentClaimViaDeviceFlow, ClientName: row.ClientName, Ip: ip, UserAgent: userAgent,
		}, now)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	invalidateTokenCache(oldHash)
	return token, &row, nil
}

const (
	AgentKeyAccountPersonal     = "personal"
	AgentKeyAccountOrganization = "organization"
)

// AgentKeyAccount is the account an API key handed to an agent belongs to. The agent shows it, so
// the user can tell when someone else approved their code (SEC-73): a leaked user code approved
// first by another account yields that account's key, which reads the agent's traffic in its logs.
type AgentKeyAccount struct {
	Type string `json:"type"`
	// Name is the username of a personal key's owner, or the organization's name.
	Name string `json:"name"`
	// Slug identifies an organization (names need not be unique); empty for a personal key.
	Slug string `json:"slug,omitempty"`
}

// AgentKeyAccountOf names the account of token: the organization when its owner is an
// organization's shadow account, the owning user otherwise.
func AgentKeyAccountOf(token *Token) AgentKeyAccount {
	var org Organization
	if err := DB.Select("id", "name", "slug").Where("shadow_user_id = ?", token.UserId).Limit(1).Find(&org).Error; err != nil {
		logger.SysError("failed to read the organization of an API key: " + err.Error())
	} else if org.Id > 0 {
		return AgentKeyAccount{Type: AgentKeyAccountOrganization, Name: org.Name, Slug: org.Slug}
	}
	return AgentKeyAccount{Type: AgentKeyAccountPersonal, Name: GetUsernameById(token.UserId)}
}

// OrganizationIdOfShadowUser returns the organization whose shadow account userId is, or 0.
func OrganizationIdOfShadowUser(userId int) int {
	var org Organization
	if err := DB.Select("id").Where("shadow_user_id = ?", userId).Limit(1).Find(&org).Error; err != nil {
		return 0
	}
	return org.Id
}
