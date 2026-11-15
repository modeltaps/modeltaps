package stmp

import (
	"context"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"errors"
	"fmt"
	"html"
	netmail "net/mail"
	"strconv"
	"strings"

	"github.com/wneessen/go-mail"
)

var SendResetError = &mail.SendError{
	Reason: mail.ErrSMTPReset,
}

// SMTP 连接加密方式。auto 维持按端口推断的历史行为，其余为管理员显式指定。
const (
	TLSModeAuto                  = "auto"
	TLSModeSSL                   = "ssl"
	TLSModeStartTLS              = "starttls"
	TLSModeStartTLSOpportunistic = "starttls_opportunistic"
	TLSModeNone                  = "none"
)

// IsValidTLSMode 判定加密方式取值是否合法（空串视为 auto）。
func IsValidTLSMode(mode string) bool {
	switch mode {
	case "", TLSModeAuto, TLSModeSSL, TLSModeStartTLS, TLSModeStartTLSOpportunistic, TLSModeNone:
		return true
	default:
		return false
	}
}

// tlsSettings 是加密方式映射到 go-mail 客户端的三项配置。
type tlsSettings struct {
	ssl    bool
	policy mail.TLSPolicy
	auth   mail.SMTPAuthType
}

// resolveTLSSettings 把加密方式与端口映射为 go-mail 客户端配置。
// 认证方式沿用历史的按端口选择（587 用 LOGIN，其余用 PLAIN）；明文投递下
// 必须改用 PLAIN-NOENC，否则 go-mail 会拒绝在未加密连接上发送 PLAIN。
func resolveTLSSettings(mode string, port int) tlsSettings {
	settings := tlsSettings{policy: mail.TLSMandatory, auth: mail.SMTPAuthPlain}
	if port == 587 {
		settings.auth = mail.SMTPAuthLogin
	}

	switch mode {
	case TLSModeSSL:
		settings.ssl = true
	case TLSModeStartTLS:
		settings.policy = mail.TLSMandatory
	case TLSModeStartTLSOpportunistic:
		settings.policy = mail.TLSOpportunistic
		settings.auth = mail.SMTPAuthPlainNoEnc
	case TLSModeNone:
		settings.policy = mail.NoTLS
		settings.auth = mail.SMTPAuthPlainNoEnc
	default:
		if port == 465 {
			settings.ssl = true
		}
	}

	return settings
}

type StmpConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// FromName is the optional display name shown before From.
	FromName string
	// ReplyTo is the optional address replies go to instead of From.
	ReplyTo string
	TLSMode string
	// Kind names what the email is for in the send record (verification, test, notice, ...).
	Kind string
}

// SendRecord is one email the platform tried to send, as the send record keeps it.
type SendRecord struct {
	Kind    string
	To      string
	Subject string
	Err     error
}

// OnSent, when set, is told about every email after it was sent or failed (the send record).
var OnSent func(SendRecord)

func NewStmp(host string, port int, username string, password string, from string, tlsMode string) *StmpConfig {
	if from == "" {
		from = username
	}

	return &StmpConfig{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
		TLSMode:  tlsMode,
	}
}

// sender splits the configured sender into display name and address. From may itself be written
// as "Name <addr>"; FromName, when set, wins over a name written there.
func (s *StmpConfig) sender() (name, address string, err error) {
	addr, err := netmail.ParseAddress(strings.TrimSpace(s.From))
	if err != nil {
		return "", "", fmt.Errorf("invalid sender address %q: %w", s.From, err)
	}
	name = addr.Name
	if fromName := strings.TrimSpace(s.FromName); fromName != "" {
		name = fromName
	}
	return name, addr.Address, nil
}

func (s *StmpConfig) Send(to, subject, body string) error {
	err := s.send(to, subject, body)
	if OnSent != nil {
		OnSent(SendRecord{Kind: s.Kind, To: to, Subject: subject, Err: err})
	}
	return err
}

func (s *StmpConfig) send(to, subject, body string) error {
	name, address, err := s.sender()
	if err != nil {
		return err
	}

	message := mail.NewMsg()
	// FromFormat encodes only the display name (RFC 2047), so a non-ASCII name such as a
	// Chinese site name reaches the inbox intact instead of as one mangled encoded word.
	if err := message.FromFormat(name, address); err != nil {
		return fmt.Errorf("invalid sender: %w", err)
	}
	if err := message.To(to); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}
	if replyTo := strings.TrimSpace(s.ReplyTo); replyTo != "" {
		if err := message.ReplyTo(replyTo); err != nil {
			return fmt.Errorf("invalid reply-to address: %w", err)
		}
	}
	message.Subject(subject)
	message.SetGenHeader("References", references(address))
	message.SetBodyString(mail.TypeTextHTML, body)
	message.SetUserAgent(fmt.Sprintf("Modeltaps %s // https://github.com/modeltaps/modeltaps", config.Version))

	settings := resolveTLSSettings(s.TLSMode, s.Port)

	client, err := mail.NewClient(
		s.Host,
		mail.WithPort(s.Port),
		mail.WithUsername(s.Username),
		mail.WithPassword(s.Password),
		mail.WithSMTPAuth(settings.auth),
	)

	if err != nil {
		return err
	}

	client.SetSSL(settings.ssl)
	client.SetTLSPolicy(settings.policy)

	if err := DialAndSend(client, message); err != nil {
		return err
	}

	return nil
}

