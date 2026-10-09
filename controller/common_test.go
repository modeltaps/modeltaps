package controller

import (
	"strings"
	"testing"
)

func TestChannelDisableNotifyContent(t *testing.T) {
	withModel := channelDisableNotifyContent(7, "main", "gpt-4o-mini", "upstream returned 401")
	if !strings.Contains(withModel, "Model: gpt-4o-mini.") {
		t.Fatalf("expected model name in content, got %q", withModel)
	}
	if !strings.Contains(withModel, `Channel "main" (#7) has been disabled.`) || !strings.Contains(withModel, "Reason: upstream returned 401") {
		t.Fatalf("unexpected content %q", withModel)
	}

	withoutModel := channelDisableNotifyContent(7, "main", "", "Insufficient balance")
	if strings.Contains(withoutModel, "Model:") {
		t.Fatalf("expected model field omitted, got %q", withoutModel)
	}
	if withoutModel != `Channel "main" (#7) has been disabled. Reason: Insufficient balance` {
		t.Fatalf("unexpected content %q", withoutModel)
	}
}
