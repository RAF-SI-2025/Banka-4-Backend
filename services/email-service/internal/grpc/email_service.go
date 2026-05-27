package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/email-service/internal/service"
)

type EmailService struct {
	pb.UnimplementedEmailServiceServer
	mailer service.Mailer
}

func NewEmailService(mailer service.Mailer) *EmailService {
	return &EmailService{mailer: mailer}
}

func (s *EmailService) SendEmail(ctx context.Context, req *pb.SendEmailRequest) (*pb.SendEmailResponse, error) {
	if err := s.mailer.Send(req.GetTo(), req.GetSubject(), req.GetBody()); err != nil {
		return nil, status.Errorf(codes.Internal, "send email: %v", err)
	}

	return &pb.SendEmailResponse{}, nil
}
