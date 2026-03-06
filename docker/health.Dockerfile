FROM golang:1.26 AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source
COPY . .

# Build binary
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o health ./cmd/health


FROM golang:1.26

WORKDIR /

COPY --from=builder /app/health /health

EXPOSE 50051

CMD ["/health"]
