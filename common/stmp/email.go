package stmp

import (
	"context"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"errors"
	"fmt"
	netmail "net/mail"
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
	TLSMode  string
}

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

// SystemStmpConfigured 判定系统 SMTP 是否配置齐全。供 GetSystemStmp 与 /api/status
// 共用，避免两处各写一份条件表达式后走偏。不暴露任何 SMTP 具体值。
func SystemStmpConfigured() bool {
	return config.SMTPServer != "" && config.SMTPPort != 0 && config.SMTPAccount != "" && config.SMTPToken != ""
}

func GetSystemStmp() (*StmpConfig, error) {
	if !SystemStmpConfigured() {
		return nil, fmt.Errorf("SMTP is not configured")
	}

	client := NewStmp(config.SMTPServer, config.SMTPPort, config.SMTPAccount, config.SMTPToken, config.SMTPFrom, config.SMTPTLSMode)
	client.FromName = config.SMTPFromName
	return client, nil
}

// SendTestEmail sends a short message through the given settings so an admin can check them
// before (or after) saving.
func SendTestEmail(s *StmpConfig, to string) error {
	subject := fmt.Sprintf("%s test email", config.SystemName)
	content := `<p style="font-size: 20px"><strong>Mail sending works.</strong></p>
	<p>This test email was sent from the mail settings of %s. No action is needed.</p>
	<p style="color: #858585; padding-top: 15px;">Server: %s:%d</p>`
	return s.Render(to, subject, fmt.Sprintf(content, config.SystemName, s.Host, s.Port))
}

func SendPasswordResetEmail(userName, email, link string) error {
	stmp, err := GetSystemStmp()

	if err != nil {
		return err
	}

	contentTemp := `<p style="font-size: 30px">Hi <strong>%s,</strong></p>
	<p>
		You requested a password reset. Click the button below to reset your password.
	</p>
	
	<p style="text-align: center; font-size: 13px;">
		<a target="__blank" href="%s" class="button" style="color: #ffffff;">Reset password</a>
	</p>
	
	<p style="color: #858585; padding-top: 15px;">
		If the button does not work, open the link below or copy it into your browser:<br> %s
	</p>
	<p style="color: #858585;">The reset link is valid for %d minutes. If you did not request this, please ignore this email.</p>`

	subject := fmt.Sprintf("%s password reset", config.SystemName)
	content := fmt.Sprintf(contentTemp, userName, link, link, common.VerificationValidMinutes)

	return stmp.Render(email, subject, content)
}

func SendVerificationCodeEmail(email, code string) error {
	stmp, err := GetSystemStmp()

	if err != nil {
		return err
	}

	contentTemp := `
	<p>
		You are verifying your email. Your verification code is:
	</p>
	
	<p style="text-align: center; font-size: 30px; color: #58a6ff;">
		<strong>%s</strong>
	</p>
	
	<p style="color: #858585; padding-top: 15px;">
		The verification code is valid for %d minutes. If you did not request this, please ignore this email.
	</p>`

	subject := fmt.Sprintf("%s email verification", config.SystemName)
	content := fmt.Sprintf(contentTemp, code, common.VerificationValidMinutes)

	return stmp.Render(email, subject, content)
}

func SendQuotaWarningCodeEmail(userName, email string, quota int, noMoreQuota bool) error {
	stmp, err := GetSystemStmp()

	if err != nil {
		return err
	}

	contentTemp := `<p style="font-size: 30px">Hi <strong>%s,</strong></p>
		<p>
			%s. Your remaining quota is %d. Please top up soon to avoid service interruption.
		</p>
		
		<p style="text-align: center; font-size: 13px;">
			<a target="__blank" href="%s" class="button" style="color: #ffffff;">Top up</a>
		</p>
		
		<p style="color: #858585; padding-top: 15px;">
			If the button does not work, open the link below or copy it into your browser:<br> %s
		</p>`

	subject := "Your quota is running low"
	if noMoreQuota {
		subject = "Your quota has been used up"
	}
	topUpLink := fmt.Sprintf("%s/panel/billing?topup=1", strings.TrimSuffix(config.ServerAddress, "/"))

	content := fmt.Sprintf(contentTemp, userName, subject, quota, topUpLink, topUpLink)

	return stmp.Render(email, subject, content)
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
