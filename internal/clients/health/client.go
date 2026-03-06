package health

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/RAF-SI-2025/Banka-4-Backend/proto/health"
)

type Client struct {
    pb.HealthServiceClient
}

func New(addr string) (*Client, error) {
    conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return nil, err
    }

    return &Client{
        pb.NewHealthServiceClient(conn),
    }, nil
}
