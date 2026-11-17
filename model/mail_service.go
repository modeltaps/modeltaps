package model

import (
	"errors"
	"net/mail"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/stmp"
	"github.com/modeltaps/modeltaps/common/utils"
	"gorm.io/gorm"
)

// MailService is one way the platform can send mail: a preset service (Resend, SES, a Gmail
// mailbox ...) or a custom SMTP server, with its sender. Several can be stored so a new one can
// be set up and tested before switching; exactly one is in use while any exist.
//
// The one in use is mirrored into the SMTP* options, which is what sending reads and what other
// nodes pick up through the option sync, so the send path does not know about this table.
type MailService struct {
	Id int `json:"id"`
	// Provider is the console preset (resend, ses, gmail ..., or custom); it only shapes the form.
	Provider    string `json:"provider" gorm:"type:varchar(32);default:''"`
	Name        string `json:"name" gorm:"type:varchar(64);default:''"`
	Server      string `json:"server" gorm:"type:varchar(255);default:''"`
	Port        int    `json:"port" gorm:"default:587"`
	Account     string `json:"account" gorm:"type:varchar(255);default:''"`
	Token       string `json:"-" gorm:"type:text;serializer:encrypted"` // encrypted at rest, see secret_storage.go
	TLSMode     string `json:"tls_mode" gorm:"type:varchar(32);default:'auto'"`
	From        string `json:"from" gorm:"type:varchar(255);default:''"`
	FromName    string `json:"from_name" gorm:"type:varchar(128);default:''"`
	ReplyTo     string `json:"reply_to" gorm:"type:varchar(255);default:''"`
	Active      bool   `json:"active" gorm:"default:false"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

var (
	ErrMailServiceIncomplete = errors.New("fill in the server, port, username, password and sender address")
	ErrMailServiceTLSMode    = errors.New("invalid SMTP encryption method")
	ErrMailServiceFrom       = errors.New("enter a valid sender address")
	ErrMailServiceReplyTo    = errors.New("enter a valid reply-to address")
	ErrMailServiceInUse      = errors.New("switch to another mail service before deleting the one in use")
)

// Normalize trims the fields and fills the encryption default.
func (s *MailService) Normalize() {
	for _, f := range []*string{&s.Provider, &s.Name, &s.Server, &s.Account, &s.TLSMode, &s.From, &s.FromName, &s.ReplyTo} {
		*f = strings.TrimSpace(*f)
	}
	if s.TLSMode == "" {
		s.TLSMode = "auto"
	}
}

// Validate checks that the service can send: every connection field, a valid encryption method
// and valid addresses.
func (s *MailService) Validate() error {
	if s.Server == "" || s.Port <= 0 || s.Port > 65535 || s.Account == "" || s.Token == "" || s.From == "" {
		return ErrMailServiceIncomplete
	}
	if !stmp.IsValidTLSMode(s.TLSMode) {
		return ErrMailServiceTLSMode
	}
	if _, err := mail.ParseAddress(s.From); err != nil {
		return ErrMailServiceFrom
	}
	if s.ReplyTo != "" {
		if _, err := mail.ParseAddress(s.ReplyTo); err != nil {
			return ErrMailServiceReplyTo
		}
	}
	return nil
}

// Client builds the mail client for the service.
func (s *MailService) Client() *stmp.StmpConfig {
	client := stmp.NewStmp(s.Server, s.Port, s.Account, s.Token, s.From, s.TLSMode)
	client.FromName = s.FromName
	client.ReplyTo = s.ReplyTo
	return client
}

// SameEndpoint reports whether other points at the same server and username, the condition for
// reusing a stored password: it is never handed to another host or account.
func (s *MailService) SameEndpoint(other *MailService) bool {
	return strings.EqualFold(s.Server, other.Server) && s.Account == other.Account
}

func GetMailServices() ([]*MailService, error) {
	var services []*MailService
	err := DB.Order("id asc").Find(&services).Error
	return services, err
}

func GetMailServiceById(id int) (*MailService, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	var s MailService
	if err := DB.First(&s, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// Insert stores a new service. The first one goes into use at once.
func (s *MailService) Insert() error {
	now := utils.GetTimestamp()
	s.CreatedTime, s.UpdatedTime = now, now
	err := DB.Transaction(func(tx *gorm.DB) error {
		var active int64
		if err := tx.Model(&MailService{}).Where("active = ?", true).Count(&active).Error; err != nil {
			return err
		}
		s.Active = active == 0
		return tx.Create(s).Error
	})
	if err != nil || !s.Active {
		return err
	}
	return mirrorMailService(s)
}

// Update saves the service; when it is the one in use the change applies to sending at once.
func (s *MailService) Update() error {
	s.UpdatedTime = utils.GetTimestamp()
	err := DB.Model(s).Select("provider", "name", "server", "port", "account", "token", "tls_mode",
		"from", "from_name", "reply_to", "updated_time").Updates(s).Error
	if err != nil || !s.Active {
		return err
	}
	return mirrorMailService(s)
}

// ActivateMailService puts the service into use; the one in use before stops being used.
func ActivateMailService(id int) (*MailService, error) {
	s, err := GetMailServiceById(id)
	if err != nil {
		return nil, err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&MailService{}).Where("active = ? AND id <> ?", true, id).Update("active", false).Error; err != nil {
			return err
		}
		return tx.Model(&MailService{}).Where("id = ?", id).Updates(map[string]any{"active": true, "updated_time": utils.GetTimestamp()}).Error
	})
	if err != nil {
		return nil, err
	}
	s.Active = true
	return s, mirrorMailService(s)
}

// DeleteMailService removes a service. The one in use can only go when it is the last one, and
// then the platform stops sending mail.
func DeleteMailService(id int) error {
	s, err := GetMailServiceById(id)
	if err != nil {
		return err
	}
	if s.Active {
		var count int64
		if err := DB.Model(&MailService{}).Count(&count).Error; err != nil {
			return err
		}
		if count > 1 {
			return ErrMailServiceInUse
		}
	}
	if err := DB.Delete(&MailService{}, "id = ?", id).Error; err != nil {
		return err
	}
	if s.Active {
		return mirrorMailService(nil)
	}
	return nil
}

// mirrorMailService writes the service in use into the SMTP* options; nil clears them.
func mirrorMailService(s *MailService) error {
	if s == nil {
		s = &MailService{Port: 587, TLSMode: "auto"}
	}
	values := []struct{ key, value string }{
		{"SMTPProvider", s.Provider},
		{"SMTPServer", s.Server},
		{"SMTPPort", strconv.Itoa(s.Port)},
		{"SMTPAccount", s.Account},
		{"SMTPToken", s.Token},
		{"SMTPTLSMode", s.TLSMode},
		{"SMTPFrom", s.From},
		{"SMTPFromName", s.FromName},
		{"SMTPReplyTo", s.ReplyTo},
	}
	for _, v := range values {
		if err := UpdateOption(v.key, v.value); err != nil {
			return err
		}
	}
	return nil
}
