package stmp

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Mail sending can be set in the deployment instead of the console: `smtp.url` in the config
// file or SMTP_URL in the environment, e.g.
//
//	smtps://user:password@smtp.example.com:465
//	smtp://apikey:SG.xxx@smtp.sendgrid.net:587?tls=starttls
//
// with smtp.from / smtp.from_name / smtp.reply_to (SMTP_FROM, ...) for the sender. When it is set
// it wins over the console settings, which turn read-only.

// ManagedSettings is the mail setup read from the deployment.
type ManagedSettings struct {
	Host     string
	Port     int
	Username string
	Password string
	TLSMode  string
	From     string
	FromName string
	ReplyTo  string
}

// ManagedURL is the raw deployment setting, empty when mail is set in the console.
func ManagedURL() string {
	return strings.TrimSpace(viper.GetString("smtp.url"))
}

// IsManaged reports whether mail sending is set in the deployment.
func IsManaged() bool {
	return ManagedURL() != ""
}

// ParseManagedURL reads an smtp:// or smtps:// URL. smtps means implicit TLS; smtp keeps the
// port-based default unless ?tls= names an encryption mode (see IsValidTLSMode).
func ParseManagedURL(raw string) (*ManagedSettings, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid smtp.url: %w", err)
	}
	s := &ManagedSettings{Host: u.Hostname(), TLSMode: TLSModeAuto}
	switch strings.ToLower(u.Scheme) {
	case "smtps":
		s.TLSMode, s.Port = TLSModeSSL, 465
	case "smtp":
		s.Port = 587
	default:
		return nil, fmt.Errorf("invalid smtp.url: scheme must be smtp or smtps, got %q", u.Scheme)
	}
	if s.Host == "" {
		return nil, fmt.Errorf("invalid smtp.url: host is missing")
	}
	if p := u.Port(); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("invalid smtp.url: bad port %q", p)
		}
		s.Port = port
	}
	if u.User != nil {
		s.Username = u.User.Username()
		s.Password, _ = u.User.Password()
	}
	if mode := u.Query().Get("tls"); mode != "" {
		if !IsValidTLSMode(mode) {
			return nil, fmt.Errorf("invalid smtp.url: unknown tls mode %q", mode)
		}
		s.TLSMode = mode
	}
	return s, nil
}

// Managed returns the deployment's mail setup, or nil when mail is set in the console.
func Managed() (*ManagedSettings, error) {
	raw := ManagedURL()
	if raw == "" {
		return nil, nil
	}
	s, err := ParseManagedURL(raw)
	if err != nil {
		return nil, err
	}
	s.From = strings.TrimSpace(viper.GetString("smtp.from"))
	s.FromName = strings.TrimSpace(viper.GetString("smtp.from_name"))
	s.ReplyTo = strings.TrimSpace(viper.GetString("smtp.reply_to"))
	return s, nil
}

func (s *ManagedSettings) client() *StmpConfig {
	c := NewStmp(s.Host, s.Port, s.Username, s.Password, s.From, s.TLSMode)
	c.FromName, c.ReplyTo = s.FromName, s.ReplyTo
	return c
}
