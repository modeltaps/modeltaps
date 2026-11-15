package model

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm/clause"
)

// AccountDailyCost is what an upstream account was billed on one UTC day, as the upstream's own
// daily report says, read with a channel's management key. The report is for the whole account:
// every channel with the same management key shares these rows, and so does anything that uses
// the account outside this site.
type AccountDailyCost struct {
	Id int `json:"id"`
	// Account identifies the upstream account without storing its key: see AccountOfChannel.
	Account string `json:"-" gorm:"type:varchar(32);uniqueIndex:idx_account_daily_cost,priority:1"`
	// Date is the UTC day, "2006-01-02".
	Date string `json:"date" gorm:"type:varchar(10);uniqueIndex:idx_account_daily_cost,priority:2"`
	// Cost is in USD. Requests is the request count of the day, -1 when the report has none.
	Cost      float64 `json:"cost"`
	Requests  int64   `json:"requests"`
	UpdatedAt int64   `json:"updated_at" gorm:"bigint"`
}

// AccountOfChannel names the upstream account a channel's management key belongs to: a digest
// of the channel type and the key, so channels of one account get the same name. "" when the
// channel has no management key.
func AccountOfChannel(channel *Channel) string {
	manageKey := channel.GetManageKey()
	if manageKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(strconv.Itoa(channel.Type) + ":" + manageKey))
	return hex.EncodeToString(sum[:16])
}

// ChannelsOfAccount returns the channels whose management key is the account's, oldest first.
func ChannelsOfAccount(account string) ([]*Channel, error) {
	if account == "" {
		return nil, nil
	}
	var channels []*Channel
	if err := DB.Where("manage_key IS NOT NULL AND manage_key <> ''").Order("id").Find(&channels).Error; err != nil {
		return nil, err
	}
	same := channels[:0]
	for _, channel := range channels {
		if AccountOfChannel(channel) == account {
			same = append(same, channel)
		}
	}
	return same, nil
}

// SaveAccountDailyCosts stores the days of an upstream report, replacing the days already known.
func SaveAccountDailyCosts(account string, days []AccountDailyCost) error {
	if account == "" || len(days) == 0 {
		return nil
	}
	now := utils.GetTimestamp()
	rows := make([]AccountDailyCost, 0, len(days))
	for _, day := range days {
		rows = append(rows, AccountDailyCost{Account: account, Date: day.Date, Cost: day.Cost, Requests: day.Requests, UpdatedAt: now})
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{"cost", "requests", "updated_at"}),
	}).Create(&rows).Error
}

// GetAccountDailyCosts returns the stored days of an account from the given UTC day on.
func GetAccountDailyCosts(account, fromDate string) ([]*AccountDailyCost, error) {
	rows := make([]*AccountDailyCost, 0)
	err := DB.Where("account = ? AND date >= ?", account, fromDate).Order("date").Find(&rows).Error
	return rows, err
}

// ExpectedDailyCost is what this site expected the upstream to charge for the requests of one
// UTC day (the sum of the consume logs' expected cost), and how many requests those were.
type ExpectedDailyCost struct {
	Date         string  `json:"date"`
	ExpectedCost float64 `json:"expected_cost"`
	RequestCount int64   `json:"request_count"`
}

// utcDayExpr is the number of the UTC day a log was written on (days since 1970-01-01).
func utcDayExpr() string {
	if common.UsingPostgreSQL || common.UsingSQLite {
		return "(created_at / 86400)" // integer division on both
	}
	return "(created_at DIV 86400)"
}

// GetExpectedDailyCosts sums the consume logs of the channels per UTC day in [start, end)
// (unix seconds), ordered by day.
func GetExpectedDailyCosts(channelIds []int, start, end int64) ([]*ExpectedDailyCost, error) {
	out := make([]*ExpectedDailyCost, 0)
	if len(channelIds) == 0 {
		return out, nil
	}
	var rows []struct {
		Day          int64
		ExpectedCost float64
		RequestCount int64
	}
	err := DB.Table("logs").
		Select(utcDayExpr()+" as day, sum(expected_cost) as expected_cost, count(*) as request_count").
		Where("type = ? AND channel_id IN ? AND created_at >= ? AND created_at < ?", LogTypeConsume, channelIds, start, end).
		Group(utcDayExpr()).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, &ExpectedDailyCost{
			Date:         time.Unix(row.Day*86400, 0).UTC().Format(time.DateOnly),
			ExpectedCost: row.ExpectedCost,
			RequestCount: row.RequestCount,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}
