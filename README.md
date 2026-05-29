# Banka-4-Backend

# Usage

This project uses a `Makefile` for common development tasks.

## Docker

```bash
make docker-up-build    # Build and start services using docker-compose-dev.yml
make docker-up          # Start services
make docker-down        # Stop services
make docker-down-rm-vol # Stop services and remove volumes
```

## Formatting

```bash
make format             # Format all Go files
```

## Swagger

Generate Swagger documentation for all services.

```bash
make swagger-docs
```

## Protobuf

Generate Go and gRPC code from protobuf definitions. This uses a pinned Docker image so all developers and CI use the same `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc` versions.

```bash
make proto
```

Check that generated protobuf files are up to date:

```bash
make proto-check
```

## Testing

```bash
make test               # Run unit tests
make test-integration   # Run integration tests
```

## Coverage

Coverage excludes infrastructure/bootstrap packages such as:
`cmd`, `docs`, `config`, `seed`, `server`, `logging`, `db`, `pb`,
`middleware`, `job`, `grpc`, and `client`.

```bash
make coverage-profile   # Generate coverage profile
make coverage           # Show total statement coverage
make coverage-report    # Show coverage grouped by layer
make coverage-html      # Generate HTML coverage report
```
