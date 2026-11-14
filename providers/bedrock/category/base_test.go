package category

import "testing"

func TestGetModelNameRegionPrefix(t *testing.T) {
	cases := []struct {
		model, region, want string
	}{
		// Region roots with their own entry.
		{"claude-opus-5-5", "us-east-1", "us.anthropic.claude-opus-5-5"},
		{"claude-opus-5-5", "eu-west-1", "eu.anthropic.claude-opus-5-5"},
		{"claude-opus-5-5", "ap-northeast-1", "global.anthropic.claude-opus-5-5"},
		// fable-5 and fable-5-1: US geo profile (also for Canada), global elsewhere.
		{"claude-fable-5", "us-east-1", "us.anthropic.claude-fable-5"},
		{"claude-fable-5", "ca-central-1", "us.anthropic.claude-fable-5"},
		{"claude-fable-5", "ca-west-1", "us.anthropic.claude-fable-5"},
		{"claude-fable-5", "eu-central-1", "global.anthropic.claude-fable-5"},
		{"claude-fable-5-1", "us-east-1", "us.anthropic.claude-fable-5-1"},
		{"claude-fable-5-1", "ca-central-1", "us.anthropic.claude-fable-5-1"},
		{"claude-fable-5-1", "ca-west-1", "us.anthropic.claude-fable-5-1"},
		{"claude-fable-5-1", "eu-central-1", "global.anthropic.claude-fable-5-1"},
		// fable-5-2: US geo profile (also for Canada), global elsewhere.
		{"claude-fable-5-2", "us-east-1", "us.anthropic.claude-fable-5-2"},
		{"claude-fable-5-2", "ca-central-1", "us.anthropic.claude-fable-5-2"},
		{"claude-fable-5-2", "ca-west-1", "us.anthropic.claude-fable-5-2"},
		{"claude-fable-5-2", "eu-central-1", "global.anthropic.claude-fable-5-2"},
		{"claude-fable-5-2", "ap-northeast-1", "global.anthropic.claude-fable-5-2"},
		// sonnet-4-6, opus-5 and opus-5-5: US geo profile also for Canada, EU geo profile in the EU.
		{"claude-sonnet-4-6", "ca-central-1", "us.anthropic.claude-sonnet-4-6"},
		{"claude-sonnet-4-6", "ca-west-1", "us.anthropic.claude-sonnet-4-6"},
		{"claude-sonnet-4-6", "eu-west-1", "eu.anthropic.claude-sonnet-4-6"},
		{"claude-opus-5", "ca-central-1", "us.anthropic.claude-opus-5"},
		{"claude-opus-5", "ca-west-1", "us.anthropic.claude-opus-5"},
		{"claude-opus-5", "eu-central-1", "eu.anthropic.claude-opus-5"},
		{"claude-opus-5-5", "ca-central-1", "us.anthropic.claude-opus-5-5"},
		{"claude-opus-5-5", "ca-west-1", "us.anthropic.claude-opus-5-5"},
		// sonnet-5: geo profiles in the US (also Canada) and the EU, global in AP and elsewhere.
		{"claude-sonnet-5", "us-east-1", "us.anthropic.claude-sonnet-5"},
		{"claude-sonnet-5", "ca-central-1", "us.anthropic.claude-sonnet-5"},
		{"claude-sonnet-5", "ca-west-1", "us.anthropic.claude-sonnet-5"},
		{"claude-sonnet-5", "eu-central-1", "eu.anthropic.claude-sonnet-5"},
		{"claude-sonnet-5", "eu-west-2", "eu.anthropic.claude-sonnet-5"},
		{"claude-sonnet-5", "ap-southeast-1", "global.anthropic.claude-sonnet-5"},
		{"claude-sonnet-5", "sa-east-1", "global.anthropic.claude-sonnet-5"},
		// "*" fallback for roots without an entry.
		{"claude-haiku-4-5-20251001", "sa-east-1", "global.anthropic.claude-haiku-4-5-20251001-v1:0"},
		{"claude-sonnet-5-5", "us-east-1", "global.anthropic.claude-sonnet-5-5"},
		// Canada uses the US geo profile where the model supports it there.
		{"claude-opus-4-7", "ca-central-1", "us.anthropic.claude-opus-4-7"},
		{"claude-opus-4-5-20251101", "ca-west-1", "global.anthropic.claude-opus-4-5-20251101-v1:0"},
		// haiku-5-5 always needs a profile: geo in us/ca/eu, global elsewhere.
		{"claude-haiku-5-5", "us-east-1", "us.anthropic.claude-haiku-5-5"},
		{"claude-haiku-5-5", "ca-central-1", "us.anthropic.claude-haiku-5-5"},
		{"claude-haiku-5-5", "ca-west-1", "us.anthropic.claude-haiku-5-5"},
		{"claude-haiku-5-5", "eu-west-1", "eu.anthropic.claude-haiku-5-5"},
		{"claude-haiku-5-5", "ap-northeast-1", "global.anthropic.claude-haiku-5-5"},
		{"claude-haiku-5-5", "sa-east-1", "global.anthropic.claude-haiku-5-5"},
		// 3-haiku has no apac. profile: bare id in AP, no wildcard elsewhere.
		{"claude-3-haiku-20240307", "ap-southeast-1", "anthropic.claude-3-haiku-20240307-v1:0"},
		{"claude-3-haiku-20240307", "sa-east-1", "anthropic.claude-3-haiku-20240307-v1:0"},
		// An explicit prefix wins over the inferred one, including au./jp.
		{"jp.claude-opus-5-5", "ap-northeast-1", "jp.anthropic.claude-opus-5-5"},
		{"au.claude-opus-5-5", "ap-southeast-2", "au.anthropic.claude-opus-5-5"},
		{"eu.claude-opus-5-5", "us-east-1", "eu.anthropic.claude-opus-5-5"},
		// Unknown models pass through untouched.
		{"some-custom-model", "us-east-1", "some-custom-model"},
	}
	for _, c := range cases {
		if got := GetModelName(c.model, c.region); got != c.want {
			t.Errorf("GetModelName(%q, %q) = %q, want %q", c.model, c.region, got, c.want)
		}
	}
}
