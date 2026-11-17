package common

import "testing"

func TestDisableChannelKeywordMatching(t *testing.T) {
	d := &DisableChannelKeyword{}
	if d.IsContains("anything") {
		t.Fatal("no keywords loaded must match nothing")
	}

	// A whitespace-only line used to reach the matcher as an empty keyword and panic.
	d.Load("  API key not valid  \n   \n\t\n额度不足\r\n👍🏽 blocked")
	for _, msg := range []string{
		`{"error":{"message":"API key not valid. Please pass a valid API key."}}`,
		"上游返回：账户额度不足，请充值",
		"user 👍🏽 blocked by policy",
	} {
		if !d.IsContains(msg) {
			t.Errorf("expected a hit for %q", msg)
		}
	}
	for _, msg := range []string{
		"api key not valid", // case-sensitive
		"API KEY NOT VALID",
		"额度充足",
		"👍 blocked", // the skin-tone modifier is part of the keyword
		"",
	} {
		if d.IsContains(msg) {
			t.Errorf("unexpected hit for %q", msg)
		}
	}

	// Replacing the list with blank lines only clears the old keywords.
	d.Load("\n  \n")
	if d.IsContains("API key not valid") {
		t.Fatal("a blank keyword list must match nothing")
	}
	d.Load(GetDefaultDisableChannelKeywords())
	if !d.IsContains("You exceeded your current quota, please check your plan") {
		t.Fatal("default keywords must match")
	}
	d.Load("")
	if d.IsContains("You exceeded your current quota") || d.GetKeywords() != "" {
		t.Fatal("an empty list must clear the keywords")
	}
}
