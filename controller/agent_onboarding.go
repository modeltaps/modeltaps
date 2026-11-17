package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// Handing an API key to an agent (AGENT-7). Pairing codes come with every new API key and can be
// issued again from the key's details; the device authorization flow (RFC 8628) lets an agent
// without a key ask for one, which the signed-in user approves on the console's /device page.
// How the plaintext key is kept out of storage is explained in model/agent_onboarding.go.
//
// The endpoints an agent calls (/api/agent/pair, /api/agent/device, /api/agent/device/token) need
// no sign-in and answer in the OAuth shape RFC 8628 clients read: errors carry `error` and
// `error_description` (next to the console's `success`, `message` and `code`, with the same
// value), and a key comes back as `access_token`.

// deviceCodeGrantType is the grant_type of RFC 8628 §3.4; a poll may leave it out.
const deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// respondAgentError answers an agent endpoint with an error.
func respondAgentError(c *gin.Context, status int, code, message string) {
	c.Header("Cache-Control", "no-store")
	c.JSON(status, gin.H{
		"success":           false,
		"message":           message,
		"code":              code,
		"error":             code,
		"error_description": message,
	})
}

// respondAgentKey answers an agent with its API key: the only response that carries it.
func respondAgentKey(c *gin.Context, token *model.Token) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "",
		"access_token": "sk-" + token.Key,
		"token_type":   "Bearer",
		"key_name":     token.Name,
		"key_hint":     token.KeyHint,
		// Unix seconds, -1 = never.
		"expires_at": token.ExpiredTime,
	})
}

// respondCreatedTokenKey answers the creation of an API key: the key once, like respondNewTokenKey,
// plus its pairing code (valid model.AgentPairingCodeLifetime) for handing it to an agent. A key
// whose pairing code could not be made is still created; a new code can be issued from its details.
func respondCreatedTokenKey(c *gin.Context, token *model.Token, createdBy int) {
	data := gin.H{
		"id":       token.Id,
		"key":      token.Key,
		"key_hint": token.KeyHint,
	}
	if code, expiresAt, err := model.CreateAgentPairingCode(token.Id, createdBy); err != nil {
		logger.SysError("failed to create a pairing code: " + err.Error())
	} else {
		data["pairing_code"] = code
		data["pairing_expires_at"] = expiresAt
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

// respondPairingCode issues a new pairing code for token (replacing a pending one) and answers.
func respondPairingCode(c *gin.Context, token *model.Token) {
	if token.Name == model.PlaygroundTokenName {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodePairingCodeManaged, "The playground API key is managed by the system and cannot be handed to an agent", nil)
		return
	}
	code, expiresAt, err := model.CreateAgentPairingCode(token.Id, c.GetInt("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"pairing_code": code, "pairing_expires_at": expiresAt},
	})
}

// CreateTokenPairingCode issues a new pairing code for one of the user's API keys. Like a reset,
// redeeming it gives the key a new secret, so the copies in use stop working.
func CreateTokenPairingCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	token, err := model.GetTokenByIds(id, c.GetInt("id"))
	if err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenNotFound, "API key not found", nil)
		return
	}
	respondPairingCode(c, token)
}

