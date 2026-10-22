package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestScheduledChannelTest(t *testing.T) {
	oldMode, oldInterval := config.ChannelTestMode, config.ChannelTestIntervalMinutes
	t.Cleanup(func() { config.ChannelTestMode, config.ChannelTestIntervalMinutes = oldMode, oldInterval })

	config.ChannelTestMode, config.ChannelTestIntervalMinutes = "recovery", 10
	if mode, interval := scheduledChannelTest(0); mode != "recovery" || interval != 10 {
		t.Fatalf("settings: %s %d", mode, interval)
	}
	// CHANNEL_TEST_FREQUENCY keeps the old behavior: test every channel at that interval.
	if mode, interval := scheduledChannelTest(1440); mode != "all" || interval != 1440 {
		t.Fatalf("env override: %s %d", mode, interval)
	}
	config.ChannelTestIntervalMinutes = 0
	if _, interval := scheduledChannelTest(0); interval != 1 {
		t.Fatalf("interval floor = %d, want 1", interval)
	}
}
