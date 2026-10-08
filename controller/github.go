package controller

import (
	"bytes"
	"context"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GitHubOAuthResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

type GitHubUser struct {
	Id        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarUrl string `json:"avatar_url"`
}

type GithubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func configureClientWithGitHubProxy(client *http.Client) error {
	if config.GitHubProxy == "" {
		return nil
	}
	proxyURL, parseErr := url.Parse(config.GitHubProxy)
	if parseErr != nil {
		logger.SysError("GitHub proxy URL parse error: " + parseErr.Error())
		return parseErr
	}

	switch proxyURL.Scheme {
	case "http", "https":
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	case "socks5":
		dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			logger.SysError("failed to create Github SOCKS5 dialer: " + err.Error())
			return err
		}
		client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.(proxy.ContextDialer).DialContext(ctx, network, addr)
			},
		}
	default:
		errMsg := "unsupported Github proxy scheme: " + proxyURL.Scheme
		logger.SysError(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

// getGitHubUserInfoByCode 取 GitHub 侧的账号与 primary+verified 邮箱。
// 声明成 var 只为给测试留一个替身入口：它内部要打三次真实 https 端点，
// 否则 GitHubOAuth / GitHubBind 两条写库路径在单测里完全够不着。生产代码不得改写它。
var getGitHubUserInfoByCode = func(code string) (*GitHubUser, error) {
	if code == "" {
		return nil, errors.New("Invalid parameters")
	}
	values := map[string]string{"client_id": config.GitHubClientId, "client_secret": config.GitHubClientSecret, "code": code}
	jsonData, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	// Configure client with GitHub proxy if needed
	if err := configureClientWithGitHubProxy(&client); err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		logger.SysError("failed to connect to GitHub server, err:" + err.Error())
		return nil, errors.New("Unable to connect to the GitHub server. Please try again later.")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("Unable to connect to the GitHub server. Please try again later.")
	}

	var oAuthResponse GitHubOAuthResponse
	err = json.NewDecoder(res.Body).Decode(&oAuthResponse)
	if err != nil {
		return nil, err
	}

	scopes := strings.Split(oAuthResponse.Scope, ",")
	hasUserEmailScope := false
	if utils.Contains("user:email", scopes) {
		hasUserEmailScope = true
	}

	req, err = http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oAuthResponse.AccessToken))
	res2, err := client.Do(req)
	if err != nil {
		logger.SysError("failed to connect to GitHub server, err:" + err.Error())
		return nil, errors.New("Unable to connect to the GitHub server. Please try again later.")
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		return nil, errors.New("Unable to connect to the GitHub server. Please try again later.")
	}

	var githubUser GitHubUser
	err = json.NewDecoder(res2.Body).Decode(&githubUser)
	if err != nil {
		return nil, err
	}
	if githubUser.Login == "" {
		return nil, errors.New("Invalid response: user field is empty. Please try again later.")
	}

	if hasUserEmailScope {
		req, err = http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oAuthResponse.AccessToken))
		res3, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res3.Body.Close()
		if res3.StatusCode != http.StatusOK {
			return nil, errors.New("Unable to connect to the GitHub server. Please try again later.")
		}

		var githubEmails []*GithubEmail
		err = json.NewDecoder(res3.Body).Decode(&githubEmails)
		if err != nil {
			return nil, err
		}

		githubUser.Email = getGithubEmail(githubEmails)
	}

	return &githubUser, nil
}

func getGithubEmail(githubEmails []*GithubEmail) string {
	for _, email := range githubEmails {
		if email.Primary && email.Verified {
			// 入口处即归一化，后续注册/绑定/查重共用同一形态
			return common.NormalizeEmail(email.Email)
		}
	}
	return ""
}

