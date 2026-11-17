package model

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
)

// AGENT-7: pairing codes and the device authorization flow hand an API key to an agent once,
// without the plaintext key ever being stored.

// setAgentClock pins agentNow for a test; the returned func moves it.
func setAgentClock(t *testing.T) func(seconds int64) {
	t.Helper()
	now := time.Now().Unix()
	prev := agentNow
	agentNow = func() int64 { return now }
	t.Cleanup(func() { agentNow = prev })
	return func(seconds int64) { now += seconds }
}

// keyWorks reports whether key (without "sk-") is the current secret of the API key tokenId.
func keyWorks(t *testing.T, tokenId int, key string) bool {
	t.Helper()
	found, err := GetTokenByKeyHash(common.HashUserTokenKey(key))
	return err == nil && found.Id == tokenId
}

func TestPairingCodeRedeemsOnceAndRotatesTheSecret(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	setAgentClock(t)
	token := newTestToken(t, user.Id, "app")
	shownAtCreation := token.Key

	code, expiresAt, err := CreateAgentPairingCode(token.Id, user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, common.PairingCodePrefix) || expiresAt != agentNow()+int64(AgentPairingCodeLifetime/time.Second) {
		t.Fatalf("unexpected code %q expiring at %d", code, expiresAt)
	}
	var stored AgentPairingCode
	DB.First(&stored, "token_id = ?", token.Id)
	_, secret, _ := common.ParsePairingCode(code)
	if stored.SecretHash != common.HashAgentSecret(secret) || strings.Contains(stored.Lookup+stored.SecretHash, secret) {
		t.Fatal("the code secret must only be stored as its hash")
	}

	claimed, err := RedeemAgentPairingCode(code, "203.0.113.7", "openclaw/1.0")
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if claimed.Id != token.Id || claimed.Key == "" || claimed.Key == shownAtCreation {
		t.Fatalf("redeeming must return the key with a fresh secret, got %+v", claimed)
	}
	if !keyWorks(t, token.Id, claimed.Key) || keyWorks(t, token.Id, shownAtCreation) {
		t.Fatal("after redemption only the agent's secret may work")
	}

	// Replay.
	if _, err := RedeemAgentPairingCode(code, "203.0.113.7", "openclaw/1.0"); !errors.Is(err, ErrPairingCodeUsed) {
		t.Fatalf("second redemption: want ErrPairingCodeUsed, got %v", err)
	}
	if !keyWorks(t, token.Id, claimed.Key) {
		t.Fatal("a refused replay must not change the key")
	}

	// Audit: time, IP and User-Agent, shown in the key's details.
	details, _ := GetTokenById(token.Id)
	AttachAgentInfo(details)
	if details.AgentClaim == nil || details.AgentClaim.Via != AgentClaimViaPairingCode || details.AgentClaim.Ip != "203.0.113.7" ||
		details.AgentClaim.UserAgent != "openclaw/1.0" || details.AgentClaim.CreatedTime != agentNow() {
		t.Fatalf("claim not recorded: %+v", details.AgentClaim)
	}
	if details.PairingExpiresAt != 0 {
		t.Fatal("a redeemed code is no longer pending")
	}
}

func TestPairingCodeExpires(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	advance := setAgentClock(t)
	token := newTestToken(t, user.Id, "app")
	code, _, err := CreateAgentPairingCode(token.Id, user.Id)
	if err != nil {
		t.Fatal(err)
	}
	advance(int64(AgentPairingCodeLifetime/time.Second) - 1)
	details, _ := GetTokenById(token.Id)
	AttachAgentInfo(details)
	if details.PairingExpiresAt == 0 {
		t.Fatal("the pending code should be shown until it expires")
	}
	advance(1)
	if _, err := RedeemAgentPairingCode(code, "", ""); !errors.Is(err, ErrPairingCodeExpired) {
		t.Fatalf("want ErrPairingCodeExpired, got %v", err)
	}
	if !keyWorks(t, token.Id, token.Key) {
		t.Fatal("an expired code must not touch the key")
	}
}

