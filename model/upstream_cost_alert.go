package model

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
)

// Checking prices against what upstreams really charged. A price list can be wrong in ways no
// sync notices: a list that lags a vendor's change, a promotion that ended, a markup on top of the
// listed price. The upstream's own bill settles it. OpenRouter reports the cost of every request
// (the consume log's upstream_cost, filled from the generation lookup when the reply had none),
// and OpenAI, Anthropic and OpenRouter accounts with a management key report what the account was
// billed per day. Both are compared with the expected cost the gateway logged from its price
// list, and a difference that holds over several requests opens an alert on the channel.

// CostMismatchAlertKind is the alert kind of a channel whose upstream charged noticeably more or
// less than its price list says. It is told as a price change notice.
const CostMismatchAlertKind = "cost_mismatch"

const (
	// costCheckWindow is how far back the comparison goes.
	costCheckWindow = 3 * 24 * time.Hour
	// costCheckSettle leaves out the newest requests, whose cost the generation lookup may still
	// be filling in.
	costCheckSettle = 30 * time.Minute
	// costMismatchShare is the difference, as a share of the expected cost, that is told.
	costMismatchShare = 0.10
	// A model is compared once it has this many requests with both costs, worth this much.
	costCheckMinRequests = 10
	costCheckMinExpected = 0.10
)

// costMismatch is one difference found: a model of a channel (per request costs), or a
// channel's account over the days its upstream report has (Model empty).
type costMismatch struct {
	ChannelId int
	Model     string
	Requests  int64
	Expected  float64
	Upstream  float64
}

func (m costMismatch) drift() float64 { return (m.Upstream - m.Expected) / m.Expected }

// item names the mismatch for the alert: "cost|model|higher|lower" or "daily|higher|lower". It
// carries the direction only, so a difference that goes on is told once while its amounts move;
// a turn from higher to lower is told again.
func (m costMismatch) item() string {
	dir := "lower"
	if m.Upstream > m.Expected {
		dir = "higher"
	}
	if m.Model == "" {
		return "daily|" + dir
	}
	return "cost|" + m.Model + "|" + dir
}

func (m costMismatch) mismatched() bool {
	return m.Requests >= costCheckMinRequests && m.Expected >= costCheckMinExpected &&
		math.Abs(m.drift()) >= costMismatchShare
}

// requestCostMismatches compares, per channel and model, the costs upstreams reported for the
// requests in [start, end) with the expected costs of the same requests.
func requestCostMismatches(start, end int64) ([]costMismatch, error) {
	var rows []struct {
		ChannelId    int
		ModelName    string
		RequestCount int64
		UpstreamCost float64
		ExpectedCost float64
	}
	err := DB.Table("logs").
		Select("channel_id, model_name, count(*) as request_count, sum(upstream_cost) as upstream_cost, sum(expected_cost) as expected_cost").
		Where("type = ? AND upstream_cost > 0 AND expected_cost > 0 AND created_at >= ? AND created_at < ?", LogTypeConsume, start, end).
		Group("channel_id, model_name").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	var out []costMismatch
	for _, r := range rows {
		m := costMismatch{ChannelId: r.ChannelId, Model: r.ModelName, Requests: r.RequestCount, Expected: r.ExpectedCost, Upstream: r.UpstreamCost}
		if m.mismatched() {
			out = append(out, m)
		}
	}
	return out, nil
}

// dailyCostMismatches compares each account's upstream daily report with the expected cost of its
// channels over the completed UTC days of the window that the report has. The mismatch goes on
// the account's oldest channel, as the daily cost table shows it.
func dailyCostMismatches(now time.Time) ([]costMismatch, error) {
	to := now.UTC().Truncate(24 * time.Hour)
	from := to.Add(-costCheckWindow)
	var accounts []string
	if err := DB.Model(&AccountDailyCost{}).Where("date >= ? AND date < ?", from.Format(time.DateOnly), to.Format(time.DateOnly)).
		Distinct("account").Pluck("account", &accounts).Error; err != nil {
		return nil, err
	}
	var out []costMismatch
	for _, account := range accounts {
		channels, err := ChannelsOfAccount(account)
		if err != nil {
			return nil, err
		}
		if len(channels) == 0 {
			continue
		}
		days, err := GetAccountDailyCosts(account, from.Format(time.DateOnly))
		if err != nil {
			return nil, err
		}
		billed := map[string]float64{}
		for _, d := range days {
			if d.Date < to.Format(time.DateOnly) {
				billed[d.Date] = d.Cost
			}
		}
		ids := make([]int, 0, len(channels))
		for _, ch := range channels {
			ids = append(ids, ch.Id)
		}
		expected, err := GetExpectedDailyCosts(ids, from.Unix(), to.Unix())
		if err != nil {
			return nil, err
		}
		m := costMismatch{ChannelId: channels[0].Id}
		for _, e := range expected {
			cost, ok := billed[e.Date]
			if !ok {
				continue
			}
			m.Requests += e.RequestCount
			m.Expected += e.ExpectedCost
			m.Upstream += cost
		}
		if m.mismatched() {
			out = append(out, m)
		}
	}
	return out, nil
}

// CheckUpstreamCosts runs both comparisons, keeps one alert per channel with what it found, and
// tells admins about the new findings. Channels with nothing left to say close their alert.
func CheckUpstreamCosts(now time.Time) error {
	end := now.Add(-costCheckSettle)
	found, err := requestCostMismatches(end.Add(-costCheckWindow).Unix(), end.Unix())
	if err != nil {
		return err
	}
	daily, err := dailyCostMismatches(now)
	if err != nil {
		return err
	}
	found = append(found, daily...)

	byChannel := map[int][]costMismatch{}
	for _, m := range found {
		byChannel[m.ChannelId] = append(byChannel[m.ChannelId], m)
	}
	ids := make([]int, 0, len(byChannel))
	for id := range byChannel {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	var keep []string
	var b strings.Builder
	for _, id := range ids {
		subject := "channel:" + strconv.Itoa(id)
		keep = append(keep, subject)
		list := byChannel[id]
		items := make([]string, 0, len(list))
		for _, m := range list {
			items = append(items, m.item())
		}
		fresh, err := ReportAlert(CostMismatchAlertKind, subject, items)
		if err != nil {
			return err
		}
		if len(fresh) == 0 {
			continue
		}
		name := subject
		if ch, err := GetChannelById(id); err == nil {
			name = ch.Name
		}
		for _, m := range list {
			for _, item := range fresh {
				if item == m.item() {
					writeCostMismatch(&b, name, m)
				}
			}
		}
	}
	if err := ResolveAlertsExcept(CostMismatchAlertKind, keep); err != nil {
		return err
	}
	if b.Len() > 0 {
		logger.SysLog("upstream cost check: new differences between expected and billed costs")
		notify.SendKind(notify.KindPriceChange, "Upstream charges differ from the price list", b.String()+
			"\nCompare them on the channel's Performance tab and correct the channel's price, fee rate or cost ratio.")
	}
	return nil
}

func writeCostMismatch(b *strings.Builder, channel string, m costMismatch) {
	what := m.Model + " on " + channel
	if m.Model == "" {
		what = channel + " (account total per day; it also counts use of the account outside this site)"
	}
	fmt.Fprintf(b, "- %s: billed $%.4f, expected $%.4f (%+.1f%%) over %d requests in the last %d days\n",
		what, m.Upstream, m.Expected, m.drift()*100, m.Requests, int(costCheckWindow/(24*time.Hour)))
}
