package model

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type fakeBrandIconFetcher struct {
	mu    sync.Mutex
	calls []string
	fail  bool
	done  chan struct{}
}

func (f *fakeBrandIconFetcher) fetch(_ context.Context, domain string) ([]byte, string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, domain)
	f.mu.Unlock()
	defer func() { f.done <- struct{}{} }()
	if f.fail {
		return nil, "", errors.New("boom")
	}
	return []byte("\x89PNG\r\n\x1a\nx"), "image/png", nil
}

func (f *fakeBrandIconFetcher) wait(t *testing.T) {
	t.Helper()
	select {
	case <-f.done:
	case <-time.After(3 * time.Second):
		t.Fatal("fetch not triggered")
	}
	for i := 0; i < 100; i++ {
		if _, busy := brandIconInflight.Load("foo.com"); !busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (f *fakeBrandIconFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func setupBrandIconCacheTest(t *testing.T) *fakeBrandIconFetcher {
	t.Helper()
	dsn := fmt.Sprintf("file:bic%d?mode=memory&cache=shared", brandIconTestSeq.Add(1))
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&BrandIconCache{}); err != nil {
		t.Fatal(err)
	}
	f := &fakeBrandIconFetcher{done: make(chan struct{}, 8)}
	oldDB, oldFetcher := DB, brandIconFetcher
	DB, brandIconFetcher = testDB, f.fetch
	t.Cleanup(func() {
		DB, brandIconFetcher = oldDB, oldFetcher
		if sqlDB, err := testDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return f
}

var brandIconTestSeq atomic.Int64

func TestBrandIconFetchSuccessAndCacheHit(t *testing.T) {
	f := setupBrandIconCacheTest(t)
	base := "https://api.foo.com/v1"
	TriggerChannelBrandIconFetch(&Channel{Type: 1, BaseURL: &base})
	f.wait(t)
	row, err := GetBrandIconCache("foo.com")
	if err != nil || row.MimeType != "image/png" || row.Sha256 == "" {
		t.Fatalf("row=%+v err=%v", row, err)
	}
	if row.ExpiresAt-row.FetchedAt != int64(brandIconOKTTL/time.Second) {
		t.Errorf("unexpected ttl %d", row.ExpiresAt-row.FetchedAt)
	}
	EnqueueBrandIconFetch("www.foo.com")
	time.Sleep(50 * time.Millisecond)
	if f.callCount() != 1 {
		t.Errorf("fresh cache must not refetch, calls=%d", f.callCount())
	}
}

func TestBrandIconNegativeCache(t *testing.T) {
	f := setupBrandIconCacheTest(t)
	f.fail = true
	EnqueueBrandIconFetch("foo.com")
	f.wait(t)
	var row BrandIconCache
	DB.First(&row, "domain = ?", "foo.com")
	if row.Status != BrandIconStatusFailed || row.ExpiresAt-row.FetchedAt != int64(brandIconNegativeTTL/time.Second) {
		t.Fatalf("unexpected negative row %+v", row)
	}
	if _, err := GetBrandIconCache("foo.com"); err == nil {
		t.Error("failed row must not be served")
	}
	EnqueueBrandIconFetch("foo.com")
	time.Sleep(50 * time.Millisecond)
	if f.callCount() != 1 {
		t.Errorf("negative cache must suppress refetch, calls=%d", f.callCount())
	}

	// 负缓存过期后重试成功即覆盖。
	DB.Model(&row).Update("expires_at", time.Now().Add(-time.Minute).Unix())
	f.fail = false
	EnqueueBrandIconFetch("foo.com")
	f.wait(t)
	if got, err := GetBrandIconCache("foo.com"); err != nil || len(got.Data) == 0 {
		t.Fatalf("retry should store icon, err=%v", err)
	}
}

func TestBrandIconRefreshFailureKeepsOldIcon(t *testing.T) {
	f := setupBrandIconCacheTest(t)
	DB.Create(&BrandIconCache{Domain: "foo.com", Status: BrandIconStatusOK, MimeType: "image/png", Data: []byte("old"),
		ExpiresAt: time.Now().Add(-time.Hour).Unix()})
	f.fail = true
	EnqueueBrandIconFetch("foo.com")
	f.wait(t)
	row, err := GetBrandIconCache("foo.com")
	if err != nil || string(row.Data) != "old" || row.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("old icon should survive with pushed expiry, row=%+v err=%v", row, err)
	}
}

func TestBrandIconTriggersSkipKnownBrands(t *testing.T) {
	f := setupBrandIconCacheTest(t)
	known := "https://api.deepseek.com/v1"
	private := "http://127.0.0.1:8080/v1"
	TriggerChannelBrandIconFetch(&Channel{Type: 8, BaseURL: &known})
	TriggerChannelBrandIconFetch(&Channel{Type: 8, BaseURL: &private})
	TriggerChannelBrandIconFetch(&Channel{Type: 1})
	TriggerOwnedByBrandIconFetch(&ModelOwnedBy{Name: "OpenAI", Slug: "openai"})
	TriggerOwnedByBrandIconFetch(&ModelOwnedBy{Name: "Some Vendor", Slug: "some-vendor"})
	time.Sleep(50 * time.Millisecond)
	if f.callCount() != 0 {
		t.Fatalf("known/invalid targets must not fetch, calls=%v", f.calls)
	}
	TriggerOwnedByBrandIconFetch(&ModelOwnedBy{Name: "foo.com"})
	f.wait(t)
	if f.callCount() != 1 || f.calls[0] != "foo.com" {
		t.Fatalf("domain-like vendor should fetch, calls=%v", f.calls)
	}
}

func TestRefreshBrandIconNowBypassesTTL(t *testing.T) {
	f := setupBrandIconCacheTest(t)
	DB.Create(&BrandIconCache{Domain: "foo.com", Status: BrandIconStatusFailed,
		ExpiresAt: time.Now().Add(time.Hour).Unix()})
	domain, err := RefreshBrandIconNow("api.Foo.com")
	if err != nil || domain != "foo.com" {
		t.Fatalf("domain=%q err=%v", domain, err)
	}
	if f.callCount() != 1 {
		t.Fatalf("refresh must fetch even while cached, calls=%d", f.callCount())
	}
	if row, err := GetBrandIconCache("foo.com"); err != nil || len(row.Data) == 0 {
		t.Fatalf("refresh should store icon synchronously, err=%v", err)
	}

	brandIconInflight.Store("foo.com", struct{}{})
	if _, err := RefreshBrandIconNow("foo.com"); !errors.Is(err, ErrBrandIconRefreshBusy) {
		t.Errorf("in-flight domain should report busy, err=%v", err)
	}
	brandIconInflight.Delete("foo.com")

	config.BrandIconFaviconFetchEnabled = false
	_, err = RefreshBrandIconNow("foo.com")
	config.BrandIconFaviconFetchEnabled = true
	if !errors.Is(err, ErrBrandIconFetchDisabled) {
		t.Errorf("disabled favicon fetching should reject refresh, err=%v", err)
	}

	for _, d := range []string{"", "localhost", "127.0.0.1"} {
		if _, err := RefreshBrandIconNow(d); !errors.Is(err, ErrInvalidBrandIconDomain) {
			t.Errorf("%q: err=%v, want invalid domain", d, err)
		}
	}
	if f.callCount() != 1 {
		t.Errorf("rejected refreshes must not fetch, calls=%d", f.callCount())
	}
}
