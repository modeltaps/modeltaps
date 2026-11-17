package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// totpClient is a browser: it keeps the session cookie between requests.
type totpClient struct {
	t       *testing.T
	r       *gin.Engine
	cookies map[string]*http.Cookie
}

func (b *totpClient) do(method, path, body string) map[string]interface{} {
	b.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, ck := range b.cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	b.r.ServeHTTP(w, req)
	for _, ck := range w.Result().Cookies() {
		b.cookies[ck.Name] = ck
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		b.t.Fatalf("decode %s %s: %v (%s)", method, path, err, w.Body.String())
	}
	return resp
}

func (b *totpClient) signedIn() bool {
	return b.do(http.MethodGet, "/api/user/totp", "")["success"] == true
}

func setupTotpRouter(t *testing.T) *gin.Engine {
	t.Helper()
	setupOidcTestDB(t)
	if err := model.DB.AutoMigrate(&model.Log{}); err != nil {
		t.Fatal(err)
	}
	old := struct {
		system   string
		password bool
	}{config.AccountSystem, config.PasswordLoginEnabled}
	config.AccountSystem, config.PasswordLoginEnabled = config.AccountSystemBuiltin, true
	t.Cleanup(func() { config.AccountSystem, config.PasswordLoginEnabled = old.system, old.password })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.POST("/api/user/login", Login)
	r.POST("/api/user/login/totp", LoginWithTotp)
	self := r.Group("/api/user", middleware.UserAuth(), middleware.SessionOnly())
	self.GET("/totp", GetTotpStatus)
	self.POST("/totp/setup", BeginTotpSetup)
	self.POST("/totp/enable", EnableTotp)
	self.POST("/totp/disable", DisableTotp)
	self.POST("/totp/recovery_codes", RegenerateTotpRecoveryCodes)
	r.DELETE("/api/user/:id/totp", middleware.RootAuth(), ResetUserTotp)
	return r
}

func newTotpClient(t *testing.T, r *gin.Engine) *totpClient {
	return &totpClient{t: t, r: r, cookies: map[string]*http.Cookie{}}
}

func seedTotpUser(t *testing.T, username string, role int) *model.User {
	t.Helper()
	hash, err := common.Password2Hash(username + "-password")
	if err != nil {
		t.Fatal(err)
	}
	return seedOidcUser(t, &model.User{
		Username: username, Password: hash, Role: role, Status: config.UserStatusEnabled, AffCode: "aff-" + username,
	})
}

func (b *totpClient) passwordLogin(username string) map[string]interface{} {
	return b.do(http.MethodPost, "/api/user/login", `{"username":"`+username+`","password":"`+username+`-password"}`)
}

func totpRequired(resp map[string]interface{}) bool {
	data, _ := resp["data"].(map[string]interface{})
	return resp["success"] == true && data["totp_required"] == true
}

// enrollThroughAPI runs setup and confirmation as the account page does; returns the secret and
// the recovery codes.
func enrollThroughAPI(t *testing.T, b *totpClient) (string, []string) {
	t.Helper()
	setup := b.do(http.MethodPost, "/api/user/totp/setup", "")
	data, _ := setup["data"].(map[string]interface{})
	secret, _ := data["secret"].(string)
	uri, _ := data["otpauth_url"].(string)
	if secret == "" || !strings.HasPrefix(uri, "otpauth://totp/") || !strings.Contains(uri, "secret="+secret) {
		t.Fatalf("setup response: %v", setup)
	}
	code, _ := model.TotpCodeAt(secret, time.Now())
	enable := b.do(http.MethodPost, "/api/user/totp/enable", `{"code":"`+code+`"}`)
	data, _ = enable["data"].(map[string]interface{})
	raw, _ := data["recovery_codes"].([]interface{})
	if enable["success"] != true || len(raw) != model.TotpRecoveryCodeCount {
		t.Fatalf("enable response: %v", enable)
	}
	codes := make([]string, len(raw))
	for i, c := range raw {
		codes[i], _ = c.(string)
	}
	return secret, codes
}

