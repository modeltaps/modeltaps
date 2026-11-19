package cheaperinference

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
)

const (
	upA = "0b9f6c1e-3d4a-4c1b-9a51-6f2f1f0d0a01"
	upB = "0b9f6c1e-3d4a-4c1b-9a51-6f2f1f0d0a02"
	upC = "0b9f6c1e-3d4a-4c1b-9a51-6f2f1f0d0a03"
)

func strp(s string) *string { return &s }

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func testQueue(t *testing.T) (*costQueue, *fakeClock, map[string]float64) {
	t.Helper()
	clock := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	q := newCostQueue()
	q.now = clock.now
	q.start.Do(func() {}) // no background loop: the test drives runOnce
	enriched := map[string]float64{}
	q.enrich = func(logID string, cost float64) error {
		enriched[logID] = cost
		return nil
	}
	return q, clock, enriched
}

func pendingCount(q *costQueue) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, list := range q.pending {
		n += len(list)
	}
	return n
}

func TestCostLookupFillsSettledAndKeepsPending(t *testing.T) {
	q, _, enriched := testQueue(t)
	account := costAccount{baseURL: "https://ci.test", key: "k"}
	var asked []string
	q.fetch = func(_ costAccount, ids []string) (map[string]usageRecord, error) {
		asked = ids
		return map[string]usageRecord{
			upA: {RequestID: upA, Status: "settled", BilledCostUSD: strp("0.001234")},
			upB: {RequestID: upB, Status: "forwarding"},
			// upC is not visible yet.
		}, nil
	}
	q.add(account, pendingCost{upstreamID: upA, logID: "log-a"})
	q.add(account, pendingCost{upstreamID: upB, logID: "log-b"})
	q.add(account, pendingCost{upstreamID: upC, logID: "log-c"})
	q.runOnce()

	if strings.Join(asked, ",") != upA+","+upB+","+upC {
		t.Errorf("asked = %v", asked)
	}
	if len(enriched) != 1 || enriched["log-a"] != 0.001234 {
		t.Errorf("enriched = %v", enriched)
	}
	if n := pendingCount(q); n != 2 {
		t.Errorf("pending = %d, want the two unsettled", n)
	}
}

func TestCostLookupRetriesUntilTheLogIsWritten(t *testing.T) {
	q, _, _ := testQueue(t)
	account := costAccount{baseURL: "https://ci.test", key: "k"}
	q.fetch = func(costAccount, []string) (map[string]usageRecord, error) {
		return map[string]usageRecord{upA: {RequestID: upA, Status: "settled", BilledCostUSD: strp("0.5")}}, nil
	}
	written := false
	q.enrich = func(string, float64) error {
		if !written {
			return gorm.ErrRecordNotFound
		}
		return nil
	}
	q.add(account, pendingCost{upstreamID: upA, logID: "log-a"})
	q.runOnce()
	if pendingCount(q) != 1 {
		t.Fatal("an unwritten log row must keep the request queued")
	}
	written = true
	q.runOnce()
	if pendingCount(q) != 0 {
		t.Fatal("the request stays queued after its cost was recorded")
	}
}

func TestCostLookupDropsFailedZeroAndOld(t *testing.T) {
	q, clock, enriched := testQueue(t)
	account := costAccount{baseURL: "https://ci.test", key: "k"}
	q.fetch = func(costAccount, []string) (map[string]usageRecord, error) {
		return map[string]usageRecord{
			upA: {RequestID: upA, Status: "failed"},
			upB: {RequestID: upB, Status: "settled", BilledCostUSD: strp("0.000000")},
		}, nil
	}
	q.add(account, pendingCost{upstreamID: upA, logID: "log-a"})
	q.add(account, pendingCost{upstreamID: upB, logID: "log-b"})
	q.add(account, pendingCost{upstreamID: upC, logID: "log-c"})
	q.runOnce()
	if len(enriched) != 0 {
		t.Errorf("a failed request or a zero charge recorded a cost: %v", enriched)
	}
	// The failed one is done; the zero charge and the unseen one wait until they age out.
	if n := pendingCount(q); n != 2 {
		t.Errorf("pending = %d, want 2", n)
	}
	clock.t = clock.t.Add(costLookupMaxAge)
	q.runOnce()
	if n := pendingCount(q); n != 0 {
		t.Errorf("pending after max age = %d", n)
	}
}