func TestPairingCodeVoidAfterFailures(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	setAgentClock(t)
	token := newTestToken(t, user.Id, "app")
	code, _, err := CreateAgentPairingCode(token.Id, user.Id)
	if err != nil {
		t.Fatal(err)
	}
	lookup, secret, _ := common.ParsePairingCode(code)
	wrong := common.PairingCodePrefix + lookup + strings.Repeat("x", len(secret))
	for i := 0; i < AgentPairingMaxFailures; i++ {
		if _, err := RedeemAgentPairingCode(wrong, "", ""); !errors.Is(err, ErrPairingCodeInvalid) {
			t.Fatalf("wrong secret %d: want ErrPairingCodeInvalid, got %v", i, err)
		}
	}
	if _, err := RedeemAgentPairingCode(code, "", ""); !errors.Is(err, ErrPairingCodeInvalid) {
		t.Fatalf("after %d failures the right code must be void too, got %v", AgentPairingMaxFailures, err)
	}
	for _, malformed := range []string{"", "pair-", "pair-short", code + "x", strings.Repeat("!", 40)} {
		if _, err := RedeemAgentPairingCode(malformed, "", ""); !errors.Is(err, ErrPairingCodeInvalid) {
			t.Fatalf("malformed %q: want ErrPairingCodeInvalid, got %v", malformed, err)
		}
	}
}

func TestPairingCodeFewerFailuresStillRedeem(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	setAgentClock(t)
	token := newTestToken(t, user.Id, "app")
	code, _, _ := CreateAgentPairingCode(token.Id, user.Id)
	lookup, secret, _ := common.ParsePairingCode(code)
	for i := 0; i < AgentPairingMaxFailures-1; i++ {
		_, _ = RedeemAgentPairingCode(lookup+strings.Repeat("y", len(secret)), "", "")
	}
	if _, err := RedeemAgentPairingCode(code, "", ""); err != nil {
		t.Fatalf("below the failure limit the code still works, got %v", err)
	}
}

func TestResetKeyVoidsPairingCodeAndClaim(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	setAgentClock(t)
	token := newTestToken(t, user.Id, "app")

	// A claim from an earlier pairing, then a new pending code.
	first, _, _ := CreateAgentPairingCode(token.Id, user.Id)
	if _, err := RedeemAgentPairingCode(first, "198.51.100.1", "agent"); err != nil {
		t.Fatal(err)
	}
	pending, _, _ := CreateAgentPairingCode(token.Id, user.Id)

	fresh, _ := GetTokenById(token.Id)
	if _, err := fresh.ResetKey(); err != nil {
		t.Fatal(err)
	}
	if _, err := RedeemAgentPairingCode(pending, "", ""); !errors.Is(err, ErrPairingCodeInvalid) {
		t.Fatalf("a reset key's pending code must be void, got %v", err)
	}
	if !keyWorks(t, token.Id, fresh.Key) {
		t.Fatal("the reset key must keep working")
	}
	details, _ := GetTokenById(token.Id)
	AttachAgentInfo(details)
	if details.AgentClaim != nil || details.PairingExpiresAt != 0 {
		t.Fatalf("after a reset no agent holds the key: %+v", details)
	}
}

func TestNewPairingCodeReplacesPendingOne(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	setAgentClock(t)
	token := newTestToken(t, user.Id, "app")
	old, _, _ := CreateAgentPairingCode(token.Id, user.Id)
	current, _, _ := CreateAgentPairingCode(token.Id, user.Id)
	if _, err := RedeemAgentPairingCode(old, "", ""); !errors.Is(err, ErrPairingCodeInvalid) {
		t.Fatalf("the replaced code must be void, got %v", err)
	}
	if _, err := RedeemAgentPairingCode(current, "", ""); err != nil {
		t.Fatalf("the new code must work, got %v", err)
	}
}

