package common

import (
	"crypto/subtle"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type verificationValue struct {
	code string
	time time.Time
	// failures 是该验证码已被猜错的次数，达到 VerificationMaxFailures 即作废
	failures int
}

// VerificationMaxFailures 是一个验证码允许猜错的次数。验证码只有 6 位十六进制（约 1677 万种），
// 不限次数时可在有效期内用大量 IP 爆破，抢先认领他人的邮箱（SEC-43）。
const VerificationMaxFailures = 5

const (
	EmailVerificationPurpose = "v"
	PasswordResetPurpose     = "r"
	// LoginCodePurpose 邮箱验证码登录
	LoginCodePurpose = "l"
)

var verificationMutex sync.Mutex
var verificationMap map[string]verificationValue
var verificationMapMaxSize = 10
var VerificationValidMinutes = 10

func GenerateVerificationCode(length int) string {
	code := uuid.New().String()
	code = strings.Replace(code, "-", "", -1)
	if length == 0 {
		return code
	}
	return code[:length]
}

func RegisterVerificationCodeWithKey(key string, code string, purpose string) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap[purpose+key] = verificationValue{
		code: code,
		time: time.Now(),
	}
	if len(verificationMap) > verificationMapMaxSize {
		removeExpiredPairs()
	}
}

// VerifyCodeWithKey 校验验证码。猜错累计 VerificationMaxFailures 次后该验证码作废，需重新发送；
// 校验通过不会删除验证码，调用方在业务完成后用 DeleteKey 使其失效（一次性）。
func VerifyCodeWithKey(key string, code string, purpose string) bool {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	mapKey := purpose + key
	value, okay := verificationMap[mapKey]
	now := time.Now()
	if !okay || int(now.Sub(value.time).Seconds()) >= VerificationValidMinutes*60 {
		return false
	}
	if code != "" && subtle.ConstantTimeCompare([]byte(code), []byte(value.code)) == 1 {
		return true
	}
	value.failures++
	if value.failures >= VerificationMaxFailures {
		delete(verificationMap, mapKey)
	} else {
		verificationMap[mapKey] = value
	}
	return false
}

// VerificationCodeSentWithin 该 key 在 within 内是否已登记过验证码（发码冷却用，不校验 code 本身）。
func VerificationCodeSentWithin(key string, purpose string, within time.Duration) bool {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	value, okay := verificationMap[purpose+key]
	return okay && time.Since(value.time) < within
}

func DeleteKey(key string, purpose string) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	delete(verificationMap, purpose+key)
}

// no lock inside, so the caller must lock the verificationMap before calling!
func removeExpiredPairs() {
	now := time.Now()
	for key := range verificationMap {
		if int(now.Sub(verificationMap[key].time).Seconds()) >= VerificationValidMinutes*60 {
			delete(verificationMap, key)
		}
	}
}

func init() {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap = make(map[string]verificationValue)
}
