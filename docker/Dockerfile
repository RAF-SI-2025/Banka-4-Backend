FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git tzdata

WORKDIR /app

COPY services/user-service/go.mod services/user-service/go.sum ./user-service/

COPY common ./common
COPY services/user-service ./user-service

WORKDIR /app/user-service
RUN go mod tidy

RUN go build -o user-service ./cmd/main.go

FROM alpine:3.18
RUN apk add --no-cache tzdata
WORKDIR /app

COPY --from=builder /app/user-service/user-service .

EXPOSE 8080
CMD ["./user-service"]