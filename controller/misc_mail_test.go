package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/stmp"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// setupMailCodeRouter configures an SMTP server that cannot be reached (no email leaves the test)
// and counts every send attempt through stmp.OnSent.
func setupMailCodeRouter(t *testing.T) (*gin.Engine, *int64) {
	t.Helper()
	setupGetUserTestDB(t)
	old := struct {
		system, server, account, token string
		port                           int
		onSent                         func(stmp.SendRecord)
	}{config.AccountSystem, config.SMTPServer, config.SMTPAccount, config.SMTPToken, config.SMTPPort, stmp.OnSent}
	config.AccountSystem = config.AccountSystemBuiltin
	config.SMTPServer, config.SMTPAccount, config.SMTPToken, config.SMTPPort = "127.0.0.1", "noreply", "secret", 1
	var sends int64
	stmp.OnSent = func(stmp.SendRecord) { atomic.AddInt64(&sends, 1) }
	t.Cleanup(func() {
		config.AccountSystem, config.SMTPServer, config.SMTPAccount, config.SMTPToken, config.SMTPPort = old.system, old.server, old.account, old.token, old.port
		stmp.OnSent = old.onSent
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/verification", SendEmailVerification)
	r.POST("/api/reset_password", SendPasswordResetEmail)
	return r, &sends
}

func postMailJSON(t *testing.T, r *gin.Engine, path string) map[string]interface{} {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, w.Body.String())
	}
	return resp
}

// SEC-69: within the cooldown a second sign-up code request answers like a sent one, but sends
// nothing and keeps the code already sent valid.
func TestSendEmailVerificationCooldown(t *testing.T) {
	r, sends := setupMailCodeRouter(t)
	const email = "newcomer@example.test"
	common.RegisterVerificationCodeWithKey(email, "abc123", common.EmailVerificationPurpose)
	t.Cleanup(func() { common.DeleteKey(email, common.EmailVerificationPurpose) })

	if resp := postMailJSON(t, r, "/api/verification?email="+email); resp["success"] != true {
		t.Fatalf("a request within the cooldown should answer success, got %v", resp)
	}
	if n := atomic.LoadInt64(sends); n != 0 {
		t.Fatalf("no email should be sent within the cooldown, got %d", n)
	}
	if !common.VerifyCodeWithKey(email, "abc123", common.EmailVerificationPurpose) {
		t.Fatal("the code already sent should stay valid")
	}
}

// A failed send does not start the cooldown: the user can ask again at once.
func TestSendEmailVerificationFailedSendAllowsRetry(t *testing.T) {
	r, sends := setupMailCodeRouter(t)
	const email = "retry@example.test"
	t.Cleanup(func() { common.DeleteKey(email, common.EmailVerificationPurpose) })

	if resp := postMailJSON(t, r, "/api/verification?email="+email); resp["success"] != false {
		t.Fatalf("the send cannot succeed against an unreachable server, got %v", resp)
	}
	if resp := postMailJSON(t, r, "/api/verification?email="+email); resp["success"] != false {
		t.Fatalf("the retry should send again (and fail again), got %v", resp)
	}
	if n := atomic.LoadInt64(sends); n != 2 {
		t.Fatalf("each request should try to send once, got %d attempts", n)
	}
}

// Password reset emails follow the same cooldown, and the link already sent keeps working.
func TestSendPasswordResetEmailCooldown(t *testing.T) {
	r, sends := setupMailCodeRouter(t)
	const email = "member@example.test"
	seedEmailUser(t, &model.User{
		Username: "member", Email: model.NullableEmail(email), EmailVerified: true,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AffCode: "aff-member",
	})
	common.RegisterVerificationCodeWithKey(email, "reset-token", common.PasswordResetPurpose)
	t.Cleanup(func() { common.DeleteKey(email, common.PasswordResetPurpose) })

	if resp := postMailJSON(t, r, "/api/reset_password?email="+email); resp["success"] != true {
		t.Fatalf("a request within the cooldown should answer success, got %v", resp)
	}
	if n := atomic.LoadInt64(sends); n != 0 {
		t.Fatalf("no email should be sent within the cooldown, got %d", n)
	}
	if !common.VerifyCodeWithKey(email, "reset-token", common.PasswordResetPurpose) {
		t.Fatal("the reset link already sent should stay valid")
	}
}

// Concurrent requests for one address register one code: only one of them may send.
func TestReserveVerificationCodeOnce(t *testing.T) {
	const email = "race@example.test"
	t.Cleanup(func() { common.DeleteKey(email, common.EmailVerificationPurpose) })
	var won int64
	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			if reserveVerificationCode(email, common.GenerateVerificationCode(6), common.EmailVerificationPurpose) {
				atomic.AddInt64(&won, 1)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
	if won != 1 {
		t.Fatalf("exactly one request should get to send, got %d", won)
	}
}
