package base

import (
	"errors"

	"github.com/modeltaps/modeltaps/common/cache"
	"testing"
	"time"
)

func TestRefreshOAuthAhead(t *testing.T) {
	cache.InitCacheManager()
	calls := 0
	refresh := func() error { calls++; return nil }
	save := func() error { return nil }

	if did, _ := RefreshOAuthAhead(time.Hour, false, time.Now(), refresh, save, "k"); did || calls != 0 {
		t.Fatal("refreshed without a refresh token")
	}
	if did, _ := RefreshOAuthAhead(time.Hour, true, time.Now().Add(3*time.Hour), refresh, save, "k"); did || calls != 0 {
		t.Fatal("refreshed a credential valid beyond the window")
	}
	if did, err := RefreshOAuthAhead(time.Hour, true, time.Now().Add(10*time.Minute), refresh, save, "k"); !did || err != nil || calls != 1 {
		t.Fatalf("expiring credential: did=%v err=%v calls=%d", did, err, calls)
	}
	if did, _ := RefreshOAuthAhead(time.Hour, true, time.Time{}, refresh, save, "k"); !did || calls != 2 {
		t.Fatal("unknown expiry should refresh")
	}
	boom := errors.New("invalid_grant")
	if did, err := RefreshOAuthAhead(time.Hour, true, time.Time{}, func() error { return boom }, save, "k"); did || !errors.Is(err, boom) {
		t.Fatalf("refresh failure: did=%v err=%v", did, err)
	}
}
