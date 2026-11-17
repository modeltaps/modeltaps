package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// signInTestSession signs user in the way setupLoginSession does (a session row plus its key in
// the cookie) through a test-only route on r, and returns the cookie header to send.
func signInTestSession(t *testing.T, r *gin.Engine, user *model.User) string {
	t.Helper()
	userSession, err := model.CreateUserSession(user.Id, model.SessionMethodPassword, "test", "127.0.0.1", 0, "")
	if err != nil {
		t.Fatalf("create session row: %v", err)
	}
	path := "/test/sign-in/" + userSession.SessionKey
	r.GET(path, func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("sid", userSession.SessionKey)
		session.Set("id", user.Id)
		session.Set("username", user.Username)
		session.Set("role", user.Role)
		session.Set("status", user.Status)
		_ = session.Save()
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	cookie := w.Header().Get("Set-Cookie")
	if cookie == "" {
		t.Fatal("test sign-in set no cookie")
	}
	return cookie
}

func seedGitHubTestUser(t *testing.T, username string, githubLogin string, githubId int) *model.User {
	t.Helper()
	return seedEmailUser(t, &model.User{
		Username: username, GitHubId: githubLogin, GitHubIdNew: githubId,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AffCode: "aff-" + username,
	})
}

func reloadUser(t *testing.T, id int) *model.User {
	t.Helper()
	user, err := model.GetUserById(id, false)
	if err != nil {
		t.Fatalf("load user %d: %v", id, err)
	}
	return user
}

// SEC-59: the OAuth callbacks are not behind UserAuth. A cookie whose session row was revoked
// (signed out, password changed) still carries the user id, and must not reach the linking
// branch: it signs in with the provider identity instead, and loses its sign-in values.
func TestGitHubOAuthRevokedSessionDoesNotLink(t *testing.T) {
	setupGetUserTestDB(t)
	enableGitHubLogin(t)
	victim := seedGitHubTestUser(t, "victim", "", 0)
	stubGitHubUserInfo(t, &GitHubUser{Id: 7100, Login: "intruder"})

	r := newGitHubRouter(t)
	cookie := signInTestSession(t, r, victim)
	if err := model.DeleteUserSessionsExcept(nil, victim.Id, ""); err != nil {
		t.Fatalf("revoke sessions: %v", err)
	}

	resp := githubOAuthLogin(t, r, cookie)
	if resp["message"] == "bind" {
		t.Fatalf("a revoked session must not link, got %v", resp)
	}
	stored := reloadUser(t, victim.Id)
	if stored.GitHubIdNew != 0 || stored.GitHubId != "" {
		t.Fatalf("victim account got the GitHub identity linked: id=%d login=%q", stored.GitHubIdNew, stored.GitHubId)
	}
	data, _ := resp["data"].(map[string]interface{})
	if resp["success"] != true || data == nil || int(data["id"].(float64)) == victim.Id {
		t.Fatalf("the callback should sign in to a new account, got %v", resp)
	}
}

// The same flow with a live session links the GitHub account, so the test above fails for the
// revocation and nothing else.
func TestGitHubOAuthLiveSessionLinks(t *testing.T) {
	setupGetUserTestDB(t)
	enableGitHubLogin(t)
	user := seedGitHubTestUser(t, "linker", "", 0)
	stubGitHubUserInfo(t, &GitHubUser{Id: 7200, Login: "linker-gh"})

	r := newGitHubRouter(t)
	resp := githubOAuthLogin(t, r, signInTestSession(t, r, user))
	if resp["success"] != true || resp["message"] != "bind" {
		t.Fatalf("a live session should link, got %v", resp)
	}
	if stored := reloadUser(t, user.Id); stored.GitHubIdNew != 7200 || stored.GitHubId != "linker-gh" {
		t.Fatalf("GitHub identity not linked: id=%d login=%q", stored.GitHubIdNew, stored.GitHubId)
	}
}

// oauthSessionUserId returns the user only while the session row is valid. A revoked cookie loses
// its sign-in values but keeps the rest (here the OAuth state of the flow under way).
func TestOauthSessionUserIdChecksSessionRow(t *testing.T) {
	setupGetUserTestDB(t)
	user := seedGitHubTestUser(t, "sessioner", "", 0)

	r := newGitHubRouter(t)
	r.GET("/test/whoami", func(c *gin.Context) {
		userId, ok := oauthSessionUserId(c)
		session := sessions.Default(c)
		_, hasId := session.Get("id").(int)
		_, hasState := session.Get("oauth_state").(string)
		c.JSON(http.StatusOK, gin.H{"ok": ok, "id": userId, "has_id": hasId, "has_state": hasState})
	})
	whoami := func(cookie string) (map[string]interface{}, string) {
		req := httptest.NewRequest(http.MethodGet, "/test/whoami", nil)
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return resp, w.Header().Get("Set-Cookie")
	}

	if resp, _ := whoami(""); resp["ok"] != true || resp["id"].(float64) != 0 {
		t.Fatalf("no cookie means signed out, got %v", resp)
	}

	cookie := signInTestSession(t, r, user)
	// Add OAuth state to the signed-in cookie, as /api/oauth/state does before the redirect.
	stateReq := httptest.NewRequest(http.MethodGet, "/api/oauth/state", nil)
	stateReq.Header.Set("Cookie", cookie)
	stateW := httptest.NewRecorder()
	r.ServeHTTP(stateW, stateReq)
	cookie = stateW.Header().Get("Set-Cookie")

	if resp, _ := whoami(cookie); resp["ok"] != true || int(resp["id"].(float64)) != user.Id {
		t.Fatalf("a live session should return its user, got %v", resp)
	}

	if err := model.DeleteUserSessionsExcept(nil, user.Id, ""); err != nil {
		t.Fatalf("revoke sessions: %v", err)
	}
	resp, cleared := whoami(cookie)
	if resp["ok"] != true || resp["id"].(float64) != 0 || resp["has_id"] != false {
		t.Fatalf("a revoked session should read as signed out, got %v", resp)
	}
	if resp["has_state"] != true {
		t.Fatalf("the OAuth state should survive, got %v", resp)
	}
	if cleared == "" {
		t.Fatal("the cookie without its sign-in values should be sent back")
	}

	// A cookie with an id but no session key (issued before session rows existed, or forged
	// from a leaked secret) is not signed in either.
	r.GET("/test/bare", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		session.Set("username", user.Username)
		_ = session.Save()
		c.Status(http.StatusOK)
	})
	bareW := httptest.NewRecorder()
	r.ServeHTTP(bareW, httptest.NewRequest(http.MethodGet, "/test/bare", nil))
	if resp, _ := whoami(bareW.Header().Get("Set-Cookie")); resp["id"].(float64) != 0 {
		t.Fatalf("a cookie without a session key should read as signed out, got %v", resp)
	}
}

