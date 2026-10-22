package model

import (
	"reflect"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ORG-14:令牌模型限制与组织 / 成员白名单共用一套匹配规则。
func TestModelAllowlistPermits(t *testing.T) {
	setupModelInfoTestDB(t)
	for _, info := range []*ModelInfo{{Model: "gemini-2.5-pro"}, {Model: "google/gemini-2.5-pro", AliasOf: "gemini-2.5-pro"}} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name      string
		allowlist []string
		model     string
		want      bool
	}{
		{"empty list does not limit", nil, "anything", true},
		{"exact", []string{"gpt-4o"}, "gpt-4o", true},
		{"case insensitive", []string{"GPT-4o"}, "gpt-4O", true},
		{"prefix wildcard", []string{"claude-3*"}, "claude-3-opus", true},
		{"prefix wildcard miss", []string{"claude-3*"}, "gpt-4o", false},
		{"not listed", []string{"gpt-4o"}, "o1-preview", false},
		{"blank entries skipped", []string{" ", "gpt-4o"}, "gpt-4o", true},
		{"canonical listed, alias requested", []string{"gemini-2.5-pro"}, "google/gemini-2.5-pro", true},
		{"alias listed, canonical requested", []string{"google/gemini-2.5-pro"}, "gemini-2.5-pro", true},
		{"wildcard matches the canonical of an alias", []string{"gemini-*"}, "google/gemini-2.5-pro", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ModelAllowlistPermits(tt.allowlist, tt.model); got != tt.want {
				t.Fatalf("ModelAllowlistPermits(%v, %q) = %v, want %v", tt.allowlist, tt.model, got, tt.want)
			}
		})
	}
}

func TestFilterModelsByTokenSetting(t *testing.T) {
	all := []string{"gpt-4o", "gpt-4.1", "claude-sonnet"}
	setting := func(enabled bool, models ...string) *TokenSetting {
		s := &TokenSetting{}
		s.Limits.LimitModelSetting = LimitModelSetting{Enabled: enabled, Models: models}
		return s
	}
	if got := FilterModelsByTokenSetting(nil, all); len(got) != 3 {
		t.Fatalf("nil setting should not filter, got %v", got)
	}
	if got := FilterModelsByTokenSetting(setting(false, "gpt-4o"), all); len(got) != 3 {
		t.Fatalf("disabled limit should not filter, got %v", got)
	}
	if got := FilterModelsByTokenSetting(setting(true), all); len(got) != 0 {
		t.Fatalf("legacy enabled + empty list should list nothing, got %v", got)
	}
	if got := FilterModelsByTokenSetting(setting(true, "GPT-4*"), all); !reflect.DeepEqual(got, []string{"gpt-4o", "gpt-4.1"}) {
		t.Fatalf("wildcard limit got %v", got)
	}
}

