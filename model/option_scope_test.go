package model

import (
	"sort"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// 改造前 GetStatus 中由 option 注册表下发的 key + 非 option 手工 key 的完整快照(42 个)。
// 该快照来自改造前 controller/misc.go GetStatus 的逐字段枚举,用于保证改造前后 key 集合 diff 为零。
var statusKeySnapshot = []string{
	"version", "start_time", "email_verification", "github_oauth", "github_client_id",
	"linuxDo_oauth", "linuxDo_client_id", "oidc_auth", "lark_login", "lark_client_id",
	"system_name", "logo", "language", "footer_html", "analytics_code",
	"wechat_qrcode", "invite_code_register", "wechat_login", "server_address", "turnstile_check",
	"turnstile_site_key", "top_up_link", "chat_link", "quota_per_unit", "display_in_currency",
	"telegram_bot", "mj_notify_enabled", "builtin_chat_enabled", "chat_links", "PaymentUSDRate",
	"PaymentMinAmount", "RechargeDiscount", "EnableSafe", "SafeToolName", "SafeKeyWords",
	"UserInvoiceMonth", "UptimeDomain", "UptimePageName", "UptimeEnabled", "GeminiAPIEnabled",
	"ClaudeAPIEnabled", "max_log_query_days",
}

// GetStatus 中仍手工下发的非 option key(版本/启动信息、viper/env 配置、运行态)。
// oidc_* 三个 key 自多提供方改造后改由 oidc_providers 表算出,不再来自 option 注册表。
var statusManualKeys = []string{
	"email_code_login",
	"passkey_login",
	"version", "start_time", "lark_client_id", "language", "telegram_bot",
	"UserInvoiceMonth", "UptimeDomain", "UptimePageName", "UptimeEnabled", "max_log_query_days",
	"max_log_lookback_days", "oidc_auth", "oidc_display_name", "oidc_providers",
	"log_io_max_body_kb",
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// 有意变更(S1, 2026-06-11):TurnstileSecretKey / CFWorkerImageKey 由 ScopeAdmin 收紧为 ScopeSecret,
// 不再通过 /api/option/ 下发。旧后缀规则(*Token/*Secret)无法覆盖这两个 key,在此显式登记为期望缺席。
var intentionallySecretKeys = map[string]struct{}{
	"TurnstileSecretKey": {},
	"CFWorkerImageKey":   {},
}

// 有意变更(L2, 2026-06-11):新增 OIDCDisplayName(ScopePublic, StatusKey "oidc_display_name"),
// 登录页未认证状态需要读取 OIDC 自定义显示名称。基线由 42→43(/api/status)、84→85(/api/option/)。
// /api/option/ 侧该 key 不带 Token/Secret 后缀,oldRule 与 newSet 同步包含,无需额外登记。
// 有意变更(2026-06-11):新增 OrganizationEnabled(ScopePublic, StatusKey "organization_enabled"),
// 前端未登录/登录态均需感知组织功能全局开关。基线 43→44(/api/status)。
// 有意变更:LogIOEnabled 由 ScopeAdmin 放开为 ScopePublic(StatusKey "log_io_enabled"),
// 前端未认证空态需区分"站点未开启"与"站点已开启但本账户/令牌未开启"。基线 44→45(/api/status)。
// 有意变更(LOGX-2):日志查询时间限制拆分为 span(max_log_query_days)与 lookback(max_log_lookback_days),
// 新增 max_log_lookback_days,前端按 lookback 计算最早可选日期。基线 45→46(/api/status)。
// 有意变更(TK8a):新增 quota_reset_timezone / quota_reset_week_start(均 ScopePublic),
// 前端设置页需读取当前站点周期重置时区与周起始日。基线 46→48(/api/status)。
// 有意变更:新增 docs_link(ScopePublic),页脚与 /llms.txt 需要文档站外链。基线 48→49(/api/status)。
// 有意变更(OIDC 多提供方):新增 oidc_providers(已启用提供方的 slug + display_name 列表);
// oidc_auth / oidc_display_name 同时从 option 注册表迁到 GetStatus 手工下发,由该列表算出,
// OIDCAuthEnabled / OIDCDisplayName 两个旧选项降为 ScopeAdmin 只读兼容。基线 49→50(/api/status)。
// 有意变更(UX-33):PasswordLoginEnabled / PasswordRegisterEnabled 由 ScopeAdmin 放开为 ScopePublic
// (StatusKey "password_login" / "password_register"),登录页据此在提交前隐藏密码表单与注册入口。基线 50→52(/api/status)。
// 有意变更(UX-34):新增 OidcAutoRedirect(StatusKey "oidc_auto_redirect"),登录页据此直接跳去唯一的 OIDC 提供方。基线 52→53。
// 有意变更(LogIO 留存开关重设计):LogIODefaultUser / OrganizationLogIODefault 由 ScopeAdmin 放开为
// ScopePublic(StatusKey "log_io_default_user" / "organization_log_io_default"),并手工新增
// log_io_max_body_kb(配置文件项,非 option),个人/组织设置页据此显示生效值与真实截断上限。基线 53→56。
var intentionallyAddedStatusKeys = []string{
	"oidc_providers",
	"oidc_display_name",
	"organization_enabled",
	"log_io_enabled",
	"max_log_lookback_days",
	"quota_reset_timezone",
	"quota_reset_week_start",
	"docs_link",
	"password_login",
	"password_register",
	"oidc_auto_redirect",
	// 有意变更(AUTH-1 账号体系):新增 account_system / admin_login_enabled(ScopePublic),
	// 以及 GetStatus 手工下发的有效开关 email_code_login / passkey_login。
	"account_system",
	"admin_login_enabled",
	"email_code_login",
	"passkey_login",
	"log_io_default_user",
	"organization_log_io_default",
	"log_io_max_body_kb",
	// 有意变更：新增 InviteRewardEnabled(ScopePublic, StatusKey "invite_reward_enabled")，
	// 前端据此决定是否展示邀请奖励入口。
	"invite_reward_enabled",
}

// 改造前 /api/option/ 的过滤规则:排除 *Token / *Secret 后缀。
// 验证 GetAllNonSecret(scope!=secret)与旧规则的 key 集合 diff 为零(intentionallySecretKeys 除外)。
func TestGetOptionsKeySetUnchanged(t *testing.T) {
	registerOptions()

	oldRule := make(map[string]struct{})
	for k := range config.GlobalOption.GetAll() {
		if strings.HasSuffix(k, "Token") || strings.HasSuffix(k, "Secret") {
			continue
		}
		if _, ok := intentionallySecretKeys[k]; ok {
			continue
		}
		oldRule[k] = struct{}{}
	}

	newSet := config.GlobalOption.GetAllNonSecret()

	for k := range intentionallySecretKeys {
		if _, ok := newSet[k]; ok {
			t.Errorf("/api/option/ must not include intentionally-secret key: %q", k)
		}
	}

	for k := range oldRule {
		if _, ok := newSet[k]; !ok {
			t.Errorf("/api/option/ key missing after refactor: %q", k)
		}
	}
	for k := range newSet {
		if _, ok := oldRule[k]; !ok {
			t.Errorf("/api/option/ unexpected new key after refactor: %q", k)
		}
	}
	if len(oldRule) != len(newSet) {
		t.Errorf("/api/option/ key count changed: old=%d new=%d", len(oldRule), len(newSet))
	}
}

// 验证 /api/status 的 key 集合(PublicOptions + 手工 key)与改造前 42 key 快照 diff 为零。
func TestGetStatusKeySetUnchanged(t *testing.T) {
	registerOptions()

	got := make(map[string]struct{})
	for _, k := range statusManualKeys {
		got[k] = struct{}{}
	}
	public := config.GlobalOption.PublicOptions()
	for k := range public {
		if _, dup := got[k]; dup {
			t.Errorf("/api/status duplicate key between manual and public: %q", k)
		}
		got[k] = struct{}{}
	}

	want := make(map[string]struct{}, len(statusKeySnapshot)+len(intentionallyAddedStatusKeys))
	for _, k := range statusKeySnapshot {
		want[k] = struct{}{}
	}
	for _, k := range intentionallyAddedStatusKeys {
		want[k] = struct{}{}
	}

	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("/api/status key missing after refactor: %q", k)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("/api/status unexpected new key after refactor: %q", k)
		}
	}
	if len(got) != len(want) {
		t.Errorf("/api/status key count changed: want=%d got=%d\nwant=%v\ngot=%v",
			len(want), len(got), statusKeySnapshot, sortedKeys(got))
	}
}

