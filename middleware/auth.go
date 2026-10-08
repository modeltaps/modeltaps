package middleware

import (
	"fmt"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func authHelper(c *gin.Context, minRole int) {
	session := sessions.Default(c)
	username := session.Get("username")
	role := session.Get("role")
	id := session.Get("id")
	status := session.Get("status")
	useAccessToken := false
	if username == nil {
		// Check access token
		accessToken := c.Request.Header.Get("Authorization")
		if accessToken == "" {
			token := c.Param("accessToken")
			if token == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"message": "Permission denied: not signed in and no access token provided",
				})
				c.Abort()
				return
			}
			accessToken = fmt.Sprintf("Bearer %s", token)
		}
		user := model.ValidateAccessToken(accessToken)
		if user != nil && user.Username != "" {
			// Token is valid
			username = user.Username
			role = user.Role
			id = user.Id
			status = user.Status
			useAccessToken = true
		} else {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Permission denied: invalid access token",
			})
			c.Abort()
			return
		}
	}
	// Cookie session 路径：不信任 cookie 中签发的 role/status，统一以服务端（缓存/DB）为准，
	// 保证降级管理员、封禁、删除用户等变更下一次请求即生效。
	// access token 路径（ValidateAccessToken）已是实时查库，无需重复校验。
	if !useAccessToken {
		idInt, ok := id.(int)
		if !ok {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Permission denied: invalid session",
			})
			c.Abort()
			return
		}
		// cookie 只是会话行的索引：行被删（改密踢出、登出其它设备、封禁）或按空闲 / 绝对期限过期即失效
		sid, _ := session.Get("sid").(string)
		if _, err := model.ValidateUserSession(sid, idInt); err != nil {
			// 401 + 清掉 cookie 会话：前端只在 401 时清本地登录态并回登录页，
			// 否则被踢的设备会一直带着死 cookie 弹错误。
			session.Clear()
			if saveErr := session.Save(); saveErr != nil {
				logger.SysError("Failed to clear expired session: " + saveErr.Error())
			}
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": err.Error(),
			})
			c.Abort()
			return
		}
		currentRole, currentStatus, err := model.CacheGetUserRoleStatus(idInt)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Permission denied: user verification failed",
			})
			c.Abort()
			return
		}
		role = currentRole
		status = currentStatus
	}
	if status.(int) == config.UserStatusDisabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "User has been banned",
		})
		c.Abort()
		return
	}
	if role.(int) < minRole {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Permission denied",
		})
		c.Abort()
		return
	}
	c.Set("username", username)
	c.Set("role", role)
	c.Set("id", id)
	c.Next()
}

func TrySetUserBySession() func(c *gin.Context) {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		id := session.Get("id")
		if id == nil {
			c.Next()
			return
		}

		idInt, ok := id.(int)
		if !ok {
			c.Next()
			return
		}
		// 会话行已失效的 cookie 按匿名处理，并顺手清掉死 cookie
		sid, _ := session.Get("sid").(string)
		if _, err := model.ValidateUserSession(sid, idInt); err != nil {
			session.Clear()
			if saveErr := session.Save(); saveErr != nil {
				logger.SysError("Failed to clear expired session: " + saveErr.Error())
			}
			c.Next()
			return
		}

		c.Set("id", idInt)
		userGroup, err := model.CacheGetUserGroup(idInt)
		if err == nil {
			c.Set("group", userGroup)
		}
		c.Next()
	}
}

func UserAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, config.RoleCommonUser)
	}
}

func AdminAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, config.RoleAdminUser)
	}
}

func RootAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, config.RoleRootUser)
	}
}

