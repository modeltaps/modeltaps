package controller

import (
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/stmp"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
)

// MailServiceResponse is a mail service as the console sees it: the password only as a masked
// echo.
type MailServiceResponse struct {
	*model.MailService
	TokenMasked string `json:"token_masked"`
}

func newMailServiceResponse(s *model.MailService) *MailServiceResponse {
	return &MailServiceResponse{MailService: s, TokenMasked: utils.MaskSecret(s.Token)}
}

// MailServiceRequest creates, updates or tests a service. An empty token keeps the stored one,
// as long as the server and username stay the stored ones.
type MailServiceRequest struct {
	Id       int    `json:"id"`
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Account  string `json:"account"`
	Token    string `json:"token"`
	TLSMode  string `json:"tls_mode"`
	From     string `json:"from"`
	FromName string `json:"from_name"`
	ReplyTo  string `json:"reply_to"`
	// To is the recipient of a test email.
	To string `json:"to"`
}

// build returns the service the request describes, over stored (nil for a new one).
func (req *MailServiceRequest) build(stored *model.MailService) *model.MailService {
	s := &model.MailService{}
	if stored != nil {
		copied := *stored
		s = &copied
	}
	s.Provider, s.Name, s.Server, s.Port = req.Provider, req.Name, req.Server, req.Port
	s.Account, s.TLSMode, s.From, s.FromName, s.ReplyTo = req.Account, req.TLSMode, req.From, req.FromName, req.ReplyTo
	s.Normalize()
	token := strings.TrimSpace(req.Token)
	switch {
	case token != "":
		s.Token = token
	case stored == nil || !s.SameEndpoint(stored):
		s.Token = ""
	}
	return s
}

// mailServiceError answers a validation or storage error with its code.
func mailServiceError(c *gin.Context, err error, passwordDropped bool) {
	switch {
	case errors.Is(err, model.ErrMailServiceIncomplete) && passwordDropped:
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeMailServicePasswordRequired, "Enter the password again: the server or username changed", nil)
	case errors.Is(err, model.ErrMailServiceIncomplete):
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeMailServiceIncomplete, "Fill in the server, port, username, password and sender address", nil)
	case errors.Is(err, model.ErrMailServiceTLSMode):
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionSmtpEncryptionInvalid, "Invalid SMTP encryption method", nil)
	case errors.Is(err, model.ErrMailServiceFrom):
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeMailServiceFromInvalid, "Enter a valid sender address", nil)
	case errors.Is(err, model.ErrMailServiceReplyTo):
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeMailServiceReplyToInvalid, "Enter a valid reply-to address", nil)
	case errors.Is(err, model.ErrMailServiceInUse):
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeMailServiceInUse, "Switch to another mail service before deleting the one in use", nil)
	default:
		common.APIRespondWithError(c, http.StatusOK, err)
	}
}

// refuseWhenManaged stops changes while the deployment sets mail (smtp.url): what is stored
// here is not used then.
func refuseWhenManaged(c *gin.Context) bool {
	if !stmp.IsManaged() {
		return false
	}
	common.APIRespondWithCode(c, http.StatusOK, common.APICodeSmtpManaged, "Mail sending is set in the deployment (smtp.url) and cannot be changed here", nil)
	return true
}

func storedMailService(c *gin.Context, id int) (*model.MailService, bool) {
	s, err := model.GetMailServiceById(id)
	if err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeMailServiceNotFound, "Mail service not found", nil)
		return nil, false
	}
	return s, true
}

func bindMailService(c *gin.Context) (*MailServiceRequest, bool) {
	var req MailServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "Invalid parameters", nil)
		return nil, false
	}
	return &req, true
}

func GetMailServices(c *gin.Context) {
	services, err := model.GetMailServices()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	out := make([]*MailServiceResponse, 0, len(services))
	for _, s := range services {
		out = append(out, newMailServiceResponse(s))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
}

// CreateMailService stores a new service; the first one goes into use.
func CreateMailService(c *gin.Context) {
	if refuseWhenManaged(c) {
		return
	}
	req, ok := bindMailService(c)
	if !ok {
		return
	}
	s := req.build(nil)
	if err := s.Validate(); err != nil {
		mailServiceError(c, err, false)
		return
	}
	if err := s.Insert(); err != nil {
		mailServiceError(c, err, false)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": newMailServiceResponse(s)})
}

func UpdateMailService(c *gin.Context) {
	if refuseWhenManaged(c) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	stored, ok := storedMailService(c, id)
	if !ok {
		return
	}
	req, ok := bindMailService(c)
	if !ok {
		return
	}
	s := req.build(stored)
	if err := s.Validate(); err != nil {
		mailServiceError(c, err, stored.Token != "" && s.Token == "")
		return
	}
	if err := s.Update(); err != nil {
		mailServiceError(c, err, false)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": newMailServiceResponse(s)})
}

// ActivateMailService puts a service into use in place of the one before.
func ActivateMailService(c *gin.Context) {
	if refuseWhenManaged(c) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	if _, ok := storedMailService(c, id); !ok {
		return
	}
	s, err := model.ActivateMailService(id)
	if err != nil {
		mailServiceError(c, err, false)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": newMailServiceResponse(s)})
}

func DeleteMailService(c *gin.Context) {
	if refuseWhenManaged(c) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	if _, ok := storedMailService(c, id); !ok {
		return
	}
	if err := model.DeleteMailService(id); err != nil {
		mailServiceError(c, err, false)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestMailService sends a test email with the service as typed, saved or not. With an id, an
// empty password uses the stored one when the server and username are unchanged.
func TestMailService(c *gin.Context) {
	req, ok := bindMailService(c)
	if !ok {
		return
	}
	req.To = strings.TrimSpace(req.To)
	if _, err := mail.ParseAddress(req.To); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeSmtpTestRecipientInvalid, "Enter a valid recipient address", nil)
		return
	}
	var stored *model.MailService
	if req.Id != 0 {
		if stored, ok = storedMailService(c, req.Id); !ok {
			return
		}
	}
	s := req.build(stored)
	if err := s.Validate(); err != nil {
		mailServiceError(c, err, stored != nil && stored.Token != "" && s.Token == "")
		return
	}
	sendSMTPTest(c, s.Client(), req.To)
}
