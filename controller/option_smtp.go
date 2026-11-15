package controller

import (
	"net/http"
	"net/mail"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/stmp"

	"github.com/gin-gonic/gin"
)

// smtpTestRequest carries the mail settings as they stand in the settings form, saved or not.
type smtpTestRequest struct {
	To       string `json:"to"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Account  string `json:"account"`
	From     string `json:"from"`
	FromName string `json:"from_name"`
	Token    string `json:"token"`
	TLSMode  string `json:"tls_mode"`
}

// smtpTestToken picks the password for a test send. An empty token means "keep the saved one",
// as when saving the form; the saved one is only reused for the saved server and account, so a
// test cannot hand the stored secret to a different host.
func smtpTestToken(req smtpTestRequest) string {
	if req.Token != "" {
		return req.Token
	}
	if strings.EqualFold(strings.TrimSpace(req.Server), strings.TrimSpace(config.SMTPServer)) &&
		strings.TrimSpace(req.Account) == strings.TrimSpace(config.SMTPAccount) {
		return config.SMTPToken
	}
	return ""
}

// TestSMTP sends a test email with the posted mail settings and reports the server's answer.
func TestSMTP(c *gin.Context) {
	var req smtpTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "Invalid parameters", nil)
		return
	}
	req.To = strings.TrimSpace(req.To)
	req.Server = strings.TrimSpace(req.Server)
	if _, err := mail.ParseAddress(req.To); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeSmtpTestRecipientInvalid, "Enter a valid recipient address", nil)
		return
	}
	if req.Server == "" || req.Port <= 0 || req.Port > 65535 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeSmtpTestIncomplete, "Fill in the mail server and port first", nil)
		return
	}
	if !stmp.IsValidTLSMode(req.TLSMode) {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionSmtpEncryptionInvalid, "Invalid SMTP encryption method", nil)
		return
	}

	client := stmp.NewStmp(req.Server, req.Port, strings.TrimSpace(req.Account), smtpTestToken(req), strings.TrimSpace(req.From), req.TLSMode)
	client.FromName = req.FromName
	if err := stmp.SendTestEmail(client, req.To); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeSmtpTestFailed, "Test email failed: "+err.Error(), map[string]any{"reason": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
