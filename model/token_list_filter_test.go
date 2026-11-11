package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupTokenListDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	if err := testDB.AutoMigrate(&Token{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

func TestListTokensConditionsAndCounts(t *testing.T) {
	setupTokenListDB(t)
	now := time.Now().Unix()
	day := int64(24 * 3600)
	seed := []Token{
		{Name: "never-expires", Status: config.TokenStatusEnabled, ExpiredTime: -1, AccessedTime: now - 60},
		{Name: "expires-soon", Status: config.TokenStatusEnabled, ExpiredTime: now + 3*day, AccessedTime: 0},
		{Name: "expires-later", Status: config.TokenStatusDisabled, ExpiredTime: now + 60*day, AccessedTime: now - 40*day},
		{Name: "expired", Status: config.TokenStatusExpired, ExpiredTime: now - day, AccessedTime: now - 2*day},
		{Name: "exhausted", Status: config.TokenStatusExhausted, ExpiredTime: -1, AccessedTime: 0},
	}
	for i := range seed {
		seed[i].UserId = 7
		seed[i].Key = fmt.Sprintf("k%d", i)
		if err := DB.Session(&gorm.Session{SkipHooks: true}).Create(&seed[i]).Error; err != nil {
			t.Fatalf("create token: %v", err)
		}
	}
	other := Token{UserId: 8, Key: "other", Name: "other", Status: config.TokenStatusEnabled, ExpiredTime: -1}
	if err := DB.Session(&gorm.Session{SkipHooks: true}).Create(&other).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	names := func(f TokenListFilter) ([]string, TokenStatusCounts) {
		t.Helper()
		params := &TokenListParams{TokenListFilter: f}
		params.Order = "name"
		result, counts, err := ListUserTokens(7, params)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		var out []string
		for _, tk := range *result.Data {
			out = append(out, tk.Name)
		}
		return out, counts
	}

	cases := []struct {
		filter TokenListFilter
		want   string
	}{
		{TokenListFilter{}, "[exhausted expired expires-later expires-soon never-expires]"},
		{TokenListFilter{Expiry: "never"}, "[exhausted never-expires]"},
		{TokenListFilter{Expiry: "expired"}, "[expired]"},
		{TokenListFilter{Expiry: "soon"}, "[expires-soon]"},
		{TokenListFilter{Activity: "never"}, "[exhausted expires-soon]"},
		{TokenListFilter{Activity: "idle"}, "[expires-later]"},
		{TokenListFilter{Status: "enabled"}, "[expires-soon never-expires]"},
		{TokenListFilter{Status: "disabled"}, "[expires-later]"},
		{TokenListFilter{Status: "invalid"}, "[exhausted expired]"},
		{TokenListFilter{Status: "invalid", Expiry: "never"}, "[exhausted]"},
		{TokenListFilter{Expiry: "bogus", Status: "bogus"}, "[exhausted expired expires-later expires-soon never-expires]"},
	}
	for _, c := range cases {
		got, _ := names(c.filter)
		if fmt.Sprint(got) != c.want {
			t.Errorf("filter %+v: got %v, want %s", c.filter, got, c.want)
		}
	}

	// Counts ignore the segment but follow the conditions.
	_, counts := names(TokenListFilter{Status: "disabled"})
	if counts != (TokenStatusCounts{All: 5, Enabled: 2, Disabled: 1, Invalid: 2}) {
		t.Errorf("counts with segment: %+v", counts)
	}
	_, counts = names(TokenListFilter{Expiry: "never"})
	if counts != (TokenStatusCounts{All: 2, Enabled: 1, Invalid: 1}) {
		t.Errorf("counts with condition: %+v", counts)
	}
}

func TestListOrgTokensCreatedBy(t *testing.T) {
	setupTokenListDB(t)
	for i, by := range []int{1, 1, 2} {
		tk := Token{UserId: 99, Key: fmt.Sprintf("o%d", i), Name: fmt.Sprintf("o%d", i), Status: config.TokenStatusEnabled, ExpiredTime: -1, CreatedBy: by}
		if err := DB.Session(&gorm.Session{SkipHooks: true}).Create(&tk).Error; err != nil {
			t.Fatalf("create token: %v", err)
		}
	}
	count := func(onlyCreatedBy, createdBy int) int64 {
		t.Helper()
		params := &TokenListParams{TokenListFilter: TokenListFilter{CreatedBy: createdBy}}
		_, counts, err := ListOrgTokens(99, onlyCreatedBy, params)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		return counts.All
	}
	if got := count(0, 0); got != 3 {
		t.Errorf("all: %d", got)
	}
	if got := count(0, 2); got != 1 {
		t.Errorf("created_by=2: %d", got)
	}
	// A member is held to their own keys whatever created_by asks for.
	if got := count(1, 2); got != 2 {
		t.Errorf("member override: %d", got)
	}
}

func TestListTokensSpendAndLimitConditions(t *testing.T) {
	setupTokenListDB(t)
	unit := int(config.QuotaPerUnit)
	periodic := Token{Name: "periodic", UnlimitedQuota: true, PeriodUsed: 90 * unit / 100, UsedQuota: 20 * unit}
	periodic.Setting.Set(TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: unit}})
	seed := []Token{
		{Name: "unlimited", UnlimitedQuota: true, UsedQuota: 5 * unit, Group: "vip"},
		{Name: "total-low", UsedQuota: unit, RemainQuota: 9 * unit},
		{Name: "total-high", UsedQuota: 9 * unit, RemainQuota: unit},
		periodic,
	}
	for i := range seed {
		seed[i].UserId = 3
		seed[i].Key = fmt.Sprintf("s%d", i)
		seed[i].Status = config.TokenStatusEnabled
		seed[i].ExpiredTime = -1
		if err := DB.Session(&gorm.Session{SkipHooks: true}).Create(&seed[i]).Error; err != nil {
			t.Fatalf("create token: %v", err)
		}
	}
	cases := []struct {
		filter TokenListFilter
		want   string
	}{
		{TokenListFilter{Limit: "none"}, "[unlimited]"},
		{TokenListFilter{Limit: "any"}, "[periodic total-high total-low]"},
		{TokenListFilter{BudgetMin: 80}, "[periodic total-high]"},
		{TokenListFilter{SpentMin: 5}, "[periodic total-high unlimited]"},
		{TokenListFilter{Group: "vip"}, "[unlimited]"},
		{TokenListFilter{Group: "_default"}, "[periodic total-high total-low]"},
		{TokenListFilter{Limit: "none", BudgetMin: 50}, "[]"},
	}
	for _, c := range cases {
		params := &TokenListParams{TokenListFilter: c.filter}
		params.Order = "name"
		result, counts, err := ListUserTokens(3, params)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		var got []string
		for _, tk := range *result.Data {
			got = append(got, tk.Name)
		}
		if fmt.Sprint(got) != c.want {
			t.Errorf("filter %+v: got %v, want %s", c.filter, got, c.want)
		}
		if counts.All != int64(len(got)) {
			t.Errorf("filter %+v: count %d, rows %d", c.filter, counts.All, len(got))
		}
	}
}
