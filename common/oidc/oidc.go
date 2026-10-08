package oidc

import (
	"context"
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig 单个提供方一次请求所需的 OAuth2 / OIDC 上下文。
type OIDCConfig struct {
	Provider     *oidc.Provider
	OAuth2Config *oauth2.Config
	Verifier     *oidc.IDTokenVerifier
	LoginURL     func(state string, opts ...oauth2.AuthCodeOption) string
}

// discovery 只缓存「按 issuer 发现出来的 *oidc.Provider」——那是唯一的网络开销。
// client_id / client_secret / scopes 等每次都从传入的提供方行现取，因此后台改配置
// 立即生效，不需要任何失效机制。发现失败只影响该 issuer，其它提供方照常可用。
var (
	discoveryMutex sync.Mutex
	discoveryCache = make(map[string]*oidc.Provider)
)

// RedirectURL 提供方的回调地址。存量 slug=oidc 继续使用无 slug 的旧地址，
// 避免升级后还要去 IdP 侧改重定向白名单。
func RedirectURL(slug string) string {
	if slug == model.LegacyOidcProviderSlug {
		return config.ServerAddress + "/oauth/oidc"
	}
	return config.ServerAddress + "/oauth/oidc/" + slug
}

// splitScopes scopes 允许用空格 / 逗号 / 制表符分隔（后台校验同口径）。
func splitScopes(scopes string) []string {
	return strings.FieldsFunc(scopes, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t' || r == '\n'
	})
}

// discover 取出（必要时发现并缓存）issuer 对应的 *oidc.Provider。
// 发现期间不持锁：同一 issuer 并发首次访问最多重复发现一次，好过阻塞其它提供方。
func discover(ctx context.Context, issuer string) (*oidc.Provider, error) {
	discoveryMutex.Lock()
	cached, ok := discoveryCache[issuer]
	discoveryMutex.Unlock()
	if ok {
		return cached, nil
	}

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		logger.SysError("OIDC identity provider discovery failed, issuer: " + issuer + ", err: " + err.Error())
		return nil, err
	}

	discoveryMutex.Lock()
	discoveryCache[issuer] = provider
	discoveryMutex.Unlock()
	return provider, nil
}

// EndSessionEndpoint 取提供方 discovery 文档里的 end_session_endpoint（RP-Initiated Logout 1.0）。
// 该字段是可选的，IdP 未声明时返回空串（调用方据此退化为纯本地退出）。
// 走的是同一份 discoveryCache，不产生额外网络请求。
func EndSessionEndpoint(ctx context.Context, provider *model.OidcProvider) (string, error) {
	discovered, err := discover(ctx, provider.Issuer)
	if err != nil {
		return "", err
	}
	var claims struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := discovered.Claims(&claims); err != nil {
		return "", err
	}
	return strings.TrimSpace(claims.EndSessionEndpoint), nil
}

// Get 按提供方行构造本次请求用的 OIDC 配置。provider 必须是调用方刚从库里读出的行。
func Get(ctx context.Context, provider *model.OidcProvider) (*OIDCConfig, error) {
	discovered, err := discover(ctx, provider.Issuer)
	if err != nil {
		return nil, err
	}

	oauth2Config := &oauth2.Config{
		ClientID:     provider.ClientId,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  RedirectURL(provider.Slug),
		Endpoint:     discovered.Endpoint(),
		Scopes:       splitScopes(provider.Scopes),
	}

	return &OIDCConfig{
		Provider:     discovered,
		OAuth2Config: oauth2Config,
		Verifier:     discovered.Verifier(&oidc.Config{ClientID: oauth2Config.ClientID}),
		// opts 供调用方追加授权参数（PKCE、nonce、ui_locales）。不再申请 access_type=offline：
		// 本站从不使用 refresh token，白白向部分 IdP 索要长期授权。
		LoginURL: func(state string, opts ...oauth2.AuthCodeOption) string {
			return oauth2Config.AuthCodeURL(state, opts...)
		},
	}, nil
}
