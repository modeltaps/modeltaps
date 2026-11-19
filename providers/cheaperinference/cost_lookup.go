package cheaperinference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
)

// Streamed chat completions and completions carry no charge, and other streams may end without
// one, so after each streamed request the charge is looked up afterwards:
// `GET /v1/usage/requests?request_ids=...` (usage:read scope) returns up to 100 named requests
// with billed_cost_usd once they settle. Lookups are batched per account on a timer, and a found
// charge fills the log's upstream cost only where the response itself carried none.
const (
	costLookupInterval = 20 * time.Second
	// costLookupMaxAge drops a request that has not settled (or whose log row never appeared)
	// by then; long streams settle when they end, and the log row is written right after.
	costLookupMaxAge = 30 * time.Minute
	// costLookupBatch is the most ids the endpoint takes at once.
	costLookupBatch = 100
	// costLookupMaxPending caps the queue per account; the oldest are dropped first.
	costLookupMaxPending = 10000
	// costLookupDeniedPause stops lookups for an account whose key lacks usage:read.
	costLookupDeniedPause = time.Hour
	costLookupTimeout     = 15 * time.Second
	costLookupMaxBody     = 4 << 20
)

// costAccount is where and with which key an account's requests are looked up.
type costAccount struct {
	baseURL, key, proxy string
}

type pendingCost struct {
	upstreamID string // Cheaper Inference's request id (UUID)
	logID      string // the gateway's request id, which the log row carries
	queuedAt   time.Time
}

type costQueue struct {
	mu      sync.Mutex
	pending map[costAccount][]pendingCost
	paused  map[costAccount]time.Time
	start   sync.Once
	// now and fetch are replaced in tests.
	now   func() time.Time
	fetch func(costAccount, []string) (map[string]usageRecord, error)
	// enrich fills the log's upstream cost; returns gorm.ErrRecordNotFound while the row is not
	// written yet.
	enrich func(logID string, cost float64) error
}

var costLookups = newCostQueue()

func newCostQueue() *costQueue {
	return &costQueue{
		pending: map[costAccount][]pendingCost{},
		paused:  map[costAccount]time.Time{},
		now:     time.Now,
		fetch:   fetchUsageRecords,
		enrich: func(logID string, cost float64) error {
			return model.EnrichLogByRequestId(logID, nil, cost)
		},
	}
}

var errUsageDenied = errors.New("usage:read scope missing")

// queueCostLookup queues the request just sent for a charge lookup. Nothing is queued without
// both request ids or with an upstream id the endpoint would reject (it takes UUIDs only, and one
// malformed id fails the whole batch).
func (p *CheaperInferenceProvider) queueCostLookup() {
	if p.Context == nil {
		return
	}
	upstreamID := strings.TrimSpace(p.Context.GetString(config.GinUpstreamRequestIdKey))
	logID := p.Context.GetString(logger.RequestIdKey)
	if upstreamID == "" || logID == "" {
		return
	}
	if _, err := uuid.Parse(upstreamID); err != nil {
		return
	}
	account := costAccount{baseURL: strings.TrimSuffix(p.GetBaseURL(), "/"), key: firstKey(p.manageKey), proxy: p.Channel.GetProxy()}
	if account.key == "" {
		return
	}
	costLookups.add(account, pendingCost{upstreamID: strings.ToLower(upstreamID), logID: logID})
}

func (q *costQueue) add(account costAccount, item pendingCost) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if until, ok := q.paused[account]; ok && q.now().Before(until) {
		return
	}
	item.queuedAt = q.now()
	list := append(q.pending[account], item)
	if len(list) > costLookupMaxPending {
		list = list[len(list)-costLookupMaxPending:]
	}
	q.pending[account] = list
	q.start.Do(func() { go q.loop() })
}

func (q *costQueue) loop() {
	ticker := time.NewTicker(costLookupInterval)
	defer ticker.Stop()
	for range ticker.C {
		q.runOnce()
	}
}