func tokenAuth(c *gin.Context, key string) {
	key = strings.TrimPrefix(key, "Bearer ")
	key = strings.TrimPrefix(key, "sk-")

	if len(key) < 48 {
		abortWithMessage(c, http.StatusUnauthorized, "Invalid API key")
		return
	}

	parts := strings.Split(key, "#")
	key = parts[0]
	token, err := model.ValidateUserToken(key)
	if err != nil {
		abortWithMessage(c, http.StatusUnauthorized, err.Error())
		return
	}

	c.Set("id", token.UserId)
	c.Set("token_id", token.Id)
	c.Set("token_name", token.Name)
	c.Set("token_group", token.Group)
	c.Set("token_backup_group", token.BackupGroup)
	c.Set("token_unlimited_quota", token.UnlimitedQuota)
	c.Set("token_created_by", token.CreatedBy)
	c.Set("token_log_io", model.ResolveTokenLogIO(token)) // 分层继承解析后的生效布尔(站点闸门关时恒 false 且零开销)
	c.Set("token_setting", utils.GetPointer(token.Setting.Data()))
	if err := checkLimitIP(c); err != nil {
		abortWithMessage(c, http.StatusForbidden, err.Error())
		return
	}
	if len(parts) > 1 {
		if model.IsAdmin(token.UserId) {
			if strings.HasPrefix(parts[1], "!") {
				channelId := utils.String2Int(parts[1][1:])
				c.Set("skip_channel_ids", []int{channelId})
			} else {
				channelId := utils.String2Int(parts[1])
				if channelId == 0 {
					abortWithMessage(c, http.StatusForbidden, "Invalid channel ID")
					return
				}
				c.Set("specific_channel_id", channelId)
				if len(parts) == 3 && parts[2] == "ignore" {
					c.Set("specific_channel_id_ignore", true)
				}
			}
		} else {
			abortWithMessage(c, http.StatusForbidden, "Regular users cannot specify a channel")
			return
		}
	}
	c.Next()
}

// 检测是否IP白名单
func checkLimitIP(c *gin.Context) (error error) {
	// 从context中获取token设置
	tokenSetting, exists := c.Get("token_setting")
	if !exists {
		// 如果没有token设置，则不进行限制
		return nil
	}
	// 类型断言为TokenSetting指针
	setting, ok := tokenSetting.(*model.TokenSetting)
	if !ok || setting == nil {
		// 类型断言失败或为空，不进行限制
		return nil
	}
	// 判断是否启用了ip限制
	if !setting.Limits.LimitsIPSetting.Enabled {
		return nil
	}
	// 未设置白名单
	if len(setting.Limits.LimitsIPSetting.Whitelist) == 0 {
		return nil
	}

	ip := c.ClientIP()
	//判断ip是否在允许范围内
	for _, allowedIP := range setting.Limits.LimitsIPSetting.Whitelist {
		// 直接IP匹配
		if allowedIP == ip {
			return nil
		}
		// CIDR格式匹配
		if strings.Contains(allowedIP, "/") {
			if utils.IsIpInCidr(ip, allowedIP) {
				return nil
			}
		}
	}

	return fmt.Errorf("IP %s is not allowed to access", ip)
}

func OpenaiAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		isWebSocket := c.GetHeader("Upgrade") == "websocket"
		key := c.Request.Header.Get("Authorization")

		if isWebSocket && key == "" {
			protocols := c.Request.Header["Sec-Websocket-Protocol"]
			if len(protocols) > 0 {
				protocolList := strings.Split(protocols[0], ",")
				for _, protocol := range protocolList {
					protocol = strings.TrimSpace(protocol)
					if strings.HasPrefix(protocol, "openai-insecure-api-key.") {
						key = strings.TrimPrefix(protocol, "openai-insecure-api-key.")
						break
					}
				}
			}
		}
		tokenAuth(c, key)
	}
}

func ClaudeAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		key := c.Request.Header.Get("x-api-key")
		if key == "" {
			key = c.Request.Header.Get("Authorization")
		}
		tokenAuth(c, key)
	}
}

func GeminiAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		key := c.Request.Header.Get("x-goog-api-key")
		if key == "" {
			// 查询GET参数
			key = c.Query("key")

			if key == "" {
				key = c.Request.Header.Get("Authorization")
			}
		}
		tokenAuth(c, key)
	}
}

func MjAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 判断path :mode
		model := c.Param("mode")

		if model != "" && model != "mj-fast" && model != "mj-turbo" && model != "mj-relax" {
			midjourneyAbortWithMessage(c, 4, "Invalid speed mode")
			return
		}

		if model == "" {
			model = "mj-fast"
		}

		model = strings.TrimPrefix(model, "mj-")
		c.Set("mj_model", model)

		key := c.Request.Header.Get("mj-api-secret")
		tokenAuth(c, key)
	}
}

func SpecifiedChannel() func(c *gin.Context) {
	return func(c *gin.Context) {
		channelId := c.GetInt("specific_channel_id")
		c.Set("specific_channel_id_ignore", false)

		if channelId <= 0 {
			abortWithMessage(c, http.StatusForbidden, "A channel must be specified")
			return
		}
		c.Next()
	}
}