// CreateOrgTokenPairingCode issues a new pairing code for an organization API key. Only the member
// who created the key may, as for a reset: the code hands over a working key.
func CreateOrgTokenPairingCode(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	id, err := strconv.Atoi(c.Param("token_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	token, err := getOrgTokenForOperator(c, id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if token.CreatedBy != c.GetInt("id") {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodePairingCodeNotCreator, "Only the member who created this API key can hand it to an agent", nil)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "token.pairing_code", fmt.Sprintf("Issued a pairing code for organization API key %s (ID: %d)", token.Name, token.Id))
	respondPairingCode(c, token)
}

type pairRequest struct {
	Code string `json:"code" form:"code"`
}

// RedeemPairingCode is the agent's side of a pairing code: POST /api/agent/pair {"code": "pair-..."}.
// It answers with the key, which gets a new secret (the one shown at creation stops working).
func RedeemPairingCode(c *gin.Context) {
	var req pairRequest
	_ = c.ShouldBind(&req)
	token, err := model.RedeemAgentPairingCode(req.Code, c.ClientIP(), c.Request.UserAgent())
	switch {
	case err == nil:
	case errors.Is(err, model.ErrPairingCodeInvalid):
		respondAgentError(c, http.StatusBadRequest, common.APICodePairingCodeInvalid, err.Error())
		return
	case errors.Is(err, model.ErrPairingCodeExpired):
		respondAgentError(c, http.StatusBadRequest, common.APICodePairingCodeExpired, err.Error())
		return
	case errors.Is(err, model.ErrPairingCodeUsed):
		respondAgentError(c, http.StatusBadRequest, common.APICodePairingCodeUsed, err.Error())
		return
	case errors.Is(err, model.ErrPairingKeyUnavailable):
		respondAgentError(c, http.StatusBadRequest, common.APICodePairingKeyUnavailable, err.Error())
		return
	default:
		logger.SysError("failed to redeem a pairing code: " + err.Error())
		respondAgentError(c, http.StatusInternalServerError, common.APICodeSystemError, "Failed to redeem the pairing code, please try again later")
		return
	}
	recordAgentClaimAudit(token, model.AgentClaimViaPairingCode)
	respondAgentKey(c, token)
}

// recordAgentClaimAudit notes in the organization's audit log that an agent claimed one of its keys.
func recordAgentClaimAudit(token *model.Token, via string) {
	orgId := model.OrganizationIdOfShadowUser(token.UserId)
	if orgId == 0 {
		return
	}
	actor := token.CreatedBy
	model.RecordOrgAudit(orgId, actor, "token.agent_claim", fmt.Sprintf("An agent claimed organization API key %s (ID: %d) via %s", token.Name, token.Id, via))
}

type deviceStartRequest struct {
	ClientName string `json:"client_name" form:"client_name"`
	ClientId   string `json:"client_id" form:"client_id"`
}

// StartDeviceAuthorization is RFC 8628 §3.1-3.2: POST /api/agent/device {"client_name": "..."}.
// The client name is what the user sees on the confirmation page, next to the request's IP.
func StartDeviceAuthorization(c *gin.Context) {
	var req deviceStartRequest
	_ = c.ShouldBind(&req)
	name := strings.TrimSpace(req.ClientName)
	if name == "" {
		name = strings.TrimSpace(req.ClientId)
	}
	deviceCode, row, err := model.CreateAgentDeviceAuthorization(name, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		logger.SysError("failed to start a device authorization: " + err.Error())
		respondAgentError(c, http.StatusInternalServerError, common.APICodeSystemError, "Failed to start the device authorization, please try again later")
		return
	}
	userCode := common.FormatDeviceUserCode(row.UserCode)
	verification := publicSiteURL(c) + "/device"
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"success":                   true,
		"message":                   "",
		"device_code":               deviceCode,
		"user_code":                 userCode,
		"verification_uri":          verification,
		"verification_uri_complete": verification + "?user_code=" + url.QueryEscape(userCode),
		"expires_in":                row.ExpiresAt - row.CreatedTime,
		"interval":                  row.Interval,
	})
}

type deviceTokenRequest struct {
	GrantType  string `json:"grant_type" form:"grant_type"`
	DeviceCode string `json:"device_code" form:"device_code"`
}

// PollDeviceToken is RFC 8628 §3.4-3.5: the agent polls with its device code until the user
// answered. The device code is single use: once the key was returned it is invalid_grant.
func PollDeviceToken(c *gin.Context) {
	var req deviceTokenRequest
	_ = c.ShouldBind(&req)
	if req.GrantType != "" && req.GrantType != deviceCodeGrantType {
		respondAgentError(c, http.StatusBadRequest, common.APICodeDeviceUnsupportedGrant, "grant_type must be "+deviceCodeGrantType)
		return
	}
	token, _, err := model.PollAgentDeviceAuthorization(strings.TrimSpace(req.DeviceCode), c.ClientIP(), c.Request.UserAgent())
	switch {
	case err == nil:
	case errors.Is(err, model.ErrDeviceAuthPending):
		respondAgentError(c, http.StatusBadRequest, common.APICodeDeviceAuthPending, err.Error())
		return
	case errors.Is(err, model.ErrDeviceSlowDown):
		respondAgentError(c, http.StatusBadRequest, common.APICodeDeviceSlowDown, err.Error())
		return
	case errors.Is(err, model.ErrDeviceAccessDenied):
		respondAgentError(c, http.StatusBadRequest, common.APICodeDeviceAccessDenied, err.Error())
		return
	case errors.Is(err, model.ErrDeviceExpired):
		respondAgentError(c, http.StatusBadRequest, common.APICodeDeviceExpiredToken, err.Error())
		return
	case errors.Is(err, model.ErrDeviceInvalidGrant):
		respondAgentError(c, http.StatusBadRequest, common.APICodeDeviceInvalidGrant, err.Error())
		return
	default:
		logger.SysError("failed to answer a device token poll: " + err.Error())
		respondAgentError(c, http.StatusInternalServerError, common.APICodeSystemError, "Failed to check the device authorization, please try again later")
		return
	}
	recordAgentClaimAudit(token, model.AgentClaimViaDeviceFlow)
	respondAgentKey(c, token)
}

// pendingDeviceAuthorization loads the authorization named by the :user_code parameter, or answers
// with device_user_code_invalid.
func pendingDeviceAuthorization(c *gin.Context) (*model.AgentDeviceAuthorization, bool) {
	row, err := model.GetPendingAgentDeviceAuthorization(c.Param("user_code"))
	if err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeDeviceUserCodeInvalid, err.Error(), nil)
		return nil, false
	}
	return row, true
}

