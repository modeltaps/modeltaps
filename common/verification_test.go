package common

import "testing"

func TestVerifyCodeWithKeyLimitsFailures(t *testing.T) {
	const key = "brute@example.com"
	RegisterVerificationCodeWithKey(key, "abc123", EmailVerificationPurpose)
	t.Cleanup(func() { DeleteKey(key, EmailVerificationPurpose) })

	for i := 0; i < VerificationMaxFailures-1; i++ {
		if VerifyCodeWithKey(key, "000000", EmailVerificationPurpose) {
			t.Fatal("wrong code must not verify")
		}
	}
	if !VerifyCodeWithKey(key, "abc123", EmailVerificationPurpose) {
		t.Fatal("correct code should still verify before the failure limit")
	}

	if VerifyCodeWithKey(key, "000000", EmailVerificationPurpose) {
		t.Fatal("wrong code must not verify")
	}
	// 第 VerificationMaxFailures 次猜错后作废，正确的码也不再通过
	if VerifyCodeWithKey(key, "abc123", EmailVerificationPurpose) {
		t.Fatal("code must be invalidated after too many failures")
	}
}

func TestVerifyCodeWithKeyRejectsEmptyCode(t *testing.T) {
	const key = "empty@example.com"
	RegisterVerificationCodeWithKey(key, "abc123", EmailVerificationPurpose)
	t.Cleanup(func() { DeleteKey(key, EmailVerificationPurpose) })
	if VerifyCodeWithKey(key, "", EmailVerificationPurpose) {
		t.Fatal("empty code must not verify")
	}
}
