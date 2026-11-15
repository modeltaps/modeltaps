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
	ReplyTo  string `json:"reply_to"`
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
	// Set in the deployment: test what is actually used, whatever the form says.
	if stmp.IsManaged() {
		client, err := stmp.GetSystemStmp()
		if err != nil {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeSmtpTestFailed, "Test email failed: "+err.Error(), map[string]any{"reason": err.Error()})
			return
		}
		sendSMTPTest(c, client, req.To)
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
	client.ReplyTo = strings.TrimSpace(req.ReplyTo)
	sendSMTPTest(c, client, req.To)
}

func sendSMTPTest(c *gin.Context, client *stmp.StmpConfig, to string) {
	if err := stmp.SendTestEmail(client, to); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeSmtpTestFailed, "Test email failed: "+err.Error(), map[string]any{"reason": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// managedMailResponse is the deployment's mail setup as the console shows it; the password is
// never included.
type managedMailResponse struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Account  string `json:"account"`
	From     string `json:"from"`
	FromName string `json:"from_name"`
	ReplyTo  string `json:"reply_to"`
	TLSMode  string `json:"tls_mode"`
	Error    string `json:"error,omitempty"`
}

// GetMailSettings reports whether mail is set in the deployment (and how), and the user email
// templates with their defaults and variables.
func GetMailSettings(c *gin.Context) {
	var managed *managedMailResponse
	if stmp.IsManaged() {
		managed = &managedMailResponse{}
		if m, err := stmp.Managed(); err != nil {
			managed.Error = err.Error()
		} else {
			*managed = managedMailResponse{
				Server: m.Host, Port: m.Port, Account: m.Username, From: m.From,
				FromName: m.FromName, ReplyTo: m.ReplyTo, TLSMode: m.TLSMode,
			}
			if m.From == "" {
				managed.Error = "smtp.from is not set"
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"managed":    managed,
		"configured": stmp.SystemStmpConfigured(),
		"templates":  stmp.TemplateSpecs,
	}})
}

// PreviewMailTemplate renders a template, as typed in the console, with sample values.
func PreviewMailTemplate(c *gin.Context) {
	var req struct {
		Template string `json:"template"`
		Subject  string `json:"subject"`
		Body     string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || stmp.TemplateSpecByID(req.Template) == nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "Invalid parameters", nil)
		return
	}
	subject, body := stmp.PreviewTemplate(req.Template, stmp.MailTemplate{Subject: req.Subject, Body: req.Body})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"subject": subject, "html": body}})
}

// smtpOptionKeys are the options a deployment-set mail setup replaces.
var smtpOptionKeys = map[string]bool{
	"SMTPProvider": true, "SMTPServer": true, "SMTPPort": true, "SMTPAccount": true, "SMTPFrom": true,
	"SMTPFromName": true, "SMTPReplyTo": true, "SMTPToken": true, "SMTPTLSMode": true,
}