func TestNormalizeLimitModelSetting(t *testing.T) {
	tests := []struct {
		name string
		in   LimitModelSetting
		want LimitModelSetting
	}{
		{"off clears the list", LimitModelSetting{Enabled: false, Models: []string{"gpt-4o"}}, LimitModelSetting{Enabled: false, Models: []string{}}},
		{"on with empty list means no limit", LimitModelSetting{Enabled: true}, LimitModelSetting{Enabled: false, Models: []string{}}},
		{"blank and duplicate entries dropped", LimitModelSetting{Enabled: true, Models: []string{" gpt-4o ", "", "gpt-4o", "claude"}}, LimitModelSetting{Enabled: true, Models: []string{"gpt-4o", "claude"}}},
		{"blank-only list means no limit", LimitModelSetting{Enabled: true, Models: []string{" "}}, LimitModelSetting{Enabled: false, Models: []string{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeLimitModelSetting(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeLimitModelSetting(%+v) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

// 令牌写入(新建与更新)都经过归一:列表为空即不限制,关闭限制即清空列表。
func TestTokenSaveNormalizesModelLimit(t *testing.T) {
	setupOrgTestDB(t)
	user := createOrgTestUser(t, "norm-user")

	token := &Token{UserId: user.Id, Name: "norm", Status: config.TokenStatusEnabled}
	s := token.Setting.Data()
	s.Limits.LimitModelSetting = LimitModelSetting{Enabled: true}
	token.Setting.Set(s)
	if err := token.Insert(); err != nil {
		t.Fatal(err)
	}
	read := func() LimitModelSetting {
		var got Token
		if err := DB.First(&got, token.Id).Error; err != nil {
			t.Fatal(err)
		}
		return got.Setting.Data().Limits.LimitModelSetting
	}
	if got := read(); got.Enabled || len(got.Models) != 0 {
		t.Fatalf("insert with on + empty list should store no limit, got %+v", got)
	}

	s = token.Setting.Data()
	s.Limits.LimitModelSetting = LimitModelSetting{Enabled: false, Models: []string{"gpt-4o"}}
	token.Setting.Set(s)
	if err := token.Update(); err != nil {
		t.Fatal(err)
	}
	if got := read(); got.Enabled || len(got.Models) != 0 {
		t.Fatalf("update with the limit off should clear the list, got %+v", got)
	}

	s = token.Setting.Data()
	s.Limits.LimitModelSetting = LimitModelSetting{Enabled: true, Models: []string{"gpt-4o"}}
	token.Setting.Set(s)
	if err := token.Update(); err != nil {
		t.Fatal(err)
	}
	if got := read(); !got.Enabled || !reflect.DeepEqual(got.Models, []string{"gpt-4o"}) {
		t.Fatalf("update with a list should keep it, got %+v", got)
	}
}

// 迁移:关闭限制的令牌清空残留列表;「开启但列表为空」原本拒绝全部请求,改为停用令牌以免放宽权限;
// 正常的限制保持不变。
func TestMigrateTokenModelLimitNormalized(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Token{}); err != nil {
		t.Fatal(err)
	}
	insertRaw := func(name string, status int, settingJSON string) {
		if err := db.Exec("INSERT INTO tokens (user_id, name, `key`, status, setting) VALUES (1, ?, ?, ?, ?)", name, name+"-key-padding-padding-padding-padding-padding", status, settingJSON).Error; err != nil {
			t.Fatal(err)
		}
	}
	insertRaw("deny-all", config.TokenStatusEnabled, `{"limits":{"limit_model_setting":{"enabled":true,"models":[]}}}`)
	insertRaw("deny-all-disabled", config.TokenStatusDisabled, `{"limits":{"limit_model_setting":{"enabled":true,"models":null}}}`)
	insertRaw("off-stale", config.TokenStatusEnabled, `{"limits":{"limit_model_setting":{"enabled":false,"models":["gpt-4o"]}}}`)
	insertRaw("limited", config.TokenStatusEnabled, `{"limits":{"limit_model_setting":{"enabled":true,"models":["gpt-4o"]}}}`)
	insertRaw("plain", config.TokenStatusEnabled, `{}`)

	if err := migrateTokenModelLimitNormalized().Migrate(db); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	read := func(name string) (int, LimitModelSetting) {
		var tok Token
		if err := db.Where("name = ?", name).First(&tok).Error; err != nil {
			t.Fatal(err)
		}
		return tok.Status, tok.Setting.Data().Limits.LimitModelSetting
	}

	if status, limit := read("deny-all"); status != config.TokenStatusDisabled || limit.Enabled {
		t.Fatalf("deny-all token should be disabled with no limit stored, got status %d limit %+v", status, limit)
	}
	if status, _ := read("deny-all-disabled"); status != config.TokenStatusDisabled {
		t.Fatalf("already disabled token should stay disabled, got %d", status)
	}
	if status, limit := read("off-stale"); status != config.TokenStatusEnabled || limit.Enabled || len(limit.Models) != 0 {
		t.Fatalf("stale list on a disabled limit should be cleared, got status %d limit %+v", status, limit)
	}
	if status, limit := read("limited"); status != config.TokenStatusEnabled || !limit.Enabled || !reflect.DeepEqual(limit.Models, []string{"gpt-4o"}) {
		t.Fatalf("a real limit should be kept, got status %d limit %+v", status, limit)
	}
	if status, limit := read("plain"); status != config.TokenStatusEnabled || limit.Enabled {
		t.Fatalf("token without a limit should be untouched, got status %d limit %+v", status, limit)
	}
}

// Key 详情的「实际可用模型」:分组模型 ∩ 令牌限制 ∩ 组织成员生效白名单,与 /v1/models 同口径。
func TestTokenAvailableModels(t *testing.T) {
	setupOrgTestDB(t)
	prevRule := ChannelGroup.Rule
	ChannelGroup.Rule = map[string]map[string][][]int{
		"default": {"gpt-a": {{1}}, "gpt-b": {{1}}, "claude-x": {{1}}},
		"vip":     {"gpt-a": {{1}}, "o-vip": {{1}}},
	}
	t.Cleanup(func() { ChannelGroup.Rule = prevRule })

	owner := createOrgTestUser(t, "avail-owner")
	if err := DB.Model(&User{}).Where("id = ?", owner.Id).Update("group", "default").Error; err != nil {
		t.Fatal(err)
	}

	personal := &Token{UserId: owner.Id, CreatedBy: owner.Id}
	got, err := TokenAvailableModels(personal)
	if err != nil || !reflect.DeepEqual(got, []string{"claude-x", "gpt-a", "gpt-b"}) {
		t.Fatalf("personal token without limit: got %v err %v", got, err)
	}

	limited := &Token{UserId: owner.Id, CreatedBy: owner.Id, Group: "vip"}
	s := limited.Setting.Data()
	s.Limits.LimitModelSetting = LimitModelSetting{Enabled: true, Models: []string{"GPT-*"}}
	limited.Setting.Set(s)
	if got, _ := TokenAvailableModels(limited); !reflect.DeepEqual(got, []string{"gpt-a"}) {
		t.Fatalf("token group and limit should both apply, got %v", got)
	}

	org, err := CreateOrganizationWithOwner("Avail Org", "", owner.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := DB.Model(&User{}).Where("id = ?", org.ShadowUserId).Update("group", "default").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateOrgMemberLimits(org, owner.Id, nil, []string{"gpt-*"}); err != nil {
		t.Fatal(err)
	}
	orgToken := &Token{UserId: org.ShadowUserId, CreatedBy: owner.Id}
	if got, err := TokenAvailableModels(orgToken); err != nil || !reflect.DeepEqual(got, []string{"gpt-a", "gpt-b"}) {
		t.Fatalf("org token should apply the member allowlist, got %v err %v", got, err)
	}
}
