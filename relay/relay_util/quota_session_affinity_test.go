package relay_util

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

func TestGetLogMetaSessionAffinity(t *testing.T) {
	q := &Quota{price: model.Price{Type: model.TokensPriceType}, sessionAffinity: "hit"}
	if got := q.GetLogMeta(&types.Usage{})["session_affinity"]; got != "hit" {
		t.Fatalf("session_affinity = %v, want hit", got)
	}
	q.sessionAffinity = ""
	if _, ok := q.GetLogMeta(&types.Usage{})["session_affinity"]; ok {
		t.Fatal("session_affinity written without a session")
	}
}
