package model

import (
	"reflect"
	"testing"

	"github.com/modeltaps/modeltaps/common"
)

func fallbackPolicy(models ...string) *RoutingPolicy {
	return &RoutingPolicy{Name: "auto/steady", Type: RoutingPolicyTypeFallback, Models: models}
}


func TestFallbackPolicyNormalizeClearsAutoFields(t *testing.T) {
	p := fallbackPolicy(" a ", "", "b")
	p.DecisionModel, p.Question, p.FallbackModel = "d", "q", "f"
	p.Routes = []RoutingRoute{{Label: "x", TargetModel: "y"}}
	p.Normalize()
	if !reflect.DeepEqual([]string(p.Models), []string{"a", "b"}) {
		t.Fatalf("models = %v", p.Models)
	}
	if p.DecisionModel != "" || p.Question != "" || p.FallbackModel != "" || p.Routes != nil {
		t.Fatalf("auto fields kept: %+v", p)
	}
	if err := p.Validate(nil); err != nil {
		t.Fatalf("valid fallback policy rejected: %v", err)
	}
}

func TestAutoPolicyNormalizeDropsModels(t *testing.T) {
	p := &RoutingPolicy{Name: "auto/x", Models: []string{"a"}}
	p.Normalize()
	if p.Type != RoutingPolicyTypeAuto || p.Models != nil {
		t.Fatalf("type=%q models=%v", p.Type, p.Models)
	}
}

func TestFallbackPolicyValidate(t *testing.T) {
	cases := []struct {
		name   string
		policy *RoutingPolicy
		code   string
	}{
		{"one model", fallbackPolicy("a"), common.APICodeRoutingPolicyModelsCount},
		{"nine models", fallbackPolicy("a", "b", "c", "d", "e", "f", "g", "h", "i"), common.APICodeRoutingPolicyModelsCount},
		{"duplicate", fallbackPolicy("a", "A"), common.APICodeRoutingPolicyModelDuplicate},
		{"itself", fallbackPolicy("a", "auto/steady"), common.APICodeRoutingPolicyModelIsPolicy},
		{"unknown type", &RoutingPolicy{Name: "auto/x", Type: "weighted"}, common.APICodeRoutingPolicyTypeInvalid},
	}
	for _, tc := range cases {
		tc.policy.Normalize()
		if got := apiCode(t, tc.policy.Validate(nil)); got != tc.code {
			t.Errorf("%s: code %q, want %q", tc.name, got, tc.code)
		}
	}
	nested := fallbackPolicy("a", "other-policy")
	nested.Normalize()
	if got := apiCode(t, nested.Validate(func(name string) bool { return name == "other-policy" })); got != common.APICodeRoutingPolicyModelIsPolicy {
		t.Errorf("another policy as a model: code %q", got)
	}
}

func TestRoutingPolicyChainAndAnchor(t *testing.T) {
	fb := fallbackPolicy("a", "b", "c")
	if got := fb.Chain(""); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("fallback chain = %v", got)
	}
	if got := fb.Chain("b"); !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Fatalf("fallback chain from b = %v", got)
	}
	if fb.AnchorModel() != "a" || !reflect.DeepEqual(fb.TargetModels(), []string{"a", "b", "c"}) {
		t.Fatalf("anchor %q targets %v", fb.AnchorModel(), fb.TargetModels())
	}
	auto := &RoutingPolicy{Type: RoutingPolicyTypeAuto, FallbackModel: "big"}
	if got := auto.Chain("cheap"); !reflect.DeepEqual(got, []string{"cheap", "big"}) {
		t.Fatalf("auto chain = %v", got)
	}
	if got := auto.Chain("big"); !reflect.DeepEqual(got, []string{"big"}) {
		t.Fatalf("auto chain to the fallback = %v", got)
	}
	if auto.AnchorModel() != "big" {
		t.Fatalf("auto anchor = %q", auto.AnchorModel())
	}
}