func TestPairingCodeOfUnavailableKey(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	setAgentClock(t)
	disabled := newTestToken(t, user.Id, "disabled")
	code, _, _ := CreateAgentPairingCode(disabled.Id, user.Id)
	DB.Model(&Token{}).Where("id = ?", disabled.Id).Update("status", config.TokenStatusDisabled)
	if _, err := RedeemAgentPairingCode(code, "", ""); !errors.Is(err, ErrPairingKeyUnavailable) {
		t.Fatalf("disabled key: want ErrPairingKeyUnavailable, got %v", err)
	}

	deleted := newTestToken(t, user.Id, "deleted")
	code, _, _ = CreateAgentPairingCode(deleted.Id, user.Id)
	if err := DeleteTokenById(deleted.Id, user.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := RedeemAgentPairingCode(code, "", ""); !errors.Is(err, ErrPairingKeyUnavailable) {
		t.Fatalf("deleted key: want ErrPairingKeyUnavailable, got %v", err)
	}
}

// Only the tokens table is reachable: a management key's id never names a pairable key, and what
// comes out is always an API key, never a management key.
func TestAgentHandoverOnlyYieldsAPIKeys(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	setAgentClock(t)
	mk, err := CreateManagementKey(user.Id, "mk", ManagementKeyScopeWrite, 0)
	if err != nil {
		t.Fatal(err)
	}
	code, _, err := CreateAgentPairingCode(mk.Id, user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RedeemAgentPairingCode(code, "", ""); !errors.Is(err, ErrPairingKeyUnavailable) {
		t.Fatalf("a management key id must not resolve to a key, got %v", err)
	}
	if _, _, err := ValidateManagementKey(mk.Key); err != nil {
		t.Fatalf("the management key must be untouched: %v", err)
	}

	token := newTestToken(t, user.Id, "app")
	code, _, _ = CreateAgentPairingCode(token.Id, user.Id)
	claimed, err := RedeemAgentPairingCode(code, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !common.LooksLikeAPIKey("sk-" + claimed.Key) {
		t.Fatal("the claimed key must be an API key")
	}
	if _, _, err := ValidateManagementKey(claimed.Key); err == nil {
		t.Fatal("the claimed key must not work as a management key")
	}
}

// ---------------------------------------------------------------- device flow

func approveForTest(t *testing.T, row *AgentDeviceAuthorization, userId int) *Token {
	t.Helper()
	token := &Token{UserId: userId, Name: "agent", Status: config.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}
	if err := ApproveAgentDeviceAuthorization(row, userId, 0, token); err != nil {
		t.Fatalf("approve: %v", err)
	}
	return token
}

func TestDeviceFlowPendingSlowDownApproved(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	advance := setAgentClock(t)

	deviceCode, row, err := CreateAgentDeviceAuthorization("OpenClaw on laptop", "203.0.113.9", "openclaw/2")
	if err != nil {
		t.Fatal(err)
	}
	if common.NormalizeDeviceUserCode(common.FormatDeviceUserCode(row.UserCode)) != row.UserCode || row.Interval != AgentDevicePollInterval {
		t.Fatalf("unexpected authorization %+v", row)
	}
	var stored AgentDeviceAuthorization
	DB.First(&stored, row.Id)
	if stored.DeviceCodeHash == deviceCode || stored.DeviceCodeHash != common.HashAgentSecret(deviceCode) {
		t.Fatal("the device code must only be stored as its hash")
	}

	if _, _, err := PollAgentDeviceAuthorization(deviceCode, "", ""); !errors.Is(err, ErrDeviceAuthPending) {
		t.Fatalf("first poll: want pending, got %v", err)
	}
	if _, _, err := PollAgentDeviceAuthorization(deviceCode, "", ""); !errors.Is(err, ErrDeviceSlowDown) {
		t.Fatalf("immediate poll: want slow_down, got %v", err)
	}
	DB.First(&stored, row.Id)
	if stored.Interval != AgentDevicePollInterval+AgentDeviceSlowDownStep {
		t.Fatalf("slow_down must grow the interval, got %d", stored.Interval)
	}
	advance(int64(stored.Interval))
	if _, _, err := PollAgentDeviceAuthorization(deviceCode, "", ""); !errors.Is(err, ErrDeviceAuthPending) {
		t.Fatalf("poll after the interval: want pending, got %v", err)
	}

	pending, err := GetPendingAgentDeviceAuthorization(strings.ToLower(common.FormatDeviceUserCode(row.UserCode)))
	if err != nil || pending.ClientName != "OpenClaw on laptop" || pending.RequestIp != "203.0.113.9" {
		t.Fatalf("the user code should find the request: %+v %v", pending, err)
	}
	token := approveForTest(t, pending, user.Id)
	if token.Key != "" {
		t.Fatal("approval must not keep the key's secret")
	}
	if _, err := GetPendingAgentDeviceAuthorization(row.UserCode); !errors.Is(err, ErrDeviceUserCodeInvalid) {
		t.Fatal("an answered request is no longer pending")
	}

	advance(int64(stored.Interval))
	claimed, _, err := PollAgentDeviceAuthorization(deviceCode, "203.0.113.10", "openclaw/2")
	if err != nil {
		t.Fatalf("poll after approval: %v", err)
	}
	if claimed.Id != token.Id || !keyWorks(t, token.Id, claimed.Key) {
		t.Fatal("the poll must return the approved key with a working secret")
	}
	details, _ := GetTokenById(token.Id)
	AttachAgentInfo(details)
	if details.AgentClaim == nil || details.AgentClaim.Via != AgentClaimViaDeviceFlow || details.AgentClaim.ClientName != "OpenClaw on laptop" ||
		details.AgentClaim.Ip != "203.0.113.10" {
		t.Fatalf("device claim not recorded: %+v", details.AgentClaim)
	}

	// Single use.
	advance(60)
	if _, _, err := PollAgentDeviceAuthorization(deviceCode, "", ""); !errors.Is(err, ErrDeviceInvalidGrant) {
		t.Fatalf("second claim: want invalid_grant, got %v", err)
	}
	if !keyWorks(t, token.Id, claimed.Key) {
		t.Fatal("a refused second poll must not change the key")
	}
}

func TestDeviceFlowDeniedAndExpired(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	advance := setAgentClock(t)

	deniedCode, denied, _ := CreateAgentDeviceAuthorization("a", "", "")
	if err := DenyAgentDeviceAuthorization(denied, user.Id); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PollAgentDeviceAuthorization(deniedCode, "", ""); !errors.Is(err, ErrDeviceAccessDenied) {
		t.Fatalf("denied: want access_denied, got %v", err)
	}
	if err := DenyAgentDeviceAuthorization(denied, user.Id); !errors.Is(err, ErrDeviceUserCodeInvalid) {
		t.Fatal("a request is answered once")
	}

	expiredCode, expired, _ := CreateAgentDeviceAuthorization("b", "", "")
	advance(int64(AgentDeviceCodeLifetime / time.Second))
	if _, _, err := PollAgentDeviceAuthorization(expiredCode, "", ""); !errors.Is(err, ErrDeviceExpired) {
		t.Fatalf("expired: want expired_token, got %v", err)
	}
	if _, err := GetPendingAgentDeviceAuthorization(expired.UserCode); !errors.Is(err, ErrDeviceUserCodeInvalid) {
		t.Fatal("an expired request cannot be looked up")
	}
	token := &Token{UserId: user.Id, Name: "late", Status: config.TokenStatusEnabled, ExpiredTime: -1}
	if err := ApproveAgentDeviceAuthorization(expired, user.Id, 0, token); !errors.Is(err, ErrDeviceUserCodeInvalid) {
		t.Fatalf("approving an expired request: want ErrDeviceUserCodeInvalid, got %v", err)
	}
	var count int64
	DB.Model(&Token{}).Where("name = ?", "late").Count(&count)
	if count != 0 {
		t.Fatal("a refused approval must not leave a key behind")
	}

	if _, _, err := PollAgentDeviceAuthorization("unknown", "", ""); !errors.Is(err, ErrDeviceInvalidGrant) {
		t.Fatalf("unknown device code: want invalid_grant, got %v", err)
	}
	if _, err := GetPendingAgentDeviceAuthorization("not-a-code"); !errors.Is(err, ErrDeviceUserCodeInvalid) {
		t.Fatal("a malformed user code is invalid")
	}
}
