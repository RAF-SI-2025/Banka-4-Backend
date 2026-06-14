package service

import (
	"strings"
	"testing"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/email-service/internal/config"
)

func TestEmailServiceSendRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	mailer := NewEmailService(&config.Configuration{
		SMTP: config.SMTPConfig{
			Host: "smtp.example.com",
			Port: "25",
			From: "noreply@example.com",
		},
	})

	tests := []struct {
		name    string
		to      string
		subject string
		body    string
	}{
		{name: "empty recipient", to: " ", subject: "Subject", body: "Body"},
		{name: "empty subject", to: "user@example.com", subject: " ", body: "Body"},
		{name: "empty body", to: "user@example.com", subject: "Subject", body: " \t"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := mailer.Send(tt.to, tt.subject, tt.body)
			if err == nil || !strings.Contains(err.Error(), "invalid email payload") {
				t.Fatalf("expected invalid payload error, got %v", err)
			}
		})
	}
}

func TestEmailServiceSendRejectsIncompleteSMTPConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  config.SMTPConfig
	}{
		{name: "missing host", cfg: config.SMTPConfig{Port: "25", From: "noreply@example.com"}},
		{name: "missing port", cfg: config.SMTPConfig{Host: "smtp.example.com", From: "noreply@example.com"}},
		{name: "missing sender", cfg: config.SMTPConfig{Host: "smtp.example.com", Port: "25"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mailer := NewEmailService(&config.Configuration{SMTP: tt.cfg})
			err := mailer.Send(" user@example.com ", " Subject ", "Body")
			if err == nil || !strings.Contains(err.Error(), "smtp configuration is incomplete") {
				t.Fatalf("expected smtp configuration error, got %v", err)
			}
		})
	}
}
