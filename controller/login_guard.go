package controller

import (
	"net"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

// Failed sign-ins (a wrong password, a wrong email code, a wrong two-factor code) are counted
// under three keys at once, each with its own threshold within the lockout window
// (login.lockout_minutes, 15 by default, counted from the latest failure):
//
//   - account + IP: login.max_failures (5). The usual "you mistyped too often" lock, and it only
//     shuts out the address that failed: someone guessing from one address cannot lock the owner
//     out, however long they keep trying.
//   - account, any IP: 4 x login.max_failures (20). Stops a botnet that rotates addresses. One
//     address adds at most 5 failures before its own lock refuses it (refused attempts are not
//     counted), so locking an account for everyone takes at least four addresses, and an
//     account sees at most 20 guesses per window however many addresses try.
//   - IP, any account: 4 x login.max_failures (20). Stops one address spraying a few guesses at
//     many accounts.
//
// Every lock answers exactly like a wrong password, so neither the response nor its timing says
// which lock applies or whether the account exists. A successful sign-in clears the account
// counters but not the IP one: otherwise signing in to one's own account between guesses would
// reset it.
const (
	loginAccountFailureFactor = 4
	loginIPFailureFactor      = 4
)

type loginGuard struct {
	accountKey string // common.LoginFailureKeyForUser / ForIdentifier
	pairKey    string
	ipKey      string
}

// newLoginGuard builds the counters for a sign-in to accountKey from the request's address.
func newLoginGuard(c *gin.Context, accountKey string) loginGuard {
	ip := loginFailureIP(c.ClientIP())
	g := loginGuard{accountKey: accountKey}
	if ip != "" {
		g.ipKey = "ip:" + ip
		if accountKey != "" {
			g.pairKey = accountKey + "|ip:" + ip
		}
	}
	return g
}

// loginFailureIP is the address failures are counted under. IPv6 counts per /64: one host usually
// holds a whole /64 and could otherwise rotate through it.
func loginFailureIP(clientIP string) string {
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return clientIP
	}
	if ip.To4() != nil {
		return ip.To4().String()
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

// locked reports whether any of the three counters has reached its threshold.
func (g loginGuard) locked() bool {
	limit := config.LoginMaxFailures
	return common.IsLoginLockedAt(g.pairKey, limit) ||
		common.IsLoginLockedAt(g.accountKey, limit*loginAccountFailureFactor) ||
		common.IsLoginLockedAt(g.ipKey, limit*loginIPFailureFactor)
}

func (g loginGuard) recordFailure() {
	common.RecordLoginFailure(g.pairKey)
	common.RecordLoginFailure(g.accountKey)
	common.RecordLoginFailure(g.ipKey)
}

// clear forgets the account's failures after a complete sign-in; the IP counter is left alone.
func (g loginGuard) clear() {
	common.ClearLoginFailures(g.pairKey)
	common.ClearLoginFailures(g.accountKey)
}
