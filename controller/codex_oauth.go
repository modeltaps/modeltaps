package controller

import (
	"crypto/rand"
	"crypto/sha256"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/providers/codex"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// Codex OAuth 状态缓存前缀
	CodexOAuthStateCachePrefix = "codex_oauth_state:"
	// Codex OAuth 状态缓存时长（10分钟）
	CodexOAuthStateCacheDuration = 10 * time.Minute
)

// Codex OAuth 配置常量
const (
	CodexAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	CodexTokenURL     = "https://auth.openai.com/oauth/token"
	CodexClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	CodexRedirectURI  = "http://localhost:1455/auth/callback"
	CodexScopes       = "openid profile email offline_access"
)

// codexAuthBaseURL is the OpenAI auth issuer; tests point it at a fake server.
var codexAuthBaseURL = "https://auth.openai.com"

// CodexOAuthStateData OAuth 状态数据
type CodexOAuthStateData struct {
	ChannelID    int    `json:"channel_id"`
	UserID       int    `json:"user_id"` // admin who started the login; only they may finish it
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state"`
	Proxy        string `json:"proxy"` // 代理配置（JSON 字符串）
	CreatedAt    int64  `json:"created_at"`
}

// StartCodexOAuthRequest 开始 OAuth 认证请求
type StartCodexOAuthRequest struct {
	ChannelID int    `json:"channel_id"` // 可选，新建时为 0
	Proxy     string `json:"proxy"`      // 可选，代理配置（JSON 字符串）
}

// generateCodexCodeVerifier 生成随机的 code verifier（PKCE）
func generateCodexCodeVerifier() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// generateCodexCodeChallenge 生成 code challenge（PKCE）
func generateCodexCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
}

// StartCodexOAuth 开始 Codex OAuth 认证流程
// POST /api/codex/oauth/start
func StartCodexOAuth(c *gin.Context) {
	var req StartCodexOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// 生成随机 state
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("failed to generate state: %w", err))
		return
	}
	state := base64.URLEncoding.EncodeToString(stateBytes)

	// 生成 PKCE code verifier
	codeVerifier, err := generateCodexCodeVerifier()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("failed to generate code verifier: %w", err))
		return
	}

	// 生成 code challenge
	codeChallenge := generateCodexCodeChallenge(codeVerifier)

	// 保存 state 到缓存（包含代理配置）
	stateData := CodexOAuthStateData{
		ChannelID:    req.ChannelID,
		UserID:       c.GetInt("id"),
		CodeVerifier: codeVerifier,
		State:        state,
		Proxy:        req.Proxy, // 保存代理配置，用于后续 token 交换
		CreatedAt:    time.Now().Unix(),
	}
	cacheKey := CodexOAuthStateCachePrefix + state
	cache.SetCache(cacheKey, stateData, CodexOAuthStateCacheDuration)

	// 构建 OAuth 授权 URL
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", CodexClientID)
	params.Set("redirect_uri", CodexRedirectURI)
	params.Set("scope", CodexScopes)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	params.Set("id_token_add_organizations", "true")
	params.Set("codex_cli_simplified_flow", "true")

	authURL := fmt.Sprintf("%s?%s", CodexAuthorizeURL, params.Encode())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"auth_url":   authURL,
			"state":      state,
			"session_id": state, // 使用 state 作为 session_id
			"expires_in": int(CodexOAuthStateCacheDuration.Seconds()),
			"instructions": []string{
				"1. Open the authorization link and sign in to your OpenAI account in the new window",
				"2. Grant the requested permissions",
				"3. The browser then opens localhost:1455 and shows an error page; this is expected",
				"4. Copy the full address from the browser address bar and paste it into the field below",
			},
		},
	})
}

// ExchangeCodexCodeRequest 交换授权码请求
type ExchangeCodexCodeRequest struct {
	SessionID         string `json:"session_id"`         // session_id (即 state)
	AuthorizationCode string `json:"authorization_code"` // 授权码或完整的回调 URL
	CallbackURL       string `json:"callback_url"`       // 完整的回调 URL（可选）
}

