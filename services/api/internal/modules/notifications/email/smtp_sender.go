package email

import (
	"context"
	"fmt"
	"net/smtp"
)

// SMTPConfig holds SMTP connection settings.
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Pass     string
	FromName string
	FromAddr string
}

// SMTPSender delivers email via net/smtp using plain auth.
type SMTPSender struct {
	Host         string
	Port         string
	User         string
	Pass         string
	FromName     string
	FromAddress  string
}

// NewSMTPSender creates an SMTPSender from config.
func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	return &SMTPSender{
		Host:        cfg.Host,
		Port:        cfg.Port,
		User:        cfg.User,
		Pass:        cfg.Pass,
		FromName:    cfg.FromName,
		FromAddress: cfg.FromAddr,
	}
}

// Send builds a multipart/alternative MIME message and dispatches it.
func (s *SMTPSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	addr := s.Host + ":" + s.Port
	auth := smtp.PlainAuth("", s.User, s.Pass, s.Host)

	from := fmt.Sprintf("%s <%s>", s.FromName, s.FromAddress)
	boundary := "ivyticketing-notification-boundary"

	msg := []byte(fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n"+
			"--%s\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n"+
			"%s\r\n\r\n"+
			"--%s\r\n"+
			"Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n"+
			"%s\r\n\r\n"+
			"--%s--\r\n",
		from, to, subject, boundary,
		boundary, textBody,
		boundary, htmlBody,
		boundary,
	))

	return smtp.SendMail(addr, auth, s.FromAddress, []string{to}, msg)
}
