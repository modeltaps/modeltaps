// Package accountcost reads an upstream account's own daily cost report with a channel's
// management key. The reports are for the whole account, in USD, per UTC day.
package accountcost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
)

// ErrNotSupported says the channel's upstream has no daily cost report this package can read.
var ErrNotSupported = errors.New("this channel's upstream has no daily cost report")

// ErrNoManageKey says the channel needs a management key for its report.
var ErrNoManageKey = errors.New("the daily cost report needs the channel's management key")

const (
	fetchTimeout = 30 * time.Second
	maxBodyBytes = 8 << 20
	// maxDays is how far back every supported report reaches in one call.
	maxDays = 30
)

// fetcher reads the days in [from, to) (UTC midnights) of the account behind the channel.
type fetcher func(channel *model.Channel, from, to time.Time) ([]model.AccountDailyCost, error)

var fetchers = map[int]fetcher{
	config.ChannelTypeOpenRouter: openRouterActivity,
	config.ChannelTypeAnthropic:  anthropicCostReport,
	config.ChannelTypeOpenAI:     openAICosts,
}

// officialHosts are the hosts the reports live on. A channel of one of these types that points
// at another host is a relay, which has no report (and must not be sent the management key).
var officialHosts = map[int]string{
	config.ChannelTypeOpenRouter: "openrouter.ai",
	config.ChannelTypeAnthropic:  "api.anthropic.com",
	config.ChannelTypeOpenAI:     "api.openai.com",
}

// anyHost lets tests point a channel at a local server.
var anyHost = false

// Supported reports whether the channel's upstream has a daily cost report.
func Supported(channel *model.Channel) bool {
	if _, ok := fetchers[channel.Type]; !ok {
		return false
	}
	return anyHost || isHost(channel.GetBaseURL(), officialHosts[channel.Type])
}

// isHost is true for an empty base URL (the default host) or one on the given host.
func isHost(baseURL, host string) bool {
	if strings.TrimSpace(baseURL) == "" {
		return true
	}
	u, err := url.Parse(baseURL)
	return err == nil && strings.EqualFold(u.Hostname(), host)
}

// Fetch reads the last completed UTC days (at most 30) of the account's cost report, oldest
// first. Today is left out: it is not complete, and one upstream does not report it at all.
func Fetch(channel *model.Channel, now time.Time) ([]model.AccountDailyCost, error) {
	if !Supported(channel) {
		return nil, ErrNotSupported
	}
	if channel.GetManageKey() == "" {
		return nil, ErrNoManageKey
	}
	to := now.UTC().Truncate(24 * time.Hour)
	from := to.AddDate(0, 0, -maxDays)
	days, err := fetchers[channel.Type](channel, from, to)
	if err != nil {
		return nil, err
	}
	kept := days[:0]
	for _, day := range days {
		if day.Date >= from.Format(time.DateOnly) && day.Date < to.Format(time.DateOnly) {
			kept = append(kept, day)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].Date < kept[j].Date })
	return kept, nil
}

// baseURL is the channel's base URL, or the upstream's own when the channel has none.
func baseURL(channel *model.Channel, fallback string) string {
	if base := strings.TrimSuffix(strings.TrimSpace(channel.GetBaseURL()), "/"); base != "" {
		return base
	}
	return fallback
}

// getJSON sends a GET with the headers through the channel's proxy and decodes the reply.
func getJSON(channel *model.Channel, reqURL string, headers map[string]string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := requester.GetHTTPClient(channel.GetProxy()).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return base.KeyRejected(fmt.Sprintf("the upstream refused the management key (HTTP %d): %s", resp.StatusCode, snippet(body)))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the cost report returned HTTP %d: %s", resp.StatusCode, snippet(body))
	}
	return json.Unmarshal(body, out)
}

func snippet(body []byte) string {
	return strings.TrimSpace(string(body[:min(len(body), 200)]))
}

// byDay sums costs and requests per day and returns one row per day.
type byDay map[string]*model.AccountDailyCost

func (d byDay) add(date string, cost float64, requests int64) {
	row, ok := d[date]
	if !ok {
		row = &model.AccountDailyCost{Date: date, Requests: -1}
		d[date] = row
	}
	row.Cost += cost
	if requests >= 0 {
		if row.Requests < 0 {
			row.Requests = 0
		}
		row.Requests += requests
	}
}

func (d byDay) rows() []model.AccountDailyCost {
	out := make([]model.AccountDailyCost, 0, len(d))
	for _, row := range d {
		out = append(out, *row)
	}
	return out
}

