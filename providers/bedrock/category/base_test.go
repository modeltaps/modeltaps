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
		{"claude-fable-5-1", "eu-central-1", "global.anthropic.claude-fable-5-1"},
		// "*" fallback for roots without an entry.
		{"claude-haiku-4-5-20251001", "sa-east-1", "global.anthropic.claude-haiku-4-5-20251001-v1:0"},
		{"claude-sonnet-5-5", "us-east-1", "global.anthropic.claude-sonnet-5-5"},
		// Canada uses the US geo profile where the model supports it there.
		{"claude-opus-4-7", "ca-central-1", "us.anthropic.claude-opus-4-7"},
		{"claude-opus-4-5-20251101", "ca-west-1", "global.anthropic.claude-opus-4-5-20251101-v1:0"},
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