func getUserByGitHub(githubUser *GitHubUser) (user *model.User, err error) {
	// 优先检测 GitHubIdNew
	if model.IsGitHubIdNewAlreadyTaken(githubUser.Id) {
		user, err = model.FindUserByField("github_id_new", githubUser.Id)
		if err != nil {
			return nil, err
		}
	}

	// 如果 GitHubIdNew 不存在，并且没有关闭 GitHubOldId登录，则检测 GitHubId
	if user == nil && !config.GitHubOldIdCloseEnabled && model.IsGitHubIdAlreadyTaken(githubUser.Login) {
		user, err = model.FindUserByField("github_id", githubUser.Login)
		if err != nil {
			return nil, err
		}
	}

	// 如果 GitHubId 不存在，则检测 Email。
	//
	// 「已验证」是双向要求：GitHub 侧只取 primary + verified 邮箱（见 getGithubEmail），
	// 本站侧命中的账号也必须 email_verified = true——管理员代填或其它未经验证写入的邮箱
	// 不得成为登录入口，否则在某个账号上填一个别人的邮箱，就能凭该邮箱的 GitHub 账号登进
	// 这个账号。未验证按「未命中」处理，继续走后面的新建账号流程（与 OIDC 的
	// lookupOidcLinkTarget 一致）。
	if user == nil && model.IsEmailAlreadyTaken(githubUser.Email) {
		matched, err := model.FindUserByField("email", githubUser.Email)
		if err != nil {
			return nil, err
		}
		if matched != nil && !matched.EmailVerified {
			logger.SysLog(fmt.Sprintf("GitHub email linking skipped: user_id=%d, account email is not verified", matched.Id))
			matched = nil
		}
		user = matched
	}

	return user, nil
}

func GitHubOAuth(c *gin.Context) {
	session := sessions.Default(c)
	state := c.Query("state")
	if state == "" || session.Get("oauth_state") == nil || state != session.Get("oauth_state").(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "state is empty or not same",
		})
		return
	}
	// state 一次性使用：校验通过后立即失效，防止重放
	session.Delete("oauth_state")
	_ = session.Save()

	username := session.Get("username")
	if username != nil {
		GitHubBind(c)
		return
	}

	if !config.EffectiveSocialLogin(config.GitHubOAuthEnabled) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "The admin has not enabled sign-in and sign-up via GitHub",
		})
		return
	}
	code := c.Query("code")
	affCode := c.Query("aff")

	githubUser, err := getGitHubUserInfoByCode(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	user, err := getUserByGitHub(githubUser)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 如果用户不存在，则创建用户
	if user == nil {
		if !config.RegisterEnabled {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "The admin has disabled new user sign-up",
			})
			return
		}

		// 验证 GitHub 提供的邮箱格式
		if githubUser.Email != "" {
			if err := common.ValidateEmailStrict(githubUser.Email); err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "Invalid email format",
				})
				return
			}
		}

		user = &model.User{
			GitHubId:    githubUser.Login,
			GitHubIdNew: githubUser.Id,
			Role:        config.RoleCommonUser,
			Status:      config.UserStatusEnabled,
			AvatarUrl:   githubUser.AvatarUrl,
		}
		// 回填的是 GitHub 侧的 primary + verified 邮箱，故与地址一并记为已验证。
		// 邮箱已被其它账号占用时（含上面因对方未验证而放过的账号）不带邮箱建号，
		// 否则 Insert 会因唯一索引直接失败，这个 GitHub 账号永远注册不进来。
		if githubUser.Email != "" && !model.IsEmailAlreadyTaken(githubUser.Email) {
			user.Email = model.NullableEmail(githubUser.Email)
			user.EmailVerified = true
		}

		// 检测推荐码
		var inviterId int
		if affCode != "" {
			inviterId, _ = model.GetUserIdByAffCode(affCode)
		}

		user.Username = oauthUsername(githubUser.Login, "github")

		if githubUser.Name != "" {
			user.DisplayName = githubUser.Name
		} else {
			user.DisplayName = user.Username
		}

		// 使用事务创建用户并处理邀请码
		err = model.DB.Transaction(func(tx *gorm.DB) error {
			// 验证和使用邀请码（如果启用）
			usedInviteCode, err := validateAndUseInviteCodeForOAuth(c, tx)
			if err != nil {
				return err
			}

			// 设置邀请人ID（使用原有推荐码逻辑）
			if inviterId > 0 {
				user.InviterId = inviterId
			}

			// 设置使用的邀请码
			if usedInviteCode != "" {
				user.UsedInviteCode = usedInviteCode
			}

			// 在事务中创建用户
			return user.InsertWithTx(tx, user.InviterId)
		})

		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

	} else {
		// 如果用户存在，则更新用户
		user.GitHubId = githubUser.Login
		user.GitHubIdNew = githubUser.Id

		// 如果用户的邮箱为空，且 GitHub 用户的邮箱不为空，且 GitHub 用户的邮箱未被注册，则更新用户的邮箱。
		// 写进来的是 GitHub 侧的 primary + verified 邮箱，故与地址一并记为已验证。
		if user.Email == "" && githubUser.Email != "" && !model.IsEmailAlreadyTaken(githubUser.Email) {
			user.Email = model.NullableEmail(githubUser.Email)
			user.EmailVerified = true
		}

		// 如果用户的头像为空，则更新用户的头像
		if user.AvatarUrl == "" {
			user.AvatarUrl = githubUser.AvatarUrl
		}
	}

	if user.Status != config.UserStatusEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "User has been banned",
			"success": false,
		})
		return
	}

	setupLoginSession(user, c, loginSessionInfo{method: model.LoginMethodGitHub})
}