// GetDeviceAuthorization shows the signed-in user what is asking for a key: the client's name,
// IP and User-Agent as the agent sent them, and until when the request is open.
func GetDeviceAuthorization(c *gin.Context) {
	row, ok := pendingDeviceAuthorization(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"user_code":          common.FormatDeviceUserCode(row.UserCode),
			"client_name":        row.ClientName,
			"request_ip":         row.RequestIp,
			"request_user_agent": row.RequestUserAgent,
			"created_time":       row.CreatedTime,
			"expires_at":         row.ExpiresAt,
			// Whether the request came from the network the user is on now: a request from
			// somewhere else is worth a second look before approving.
			"same_network": row.RequestIp != "" && row.RequestIp == c.ClientIP(),
		},
	})
}

// GetDeviceAuthorizationStatus follows a request the signed-in user answered: approved, denied,
// claimed (the agent received its key, with when and from where) or expired (approved but not
// claimed in time). The confirmation page polls it after the answer.
func GetDeviceAuthorizationStatus(c *gin.Context) {
	row, claim, err := model.GetAnsweredAgentDeviceAuthorization(c.Param("user_code"), c.GetInt("id"))
	if err != nil {
		if !errors.Is(err, model.ErrDeviceUserCodeInvalid) {
			logger.SysError("failed to read a device authorization: " + err.Error())
		}
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeDeviceUserCodeInvalid, model.ErrDeviceUserCodeInvalid.Error(), nil)
		return
	}
	data := gin.H{"status": row.AnswerStatus(), "expires_at": row.ExpiresAt}
	if claim != nil {
		data["claim"] = gin.H{"claimed_at": claim.CreatedTime, "ip": claim.Ip, "user_agent": claim.UserAgent}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

// defaultDeviceKeyName names a key approved without a name after the requesting client.
func defaultDeviceKeyName(token *model.Token, row *model.AgentDeviceAuthorization) {
	token.Name = strings.TrimSpace(token.Name)
	if token.Name != "" {
		return
	}
	name := []rune(strings.TrimSpace(row.ClientName))
	if len(name) == 0 {
		token.Name = "agent-" + strings.ToLower(row.UserCode[:4])
		return
	}
	if len(name) > 30 {
		name = name[:30]
	}
	token.Name = string(name)
	for len(token.Name) > 30 {
		name = name[:len(name)-1]
		token.Name = string(name)
	}
}

// approveDeviceAuthorization binds a prepared key (or the error preparing it) to the request.
func approveDeviceAuthorization(c *gin.Context, row *model.AgentDeviceAuthorization, orgId int, token *model.Token, err error) bool {
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return false
	}
	if err := model.ApproveAgentDeviceAuthorization(row, c.GetInt("id"), orgId, token); err != nil {
		if errors.Is(err, model.ErrDeviceUserCodeInvalid) {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeDeviceUserCodeInvalid, err.Error(), nil)
			return false
		}
		logger.SysError("failed to approve a device authorization: " + err.Error())
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeDeviceAuthorizeFailed, "Failed to create the API key, please try again", nil)
		return false
	}
	// No key in this answer: the agent's next poll receives it.
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"id": token.Id, "name": token.Name},
	})
	return true
}

// ApproveDeviceAuthorization creates a personal API key with the user's choices (the fields of the
// console's create) for the requesting agent.
func ApproveDeviceAuthorization(c *gin.Context) {
	row, ok := pendingDeviceAuthorization(c)
	if !ok {
		return
	}
	input := model.Token{}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	defaultDeviceKeyName(&input, row)
	token, err := newPersonalToken(c.GetInt("id"), c.GetInt("role"), &input)
	approveDeviceAuthorization(c, row, 0, token, err)
}

// ApproveOrgDeviceAuthorization creates an organization API key for the requesting agent, as the
// member's own create would (model access, budgets and created_by apply).
func ApproveOrgDeviceAuthorization(c *gin.Context) {
	org, operatorRole := getOrgFromContext(c)
	row, ok := pendingDeviceAuthorization(c)
	if !ok {
		return
	}
	input := model.Token{}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	defaultDeviceKeyName(&input, row)
	userId := c.GetInt("id")
	token, err := newOrgToken(org, userId, operatorRole, c.GetInt("role"), &input)
	if approveDeviceAuthorization(c, row, org.Id, token, err) {
		model.RecordOrgAudit(org.Id, userId, "token.create", fmt.Sprintf("Created organization API key %s (ID: %d) for an agent (device authorization)", token.Name, token.Id))
	}
}

// DenyDeviceAuthorization turns the request down; the agent's next poll gets access_denied.
func DenyDeviceAuthorization(c *gin.Context) {
	row, ok := pendingDeviceAuthorization(c)
	if !ok {
		return
	}
	if err := model.DenyAgentDeviceAuthorization(row, c.GetInt("id")); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeDeviceUserCodeInvalid, err.Error(), nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
