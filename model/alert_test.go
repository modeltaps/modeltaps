package model

import (
	"reflect"
	"testing"
)

func useAlertTestDB(t *testing.T) {
	t.Helper()
	useSupplyTestDB(t)
	if err := DB.AutoMigrate(&Alert{}); err != nil {
		t.Fatal(err)
	}
}

func TestReportAlertTellsOnlyNewItems(t *testing.T) {
	useAlertTestDB(t)
	step := func(items []string, want []string) {
		t.Helper()
		got, err := ReportAlert("model_drift", "channel:4", items)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("report %v: new = %v, want %v", items, got, want)
		}
	}
	step([]string{"gone:a", "gone:b"}, []string{"gone:a", "gone:b"})
	step([]string{"gone:b", "gone:a"}, nil) // the same condition again
	step([]string{"gone:a"}, nil)           // it shrinks
	step([]string{"gone:a", "gone:c"}, []string{"gone:c"})
	step(nil, nil) // resolved
	open, err := OpenAlerts("model_drift")
	if err != nil || len(open) != 0 {
		t.Fatalf("open = %v, %v", open, err)
	}
	step([]string{"gone:a"}, []string{"gone:a"}) // back after it was resolved: told again
}

func TestResolveAlertsExcept(t *testing.T) {
	useAlertTestDB(t)
	for _, s := range []string{"channel:1", "channel:2"} {
		if _, err := ReportAlert("credential", s, []string{"x"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := ResolveAlertsExcept("credential", []string{"channel:2"}); err != nil {
		t.Fatal(err)
	}
	open, _ := OpenAlerts("credential")
	if len(open) != 1 || open[0].Subject != "channel:2" {
		t.Fatalf("open = %+v", open)
	}
}
