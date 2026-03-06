package main

import (
    "log"
    "net"
    "os"

    "google.golang.org/grpc"

    pb "github.com/RAF-SI-2025/Banka-4-Backend/proto/health"
    "github.com/RAF-SI-2025/Banka-4-Backend/internal/grpc/health"

    healthSvc "github.com/RAF-SI-2025/Banka-4-Backend/internal/services/health"
)

func main() {
	port := os.Getenv("GRPC_PORT")

	lis, err := net.Listen("tcp", ":" + port)
	if err != nil {
		log.Fatal(err)
	}

    healthService := healthSvc.NewService()

    grpcServer := grpc.NewServer()

    healthServer := health.New(healthService)
    pb.RegisterHealthServiceServer(grpcServer, healthServer)

    log.Println("health service listening on :" + port)
    grpcServer.Serve(lis)
}