// The whole flow: enroll, sign in with password + code, replay refused, recovery code once,
// disabling needs a code.
func TestTotpSignInFlow(t *testing.T) {
	r := setupTotpRouter(t)
	user := seedTotpUser(t, "tina", config.RoleCommonUser)

	browser := newTotpClient(t, r)
	if resp := browser.passwordLogin("tina"); resp["success"] != true || totpRequired(resp) {
		t.Fatalf("without two-factor the password signs in: %v", resp)
	}
	secret, recovery := enrollThroughAPI(t, browser)

	// A new browser: the password alone no longer signs in.
	other := newTotpClient(t, r)
	resp := other.passwordLogin("tina")
	if !totpRequired(resp) {
		t.Fatalf("password step should ask for the second factor: %v", resp)
	}
	if other.signedIn() {
		t.Fatal("the pending step must not be a session")
	}
	var sessionsBefore int64
	model.DB.Model(&model.UserSession{}).Where("user_id = ?", user.Id).Count(&sessionsBefore)

	// The enrollment code was used; the code of the next step is fresh.
	next := time.Now().Add(model.TotpPeriod * time.Second)
	code, _ := model.TotpCodeAt(secret, next)
	wrong := "000000"
	if wrong == code {
		wrong = "111111"
	}
	if resp := other.do(http.MethodPost, "/api/user/login/totp", `{"code":"`+wrong+`"}`); resp["code"] != common.APICodeTotpCodeInvalid {
		t.Fatalf("wrong code: %v", resp)
	}
	if resp := other.do(http.MethodPost, "/api/user/login/totp", `{"code":"`+code+`"}`); resp["success"] != true {
		t.Fatalf("right code should sign in: %v", resp)
	}
	if !other.signedIn() {
		t.Fatal("second step should create a session")
	}
	var sessionsAfter int64
	model.DB.Model(&model.UserSession{}).Where("user_id = ?", user.Id).Count(&sessionsAfter)
	if sessionsAfter != sessionsBefore+1 {
		t.Errorf("sessions %d -> %d", sessionsBefore, sessionsAfter)
	}

	// The same code again, on a new sign-in: refused (single use within its window).
	third := newTotpClient(t, r)
	if !totpRequired(third.passwordLogin("tina")) {
		t.Fatal("expected the second step")
	}
	if resp := third.do(http.MethodPost, "/api/user/login/totp", `{"code":"`+code+`"}`); resp["code"] != common.APICodeTotpCodeInvalid {
		t.Fatalf("replayed code: %v", resp)
	}
	// A recovery code works once.
	if resp := third.do(http.MethodPost, "/api/user/login/totp", `{"code":"`+strings.ToLower(recovery[0])+`"}`); resp["success"] != true {
		t.Fatalf("recovery code: %v", resp)
	}
	fourth := newTotpClient(t, r)
	fourth.passwordLogin("tina")
	if resp := fourth.do(http.MethodPost, "/api/user/login/totp", `{"code":"`+recovery[0]+`"}`); resp["code"] != common.APICodeTotpCodeInvalid {
		t.Fatalf("used recovery code: %v", resp)
	}
	status := third.do(http.MethodGet, "/api/user/totp", "")
	data, _ := status["data"].(map[string]interface{})
	if data["enabled"] != true || data["recovery_codes_left"] != float64(model.TotpRecoveryCodeCount-1) {
		t.Errorf("status: %v", status)
	}

	// Turning it off needs a valid code.
	if resp := third.do(http.MethodPost, "/api/user/totp/disable", `{"code":"123"}`); resp["success"] != false {
		t.Fatalf("disable without a valid code: %v", resp)
	}
	if on, _ := model.UserTotpEnabled(user.Id); !on {
		t.Fatal("two-factor should still be on")
	}
	if resp := third.do(http.MethodPost, "/api/user/totp/disable", `{"code":"`+recovery[1]+`"}`); resp["success"] != true {
		t.Fatalf("disable with a recovery code: %v", resp)
	}
	if resp := newTotpClient(t, r).passwordLogin("tina"); resp["success"] != true || totpRequired(resp) {
		t.Fatalf("after disabling, the password signs in again: %v", resp)
	}
}

