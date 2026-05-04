package grpc

import (
	"context"
	"fmt"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
)

type UserServiceClient struct {
	stub pb.UserServiceClient
}

func NewUserServiceClient(conn *client.UserConn) *UserServiceClient {
	return &UserServiceClient{stub: pb.NewUserServiceClient(conn.ClientConn)}
}

func (c *UserServiceClient) GetClientById(ctx context.Context, id uint64) (*pb.GetClientByIdResponse, error) {
	resp, err := c.stub.GetClientById(ctx, &pb.GetClientByIdRequest{Id: id})
	if err != nil {
		return nil, fmt.Errorf("user client GetClientById: %w", err)
	}
	return resp, nil
}

func (c *UserServiceClient) GetEmployeeById(ctx context.Context, id uint64) (*pb.GetEmployeeByIdResponse, error) {
	resp, err := c.stub.GetEmployeeById(ctx, &pb.GetEmployeeByIdRequest{Id: id})
	if err != nil {
		return nil, fmt.Errorf("user client GetEmployeeById: %w", err)
	}
	return resp, nil
}

func (c *UserServiceClient) GetClientByIdentityId(ctx context.Context, identityId uint64) (*pb.GetClientByIdResponse, error) {
	resp, err := c.stub.GetClientByIdentityId(ctx, &pb.GetClientByIdentityIdRequest{IdentityId: identityId})
	if err != nil {
		return nil, fmt.Errorf("user client GetClientByIdentityId: %w", err)
	}
	return resp, nil
}

func (c *UserServiceClient) GetEmployeeByIdentityId(ctx context.Context, identityId uint64) (*pb.GetEmployeeByIdResponse, error) {
	resp, err := c.stub.GetEmployeeByIdentityId(ctx, &pb.GetEmployeeByIdentityIdRequest{IdentityId: identityId})
	if err != nil {
		return nil, fmt.Errorf("user client GetEmployeeByIdentityId: %w", err)
	}
	return resp, nil
}

func (c *UserServiceClient) GetAllClients(ctx context.Context, page, pageSize int32, firstName, lastName string) (*pb.GetAllClientsResponse, error) {
	resp, err := c.stub.GetAllClients(ctx, &pb.GetAllClientsRequest{
		Page: page, PageSize: pageSize, FirstName: firstName, LastName: lastName,
	})
	if err != nil {
		return nil, fmt.Errorf("user client GetAllClients: %w", err)
	}
	return resp, nil
}

func (c *UserServiceClient) GetAllActuaries(ctx context.Context, page, pageSize int32, firstName, lastName string) (*pb.GetAllActuariesResponse, error) {
	resp, err := c.stub.GetAllActuaries(ctx, &pb.GetAllActuariesRequest{
		Page: page, PageSize: pageSize, FirstName: firstName, LastName: lastName,
	})
	if err != nil {
		return nil, fmt.Errorf("user client GetAllActuaries: %w", err)
	}
	return resp, nil
}

func (c *UserServiceClient) GetIdentityByUserId(ctx context.Context, userID uint64, userType string) (*pb.GetIdentityByUserIdResponse, error) {
	resp, err := c.stub.GetIdentityByUserId(ctx, &pb.GetIdentityByUserIdRequest{
		UserId: userID, UserType: userType,
	})
	if err != nil {
		return nil, fmt.Errorf("user client GetIdentityByUserId: %w", err)
	}
	return resp, nil
}

func (c *UserServiceClient) IncrementUsedLimit(ctx context.Context, employeeID uint64, amount float64) (*pb.IncrementUsedLimitResponse, error) {
	resp, err := c.stub.IncrementUsedLimit(ctx, &pb.IncrementUsedLimitRequest{
		EmployeeId: employeeID,
		Amount:     amount,
	})
	if err != nil {
		return nil, fmt.Errorf("user client IncrementUsedLimit: %w", err)
	}
	return resp, nil
}
