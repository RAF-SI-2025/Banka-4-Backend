package grpc

import (
	"context"
	"fmt"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
)

type EmailClient struct {
	client pb.EmailServiceClient
}

func NewEmailClient(conn *client.EmailServiceConn) *EmailClient {
	return &EmailClient{client: pb.NewEmailServiceClient(conn.ClientConn)}
}

func (c *EmailClient) Send(to, subject, body string) error {
	_, err := c.client.SendEmail(context.Background(), &pb.SendEmailRequest{
		To:      to,
		Subject: subject,
		Body:    body,
	})
	if err != nil {
		return fmt.Errorf("email client Send: %w", err)
	}
	return nil
}