// The second step cannot be reached without the password step, and closes after too many wrong
// codes; wrong codes count against the account lock, which the password does not reset.
func TestTotpChallengeLimits(t *testing.T) {
	r := setupTotpRouter(t)
	user := seedTotpUser(t, "uma", config.RoleCommonUser)
	browser := newTotpClient(t, r)
	browser.passwordLogin("uma")
	secret, _ := enrollThroughAPI(t, browser)
	next := time.Now().Add(model.TotpPeriod * time.Second)
	good, _ := model.TotpCodeAt(secret, next)
	bad := "000000"
	if good == bad {
		bad = "111111"
	}

	if resp := newTotpClient(t, r).do(http.MethodPost, "/api/user/login/totp", `{"code":"`+good+`"}`); resp["code"] != common.APICodeTotpChallengeExpired {
		t.Fatalf("a code without the password step: %v", resp)
	}

	attacker := newTotpClient(t, r)
	attacker.passwordLogin("uma")
	for i := 0; i < model.TotpChallengeMaxFailures; i++ {
		attacker.do(http.MethodPost, "/api/user/login/totp", `{"code":"`+bad+`"}`)
	}
	if resp := attacker.do(http.MethodPost, "/api/user/login/totp", `{"code":"`+good+`"}`); resp["code"] != common.APICodeTotpChallengeExpired {
		t.Fatalf("challenge should close after %d wrong codes: %v", model.TotpChallengeMaxFailures, resp)
	}
	// The wrong codes locked this account from this address; entering the password again does
	// not give fresh guesses.
	if resp := attacker.passwordLogin("uma"); resp["success"] != false {
		t.Fatalf("password after the wrong codes should hit the lock: %v", resp)
	}
	if !common.IsLoginLocked(testPairKey(common.LoginFailureKeyForUser(user.Id))) {
		t.Fatal("wrong two-factor codes should count against the sign-in lock")
	}
}

// Root resets another user's two-factor (written to the log); not its own, which needs a code.
func TestRootResetsUserTotp(t *testing.T) {
	r := setupTotpRouter(t)
	root := seedTotpUser(t, "rooty", config.RoleRootUser)
	user := seedTotpUser(t, "vera", config.RoleCommonUser)
	member := newTotpClient(t, r)
	member.passwordLogin("vera")
	enrollThroughAPI(t, member)

	admin := newTotpClient(t, r)
	if resp := admin.passwordLogin("rooty"); resp["success"] != true {
		t.Fatalf("root sign-in: %v", resp)
	}
	enrollThroughAPI(t, admin)
	if resp := admin.do(http.MethodDelete, "/api/user/"+strconv.Itoa(root.Id)+"/totp", ""); resp["success"] != false {
		t.Fatalf("root must not reset its own two-factor here: %v", resp)
	}
	if resp := member.do(http.MethodDelete, "/api/user/"+strconv.Itoa(root.Id)+"/totp", ""); resp["success"] != false {
		t.Fatalf("a common user must not reset: %v", resp)
	}
	if resp := admin.do(http.MethodDelete, "/api/user/"+strconv.Itoa(user.Id)+"/totp", ""); resp["success"] != true {
		t.Fatalf("root reset: %v", resp)
	}
	if on, _ := model.UserTotpEnabled(user.Id); on {
		t.Fatal("reset should turn two-factor off")
	}
	var logs int64
	model.DB.Model(&model.Log{}).Where("user_id = ? AND type = ? AND content LIKE ?", user.Id, model.LogTypeManage, "%reset two-factor%").Count(&logs)
	if logs != 1 {
		t.Errorf("reset log lines: %d", logs)
	}
}

// An email code replaces the password, not the second factor.
func TestEmailCodeLoginAsksForTotp(t *testing.T) {
	r := setupEmailCodeLogin(t)
	user := seedOidcUser(t, &model.User{
		Username: "walt", Email: model.NullableEmail("walt@example.test"), Role: config.RoleCommonUser,
		Status: config.UserStatusEnabled, AffCode: "aff-walt",
	})
	secret, err := model.BeginUserTotpSetup(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	code, _ := model.TotpCodeAt(secret, time.Now())
	if _, err := model.EnableUserTotp(user.Id, code, time.Now()); err != nil {
		t.Fatal(err)
	}
	common.RegisterVerificationCodeWithKey("walt@example.test", "abc123", common.LoginCodePurpose)
	t.Cleanup(func() { common.DeleteKey("walt@example.test", common.LoginCodePurpose) })

	_, resp := postJSON(t, r, "/login", `{"email":"walt@example.test","code":"abc123"}`)
	if !totpRequired(resp) {
		t.Fatalf("email code sign-in should ask for the second factor: %v", resp)
	}
	var sessionCount int64
	model.DB.Model(&model.UserSession{}).Where("user_id = ?", user.Id).Count(&sessionCount)
	if sessionCount != 0 {
		t.Fatal("no session before the second factor")
	}
}
