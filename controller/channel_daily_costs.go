package controller

import (
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/accountcost"

	"github.com/gin-gonic/gin"
)

// dailyCostDays is how many completed UTC days the comparison covers.
const dailyCostDays = 30

// dailyCostRefreshEvery is the least time between two scheduled reads of one account's report.
const dailyCostRefreshEvery = 6 * time.Hour

var (
	dailyCostFetchedMu sync.Mutex
	dailyCostFetched   = map[string]time.Time{} // account -> last scheduled read
)

// channelDailyCostDay is one UTC day of the comparison: what the upstream billed the account
// and what this site expected for the requests it sent through the account's channels.
type channelDailyCostDay struct {
	Date string `json:"date"`
	// UpstreamCost is nil for a day the upstream report does not have (yet).
	UpstreamCost     *float64 `json:"upstream_cost"`
	UpstreamRequests *int64   `json:"upstream_requests"`
	ExpectedCost     float64  `json:"expected_cost"`
	RequestCount     int64    `json:"request_count"`
}

type channelDailyCosts struct {
	// Supported is false when the channel's upstream has no daily cost report.
	Supported    bool `json:"supported"`
	HasManageKey bool `json:"has_manage_key"`
	// Channels are the channels of the same upstream account (same management key), this one
	// included: the upstream bills the account, so their requests are summed.
	Channels  []channelRef          `json:"channels"`
	UpdatedAt int64                 `json:"updated_at"`
	Days      []channelDailyCostDay `json:"days"`
}

type channelRef struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// buildChannelDailyCosts lines up the stored upstream report of the channel's account with the
// expected cost of the account's channels, for the last completed UTC days.
func buildChannelDailyCosts(channel *model.Channel, now time.Time) (*channelDailyCosts, error) {
	out := &channelDailyCosts{
		Supported:    accountcost.Supported(channel),
		HasManageKey: channel.GetManageKey() != "",
		Channels:     []channelRef{},
		Days:         []channelDailyCostDay{},
	}
	account := model.AccountOfChannel(channel)
	if !out.Supported || account == "" {
		return out, nil
	}

	channels, err := model.ChannelsOfAccount(account)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(channels))
	for _, ch := range channels {
		ids = append(ids, ch.Id)
		out.Channels = append(out.Channels, channelRef{Id: ch.Id, Name: ch.Name})
	}

	to := now.UTC().Truncate(24 * time.Hour)
	from := to.AddDate(0, 0, -dailyCostDays)
	upstream, err := model.GetAccountDailyCosts(account, from.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	expected, err := model.GetExpectedDailyCosts(ids, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}

	days := map[string]*channelDailyCostDay{}
	day := func(date string) *channelDailyCostDay {
		if d, ok := days[date]; ok {
			return d
		}
		d := &channelDailyCostDay{Date: date}
		days[date] = d
		return d
	}
	for _, row := range upstream {
		if row.Date >= to.Format(time.DateOnly) {
			continue
		}
		d := day(row.Date)
		cost := row.Cost
		d.UpstreamCost = &cost
		if row.Requests >= 0 {
			requests := row.Requests
			d.UpstreamRequests = &requests
		}
		if row.UpdatedAt > out.UpdatedAt {
			out.UpdatedAt = row.UpdatedAt
		}
	}
	for _, row := range expected {
		d := day(row.Date)
		d.ExpectedCost, d.RequestCount = row.ExpectedCost, row.RequestCount
	}
	// Newest day first; days with nothing on either side are left out.
	for date := to.AddDate(0, 0, -1); !date.Before(from); date = date.AddDate(0, 0, -1) {
		if d, ok := days[date.Format(time.DateOnly)]; ok {
			out.Days = append(out.Days, *d)
		}
	}
	return out, nil
}

// refreshChannelDailyCosts reads the upstream report of the channel's account and stores it.
func refreshChannelDailyCosts(channel *model.Channel, now time.Time) error {
	days, err := accountcost.Fetch(channel, now)
	if err != nil {
		return err
	}
	return model.SaveAccountDailyCosts(model.AccountOfChannel(channel), days)
}

// refreshAllAccountDailyCosts reads the daily cost report of every upstream account that has
// one, once per account and at most every dailyCostRefreshEvery. Failures are only logged.
func refreshAllAccountDailyCosts(channels []*model.Channel) {
	now := time.Now()
	for _, channel := range channels {
		account := model.AccountOfChannel(channel)
		if account == "" || !accountcost.Supported(channel) {
			continue
		}
		dailyCostFetchedMu.Lock()
		last, seen := dailyCostFetched[account]
		due := !seen || now.Sub(last) >= dailyCostRefreshEvery
		if due {
			dailyCostFetched[account] = now
		}
		dailyCostFetchedMu.Unlock()
		if !due {
			continue
		}
		if err := refreshChannelDailyCosts(channel, now); err != nil {
			logger.SysError("daily cost report failed for channel " + channel.Name + " (ID " + strconv.Itoa(channel.Id) + "): " + err.Error())
		}
	}
}

func channelForDailyCosts(c *gin.Context) *model.Channel {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return nil
	}
	channel, err := model.GetChannelById(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return nil
	}
	return channel
}

func respondChannelDailyCosts(c *gin.Context, channel *model.Channel) {
	data, err := buildChannelDailyCosts(channel, time.Now())
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

// GetChannelDailyCosts returns, per completed UTC day of the last 30, what the upstream billed
// the channel's account (from the stored report) and what this site expected for the account's
// channels.
func GetChannelDailyCosts(c *gin.Context) {
	if channel := channelForDailyCosts(c); channel != nil {
		respondChannelDailyCosts(c, channel)
	}
}

// RefreshChannelDailyCosts reads the upstream report now, stores it and returns the comparison.
func RefreshChannelDailyCosts(c *gin.Context) {
	channel := channelForDailyCosts(c)
	if channel == nil {
		return
	}
	if err := refreshChannelDailyCosts(channel, time.Now()); err != nil {
		if errors.Is(err, accountcost.ErrNotSupported) || errors.Is(err, accountcost.ErrNoManageKey) {
			respondChannelDailyCosts(c, channel)
			return
		}
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	respondChannelDailyCosts(c, channel)
}
