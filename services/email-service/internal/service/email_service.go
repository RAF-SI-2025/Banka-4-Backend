package service

import (
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/email-service/internal/config"
)

type Mailer interface {
	Send(to, subject, body string) error
}

type EmailService struct {
	cfg *config.Configuration
}

func NewEmailService(cfg *config.Configuration) Mailer {
	return &EmailService{cfg: cfg}
}

func (es *EmailService) Send(to, subject, body string) error {
	to = strings.TrimSpace(to)
	subject = strings.TrimSpace(subject)

	if to == "" || subject == "" || strings.TrimSpace(body) == "" {
		return fmt.Errorf("invalid email payload")
	}

	host := strings.TrimSpace(es.cfg.SMTP.Host)
	port := strings.TrimSpace(es.cfg.SMTP.Port)
	user := strings.TrimSpace(es.cfg.SMTP.User)
	pass := es.cfg.SMTP.Pass
	from := strings.TrimSpace(es.cfg.SMTP.From)

	if host == "" || port == "" || from == "" {
		return fmt.Errorf("smtp configuration is incomplete")
	}

	addr := net.JoinHostPort(host, port)
	var auth smtp.Auth
	if user != "" && pass != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}

	msg := strings.Join([]string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		`Content-Type: text/plain; charset="UTF-8"`,
		"",
		body,
	}, "\r\n")

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send failed: %w", err)
	}

	return nil
}
