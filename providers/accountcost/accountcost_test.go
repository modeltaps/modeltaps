package accountcost

import (
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
)

// now is 2026-11-15 10:00 UTC: the last completed day is 2026-11-14.
var now = time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)

// reportChannel is a channel of the type whose report is served by the handler.
func reportChannel(t *testing.T, channelType int, handler http.HandlerFunc) *model.Channel {
	t.Helper()
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	anyHost = true
	t.Cleanup(func() { anyHost = false })
	baseURL, manageKey := upstream.URL, "manage-key"
	return &model.Channel{Type: channelType, Key: "inference-key", ManageKey: &manageKey, BaseURL: &baseURL}
}

func costOf(t *testing.T, days []model.AccountDailyCost, date string) model.AccountDailyCost {
	t.Helper()
	for _, day := range days {
		if day.Date == date {
			return day
		}
	}
	t.Fatalf("no day %s in %+v", date, days)
	return model.AccountDailyCost{}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestOpenRouterActivity(t *testing.T) {
	var gotPath, gotAuth string
	channel := reportChannel(t, config.ChannelTypeOpenRouter, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[
		 {"date":"2026-11-14","model":"a","usage":1.25,"byok_usage_inference":9,"requests":10},
		 {"date":"2026-11-14","model":"b","usage":0.75,"requests":5},
		 {"date":"2026-11-13 00:00:00","model":"a","usage":2,"requests":1},
		 {"date":"2026-11-15","model":"a","usage":7,"requests":1}
		]}`))
	})
	days, err := Fetch(channel, now)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/activity" || gotAuth != "Bearer manage-key" {
		t.Errorf("request = %s with %q, want the management key", gotPath, gotAuth)
	}
	// Today is left out; the rows of a day are summed; cost on the user's own keys is not counted.
	if len(days) != 2 || days[0].Date != "2026-11-13" {
		t.Fatalf("days = %+v", days)
	}
	if day := costOf(t, days, "2026-11-14"); !near(day.Cost, 2) || day.Requests != 15 {
		t.Errorf("2026-11-14 = %+v, want 2 USD over 15 requests", day)
	}
}

func TestAnthropicCostReport(t *testing.T) {
	var gotKey, gotStart, gotEnd string
	calls := 0
	channel := reportChannel(t, config.ChannelTypeAnthropic, func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotKey, gotStart, gotEnd = r.Header.Get("x-api-key"), r.URL.Query().Get("starting_at"), r.URL.Query().Get("ending_at")
		if r.URL.Path != "/v1/organizations/cost_report" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") == "" {
			_, _ = w.Write([]byte(`{"data":[{"starting_at":"2026-11-13T00:00:00Z","ending_at":"2026-11-14T00:00:00Z",
			 "results":[{"amount":"123.45","currency":"USD"},{"amount":"76.55","currency":"USD"}]}],
			 "has_more":true,"next_page":"p2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"starting_at":"2026-11-14T00:00:00Z","ending_at":"2026-11-15T00:00:00Z","results":[]}],
		 "has_more":false,"next_page":null}`))
	})
	days, err := Fetch(channel, now)
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "manage-key" || gotStart != "2026-10-16T00:00:00Z" || gotEnd != "2026-11-15T00:00:00Z" || calls != 2 {
		t.Errorf("request: key %q, %s to %s, %d calls", gotKey, gotStart, gotEnd, calls)
	}
	// Amounts are cents: 123.45 + 76.55 = 200 cents.
	if day := costOf(t, days, "2026-11-13"); !near(day.Cost, 2) || day.Requests != -1 {
		t.Errorf("2026-11-13 = %+v, want 2 USD", day)
	}
	// A day the report lists without costs is a reported day of 0.
	if day := costOf(t, days, "2026-11-14"); day.Cost != 0 {
		t.Errorf("2026-11-14 = %+v, want 0", day)
	}
}

func TestOpenAICosts(t *testing.T) {
	var gotAuth, gotStart string
	channel := reportChannel(t, config.ChannelTypeOpenAI, func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotStart = r.Header.Get("Authorization"), r.URL.Query().Get("start_time")
		if r.URL.Path != "/v1/organization/costs" {
			t.Errorf("path = %s", r.URL.Path)
		}
		// 1794614400 is 2026-11-14T00:00:00Z.
		_, _ = w.Write([]byte(`{"object":"page","data":[{"object":"bucket","start_time":1794614400,"end_time":1794700800,
		 "results":[{"amount":{"value":0.06,"currency":"usd"}},{"amount":{"value":1.5,"currency":"usd"}}]}],"has_more":false,"next_page":null}`))
	})
	days, err := Fetch(channel, now)
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer manage-key" || gotStart != "1792108800" {
		t.Errorf("request: auth %q, start %s", gotAuth, gotStart)
	}
	if day := costOf(t, days, "2026-11-14"); !near(day.Cost, 1.56) {
		t.Errorf("2026-11-14 = %+v, want 1.56 USD", day)
	}
}

func TestFetchRefusals(t *testing.T) {
	channel := reportChannel(t, config.ChannelTypeOpenRouter, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403,"message":"Only management keys can perform this operation"}}`))
	})
	if _, err := Fetch(channel, now); err == nil || !strings.Contains(err.Error(), "management key") {
		t.Errorf("a refused key: err = %v", err)
	}

	noKey := *channel
	noKey.ManageKey = nil
	if _, err := Fetch(&noKey, now); !errors.Is(err, ErrNoManageKey) {
		t.Errorf("no management key: err = %v", err)
	}
	if _, err := Fetch(&model.Channel{Type: config.ChannelTypeGroq, ManageKey: channel.ManageKey}, now); !errors.Is(err, ErrNotSupported) {
		t.Errorf("a type without a report: err = %v", err)
	}
}

// A channel of a supported type that points at another host is a relay: it has no report, and
// the management key is not sent to it.
func TestSupportedOnlyOnTheOfficialHost(t *testing.T) {
	relay, official, key := "https://relay.example.com", "https://api.openai.com/", "k"
	cases := []struct {
		channel model.Channel
		want    bool
	}{
		{model.Channel{Type: config.ChannelTypeOpenAI}, true},
		{model.Channel{Type: config.ChannelTypeOpenAI, BaseURL: &official}, true},
		{model.Channel{Type: config.ChannelTypeOpenAI, BaseURL: &relay}, false},
		{model.Channel{Type: config.ChannelTypeAnthropic, BaseURL: &relay}, false},
		{model.Channel{Type: config.ChannelTypeOpenRouter}, true},
		{model.Channel{Type: config.ChannelTypeGroq}, false},
	}
	for _, tc := range cases {
		tc.channel.ManageKey = &key
		if got := Supported(&tc.channel); got != tc.want {
			t.Errorf("type %d base %q: supported = %v, want %v", tc.channel.Type, tc.channel.GetBaseURL(), got, tc.want)
		}
	}
}
