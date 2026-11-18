package model

import (
	"reflect"
	"testing"
)

func change(model, kind string, oldIn, oldOut, newIn, newOut float64) PriceChange {
	return PriceChange{Model: model, Kind: kind, OldInput: oldIn, OldOutput: oldOut, NewInput: newIn, NewOutput: newOut, price: &Price{Model: model}}
}

func everyModel(string) bool { return true }

func TestPlanPriceAlertWaitsForTheNextSync(t *testing.T) {
	held := change("gpt-5.5", PriceChangeLowered, 5, 30, 4, 24)
	first := planPriceAlert(&PriceSyncReport{Held: []PriceChange{held}}, everyModel, nil)
	if len(first.items()) != 0 || len(first.pending) != 1 {
		t.Fatalf("first sync tells nothing and keeps it pending: %+v", first)
	}
	second := planPriceAlert(&PriceSyncReport{Held: []PriceChange{held}}, everyModel, first.pending)
	if len(second.held) != 1 {
		t.Fatalf("the next sync that still finds it tells it: %+v", second)
	}
}

func TestPlanPriceAlertSkipsSmallAndUnservedChanges(t *testing.T) {
	small := change("a", PriceChangeLowered, 10, 10, 9.5, 10)
	unserved := change("b", PriceChangeLowered, 10, 10, 1, 1)
	served := func(m string) bool { return m != "b" }
	plan := planPriceAlert(&PriceSyncReport{Held: []PriceChange{small, unserved}}, served, nil)
	if len(plan.pending) != 0 {
		t.Fatalf("pending = %v", plan.pending)
	}
}

func TestPlanPriceAlertUndoesASpikeThatCameBack(t *testing.T) {
	raise := change("glm-5.3", PriceChangeRaised, 1, 3, 2, 6)
	first := planPriceAlert(&PriceSyncReport{Applied: []PriceChange{raise}}, everyModel, nil)
	if len(first.items()) != 0 || len(first.pending) != 1 {
		t.Fatalf("an applied increase waits for the next sync: %+v", first)
	}
	back := change("glm-5.3", PriceChangeLowered, 2, 6, 1, 3)
	second := planPriceAlert(&PriceSyncReport{Held: []PriceChange{back}}, everyModel, first.pending)
	if len(second.items()) != 0 || len(second.pending) != 0 {
		t.Fatalf("a spike that came back is told nowhere: %+v", second)
	}
	if len(second.revert) != 1 || second.revert[0].Model != "glm-5.3" {
		t.Fatalf("the held cut back to the old price is written: %+v", second.revert)
	}
	stood := planPriceAlert(&PriceSyncReport{}, everyModel, first.pending)
	if !reflect.DeepEqual(stood.items(), []string{"raised|glm-5.3|1|3|2|6"}) {
		t.Fatalf("an increase the next sync leaves alone is told: %v", stood.items())
	}
}

func TestPlanPriceAlertTellsLockedLossAtOnce(t *testing.T) {
	locked := change("hy3", PriceChangeRaised, 1, 1, 1.05, 1)
	plan := planPriceAlert(&PriceSyncReport{Locked: []PriceChange{locked}}, everyModel, nil)
	if len(plan.locked) != 1 {
		t.Fatalf("locked = %+v", plan.locked)
	}
}
