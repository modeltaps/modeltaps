package model

import (
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/gorm"
)

// Windows used by the API key list conditions.
const (
	tokenExpirySoonWindow = 14 * 24 * time.Hour
	tokenIdleWindow       = 30 * 24 * time.Hour
)

// TokenListFilter holds the conditions the API key list can be narrowed by. Every field is
// optional; unknown values are ignored so an old or hand-edited URL never errors.
type TokenListFilter struct {
	// Status picks one of the list's segments: enabled, disabled or invalid (expired or used up).
	Status string `form:"status"`
	// Expiry: never (no expiry date), expired (date passed) or soon (expires within 14 days).
	Expiry string `form:"expiry"`
	// Activity: never (no request yet) or idle (no request in the last 30 days).
	Activity string `form:"activity"`
	// CreatedBy narrows an organization's keys to one member's; ignored outside organizations.
	CreatedBy int `form:"created_by"`
	// Group: the key's group symbol; "_default" picks keys that follow the account's group.
	Group string `form:"group"`
	// SpentMin keeps keys whose lifetime spend is at least this many US dollars.
	SpentMin float64 `form:"spent_min"`
	// Limit: none (no spending limit at all) or any (a total or periodic limit).
	Limit string `form:"limit"`
	// BudgetMin keeps keys with a limit that have used at least this percentage of it (the current
	// period's cap for periodic keys).
	BudgetMin float64 `form:"budget_min"`
}

// tokenDefaultGroupFilter is the group condition value for keys without a group of their own.
const tokenDefaultGroupFilter = "_default"


// TokenListParams is the query of the API key list: paging, name keyword and conditions.
type TokenListParams struct {
	GenericParams
	TokenListFilter
}

// TokenStatusCounts counts the keys behind each list segment. The counts apply every condition
// except the segment itself, so switching segments shows how many keys each one holds.
type TokenStatusCounts struct {
	All      int64 `json:"all"`
	Enabled  int64 `json:"enabled"`
	Disabled int64 `json:"disabled"`
	Invalid  int64 `json:"invalid"`
}

// applyTokenConditions narrows db by the expiry and activity conditions.
func applyTokenConditions(db *gorm.DB, f *TokenListFilter, now time.Time) *gorm.DB {
	nowUnix := now.Unix()
	switch f.Expiry {
	case "never":
		db = db.Where("expired_time = ?", -1)
	case "expired":
		db = db.Where("expired_time <> ? AND expired_time <= ?", -1, nowUnix)
	case "soon":
		db = db.Where("expired_time > ? AND expired_time <= ?", nowUnix, now.Add(tokenExpirySoonWindow).Unix())
	}
	// group is a reserved word: quote it for the database in use.
	groupCol := "`group`"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
	}
	if f.Group == tokenDefaultGroupFilter {
		db = db.Where(groupCol+" = ?", "")
	} else if f.Group != "" {
		db = db.Where(groupCol+" = ?", f.Group)
	}
	if f.SpentMin > 0 {
		db = db.Where("used_quota >= ?", int64(f.SpentMin*config.QuotaPerUnit))
	}
	switch f.Activity {
	case "never":
		db = db.Where("accessed_time <= ?", 0)
	case "idle":
		db = db.Where("accessed_time > ? AND accessed_time < ?", 0, now.Add(-tokenIdleWindow).Unix())
	}
	return db
}

// applyTokenLimitConditions narrows db by the limit conditions. A periodic cap lives in the
// setting JSON, which the supported databases cannot query alike, so the candidate keys (one
// owner's or organization's, already narrowed by the other conditions) are checked here and the
// matches passed back as an id list.
func applyTokenLimitConditions(db *gorm.DB, f *TokenListFilter) (*gorm.DB, error) {
	if f.Limit != "none" && f.Limit != "any" && f.BudgetMin <= 0 {
		return db, nil
	}
	var candidates []*Token
	if err := db.Session(&gorm.Session{}).
		Select("id", "used_quota", "remain_quota", "unlimited_quota", "period_used", "setting").
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(candidates))
	for _, tk := range candidates {
		used, limit := tokenSpendAgainstLimit(tk)
		switch {
		case f.Limit == "none" && limit > 0, f.Limit == "any" && limit == 0:
			continue
		case f.BudgetMin > 0 && (limit == 0 || float64(used)*100 < f.BudgetMin*float64(limit)):
			continue
		}
		ids = append(ids, tk.Id)
	}
	if len(ids) == 0 {
		return db.Where("1 = 0"), nil
	}
	return db.Where("id IN ?", ids), nil
}