// openRouterActivity reads `GET /api/v1/activity`: one row per day, model and endpoint for the
// last 30 completed UTC days, which only a management key may read. `usage` is the cost in USD
// charged to the account; inference paid with the user's own provider keys is reported apart
// and not counted. https://openrouter.ai/docs/api/api-reference/analytics/get-user-activity
func openRouterActivity(channel *model.Channel, _, _ time.Time) ([]model.AccountDailyCost, error) {
	var out struct {
		Data []struct {
			Date     string  `json:"date"`
			Usage    float64 `json:"usage"`
			Requests int64   `json:"requests"`
		} `json:"data"`
	}
	headers := map[string]string{"Authorization": "Bearer " + channel.GetManageKey()}
	if err := getJSON(channel, baseURL(channel, "https://openrouter.ai/api")+"/v1/activity", headers, &out); err != nil {
		return nil, err
	}
	days := byDay{}
	for _, item := range out.Data {
		// The date may come with a time of day; the day is its first ten characters.
		if len(item.Date) >= 10 {
			days.add(item.Date[:10], item.Usage, item.Requests)
		}
	}
	return days.rows(), nil
}

// anthropicCostReport reads `GET /v1/organizations/cost_report` with an Admin API key: daily
// buckets in UTC, amounts as decimal strings in cents. Priority Tier costs are not in it.
// https://platform.claude.com/docs/en/api/beta/organization/cost_report/retrieve
func anthropicCostReport(channel *model.Channel, from, to time.Time) ([]model.AccountDailyCost, error) {
	headers := map[string]string{"x-api-key": channel.GetManageKey(), "anthropic-version": "2023-06-01"}
	days := byDay{}
	page := ""
	for range 10 {
		query := url.Values{"starting_at": {from.Format(time.RFC3339)}, "ending_at": {to.Format(time.RFC3339)}, "limit": {"31"}}
		if page != "" {
			query.Set("page", page)
		}
		var out struct {
			Data []struct {
				StartingAt string `json:"starting_at"`
				Results    []struct {
					Amount string `json:"amount"`
				} `json:"results"`
			} `json:"data"`
			HasMore  bool   `json:"has_more"`
			NextPage string `json:"next_page"`
		}
		reqURL := baseURL(channel, "https://api.anthropic.com") + "/v1/organizations/cost_report?" + query.Encode()
		if err := getJSON(channel, reqURL, headers, &out); err != nil {
			return nil, err
		}
		for _, bucket := range out.Data {
			if len(bucket.StartingAt) < 10 {
				continue
			}
			date := bucket.StartingAt[:10]
			days.add(date, 0, -1) // a day without costs is still a reported day
			for _, result := range bucket.Results {
				cents, err := strconv.ParseFloat(result.Amount, 64)
				if err != nil {
					return nil, fmt.Errorf("cost report amount %q is not a number", result.Amount)
				}
				days.add(date, cents/100, -1)
			}
		}
		if !out.HasMore || out.NextPage == "" {
			break
		}
		page = out.NextPage
	}
	return days.rows(), nil
}

// openAICosts reads `GET /v1/organization/costs` with an admin key: daily buckets in UTC,
// amounts in USD.
// https://developers.openai.com/api/reference/resources/organization/subresources/audit_logs/methods/get_costs
func openAICosts(channel *model.Channel, from, to time.Time) ([]model.AccountDailyCost, error) {
	headers := map[string]string{"Authorization": "Bearer " + channel.GetManageKey()}
	days := byDay{}
	page := ""
	for range 10 {
		query := url.Values{
			"start_time":   {strconv.FormatInt(from.Unix(), 10)},
			"end_time":     {strconv.FormatInt(to.Unix(), 10)},
			"bucket_width": {"1d"},
			"limit":        {"31"},
		}
		if page != "" {
			query.Set("page", page)
		}
		var out struct {
			Data []struct {
				StartTime int64 `json:"start_time"`
				Results   []struct {
					Amount struct {
						Value    float64 `json:"value"`
						Currency string  `json:"currency"`
					} `json:"amount"`
				} `json:"results"`
			} `json:"data"`
			HasMore  bool   `json:"has_more"`
			NextPage string `json:"next_page"`
		}
		reqURL := baseURL(channel, "https://api.openai.com") + "/v1/organization/costs?" + query.Encode()
		if err := getJSON(channel, reqURL, headers, &out); err != nil {
			return nil, err
		}
		for _, bucket := range out.Data {
			date := time.Unix(bucket.StartTime, 0).UTC().Format(time.DateOnly)
			days.add(date, 0, -1)
			for _, result := range bucket.Results {
				if c := result.Amount.Currency; c != "" && !strings.EqualFold(c, "usd") {
					return nil, fmt.Errorf("the cost report is in %s, only USD is supported", c)
				}
				days.add(date, result.Amount.Value, -1)
			}
		}
		if !out.HasMore || out.NextPage == "" {
			break
		}
		page = out.NextPage
	}
	return days.rows(), nil
}
