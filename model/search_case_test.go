package model

import (
	"slices"
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"github.com/spf13/viper"
)

// Searches must ignore case on every database. PostgreSQL LIKE is case-sensitive, so before
// caseInsensitiveLike a search for "TOR" found torworx@example.com on SQLite but nothing on PostgreSQL.
func TestKeywordSearchIgnoresCase(t *testing.T) {
	// Token.AfterCreate encodes the token id, which needs the encoder initialised.
	if viper.GetString("user_token_secret") == "" {
		viper.Set("user_token_secret", "search-case-test-secret")
		if err := common.InitUserToken(); err != nil {
			t.Fatal(err)
		}
	}
	for name, open := range exactModelDialects(t) {
		t.Run(name, func(t *testing.T) {
			testDB, err := open()
			if err != nil {
				t.Fatal(err)
			}
			if sqlDB, err := testDB.DB(); err == nil {
				sqlDB.SetMaxOpenConns(1)
				t.Cleanup(func() { sqlDB.Close() })
			}
			tables := []any{&User{}, &Organization{}, &OrganizationMember{}, &Channel{}, &Token{}}
			if err := testDB.Migrator().DropTable(tables...); err != nil {
				t.Fatal(err)
			}
			if err := testDB.AutoMigrate(tables...); err != nil {
				t.Fatal(err)
			}
			oldDB, oldPG := DB, common.UsingPostgreSQL
			DB, common.UsingPostgreSQL = testDB, name == "postgres"
			t.Cleanup(func() { DB, common.UsingPostgreSQL = oldDB, oldPG })

			users := []*User{
				{Username: "torworx", Email: "Torworx@Example.com", DisplayName: "Tor", AccessToken: "a1", AffCode: "c1"},
				{Username: "alice", Email: "alice@example.com", DisplayName: "Alice", AccessToken: "a2", AffCode: "c2"},
			}
			for _, u := range users {
				if err := testDB.Create(u).Error; err != nil {
					t.Fatal(err)
				}
			}
			for _, keyword := range []string{"tor", "TOR", "Tor"} {
				result, err := GetUsersList(&SearchUserParams{GenericParams: GenericParams{
					Keyword: keyword, PaginationParams: PaginationParams{Page: 1, Size: 10}}})
				if err != nil {
					t.Fatalf("users %q: %v", keyword, err)
				}
				got := []string{}
				for _, u := range *result.Data {
					got = append(got, u.Username)
				}
				if !slices.Equal(got, []string{"torworx"}) {
					t.Errorf("users keyword=%q: got %v, want [torworx]", keyword, got)
				}
			}

			for _, ch := range []*Channel{
				{Name: "OpenRouter Main", Key: "k1", Status: 1, BaseURL: ptr("https://OpenRouter.ai/api")},
				{Name: "anthropic", Key: "k2", Status: 1},
			} {
				if err := testDB.Create(ch).Error; err != nil {
					t.Fatal(err)
				}
			}
			for _, params := range []*SearchChannelsParams{
				{Channel: Channel{Name: "openrouter"}},
				{Channel: Channel{Name: "OPENROUTER MAIN"}},
				{BaseURL: "openrouter.AI"},
			} {
				params.PaginationParams = PaginationParams{Page: 1, Size: 10}
				result, err := GetChannelsList(params)
				if err != nil {
					t.Fatalf("channels %+v: %v", params, err)
				}
				if len(*result.Data) != 1 || (*result.Data)[0].Name != "OpenRouter Main" {
					t.Errorf("channels name=%q base_url=%q: got %d rows", params.Name, params.BaseURL, len(*result.Data))
				}
			}

			if err := testDB.Create(&Token{UserId: 1, Name: "Claude Code", Key: "tk1"}).Error; err != nil {
				t.Fatal(err)
			}
			result, err := GetUserTokensList(1, &GenericParams{Keyword: "claude", PaginationParams: PaginationParams{Page: 1, Size: 10}})
			if err != nil {
				t.Fatal(err)
			}
			if len(*result.Data) != 1 {
				t.Errorf("tokens keyword=claude: got %d rows, want 1", len(*result.Data))
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
