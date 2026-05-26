# syntax=docker/dockerfile:1.7
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git tzdata ca-certificates

ARG SERVICE

WORKDIR /src

# Copy workspace and module manifests first for better cache reuse.
COPY go.work go.work.sum ./
COPY common/go.mod common/go.sum ./common/
COPY services/user-service/go.mod services/user-service/go.sum ./services/user-service/
COPY services/banking-service/go.mod services/banking-service/go.sum ./services/banking-service/
COPY services/trading-service/go.mod services/trading-service/go.sum ./services/trading-service/
COPY services/interbank-service/go.mod services/interbank-service/go.sum ./services/interbank-service/
COPY services/email-service/go.mod services/email-service/go.sum ./services/email-service/

WORKDIR /src/services/${SERVICE}
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy only required source modules.
WORKDIR /src
COPY common ./common
COPY services/${SERVICE} ./services/${SERVICE}

WORKDIR /src/services/${SERVICE}
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 go build -o /out/${SERVICE} ./cmd/main.go

FROM alpine:3.18
RUN apk add --no-cache tzdata ca-certificates curl
WORKDIR /app

ARG SERVICE
COPY --from=builder /out/${SERVICE} /app/service

EXPOSE 8080
ENTRYPOINT ["/app/service"]
