package client

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
)

type UserServiceClient interface {
	GetClientById(ctx context.Context, id uint64) (*pb.GetClientByIdResponse, error)
	GetClientByIdentityId(ctx context.Context, identityId uint64) (*pb.GetClientByIdResponse, error)
	GetEmployeeById(ctx context.Context, id uint64) (*pb.GetEmployeeByIdResponse, error)
	GetEmployeeByIdentityId(ctx context.Context, identityId uint64) (*pb.GetEmployeeByIdResponse, error)
	GetAllClients(ctx context.Context, page, pageSize int32, firstName, lastName string) (*pb.GetAllClientsResponse, error)
	GetAllActuaries(ctx context.Context, page, pageSize int32, firstName, lastName string) (*pb.GetAllActuariesResponse, error)
	GetIdentityByUserId(ctx context.Context, userID uint64, userType string) (*pb.GetIdentityByUserIdResponse, error)
	IncrementUsedLimit(ctx context.Context, employeeID uint64, amount float64) (*pb.IncrementUsedLimitResponse, error)
}
