package auth

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

type EmailVerificationSenderConfig struct {
	Mode      string
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
	TLSMode   string
}

type devEmailVerificationSender struct{}

func NewDevEmailVerificationSender() EmailVerificationSender {
	return devEmailVerificationSender{}
}

func (devEmailVerificationSender) SendEmailVerificationCode(email, code, expiresAt string) (string, error) {
	return "dev", nil
}

type disabledEmailVerificationSender struct{}

func NewDisabledEmailVerificationSender() EmailVerificationSender {
	return disabledEmailVerificationSender{}
}

func (disabledEmailVerificationSender) SendEmailVerificationCode(email, code, expiresAt string) (string, error) {
	return "", errEmailDeliveryNotConfigured
}

type smtpEmailVerificationSender struct {
	cfg EmailVerificationSenderConfig
}

func NewSMTPEmailVerificationSender(cfg EmailVerificationSenderConfig) EmailVerificationSender {
	return smtpEmailVerificationSender{cfg: cfg}
}

func (s smtpEmailVerificationSender) SendEmailVerificationCode(email, code, expiresAt string) (string, error) {
	host := strings.TrimSpace(s.cfg.Host)
	fromEmail := strings.TrimSpace(s.cfg.FromEmail)

	if host == "" || s.cfg.Port <= 0 || fromEmail == "" {
		return "", errEmailDeliveryNotConfigured
	}

	addr := fmt.Sprintf("%s:%d", host, s.cfg.Port)
	from := formatEmailAddress(s.cfg.FromName, fromEmail)

	subject := "Secure Voting: код подтверждения почты"
	body := buildEmailVerificationMessage(code, expiresAt)

	message := strings.Join([]string{
		"From: " + from,
		"To: " + email,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	auth := smtpAuth(host, s.cfg.Username, s.cfg.Password)

	switch strings.ToLower(strings.TrimSpace(s.cfg.TLSMode)) {
	case "tls":
		return "smtp", sendMailTLS(addr, host, auth, fromEmail, []string{email}, []byte(message))
	case "starttls", "":
		return "smtp", sendMailStartTLS(addr, host, auth, fromEmail, []string{email}, []byte(message))
	default:
		return "", errEmailDeliveryNotConfigured
	}
}

func smtpAuth(host, username, password string) smtp.Auth {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil
	}
	return smtp.PlainAuth("", username, password, host)
}

func formatEmailAddress(name, email string) string {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" {
		return email
	}
	escaped := strings.ReplaceAll(name, `"`, `'`)
	return fmt.Sprintf(`"%s" <%s>`, escaped, email)
}

func buildEmailVerificationMessage(code, expiresAt string) string {
	return strings.Join([]string{
		"Для подтверждения почты в Secure Voting введите следующий код:",
		"",
		code,
		"",
		"Срок действия кода: " + expiresAt,
		"",
		"Если вы не запрашивали подтверждение почты, просто проигнорируйте это письмо.",
	}, "\n")
}

func sendMailStartTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	if err := c.Hello("secure-voting.local"); err != nil {
		return err
	}

	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("smtp server does not support STARTTLS")
	}

	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
	}

	if err := c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err := c.Rcpt(addr); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	return c.Quit()
}

func sendMailTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.DialWithDialer(&net.Dialer{}, "tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
	}

	if err := c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err := c.Rcpt(addr); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	return c.Quit()
}