func TestCostLookupPausesAnAccountWithoutUsageScope(t *testing.T) {
	q, clock, _ := testQueue(t)
	account := costAccount{baseURL: "https://ci.test", key: "inference-only"}
	calls := 0
	q.fetch = func(costAccount, []string) (map[string]usageRecord, error) {
		calls++
		return nil, errUsageDenied
	}
	q.add(account, pendingCost{upstreamID: upA, logID: "log-a"})
	q.runOnce()
	q.add(account, pendingCost{upstreamID: upB, logID: "log-b"})
	q.runOnce()
	if calls != 1 || pendingCount(q) != 0 {
		t.Fatalf("calls = %d, pending = %d: a denied account must be paused", calls, pendingCount(q))
	}
	clock.t = clock.t.Add(costLookupDeniedPause)
	q.add(account, pendingCost{upstreamID: upC, logID: "log-c"})
	if pendingCount(q) != 1 {
		t.Fatal("the pause must end")
	}
}

func TestCostLookupBatchesAtMost100(t *testing.T) {
	q, _, _ := testQueue(t)
	account := costAccount{baseURL: "https://ci.test", key: "k"}
	var sizes []int
	q.fetch = func(_ costAccount, ids []string) (map[string]usageRecord, error) {
		sizes = append(sizes, len(ids))
		return map[string]usageRecord{}, nil
	}
	for i := 0; i < 150; i++ {
		q.add(account, pendingCost{upstreamID: upA, logID: "log"})
	}
	q.runOnce()
	if len(sizes) != 1 || sizes[0] != costLookupBatch {
		t.Errorf("batch sizes = %v", sizes)
	}
}

func TestFetchUsageRecords(t *testing.T) {
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	var gotPath, gotQuery, gotAuth string
	status := http.StatusOK
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotAuth = r.URL.Path, r.URL.Query().Get("request_ids"), r.Header.Get("Authorization")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"object":"list","data":[{"request_id":"` + strings.ToUpper(upA) + `","status":"settled","billed_cost_usd":"0.000123"}],"has_more":false,"next_cursor":null,"missing_request_ids":["` + upB + `"]}`))
	}))
	defer upstream.Close()

	account := costAccount{baseURL: upstream.URL, key: "ci_live_manage"}
	records, err := fetchUsageRecords(account, []string{upA, upB})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/usage/requests" || gotQuery != upA+","+upB || gotAuth != "Bearer ci_live_manage" {
		t.Errorf("request = %s ?request_ids=%s with %q", gotPath, gotQuery, gotAuth)
	}
	if cost, ok := records[upA].cost(); !ok || cost != 0.000123 {
		t.Errorf("records = %+v", records)
	}

	status = http.StatusForbidden
	if _, err := fetchUsageRecords(account, []string{upA}); err != errUsageDenied {
		t.Errorf("forbidden = %v", err)
	}
}

func TestQueueCostLookupNeedsUUIDAndBothIds(t *testing.T) {
	saved := costLookups
	t.Cleanup(func() { costLookups = saved })
	costLookups = newCostQueue()
	costLookups.start.Do(func() {})

	channel := channelWith(nil)
	manage := "ci_live_manage"
	channel.ManageKey = &manage
	p := CheaperInferenceProviderFactory{}.Create(channel).(*CheaperInferenceProvider)
	ctx := testContext(`{}`)
	p.SetContext(ctx)

	ctx.Set(logger.RequestIdKey, "gw-1")
	ctx.Set(config.GinUpstreamRequestIdKey, "req_not_a_uuid")
	p.queueCostLookup()
	if pendingCount(costLookups) != 0 {
		t.Fatal("a non-UUID id would fail the whole batch")
	}
	ctx.Set(config.GinUpstreamRequestIdKey, strings.ToUpper(upA))
	p.queueCostLookup()
	costLookups.mu.Lock()
	defer costLookups.mu.Unlock()
	for account, list := range costLookups.pending {
		if account.key != "ci_live_manage" || account.baseURL != DefaultBaseURL || len(list) != 1 || list[0].upstreamID != upA || list[0].logID != "gw-1" {
			t.Errorf("queued %+v %+v", account, list)
		}
		return
	}
	t.Error("nothing queued")
}

func TestUpstreamRequestIDPrefersTheRouterHeader(t *testing.T) {
	h := http.Header{}
	h.Set("x-request-id", "edge-id")
	h.Set("x-ci-request-id", upA)
	if got := requester.ExtractUpstreamRequestID(h); got != upA {
		t.Errorf("request id = %q", got)
	}
}