// runOnce looks up one batch per account: the oldest ids first.
func (q *costQueue) runOnce() {
	defer func() {
		if r := recover(); r != nil {
			logger.SysError(fmt.Sprintf("cheaper inference cost lookup panic: %v, stack: %s", r, debug.Stack()))
		}
	}()
	q.mu.Lock()
	batches := map[costAccount][]pendingCost{}
	now := q.now()
	for account, list := range q.pending {
		kept := list[:0]
		for _, item := range list {
			if now.Sub(item.queuedAt) < costLookupMaxAge {
				kept = append(kept, item)
			}
		}
		if len(kept) == 0 {
			delete(q.pending, account)
			continue
		}
		q.pending[account] = kept
		n := min(len(kept), costLookupBatch)
		batches[account] = append([]pendingCost(nil), kept[:n]...)
	}
	q.mu.Unlock()

	for account, batch := range batches {
		q.lookup(account, batch)
	}
}

func (q *costQueue) lookup(account costAccount, batch []pendingCost) {
	ids := make([]string, 0, len(batch))
	for _, item := range batch {
		ids = append(ids, item.upstreamID)
	}
	records, err := q.fetch(account, ids)
	if err != nil {
		if errors.Is(err, errUsageDenied) {
			q.mu.Lock()
			delete(q.pending, account)
			q.paused[account] = q.now().Add(costLookupDeniedPause)
			q.mu.Unlock()
			logger.SysLog("cheaper inference: the channel's key cannot read usage (usage:read), so streamed requests get no upstream cost; set a management key with that scope")
		}
		return
	}

	done := map[string]bool{}
	for _, item := range batch {
		rec, ok := records[item.upstreamID]
		if !ok {
			continue // not visible yet
		}
		switch rec.Status {
		case "failed":
			done[item.upstreamID] = true
		case "settled", "cached":
			cost, ok := rec.cost()
			if !ok {
				continue
			}
			err := q.enrich(item.logID, cost)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue // the log row is not written yet
			}
			if err != nil {
				logger.SysError("cheaper inference upstream cost update failed: " + err.Error())
			}
			done[item.upstreamID] = true
		}
	}
	if len(done) == 0 {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	list := q.pending[account]
	kept := list[:0]
	for _, item := range list {
		if !done[item.upstreamID] {
			kept = append(kept, item)
		}
	}
	if len(kept) == 0 {
		delete(q.pending, account)
	} else {
		q.pending[account] = kept
	}
}

// usageRecord is one row of `GET /v1/usage/requests`.
type usageRecord struct {
	RequestID     string  `json:"request_id"`
	Status        string  `json:"status"`
	BilledCostUSD *string `json:"billed_cost_usd"`
}

// cost is the settled charge in USD. A settled zero charge is not a cost to record.
func (r usageRecord) cost() (float64, bool) {
	if r.BilledCostUSD == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(*r.BilledCostUSD), 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

type usageRecordList struct {
	Data []usageRecord `json:"data"`
}

func fetchUsageRecords(account costAccount, ids []string) (map[string]usageRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), costLookupTimeout)
	defer cancel()
	reqURL := account.baseURL + "/v1/usage/requests?request_ids=" + url.QueryEscape(strings.Join(ids, ","))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+account.key)
	req.Header.Set("Accept", "application/json")
	resp, err := requester.GetHTTPClient(account.proxy).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, errUsageDenied
	case resp.StatusCode != http.StatusOK:
		return nil, errors.New("cheaper inference usage lookup: " + resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, costLookupMaxBody))
	if err != nil {
		return nil, err
	}
	var list usageRecordList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}
	out := make(map[string]usageRecord, len(list.Data))
	for _, r := range list.Data {
		out[strings.ToLower(r.RequestID)] = r
	}
	return out, nil
}

// firstKey is the first non-empty line of a channel key (several keys are one per line).
func firstKey(raw string) string {
	for _, k := range strings.Split(raw, "\n") {
		if k = strings.TrimSpace(k); k != "" {
			return k
		}
	}
	return ""
}
