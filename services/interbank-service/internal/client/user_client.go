package client

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
)

// UserClient wraps the user-service gRPC surface used by interbank-service.
type UserClient interface {
	GetClientByID(ctx context.Context, id uint64) (*pb.GetClientByIdResponse, error)
}