// CodexOAuthCallback 处理用户提交的授权码
// POST /api/codex/oauth/exchange-code
//
// The session is kept when the pasted text is wrong or the code is rejected, so the admin can
// paste again or re-open the same authorization link; it is deleted once tokens are issued.
func CodexOAuthCallback(c *gin.Context) {
	var req ExchangeCodexCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if req.SessionID == "" || (req.AuthorizationCode == "" && req.CallbackURL == "") {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("session_id and authorization_code (or callback_url) are required"))
		return
	}

	cacheKey := CodexOAuthStateCachePrefix + req.SessionID
	stateData, err := cache.GetCache[CodexOAuthStateData](cacheKey)
	if err != nil || stateData.UserID != c.GetInt("id") {
		respondOAuthError(c, common.NewAPIError(common.APICodeChannelOAuthSessionExpired, "This login session has expired. Start the authorization again.", nil))
		return
	}

	inputValue := req.CallbackURL
	if inputValue == "" {
		inputValue = req.AuthorizationCode
	}
	code, pasteErr := checkOAuthPaste(inputValue, stateData.State)
	if pasteErr != nil {
		respondOAuthError(c, pasteErr)
		return
	}

	tokenResp, err := exchangeCodexCodeForToken(code, stateData.CodeVerifier, CodexRedirectURI, stateData.Proxy)
	if err != nil {
		logger.SysError(fmt.Sprintf("Failed to exchange code for token: %s", err.Error()))
		if errors.Is(err, errOAuthCodeRejected) {
			respondOAuthError(c, common.NewAPIError(common.APICodeChannelOAuthCodeRejected, "This authorization code was already used or has expired. Open the authorization page again and paste the new address.", nil))
			return
		}
		respondOAuthError(c, oauthAPIError(common.APICodeChannelOAuthExchangeFailed, "Failed to exchange the authorization code for credentials", err.Error()))
		return
	}
	cache.DeleteCache(cacheKey)

	respondCodexCredentials(c, tokenResp)
}

// respondCodexCredentials answers with the channel key JSON built from a token response. The
// account ID comes from the id_token, falling back to the access token.
func respondCodexCredentials(c *gin.Context, tokenResp *codex.TokenRefreshResponse) {
	accountID := ""
	if tokenResp.IDToken != "" {
		accountID = extractAccountIDFromToken(tokenResp.IDToken)
	}
	if accountID == "" && tokenResp.AccessToken != "" {
		accountID = extractAccountIDFromToken(tokenResp.AccessToken)
	}

	credentials := &codex.OAuth2Credentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     CodexClientID,
		AccountID:    accountID,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}
	credentialsJSON, err := credentials.ToJSON()
	if err != nil {
		logger.SysError(fmt.Sprintf("Failed to serialize credentials: %s", err.Error()))
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("failed to serialize credentials: %w", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Authorization succeeded",
		"data": gin.H{
			"status":      "completed",
			"credentials": credentialsJSON,
		},
	})
}

// extractAccountIDFromToken 从 JWT access_token 中提取 account_id
func extractAccountIDFromToken(accessToken string) string {
	// 解析 JWT（不验证签名，只提取 payload）
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, _, err := parser.ParseUnverified(accessToken, jwt.MapClaims{})
	if err != nil {
		logger.SysError(fmt.Sprintf("Failed to parse JWT: %s", err.Error()))
		return ""
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}

	// 提取 https://api.openai.com/auth.chatgpt_account_id
	authClaims, ok := claims["https://api.openai.com/auth"].(map[string]interface{})
	if !ok {
		return ""
	}

	accountID, ok := authClaims["chatgpt_account_id"].(string)
	if !ok {
		return ""
	}

	return accountID
}

// codexOAuthHTTPClient returns the client for the auth issuer, through the channel proxy if set.
func codexOAuthHTTPClient(proxyURL string) (*http.Client, error) {
	client, err := utils.NewProxyHTTPClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy: %w", err)
	}
	client.Timeout = 30 * time.Second
	return client, nil
}

// exchangeCodexCodeForToken 使用授权码交换访问令牌（支持代理）
// The device flow passes its own redirect URI and the verifier issued by the device endpoint.
func exchangeCodexCodeForToken(code, codeVerifier, redirectURI, proxyURL string) (*codex.TokenRefreshResponse, error) {
	requestBody := url.Values{}
	requestBody.Set("grant_type", "authorization_code")
	requestBody.Set("client_id", CodexClientID)
	requestBody.Set("code", code)
	requestBody.Set("redirect_uri", redirectURI)
	requestBody.Set("code_verifier", codeVerifier)

	req, err := http.NewRequest("POST", codexAuthBaseURL+"/oauth/token", strings.NewReader(requestBody.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", codex.DefaultCodexUserAgent)
	req.Header.Set("Accept", "application/json")

	client, err := codexOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if isOAuthCodeRejected(resp.StatusCode, bodyBytes) {
			return nil, fmt.Errorf("%w: status %d: %s", errOAuthCodeRejected, resp.StatusCode, string(bodyBytes))
		}
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp codex.TokenRefreshResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("token response is missing access_token or refresh_token")
	}

	return &tokenResp, nil
}