func references(address string) string {
	local, domain, ok := strings.Cut(address, "@")
	if !ok {
		domain = "localhost"
	}
	return fmt.Sprintf("<%s.%s@%s>", local, utils.GetUUID(), domain)
}

func (s *StmpConfig) Render(to, subject, content string) error {
	body := getDefaultTemplate(content)

	return s.Send(to, subject, body)
}

// SystemStmpConfigured reports whether the platform can send mail: set in the deployment, or
// complete in the console. Shared by GetSystemStmp and /api/status; it reveals no setting.
func SystemStmpConfigured() bool {
	if IsManaged() {
		m, err := Managed()
		return err == nil && m.From != ""
	}
	return config.SMTPServer != "" && config.SMTPPort != 0 && config.SMTPAccount != "" && config.SMTPToken != ""
}

// GetSystemStmp returns the platform's mail client: the deployment's when smtp.url is set, else
// the console's.
func GetSystemStmp() (*StmpConfig, error) {
	if IsManaged() {
		m, err := Managed()
		if err != nil {
			return nil, err
		}
		if m.From == "" {
			return nil, fmt.Errorf("smtp.from is not set")
		}
		return m.client(), nil
	}
	if !SystemStmpConfigured() {
		return nil, fmt.Errorf("SMTP is not configured")
	}

	client := NewStmp(config.SMTPServer, config.SMTPPort, config.SMTPAccount, config.SMTPToken, config.SMTPFrom, config.SMTPTLSMode)
	client.FromName = config.SMTPFromName
	client.ReplyTo = config.SMTPReplyTo
	return client, nil
}

// SendTestEmail sends a short message through the given settings so an admin can check them
// before (or after) saving.
func SendTestEmail(s *StmpConfig, to string) error {
	subject := fmt.Sprintf("%s test email", config.SystemName)
	content := `<p style="font-size: 20px"><strong>Mail sending works.</strong></p>
	<p>This test email was sent from the mail settings of %s. No action is needed.</p>
	<p style="color: #858585; padding-top: 15px;">Server: %s:%d</p>`
	s.Kind = "test"
	return s.Render(to, subject, fmt.Sprintf(content, html.EscapeString(config.SystemName), html.EscapeString(s.Host), s.Port))
}

// sendTemplate sends one of the user emails (mail_templates.go) through the platform's client.
func sendTemplate(id, to string, vars map[string]string) error {
	client, err := GetSystemStmp()
	if err != nil {
		return err
	}
	client.Kind = id
	subject, body := RenderTemplate(id, vars, nil)
	return client.Send(to, subject, body)
}

func SendPasswordResetEmail(userName, email, link string) error {
	return sendTemplate(TemplatePasswordReset, email, map[string]string{
		"user": userName, "link": link, "minutes": strconv.Itoa(common.VerificationValidMinutes),
	})
}

func SendVerificationCodeEmail(email, code string) error {
	return sendTemplate(TemplateVerification, email, map[string]string{
		"code": code, "minutes": strconv.Itoa(common.VerificationValidMinutes),
	})
}

func SendQuotaWarningCodeEmail(userName, email string, quota int, noMoreQuota bool) error {
	status := "Your quota is running low"
	if noMoreQuota {
		status = "Your quota has been used up"
	}
	topUpLink := fmt.Sprintf("%s/panel/billing?topup=1", strings.TrimSuffix(config.ServerAddress, "/"))
	return sendTemplate(TemplateQuotaWarning, email, map[string]string{
		"user": userName, "quota": strconv.Itoa(quota), "status": status, "link": topUpLink,
	})
}

func DialAndSend(c *mail.Client, messages ...*mail.Msg) error {
	ctx := context.Background()
	if err := c.DialWithContext(ctx); err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer c.Close()

	if err := c.Send(messages...); err != nil {
		if errors.Is(err, SendResetError) {
			return nil
		}
		return fmt.Errorf("send failed: %w", err)
	}
	return nil
}
