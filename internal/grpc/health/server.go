package health

import (
    "context"

    pb "github.com/RAF-SI-2025/Banka-4-Backend/proto/health"
    healthSvc "github.com/RAF-SI-2025/Banka-4-Backend/internal/services/health"
)

type Server struct {
    pb.UnimplementedHealthServiceServer
    service *healthSvc.Service
}

func New(svc *healthSvc.Service) *Server {
	return &Server{service: svc}
}

func (s *Server) Check(ctx context.Context, req *pb.HealthRequest,) (*pb.HealthResponse, error) {
	status := s.service.Check()
	return &pb.HealthResponse{Status: status}, nil
}
