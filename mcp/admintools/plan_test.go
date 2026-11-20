package admintools

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

func TestPlanChanges(t *testing.T) {
	model.RegisterOptions()
	plan := PlanChanges([]ProposedChange{
		{Key: "RetryTimes", Value: "4", Reason: "four channels"},
		{Key: "RetryTimes", Value: "-1"},
		{Key: "SessionAffinityStrict", Value: "False"},
		{Key: "SupplyModeDefault", Value: "round-robin"},
		{Key: "PasswordLoginEnabled", Value: "false"},
		{Key: "PaymentUSDRate", Value: "7"},
		{Key: "GitHubClientSecret", Value: "x"},
		{Key: "ServerAddress", Value: "https://x"},
		{Key: "channel.weight", Value: "3"},
		{Key: "NoSuchSetting", Value: "1"},
		{Key: "PauseRules", Value: "{not json"},
	})
	want := []struct {
		allowed bool
		value   string
	}{
		{true, "4"}, {false, ""}, {true, "false"}, {false, ""}, {false, ""}, {false, ""}, {false, ""}, {false, ""}, {false, ""}, {false, ""}, {false, ""},
	}
	for i, w := range want {
		p := plan[i]
		if p.Allowed != w.allowed || (w.allowed && p.Value != w.value) || (!p.Allowed && p.Blocked == "") {
			t.Errorf("%d %s: %+v, want allowed=%v value=%q", i, p.Key, p, w.allowed, w.value)
		}
	}
	if plan[0].Label == "" || plan[0].ConsoleURL == "" || plan[0].Current == "" || plan[0].Reason != "four channels" {
		t.Errorf("an allowed change misses its details: %+v", plan[0])
	}
	if same := PlanChanges([]ProposedChange{{Key: "RetryTimes", Value: plan[0].Current}}); !same[0].Unchanged {
		t.Errorf("the current value is not marked unchanged: %+v", same[0])
	}
}
