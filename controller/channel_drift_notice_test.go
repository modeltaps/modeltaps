package controller

import (
	"strings"
	"testing"
)

func TestModelDriftNoticeSkipsNewModels(t *testing.T) {
	summary := &ModelDriftCheckSummary{WithNew: 1, Added: 1, Channels: []DriftedChannel{
		{ChannelId: 4, ChannelName: "requesty", NewModels: []string{"openai/gpt-5.5"}, AddedModels: []string{"x"}, PendingModels: []string{"y"}},
	}}
	if subject, body := modelDriftNotice(summary, allNew); body != "" || subject != "" {
		t.Fatalf("new, added and pending models alone send nothing, got %q / %q", subject, body)
	}
}

func TestModelDriftNoticeSplitsRenamesFromGoneModels(t *testing.T) {
	summary := &ModelDriftCheckSummary{Channels: []DriftedChannel{
		{ChannelId: 4, ChannelName: "requesty", MissingModels: []string{"gpt-5.5", "deepseek-v4-pro"}, Renamed: map[string]string{"gpt-5.5": "openai/gpt-5.5"}},
	}}
	subject, body := modelDriftNotice(summary, allNew)
	if !strings.Contains(subject, "1 channel") {
		t.Fatalf("subject = %q", subject)
	}
	if !strings.Contains(body, "gpt-5.5 → openai/gpt-5.5") || !strings.Contains(body, "no longer lists models still in use: deepseek-v4-pro") {
		t.Fatalf("body = %q", body)
	}
}

// allNew treats every reported item as new, as a first check does.
func allNew(_ int, items []string) []string { return items }

func TestModelDriftNoticeTellsOnlyChannelsWithSomethingNew(t *testing.T) {
	summary := &ModelDriftCheckSummary{
		Channels: []DriftedChannel{
			{ChannelId: 4, ChannelName: "requesty", MissingModels: []string{"gpt-5.5"}},
			{ChannelId: 5, ChannelName: "other", MissingModels: []string{"glm-5.3"}},
		},
		checkedIds: []int{4, 5, 6},
	}
	reported := map[int][]string{}
	known := map[int]bool{4: true}
	report := func(id int, items []string) []string {
		reported[id] = items
		if known[id] {
			return nil
		}
		return items
	}
	_, body := modelDriftNotice(summary, report)
	if strings.Contains(body, "requesty") || !strings.Contains(body, "other") {
		t.Fatalf("body = %q", body)
	}
	if items, ok := reported[6]; !ok || items != nil {
		t.Fatalf("a checked channel with nothing closes its alert: %v", reported)
	}
}