// tokenSpendAgainstLimit returns what a key has spent against its limit: the period counter
// against a periodic cap, or the total spend against a total limit. limit is 0 without one.
func tokenSpendAgainstLimit(tk *Token) (used int64, limit int64) {
	setting := tk.Setting.Data()
	if tokenInPeriodMode(&setting) {
		return int64(tk.PeriodUsed), int64(setting.QuotaReset.Limit)
	}
	if tk.UnlimitedQuota {
		return int64(tk.UsedQuota), 0
	}
	return int64(tk.UsedQuota), int64(tk.UsedQuota) + int64(tk.RemainQuota)
}

// applyTokenStatus narrows db to one list segment.
func applyTokenStatus(db *gorm.DB, status string) *gorm.DB {
	switch status {
	case "enabled":
		return db.Where("status = ?", config.TokenStatusEnabled)
	case "disabled":
		return db.Where("status = ?", config.TokenStatusDisabled)
	case "invalid":
		return db.Where("status IN ?", []int{config.TokenStatusExpired, config.TokenStatusExhausted})
	}
	return db
}

// countTokensByStatus counts the keys matched by db in each segment.
func countTokensByStatus(db *gorm.DB) (TokenStatusCounts, error) {
	var rows []struct {
		Status int
		N      int64
	}
	var counts TokenStatusCounts
	if err := db.Model(&Token{}).Select("status, COUNT(*) AS n").Group("status").Scan(&rows).Error; err != nil {
		return counts, err
	}
	for _, r := range rows {
		counts.All += r.N
		switch r.Status {
		case config.TokenStatusEnabled:
			counts.Enabled += r.N
		case config.TokenStatusDisabled:
			counts.Disabled += r.N
		case config.TokenStatusExpired, config.TokenStatusExhausted:
			counts.Invalid += r.N
		}
	}
	return counts, nil
}

// listTokens pages the keys matched by base (owner and keyword already applied) after the
// conditions and segment, and counts the segments.
func listTokens(base *gorm.DB, params *TokenListParams) (*DataResult[Token], TokenStatusCounts, error) {
	conditioned, err := applyTokenLimitConditions(applyTokenConditions(base, &params.TokenListFilter, time.Now()), &params.TokenListFilter)
	if err != nil {
		return nil, TokenStatusCounts{}, err
	}
	conditioned = conditioned.Session(&gorm.Session{})
	counts, err := countTokensByStatus(conditioned.Session(&gorm.Session{}))
	if err != nil {
		return nil, counts, err
	}
	var tokens []*Token
	db := applyTokenStatus(conditioned.Session(&gorm.Session{}), params.Status)
	result, err := PaginateAndOrder(db, &params.PaginationParams, &tokens, allowedTokenOrderFields)
	return result, counts, err
}

// ListUserTokens is the personal API key list with conditions and segment counts.
func ListUserTokens(userId int, params *TokenListParams) (*DataResult[Token], TokenStatusCounts, error) {
	db := DB.Model(&Token{}).Where("user_id = ?", userId)
	if params.Keyword != "" {
		db = db.Where("name LIKE ?", params.Keyword+"%")
	}
	return listTokens(db, params)
}

// ListOrgTokens is an organization's API key list with conditions and segment counts. A
// non-zero onlyCreatedBy (members see only their own keys) overrides params.CreatedBy.
func ListOrgTokens(shadowUserId int, onlyCreatedBy int, params *TokenListParams) (*DataResult[Token], TokenStatusCounts, error) {
	db := DB.Model(&Token{}).Where("user_id = ?", shadowUserId)
	createdBy := params.CreatedBy
	if onlyCreatedBy > 0 {
		createdBy = onlyCreatedBy
	}
	if createdBy > 0 {
		db = db.Where("created_by = ?", createdBy)
	}
	if params.Keyword != "" {
		db = db.Where("name LIKE ?", params.Keyword+"%")
	}
	return listTokens(db, params)
}
