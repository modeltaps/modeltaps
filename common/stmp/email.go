package stmp

import (
	"context"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"errors"
	"fmt"
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

func (s *StmpConfig) Send(to, subject, body string) error {
	message := mail.NewMsg()
	message.From(s.From)
	message.To(to)
	message.Subject(subject)
	message.SetGenHeader("References", s.getReferences())
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

func (s *StmpConfig) getReferences() string {
	froms := strings.Split(s.From, "@")
	return fmt.Sprintf("<%s.%s@%s>", froms[0], utils.GetUUID(), froms[1])
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

	return NewStmp(config.SMTPServer, config.SMTPPort, config.SMTPAccount, config.SMTPToken, config.SMTPFrom, config.SMTPTLSMode), nil
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
	topUpLink := fmt.Sprintf("%s/topup", config.ServerAddress)

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
