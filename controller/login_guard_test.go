package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

// testClientIP is the address httptest.NewRequest gives every request.
const testClientIP = "192.0.2.1"

func testPairKey(accountKey string) string { return accountKey + "|ip:" + testClientIP }

// resetLoginGuardState forgets the failures earlier tests left in the in-memory counters: every
// test request comes from testClientIP, and each test's fresh database hands out the same ids.
func resetLoginGuardState(t *testing.T) {
	t.Helper()
	clear := func() {
		common.ClearLoginFailures("ip:" + testClientIP)
		for id := 1; id <= 200; id++ {
			key := common.LoginFailureKeyForUser(id)
			common.ClearLoginFailures(key)
			common.ClearLoginFailures(testPairKey(key))
		}
	}
	clear()
	t.Cleanup(clear)
}

func loginFromIP(t *testing.T, r *gin.Engine, ip, username, password string) map[string]interface{} {
	t.Helper()
	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", body)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return decodeResp(t, w)
}

func TestLoginFailureIPGroupsIPv6By64(t *testing.T) {
	if got := loginFailureIP("2001:db8:1:2:aaaa::1"); got != "2001:db8:1:2::/64" {
		t.Errorf("IPv6 = %q", got)
	}
	if loginFailureIP("2001:db8:1:2::1") != loginFailureIP("2001:db8:1:2:ffff:ffff:ffff:ffff") {
		t.Error("addresses of one /64 should share a counter")
	}
	if got := loginFailureIP("203.0.113.9"); got != "203.0.113.9" {
		t.Errorf("IPv4 = %q", got)
	}
}

// One address that keeps guessing only locks itself out: the owner signs in from elsewhere.
func TestLoginOneIPCannotLockAccountForEveryone(t *testing.T) {
	setupGetUserTestDB(t)
	r := setupLoginRouter(t)
	u := createPasswordUser(t, "grace", "grace-password")
	t.Cleanup(func() {
		for _, ip := range []string{"198.51.100.7", "198.51.100.8"} {
			common.ClearLoginFailures("ip:" + ip)
			common.ClearLoginFailures(common.LoginFailureKeyForUser(u.Id) + "|ip:" + ip)
		}
	})

	for i := 0; i < config.LoginMaxFailures*3; i++ {
		loginFromIP(t, r, "198.51.100.7", "grace", "wrong-password")
	}
	if resp := loginFromIP(t, r, "198.51.100.7", "grace", "grace-password"); resp["success"] != false {
		t.Fatal("the guessing address should be locked out of the account")
	}
	if resp := loginFromIP(t, r, "198.51.100.8", "grace", "grace-password"); resp["success"] != true {
		t.Fatalf("the owner from another address should still sign in, got %v", resp)
	}
}

// Rotating addresses does not bypass the account-wide count: after 4 x max_failures failures
// from different addresses the account refuses even the right password, from a new address too.
func TestLoginRotatingIPsLockTheAccount(t *testing.T) {
	setupGetUserTestDB(t)
	r := setupLoginRouter(t)
	u := createPasswordUser(t, "heidi", "heidi-password")
	accountKey := common.LoginFailureKeyForUser(u.Id)
	var ips []string
	t.Cleanup(func() {
		common.ClearLoginFailures(accountKey)
		for _, ip := range ips {
			common.ClearLoginFailures("ip:" + ip)
			common.ClearLoginFailures(accountKey + "|ip:" + ip)
		}
	})

	total := config.LoginMaxFailures * loginAccountFailureFactor
	for i := 0; i < total; i++ {
		// Fewer failures per address than its own lock needs.
		ip := "203.0.113." + strconv.Itoa(i/(config.LoginMaxFailures-1)+1)
		if len(ips) == 0 || ips[len(ips)-1] != ip {
			ips = append(ips, ip)
		}
		loginFromIP(t, r, ip, "heidi", "wrong-password")
	}
	fresh := "203.0.113.200"
	ips = append(ips, fresh)
	resp := loginFromIP(t, r, fresh, "heidi", "heidi-password")
	if resp["success"] != false || resp["message"] != "incorrect username or password, or the user is banned" {
		t.Fatalf("a botnet's failures should lock the account, got %v", resp)
	}
}

// One address spraying many accounts is stopped by the per-IP count.
func TestLoginOneIPSprayingAccountsIsLocked(t *testing.T) {
	setupGetUserTestDB(t)
	r := setupLoginRouter(t)
	createPasswordUser(t, "ivan", "ivan-password")
	const ip = "198.51.100.50"
	t.Cleanup(func() {
		common.ClearLoginFailures("ip:" + ip)
		for i := 0; i < 30; i++ {
			name := "spray" + strconv.Itoa(i)
			common.ClearLoginFailures(common.LoginFailureKeyForIdentifier(name))
			common.ClearLoginFailures(common.LoginFailureKeyForIdentifier(name) + "|ip:" + ip)
		}
	})

	for i := 0; i < config.LoginMaxFailures*loginIPFailureFactor; i++ {
		// A different (unknown) account each time, so no account or pair lock is reached.
		loginFromIP(t, r, ip, "spray"+strconv.Itoa(i), "guess")
	}
	if resp := loginFromIP(t, r, ip, "ivan", "ivan-password"); resp["success"] != false {
		t.Fatal("an address over the per-IP limit should be refused")
	}
	if resp := loginFromIP(t, r, "198.51.100.51", "ivan", "ivan-password"); resp["success"] != true {
		t.Fatalf("other addresses are not affected, got %v", resp)
	}
}
