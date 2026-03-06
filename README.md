# Banka-4-Backend

## Početna struktura projekta
.
├── api
│   └── swagger
├── cmd
│   ├── gateway
│   │   └── main.go
│   └── health
│       └── main.go
├── docker
│   ├── gateway.Dockerfile
│   └── health.Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
├── internal
│   ├── clients
│   │   └── health
│   │       └── client.go
│   ├── grpc
│   │   └── health
│   │       └── server.go
│   ├── http
│   │   └── handlers
│   │       └── health.go
│   └── services
│       └── health
│           └── service.go
├── Makefile
├── proto
│   └── health
│       ├── health_grpc.pb.go
│       ├── health.pb.go
│       └── health.proto
└── README.md

- Implementiran je health servis za probu i primer.
- Nakon promene proto fajlova: `make proto`
- Podizanje svih servisa: `make docker-up`
- Gašenje svih servisa: `make docker-down`

- Proverite da li radi sve na localhost:8080/health