func GitHubBind(c *gin.Context) {
	if !config.EffectiveSocialLogin(config.GitHubOAuthEnabled) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "The admin has not enabled sign-in and sign-up via GitHub",
		})
		return
	}
	code := c.Query("code")
	githubUser, err := getGitHubUserInfoByCode(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	user := model.User{
		GitHubId: githubUser.Login,
	}
	if model.IsGitHubIdAlreadyTaken(user.GitHubId) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "This GitHub account is already linked",
		})
		return
	}
	session := sessions.Default(c)
	id := session.Get("id")
	// id := c.GetInt("id")  // critical bug!
	user.Id = id.(int)
	err = user.FillUserById()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	user.GitHubId = githubUser.Login
	user.GitHubIdNew = githubUser.Id

	if user.AvatarUrl == "" {
		user.AvatarUrl = githubUser.AvatarUrl
	}

	// 同上：GitHub 只下发 primary + verified 邮箱，回填时一并记为已验证。
	if user.Email == "" && githubUser.Email != "" && !model.IsEmailAlreadyTaken(githubUser.Email) {
		user.Email = model.NullableEmail(githubUser.Email)
		user.EmailVerified = true
	}

	err = user.Update(false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "bind",
	})
}

func GenerateOAuthCode(c *gin.Context) {
	session := sessions.Default(c)
	state := utils.GetSecureRandomString(32)
	session.Set("oauth_state", state)
	err := session.Save()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    state,
	})
}

// SetOAuthInviteCode 为第三方登录设置邀请码
func SetOAuthInviteCode(c *gin.Context) {
	var req struct {
		InviteCode string `json:"invite_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameter: invite code must not be empty",
		})
		return
	}

	// 清理输入数据
	req.InviteCode = strings.TrimSpace(req.InviteCode)
	if req.InviteCode == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invite code must not be empty",
		})
		return
	}

	// 如果启用了邀请码注册，验证邀请码
	if config.InviteCodeRegisterEnabled {
		// 验证邀请码有效性（不消费）
		if err := model.CheckInviteCode(req.InviteCode); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	} else {
		// 未启用邀请码注册时不应该调用此接口
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invite code sign-up is not enabled",
		})
		return
	}

	// 将邀请码存储到会话中
	session := sessions.Default(c)
	session.Set("oauth_invite_code", req.InviteCode)

	if err := session.Save(); err != nil {
		logger.SysError("Failed to save OAuth invite code to session: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "System error, please try again",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Invite code set successfully",
	})
}
