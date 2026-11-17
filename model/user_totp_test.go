package model

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupUserTotpDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	if err := testDB.AutoMigrate(&UserTotp{}, &UserTotpRecoveryCode{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

// RFC 4226 appendix D: the HOTP values of the secret "12345678901234567890" for counters 0-9.
func TestHotpRFC4226Vectors(t *testing.T) {
	key := []byte("12345678901234567890")
	want := []string{"755224", "287082", "359152", "969429", "338314", "254676", "287922", "162583", "399871", "520489"}
	for counter, code := range want {
		if got := hotp(key, uint64(counter), 6); got != code {
			t.Errorf("counter %d: got %s, want %s", counter, got, code)
		}
	}
}

// RFC 6238 appendix B, the SHA-1 rows (8 digits), through the base32 secret path used at sign-in.
func TestTotpRFC6238Vectors(t *testing.T) {
	secret := totpEncoding.EncodeToString([]byte("12345678901234567890"))
	key, err := decodeTotpSecret(strings.ToLower(secret))
	if err != nil {
		t.Fatal(err)
	}
	vectors := []struct {
		unix int64
		code string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	}
	for _, v := range vectors {
		step := totpStep(time.Unix(v.unix, 0))
		if got := hotp(key, uint64(step), 8); got != v.code {
			t.Errorf("T=%d: got %s, want %s", v.unix, got, v.code)
		}
		// The 6-digit code is the same value modulo 10^6.
		six, _ := TotpCodeAt(secret, time.Unix(v.unix, 0))
		if six != v.code[2:] {
			t.Errorf("T=%d: 6-digit code %s, want %s", v.unix, six, v.code[2:])
		}
	}
}

func TestMatchTotpStepAcceptsOneStepOfDrift(t *testing.T) {
	secret, err := newTotpSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	for _, offset := range []int{-1, 0, 1} {
		code, _ := TotpCodeAt(secret, now.Add(time.Duration(offset*TotpPeriod)*time.Second))
		step, ok := matchTotpStep(secret, code, now)
		if !ok || step != totpStep(now)+int64(offset) {
			t.Errorf("offset %d: step %d ok %v", offset, step, ok)
		}
	}
	for _, offset := range []int{-2, 2} {
		code, _ := TotpCodeAt(secret, now.Add(time.Duration(offset*TotpPeriod)*time.Second))
		if _, ok := matchTotpStep(secret, code, now); ok {
			// A collision with a neighbouring step is possible but one in a million.
			t.Errorf("offset %d should not be accepted", offset)
		}
	}
}

// enrollTotp runs the setup of a user and returns the secret and recovery codes.
func enrollTotp(t *testing.T, userId int, now time.Time) (string, []string) {
	t.Helper()
	secret, err := BeginUserTotpSetup(userId)
	if err != nil {
		t.Fatalf("begin setup: %v", err)
	}
	code, _ := TotpCodeAt(secret, now)
	codes, err := EnableUserTotp(userId, code, now)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	return secret, codes
}

func TestUserTotpEnrollmentAndReplay(t *testing.T) {
	setupUserTotpDB(t)
	now := time.Now()

	// Unconfirmed setup is not "enabled" and can be restarted with a new secret.
	first, err := BeginUserTotpSetup(7)
	if err != nil {
		t.Fatal(err)
	}
	if on, _ := UserTotpEnabled(7); on {
		t.Fatal("an unconfirmed setup must not be enabled")
	}
	right, _ := TotpCodeAt(first, now)
	wrong := "000000"
	if right == wrong {
		wrong = "111111"
	}
	if _, err := EnableUserTotp(7, wrong, now); !errors.Is(err, ErrTotpCodeInvalid) {
		t.Fatalf("a wrong code should be refused, got %v", err)
	}

	secret, codes := enrollTotp(t, 7, now)
	if secret == first {
		t.Error("restarting the setup should draw a new secret")
	}
	if len(codes) != TotpRecoveryCodeCount {
		t.Fatalf("got %d recovery codes", len(codes))
	}
	if on, _ := UserTotpEnabled(7); !on {
		t.Fatal("confirmed setup should be enabled")
	}
	if _, err := BeginUserTotpSetup(7); !errors.Is(err, ErrTotpAlreadyEnabled) {
		t.Errorf("setup while enabled: %v", err)
	}

	// The code that confirmed the setup was used: it is not accepted again in its window.
	code, _ := TotpCodeAt(secret, now)
	if _, err := VerifyUserTotp(7, code, now); !errors.Is(err, ErrTotpCodeInvalid) {
		t.Errorf("replayed enrollment code: %v", err)
	}
	next := now.Add(TotpPeriod * time.Second)
	nextCode, _ := TotpCodeAt(secret, next)
	if recovery, err := VerifyUserTotp(7, nextCode, next); err != nil || recovery {
		t.Fatalf("next code: recovery %v err %v", recovery, err)
	}
	if _, err := VerifyUserTotp(7, nextCode, next); !errors.Is(err, ErrTotpCodeInvalid) {
		t.Errorf("same code twice in its window: %v", err)
	}
	// An older code still inside the drift window is refused once a later one was used.
	if _, err := VerifyUserTotp(7, code, next); !errors.Is(err, ErrTotpCodeInvalid) {
		t.Errorf("earlier code after a later one: %v", err)
	}
}

func TestUserTotpSetupExpires(t *testing.T) {
	setupUserTotpDB(t)
	secret, err := BeginUserTotpSetup(8)
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(totpSetupTTL + time.Minute)
	code, _ := TotpCodeAt(secret, later)
	if _, err := EnableUserTotp(8, code, later); !errors.Is(err, ErrTotpSetupRequired) {
		t.Errorf("stale setup: %v", err)
	}
}

func TestUserTotpRecoveryCodesSingleUse(t *testing.T) {
	setupUserTotpDB(t)
	now := time.Now()
	_, codes := enrollTotp(t, 9, now)

	var stored []UserTotpRecoveryCode
	DB.Where("user_id = ?", 9).Find(&stored)
	for _, row := range stored {
		for _, code := range codes {
			if strings.Contains(row.CodeHash, strings.ReplaceAll(code, "-", "")) {
				t.Fatal("recovery codes must be stored as hashes only")
			}
		}
	}

	// Typed in lower case and without dashes, it still matches; then it is used up.
	typed := strings.ToLower(strings.ReplaceAll(codes[3], "-", ""))
	if recovery, err := VerifyUserTotp(9, typed, now); err != nil || !recovery {
		t.Fatalf("recovery code: recovery %v err %v", recovery, err)
	}
	if _, err := VerifyUserTotp(9, codes[3], now); !errors.Is(err, ErrTotpCodeInvalid) {
		t.Errorf("recovery code used twice: %v", err)
	}
	if left, _ := UserTotpRecoveryCodesLeft(9); left != TotpRecoveryCodeCount-1 {
		t.Errorf("codes left = %d", left)
	}
	// Another user's code does not work here.
	_, others := enrollTotp(t, 10, now)
	if _, err := VerifyUserTotp(9, others[0], now); !errors.Is(err, ErrTotpCodeInvalid) {
		t.Errorf("another user's recovery code: %v", err)
	}

	fresh, err := RegenerateUserTotpRecoveryCodes(9, now)
	if err != nil || len(fresh) != TotpRecoveryCodeCount {
		t.Fatalf("regenerate: %d %v", len(fresh), err)
	}
	if _, err := VerifyUserTotp(9, codes[0], now); !errors.Is(err, ErrTotpCodeInvalid) {
		t.Errorf("old recovery code after regenerating: %v", err)
	}

	if err := DeleteUserTotp(9); err != nil {
		t.Fatal(err)
	}
	if on, _ := UserTotpEnabled(9); on {
		t.Error("deleted two-factor should be off")
	}
	if left, _ := UserTotpRecoveryCodesLeft(9); left != 0 {
		t.Errorf("recovery codes left after delete: %d", left)
	}
}

func TestUserTotpChallenge(t *testing.T) {
	setupUserTotpDB(t)
	now := time.Now()
	if _, err := StartUserTotpChallenge(11, now); !errors.Is(err, ErrTotpNotEnabled) {
		t.Errorf("challenge without two-factor: %v", err)
	}
	enrollTotp(t, 11, now)

	token, err := StartUserTotpChallenge(11, now)
	if err != nil {
		t.Fatal(err)
	}
	if !CheckUserTotpChallenge(11, token, now) {
		t.Fatal("fresh challenge should be open")
	}
	if CheckUserTotpChallenge(12, token, now) || CheckUserTotpChallenge(11, token+"x", now) {
		t.Error("challenge must match its user and token")
	}
	if CheckUserTotpChallenge(11, token, now.Add(TotpChallengeTTL+time.Second)) {
		t.Error("challenge should expire")
	}
	row, _ := GetUserTotp(11)
	if strings.Contains(row.ChallengeHash, token) {
		t.Error("only the token's hash is stored")
	}

	for i := 0; i < TotpChallengeMaxFailures; i++ {
		if err := RecordUserTotpChallengeFailure(11); err != nil {
			t.Fatal(err)
		}
	}
	if CheckUserTotpChallenge(11, token, now) {
		t.Error("challenge should close after too many wrong codes")
	}

	// A new password sign-in opens a new challenge and the old token stops working.
	again, _ := StartUserTotpChallenge(11, now)
	if !CheckUserTotpChallenge(11, again, now) || CheckUserTotpChallenge(11, token, now) {
		t.Error("a new challenge replaces the old one")
	}
	if err := ClearUserTotpChallenge(11); err != nil {
		t.Fatal(err)
	}
	if CheckUserTotpChallenge(11, again, now) {
		t.Error("cleared challenge should be closed")
	}
}

func TestUserTotpSecretEncrypted(t *testing.T) {
	setupUserTotpDB(t)
	useEncryptionKey(t, secretTestKey(3), "")
	now := time.Now()

	secret, _ := enrollTotp(t, 13, now)
	assertCiphertext(t, rawColumn(t, "user_totps", "secret", "user_id = ?", 13), secret)
	row, err := GetUserTotp(13)
	if err != nil || row.Secret != secret {
		t.Fatalf("decrypted secret %q, %v", row.Secret, err)
	}
	// Updates through maps (challenge, last used step) leave the ciphertext intact.
	if _, err := StartUserTotpChallenge(13, now); err != nil {
		t.Fatal(err)
	}
	next := now.Add(TotpPeriod * time.Second)
	code, _ := TotpCodeAt(secret, next)
	if _, err := VerifyUserTotp(13, code, next); err != nil {
		t.Fatalf("verify after encryption: %v", err)
	}
	assertCiphertext(t, rawColumn(t, "user_totps", "secret", "user_id = ?", 13), secret)
}
