package base

import (
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

func TestObserveUpstreamModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("new_model", "gpt-6.1-sol")

	// A handler that only knows the request model reports the mapped name first.
	ObserveUpstreamModel(c, "gpt-6.1-sol")
	// The upstream's own declaration replaces it.
	ObserveUpstreamModel(c, "gpt-4o-mini")
	// Later echoes of the request model do not hide the swap.
	ObserveUpstreamModel(c, "gpt-6.1-sol")
	if got := c.GetString(config.GinUpstreamResponseModelKey); got != "gpt-4o-mini" {
		t.Fatalf("observed = %q, want gpt-4o-mini", got)
	}
}
