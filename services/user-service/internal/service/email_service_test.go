package service

import (
	"net"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/config"
)

func TestEmailServiceSend(t *testing.T) {
	conn, err := net.DialTimeout("tcp", "localhost:1025", time.Second)
	if err != nil {
		t.Skip("MailDev not available; start Docker to run this test")
	}
	_ = conn.Close()

	// Configuration for MailDev (running in docker-compose-dev.yml)
	cfg := &config.Configuration{
		SMTP: config.SMTPConfig{
			Host: "localhost",
			Port: "1025",
			User: "test@example.com",
			Pass: "",
			From: "test@example.com",
		},
	}

	service := NewEmailService(cfg)

	// Test sending an email
	err = service.Send(
		"recipient@example.com",
		"Test Email Subject",
		"This is a test email body from the email service.",
	)

	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	t.Log("Email sent successfully! Check MailDev at http://localhost:1080")
}

// TestEmailServiceSendWithValidation tests email validation
func TestEmailServiceSendWithValidation(t *testing.T) {
	conn, err := net.DialTimeout("tcp", "localhost:1025", time.Second)
	if err != nil {
		t.Skip("MailDev not available; start Docker to run this test")
	}
	_ = conn.Close()

	cfg := &config.Configuration{
		SMTP: config.SMTPConfig{
			Host: "localhost",
			Port: "1025",
			User: "test@example.com",
			Pass: "",
			From: "test@example.com",
		},
	}

	service := NewEmailService(cfg)

	tests := []struct {
		name    string
		to      string
		subject string
		body    string
		wantErr bool
	}{
		{
			name:    "Valid email",
			to:      "user@example.com",
			subject: "Test",
			body:    "Test body",
			wantErr: false,
		},
		{
			name:    "Empty recipient",
			to:      "",
			subject: "Test",
			body:    "Test body",
			wantErr: true,
		},
		{
			name:    "Empty subject",
			to:      "user@example.com",
			subject: "",
			body:    "Test body",
			wantErr: true,
		},
		{
			name:    "Empty body",
			to:      "user@example.com",
			subject: "Test",
			body:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.Send(tt.to, tt.subject, tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
