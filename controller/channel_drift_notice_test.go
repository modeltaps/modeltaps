package controller

import (
	"strings"
	"testing"
)

func TestModelDriftNoticeSkipsNewModels(t *testing.T) {
	summary := &ModelDriftCheckSummary{WithNew: 1, Added: 1, Channels: []DriftedChannel{
		{ChannelId: 4, ChannelName: "requesty", NewModels: []string{"openai/gpt-5.5"}, AddedModels: []string{"x"}, PendingModels: []string{"y"}},
	}}
	if subject, body := modelDriftNotice(summary); body != "" || subject != "" {
		t.Fatalf("new, added and pending models alone send nothing, got %q / %q", subject, body)
	}
}

func TestModelDriftNoticeSplitsRenamesFromGoneModels(t *testing.T) {
	summary := &ModelDriftCheckSummary{Channels: []DriftedChannel{
		{ChannelId: 4, ChannelName: "requesty", MissingModels: []string{"gpt-5.5", "deepseek-v4-pro"}, Renamed: map[string]string{"gpt-5.5": "openai/gpt-5.5"}},
	}}
	subject, body := modelDriftNotice(summary)
	if !strings.Contains(subject, "1 channel") {
		t.Fatalf("subject = %q", subject)
	}
	if !strings.Contains(body, "gpt-5.5 → openai/gpt-5.5") || !strings.Contains(body, "no longer lists models still in use: deepseek-v4-pro") {
		t.Fatalf("body = %q", body)
	}
}