// 登录页需要在提交前就知道密码登录 / 密码注册是否开放(UX-33):两项以 password_login /
// password_register 公开下发,取值随配置变化,且不再以注册 key 原名出现。
func TestPasswordLoginFlagsPublic(t *testing.T) {
	registerOptions()
	origLogin, origRegister := config.PasswordLoginEnabled, config.PasswordRegisterEnabled
	t.Cleanup(func() {
		config.PasswordLoginEnabled, config.PasswordRegisterEnabled = origLogin, origRegister
	})

	for _, tc := range []struct{ login, register bool }{
		{true, true}, {false, true}, {true, false}, {false, false},
	} {
		config.PasswordLoginEnabled, config.PasswordRegisterEnabled = tc.login, tc.register
		public := config.GlobalOption.PublicOptions()
		if v, ok := public["password_login"]; !ok || v != tc.login {
			t.Errorf("password_login: want %v, got %#v (present=%v)", tc.login, v, ok)
		}
		if v, ok := public["password_register"]; !ok || v != tc.register {
			t.Errorf("password_register: want %v, got %#v (present=%v)", tc.register, v, ok)
		}
		for _, raw := range []string{"PasswordLoginEnabled", "PasswordRegisterEnabled"} {
			if _, ok := public[raw]; ok {
				t.Errorf("/api/status must not expose %q under its registration key", raw)
			}
		}
	}
}