// SEC-58: a GitHub login name is not a stable identity. An account that stores a GitHub id is
// matched by that id only, so someone who registers the login after its owner renamed it does
// not get the account.
func TestGetUserByGitHubIgnoresLoginOfAccountWithId(t *testing.T) {
	setupGetUserTestDB(t)
	seedGitHubTestUser(t, "renamed", "oldname", 7300)

	matched, err := getUserByGitHub(&GitHubUser{Id: 7301, Login: "oldname"})
	if err != nil {
		t.Fatalf("getUserByGitHub: %v", err)
	}
	if matched != nil {
		t.Fatalf("another GitHub account with the old login must not match, got user_id=%d", matched.Id)
	}
}

// End to end: the squatter's sign-in creates a new account and leaves the victim's link alone.
func TestGitHubOAuthSquattedLoginDoesNotTakeOver(t *testing.T) {
	setupGetUserTestDB(t)
	enableGitHubLogin(t)
	victim := seedGitHubTestUser(t, "ghvictim2", "oldname", 7400)
	stubGitHubUserInfo(t, &GitHubUser{Id: 7401, Login: "oldname"})

	resp := githubOAuthLogin(t, newGitHubRouter(t), "")
	data, _ := resp["data"].(map[string]interface{})
	if data != nil && int(data["id"].(float64)) == victim.Id {
		t.Fatalf("the squatter signed in to the victim account: %v", resp)
	}
	if stored := reloadUser(t, victim.Id); stored.GitHubIdNew != 7400 {
		t.Fatalf("victim GitHub id overwritten with %d", stored.GitHubIdNew)
	}
}

// Legacy accounts that stored only the login name are matched by it once: the sign-in stores the
// GitHub id, and from then on only that id matches.
func TestGitHubOAuthMigratesLegacyLoginOnce(t *testing.T) {
	setupGetUserTestDB(t)
	enableGitHubLogin(t)
	legacy := seedGitHubTestUser(t, "legacy", "legacy-gh", 0)
	stubGitHubUserInfo(t, &GitHubUser{Id: 7500, Login: "legacy-gh"})

	resp := githubOAuthLogin(t, newGitHubRouter(t), "")
	data, _ := resp["data"].(map[string]interface{})
	if resp["success"] != true || data == nil || int(data["id"].(float64)) != legacy.Id {
		t.Fatalf("the legacy account should be signed in, got %v", resp)
	}
	if stored := reloadUser(t, legacy.Id); stored.GitHubIdNew != 7500 {
		t.Fatalf("the GitHub id should be stored on the legacy account, got %d", stored.GitHubIdNew)
	}

	matched, err := getUserByGitHub(&GitHubUser{Id: 7501, Login: "legacy-gh"})
	if err != nil {
		t.Fatalf("getUserByGitHub: %v", err)
	}
	if matched != nil {
		t.Fatalf("after migration the login must not match another GitHub account, got user_id=%d", matched.Id)
	}
}

