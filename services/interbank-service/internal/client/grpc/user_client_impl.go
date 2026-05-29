package grpc

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/client"
)

type userClient struct {
	c pb.UserServiceClient
}

func NewUserClient(conn *client.UserServiceConn) client.UserClient {
	return &userClient{c: pb.NewUserServiceClient(conn.ClientConn)}
}

func (u *userClient) GetClientByID(ctx context.Context, id uint64) (*pb.GetClientByIdResponse, error) {
	return u.c.GetClientById(ctx, &pb.GetClientByIdRequest{Id: id})
}
