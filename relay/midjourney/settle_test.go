package midjourney

import (
	"net/http/httptest"
	"testing"

	provider "github.com/modeltaps/modeltaps/providers/midjourney"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

type fakeReservation struct {
	consumed, undone int
	charged          int
}

func (f *fakeReservation) Consume(*gin.Context, *types.Usage, bool) { f.consumed++ }
func (f *fakeReservation) Undo(*gin.Context)                        { f.undone++ }
func (f *fakeReservation) ChargedQuota() int {
	if f.consumed == 0 {
		return 0
	}
	return f.charged
}

func newSettleContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/mj/submit/action", nil)
	return c
}

// SEC-63: the task records what the settlement charged, so an uncharged submission records 0
// and its later failure refunds nothing.
func TestMjSettlerRecordsNothingWhenNotCharged(t *testing.T) {
	c := newSettleContext()
	res := &fakeReservation{charged: 500}
	var s mjSettler
	if got := s.settle(c, res, false); got != 0 {
		t.Fatalf("uncharged submission recorded %d, want 0", got)
	}
	// The deferred call after the explicit one must not settle again.
	if got := s.settle(c, res, true); got != 0 {
		t.Fatalf("second settle recorded %d, want 0", got)
	}
	if res.undone != 1 || res.consumed != 0 {
		t.Fatalf("undo=%d consume=%d, want one refund and no charge", res.undone, res.consumed)
	}
}

func TestMjSettlerRecordsTheCharge(t *testing.T) {
	c := newSettleContext()
	res := &fakeReservation{charged: 500}
	var s mjSettler
	if got := s.settle(c, res, true); got != 500 {
		t.Fatalf("charged submission recorded %d, want 500", got)
	}
	if got := s.settle(c, res, false); got != 500 {
		t.Fatalf("second settle recorded %d, want 500", got)
	}
	if res.consumed != 1 || res.undone != 0 {
		t.Fatalf("consume=%d undo=%d, want exactly one charge", res.consumed, res.undone)
	}
}

func TestMjActionCharged(t *testing.T) {
	for _, action := range []string{provider.MjActionInPaint, provider.MjActionCustomZoom} {
		if mjActionCharged(action) {
			t.Errorf("%s only opens the modal and must not be charged", action)
		}
	}
	for _, action := range []string{provider.MjActionModal, provider.MjActionImagine, provider.MjActionUpscale,
		provider.MjActionVariation, provider.MjActionZoom} {
		if !mjActionCharged(action) {
			t.Errorf("%s must be charged", action)
		}
	}
}
