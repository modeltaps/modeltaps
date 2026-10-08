package controller

import (
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// userSessionView GET /api/user/sessions 的一行：不含 session_key 与 id_token，只给账号安全页展示与吊销用。
type userSessionView struct {
	Id           int    `json:"id"`
	LoginMethod  string `json:"login_method"`
	UserAgent    string `json:"user_agent"`
	Ip           string `json:"ip"`
	CreatedTime  int64  `json:"created_time"`
	LastSeenTime int64  `json:"last_seen_time"`
	// Current 是否就是发起本次请求的会话（前端据此标「当前设备」并禁用它的登出按钮）
	Current bool `json:"current"`
}

// currentSessionKey 取本次请求 cookie 会话里的会话键；access token 鉴权的请求、或没挂会话中间件的
// 上下文（单测直接调 handler）都返回空串，不能 panic。
func currentSessionKey(c *gin.Context) string {
	value, exists := c.Get(sessions.DefaultKey)
	if !exists {
		return ""
	}
	session, ok := value.(sessions.Session)
	if !ok {
		return ""
	}
	key, _ := session.Get("sid").(string)
	return key
}

// GetUserSessions 列出当前用户的有效登录会话，最近活跃的在前。
func GetUserSessions(c *gin.Context) {
	userId := c.GetInt("id")
	sessionList, err := model.ListUserSessions(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	current := currentSessionKey(c)
	views := make([]userSessionView, 0, len(sessionList))
	for _, session := range sessionList {
		views = append(views, userSessionView{
			Id:           session.Id,
			LoginMethod:  session.LoginMethod,
			UserAgent:    session.UserAgent,
			Ip:           session.Ip,
			CreatedTime:  session.CreatedTime,
			LastSeenTime: session.LastSeenTime,
			Current:      current != "" && session.SessionKey == current,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    views,
	})
}

// RevokeUserSession 登出自己的某一个会话；不允许登出当前会话（那是「退出登录」的事）。
func RevokeUserSession(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}
	if current := currentSessionKey(c); current != "" {
		if session, err := model.GetUserSessionByKey(current); err == nil && session.Id == id {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot revoke the current session, use \"Sign out\" instead",
			})
			return
		}
	}
	deleted, err := model.DeleteUserSessionById(userId, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if !deleted {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Session not found",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// LogoutOtherSessions 登出当前用户除本次会话外的全部会话。
func LogoutOtherSessions(c *gin.Context) {
	userId := c.GetInt("id")
	if err := model.DeleteUserSessionsExcept(nil, userId, currentSessionKey(c)); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