// Two legacy accounts with the same login cannot be told apart, and the admin switch turns the
// login fallback off entirely.
func TestGetUserByGitHubLegacyLoginLimits(t *testing.T) {
	setupGetUserTestDB(t)
	seedGitHubTestUser(t, "dup1", "dup-gh", 0)
	seedGitHubTestUser(t, "dup2", "dup-gh", 0)
	if matched, err := getUserByGitHub(&GitHubUser{Id: 7600, Login: "dup-gh"}); err != nil || matched != nil {
		t.Fatalf("an ambiguous legacy login must not match, got %v err=%v", matched, err)
	}

	solo := seedGitHubTestUser(t, "solo", "solo-gh", 0)
	if matched, err := getUserByGitHub(&GitHubUser{Id: 7601, Login: "solo-gh"}); err != nil || matched == nil || matched.Id != solo.Id {
		t.Fatalf("a single legacy account should match, got %v err=%v", matched, err)
	}
	old := config.GitHubOldIdCloseEnabled
	config.GitHubOldIdCloseEnabled = true
	t.Cleanup(func() { config.GitHubOldIdCloseEnabled = old })
	if matched, err := getUserByGitHub(&GitHubUser{Id: 7601, Login: "solo-gh"}); err != nil || matched != nil {
		t.Fatalf("with the login fallback turned off nothing should match, got %v err=%v", matched, err)
	}
}

// Linking checks the GitHub id too: one GitHub account must not be linked to two accounts, even
// when it was renamed since it was linked first.
func TestGitHubBindRejectsIdLinkedElsewhere(t *testing.T) {
	setupGetUserTestDB(t)
	enableGitHubLogin(t)
	seedGitHubTestUser(t, "owner", "before-rename", 7700)
	other := seedGitHubTestUser(t, "other", "", 0)
	stubGitHubUserInfo(t, &GitHubUser{Id: 7700, Login: "after-rename"})

	r := newGitHubRouter(t)
	resp := githubOAuthLogin(t, r, signInTestSession(t, r, other))
	if resp["success"] != false {
		t.Fatalf("linking a GitHub id linked elsewhere should fail, got %v", resp)
	}
	if stored := reloadUser(t, other.Id); stored.GitHubIdNew != 0 {
		t.Fatalf("the GitHub id was linked a second time: %d", stored.GitHubIdNew)
	}
}

// A stale login left on an account that stores another GitHub id does not block linking: that
// account is matched by its id, never by the login.
func TestGitHubBindIgnoresStaleLoginOfAccountWithId(t *testing.T) {
	setupGetUserTestDB(t)
	enableGitHubLogin(t)
	seedGitHubTestUser(t, "renamer", "taken-name", 7800)
	binder := seedGitHubTestUser(t, "binder2", "", 0)
	stubGitHubUserInfo(t, &GitHubUser{Id: 7801, Login: "taken-name"})

	r := newGitHubRouter(t)
	resp := githubOAuthLogin(t, r, signInTestSession(t, r, binder))
	if resp["success"] != true || resp["message"] != "bind" {
		t.Fatalf("linking should succeed, got %v", resp)
	}
	if stored := reloadUser(t, binder.Id); stored.GitHubIdNew != 7801 {
		t.Fatalf("GitHub id not linked: %d", stored.GitHubIdNew)
	}
}

// SEC-53 applies to the legacy login fallback too: an admin or root account is never matched by
// the login name. The sign-in is refused (no account is created that would take the GitHub id),
// and the admin can still sign in otherwise and link GitHub explicitly.
func TestGitHubLegacyLoginNeverMatchesPrivilegedAccount(t *testing.T) {
	for _, role := range []int{config.RoleAdminUser, config.RoleRootUser} {
		setupGetUserTestDB(t)
		enableGitHubLogin(t)
		admin := seedEmailUser(t, &model.User{
			Username: "legacy-admin", GitHubId: "admin-gh", Role: role, Status: config.UserStatusEnabled,
			Type: config.UserTypeNormal, AffCode: "aff-legacy-admin",
		})
		githubUser := &GitHubUser{Id: 7900, Login: "admin-gh"}

		if matched, err := getUserByGitHub(githubUser); matched != nil || !errors.Is(err, errGitHubLegacyPrivileged) {
			t.Fatalf("role %d: the login must not match a privileged account, got %v err=%v", role, matched, err)
		}

		stubGitHubUserInfo(t, githubUser)
		r := newGitHubRouter(t)
		if resp := githubOAuthLogin(t, r, ""); resp["success"] != false {
			t.Fatalf("role %d: the sign-in should be refused, got %v", role, resp)
		}
		var count int64
		if err := model.DB.Model(&model.User{}).Where("github_id_new = ?", githubUser.Id).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("role %d: no account should take the GitHub id, got %d err=%v", role, count, err)
		}

		resp := githubOAuthLogin(t, r, signInTestSession(t, r, admin))
		if resp["success"] != true || resp["message"] != "bind" {
			t.Fatalf("role %d: the signed-in admin should link GitHub explicitly, got %v", role, resp)
		}
		if stored := reloadUser(t, admin.Id); stored.GitHubIdNew != githubUser.Id {
			t.Fatalf("role %d: GitHub id not linked: %d", role, stored.GitHubIdNew)
		}
	}
}