// 登录直达 OIDC(UX-34):oidc_auto_redirect 公开下发、默认 false、随配置变化,且不以注册 key 原名出现。
func TestOidcAutoRedirectPublic(t *testing.T) {
	registerOptions()
	orig := config.OidcAutoRedirect
	t.Cleanup(func() { config.OidcAutoRedirect = orig })

	for _, want := range []bool{false, true} {
		config.OidcAutoRedirect = want
		public := config.GlobalOption.PublicOptions()
		if v, ok := public["oidc_auto_redirect"]; !ok || v != want {
			t.Errorf("oidc_auto_redirect: want %v, got %#v (present=%v)", want, v, ok)
		}
		if _, ok := public["OidcAutoRedirect"]; ok {
			t.Errorf("/api/status must not expose OidcAutoRedirect under its registration key")
		}
	}
}

// 留存开关要显示生效值:两个站点默认值以 log_io_default_user / organization_log_io_default
// 公开下发,随配置变化,且不以注册 key 原名出现。
func TestLogIOSiteDefaultsPublic(t *testing.T) {
	registerOptions()
	origUser, origOrg := config.LogIODefaultUser, config.OrganizationLogIODefault
	t.Cleanup(func() {
		config.LogIODefaultUser, config.OrganizationLogIODefault = origUser, origOrg
	})

	for _, tc := range []struct{ user, org bool }{
		{false, false}, {true, false}, {false, true}, {true, true},
	} {
		config.LogIODefaultUser, config.OrganizationLogIODefault = tc.user, tc.org
		public := config.GlobalOption.PublicOptions()
		if v, ok := public["log_io_default_user"]; !ok || v != tc.user {
			t.Errorf("log_io_default_user: want %v, got %#v (present=%v)", tc.user, v, ok)
		}
		if v, ok := public["organization_log_io_default"]; !ok || v != tc.org {
			t.Errorf("organization_log_io_default: want %v, got %#v (present=%v)", tc.org, v, ok)
		}
		for _, raw := range []string{"LogIODefaultUser", "OrganizationLogIODefault"} {
			if _, ok := public[raw]; ok {
				t.Errorf("/api/status must not expose %q under its registration key", raw)
			}
		}
	}
}
