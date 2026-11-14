package relay_util

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func TestGetLogMetaClientCancelled(t *testing.T) {
	q := &Quota{}
	meta := q.GetLogMeta(&types.Usage{})
	if _, ok := meta["client_cancelled"]; ok {
		t.Error("client_cancelled should be absent by default")
	}
	if _, ok := meta["usage_estimated"]; ok {
		t.Error("usage_estimated should be absent by default")
	}

	q.clientCancelled, q.usageEstimated = true, true
	meta = q.GetLogMeta(&types.Usage{})
	if meta["client_cancelled"] != true || meta["usage_estimated"] != true {
		t.Errorf("flags missing from meta: %v", meta)
	}
}
