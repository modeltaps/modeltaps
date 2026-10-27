package controller

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// TestRefreshChannelCredentialsOnceSkips: plain access-token Codex keys and manually disabled
// channels are left alone; nothing is refreshed or reported.
func TestRefreshChannelCredentialsOnceSkips(t *testing.T) {
	setupBalanceTestDB(t)
	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	for _, ch := range []model.Channel{
		{Name: "plain", Type: config.ChannelTypeCodex, Key: "plain-access-token", Status: config.ChannelStatusEnabled},
		{Name: "fresh", Type: config.ChannelTypeCodex, Status: config.ChannelStatusAutoDisabled,
			Key: `{"access_token":"a","refresh_token":"r","expires_at":"` + future + `"}`},
		{Name: "manual", Type: config.ChannelTypeCodex, Status: config.ChannelStatusManuallyDisabled,
			Key: `{"access_token":"a","refresh_token":"r"}`},
		{Name: "openai", Type: config.ChannelTypeOpenAI, Key: "sk", Status: config.ChannelStatusEnabled},
	} {
		ch := ch
		if err := model.DB.Create(&ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	before := testNotifier.count()
	n, failed := RefreshChannelCredentialsOnce(time.Hour)
	if n != 0 || len(failed) != 0 || testNotifier.count() != before {
		t.Fatalf("refreshed=%d failed=%v notices=%d", n, failed, testNotifier.count()-before)
	}
}
