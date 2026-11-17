package midjourney

import (
	provider "github.com/modeltaps/modeltaps/providers/midjourney"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// mjUsage is the usage of one Midjourney submission: every action is billed per request.
var mjUsage = types.Usage{CompletionTokens: 0, PromptTokens: 1, TotalTokens: 1}

// mjReservation is the part of relay_util.Quota a submission settles.
type mjReservation interface {
	Consume(c *gin.Context, usage *types.Usage, isStream bool)
	Undo(c *gin.Context)
	ChargedQuota() int
}

// mjSettler settles a submission's reservation once and remembers what it charged. The task row
// stores that amount, and the task poller refunds exactly it when the task fails, so a refund can
// never exceed the charge (SEC-63).
type mjSettler struct {
	done    bool
	charged int
}

// settle charges the reservation when charge is true and refunds it otherwise, on the first call
// only; it returns the quota the account was charged (0 when refunded).
func (s *mjSettler) settle(c *gin.Context, reservation mjReservation, charge bool) int {
	if s.done {
		return s.charged
	}
	s.done = true
	if charge {
		usage := mjUsage
		reservation.Consume(c, &usage, false)
		s.charged = reservation.ChargedQuota()
	} else {
		reservation.Undo(c)
	}
	return s.charged
}

// mjActionCharged reports whether submitting the action is billed. INPAINT and CUSTOM_ZOOM only
// open the upstream's input dialog (the task comes back in MODAL state); the work is submitted
// with /mj/submit/modal and billed there as a MODAL action, so charging the first step as well
// would bill the edit twice.
func mjActionCharged(action string) bool {
	return action != provider.MjActionInPaint && action != provider.MjActionCustomZoom
}
