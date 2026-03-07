# Architecture Guide

## Project Structure

This is a Go monorepo using [Go Workspaces](https://go.dev/doc/tutorial/workspaces) to manage multiple microservices and a shared `common` module.

```
Banka-4-Backend/
├── go.work                         # Go workspace — lists all modules
├── go.work.sum
├── common/                         # Shared library used by all services
│   ├── go.mod
│   └── pkg/
│       ├── db/                     # GORM database connection
│       │   └── db.go
│       ├── errors/                 # Structured error handling
│       │   └── errors.go
│       └── logging/                # Structured logging + Gin middleware
│           └── logging.go
└── services/
    └── user-service/               # Each microservice lives here
        ├── go.mod
        ├── cmd/
        │   └── main.go             # Entry point
        └── internal/
            ├── config/             # Environment-based configuration
            │   └── config.go
            ├── handler/            # HTTP handlers (controllers)
            │   └── health_handler.go
            └── server/             # Server setup, routing, lifecycle
                └── rest.go
```

### Adding a new service

1. Create `services/<service-name>/`.
2. Inside it, follow the same layout: `cmd/main.go`, `internal/config`, `internal/handler`, `internal/server`.
3. Run `go mod init <service-name>` inside the service directory.
4. Add the service to `go.work`:

```go
use (
    common
    services/user-service
    services/<service-name>
)
```

---

## Layered Architecture (per service)

Each service follows a layered architecture with clear separation of concerns. The layers communicate top-down — a handler calls a service, a service calls a repository. Never skip layers.

```
HTTP Request
    │
    ▼
┌──────────┐
│ Handler  │  Parses request, validates input, calls service, writes response
└────┬─────┘
     │
     ▼
┌──────────┐
│ Service  │  Business logic, orchestrates repositories, returns domain errors
└────┬─────┘
     │
     ▼
┌──────────┐
│   Repo   │  Database access, translates between DB records and domain models
└──────────┘
```

### Directory layout for a fully built service

```
services/user-service/internal/
├── config/
│   └── config.go
├── handler/
│   ├── health_handler.go
│   └── user_handler.go
├── service/
│   └── user_service.go
├── repository/
│   └── user_repository.go
├── model/
│   └── user.go
├── dto/
│   ├── user_request.go
│   └── user_response.go
└── server/
    └── rest.go
```

---

## Dependency Injection

Services use [Uber Fx](https://uber-go.github.io/fx/) for dependency injection. All components are wired through constructors (`NewXxx` functions) provided to `fx.Provide`.

```go
// cmd/main.go
func main() {
    fx.New(
        fx.Provide(
            config.Load,
            func(cfg *config.Configuration) (*gorm.DB, error) {
                return db.New(cfg.DB.DSN())
            },
            handler.NewHealthHandler,
            handler.NewUserHandler,
            service.NewUserService,
            repository.NewUserRepository,
        ),
        fx.Invoke(func(cfg *config.Configuration) error {
            return logging.Init(cfg.Env)
        }),
        fx.Invoke(func(_ *gorm.DB) {}),
        fx.Invoke(server.NewServer),
    ).Run()
}
```

Every constructor declares its dependencies as function parameters. Fx resolves the dependency graph automatically:

```go
func NewUserHandler(service *service.UserService) *UserHandler {
    return &UserHandler{service: service}
}

func NewUserService(repo *repository.UserRepository) *UserService {
    return &UserService{repo: repo}
}

func NewUserRepository(db *gorm.DB) *UserRepository {
    return &UserRepository{db: db}
}
```

When adding a new component, just write the constructor and add it to `fx.Provide`.

---

## Error Handling

All API errors go through the `common/pkg/errors` package. It provides `AppError`, a structured error type with an HTTP status code, client-facing message, and timestamp.

### Using existing constructors

**4xx errors** take a `message string` — the caller decides what the client sees:

```go
errors.BadRequestErr("email is required")
errors.UnauthorizedErr("invalid credentials")
errors.ForbiddenErr("insufficient permissions")
errors.NotFoundErr("user not found")
errors.MethodNotAllowedErr("GET not supported")
errors.ConflictErr("email already registered")
errors.UnprocessableEntityErr("cannot transfer to yourself")
errors.RateLimitErr("too many login attempts")
```

**5xx errors** take an `err error` — the message is always generic so internal details are never leaked:

```go
errors.InternalErr(err)
errors.ServiceUnavailableErr(err)
errors.GatewayTimeoutErr(err)
```

### Creating custom errors

For status codes not covered by the existing constructors, use `NewAppError`:

```go
errors.NewAppError(http.StatusPaymentRequired, "insufficient funds", nil)
```

### How errors flow

1. A service or handler creates an `AppError`.
2. The handler attaches it to the Gin context with `c.Error(...)`.
3. The `ErrorHandler` middleware (registered in `InitRouter`) catches it after `c.Next()` and writes the JSON response.

**In a handler:**

```go
func (h *UserHandler) GetUser(c *gin.Context) {
    user, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
    if err != nil {
        c.Error(err)
        return
    }

    c.JSON(http.StatusOK, dto.ToUserResponse(user))
}
```

**In a service:**

```go
func (s *UserService) GetByID(ctx context.Context, id string) (*model.User, error) {
    user, err := s.repo.FindByID(ctx, id)
    if err != nil {
        return nil, errors.InternalErr(err)
    }
    if user == nil {
        return nil, errors.NotFoundErr("user not found")
    }
    return user, nil
}
```

The handler doesn't need to know which HTTP status code to use — the service already encoded that into the `AppError`. The handler just forwards it with `c.Error(err)`.

---

## Models

Models represent domain entities. They live in `internal/model/` and map to PostgreSQL tables via [GORM](https://gorm.io/) struct tags.

```go
// internal/model/user.go
package model

import "time"

type User struct {
    ID        uint      `gorm:"primaryKey"`
    Email     string    `gorm:"uniqueIndex;not null"`
    Password  string    `gorm:"not null"`
    FirstName string    `gorm:"not null"`
    LastName  string    `gorm:"not null"`
    Accounts  []Account `gorm:"foreignKey:UserID"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

GORM conventions:
- `CreatedAt` and `UpdatedAt` are auto-managed by GORM — no need for manual timestamps.
- Use `gorm:"primaryKey"` for the primary key. GORM auto-increments `uint` IDs.
- Use `gorm:"uniqueIndex"` for unique constraints.
- Use `gorm:"foreignKey:UserID"` to define relationships (like JPA's `@OneToMany`).
- Relations are **not loaded by default**. Use `Preload("Accounts")` at query time to load them (see [Repositories](#repositories)).

Models should only contain data, GORM tags, and basic methods directly related to the entity. No HTTP concerns (`json` tags), no business logic.

---

## DTOs (Data Transfer Objects)

DTOs define the shape of HTTP request and response bodies. They live in `internal/dto/` and are separate from models so that internal fields (passwords, internal IDs) are never accidentally exposed.

**Request DTO** — what the client sends. Use `binding` tags for validation:

```go
// internal/dto/user_request.go
package dto

type CreateUserRequest struct {
    Email     string `json:"email"     binding:"required,email"`
    Password  string `json:"password"  binding:"required,min=8"`
    FirstName string `json:"firstName" binding:"required"`
    LastName  string `json:"lastName"  binding:"required"`
}
```

**Response DTO** — what the client receives. Never includes sensitive fields:

```go
// internal/dto/user_response.go
package dto

import (
    "user-service/internal/model"
    "time"
)

type UserResponse struct {
    ID        uint      `json:"id"`
    Email     string    `json:"email"`
    FirstName string    `json:"firstName"`
    LastName  string    `json:"lastName"`
    CreatedAt time.Time `json:"createdAt"`
}

func ToUserResponse(u *model.User) *UserResponse {
    return &UserResponse{
        ID:        u.ID,
        Email:     u.Email,
        FirstName: u.FirstName,
        LastName:  u.LastName,
        CreatedAt: u.CreatedAt,
    }
}
```

Conversion functions (`ToXxxResponse`) live alongside the response DTO.

---

## Handlers

Handlers are the HTTP layer. They parse requests, validate input via DTO binding tags, delegate to the service layer, and write responses. They live in `internal/handler/`.

```go
// internal/handler/user_handler.go
package handler

import (
    "net/http"

    "common/pkg/errors"
    "user-service/internal/dto"
    "user-service/internal/service"

    "github.com/gin-gonic/gin"
)

type UserHandler struct {
    service *service.UserService
}

func NewUserHandler(service *service.UserService) *UserHandler {
    return &UserHandler{service: service}
}

func (h *UserHandler) Create(c *gin.Context) {
    var req dto.CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.Error(errors.BadRequestErr(err.Error()))
        return
    }

    user, err := h.service.Create(c.Request.Context(), &req)
    if err != nil {
        c.Error(err)
        return
    }

    c.JSON(http.StatusCreated, dto.ToUserResponse(user))
}
```

Rules for handlers:
- No business logic — delegate to the service.
- Validate input with `ShouldBindJSON` (uses the `binding` tags on the DTO).
- On error, call `c.Error(err)` and `return`. The middleware handles the response.
- On success, call `c.JSON(...)` with a response DTO.

---

## Services

Services contain business logic. They orchestrate repositories, enforce rules, and return `AppError` when something goes wrong. They live in `internal/service/`.

```go
// internal/service/user_service.go
package service

import (
    "context"

    "common/pkg/errors"
    "user-service/internal/dto"
    "user-service/internal/model"
    "user-service/internal/repository"
)

type UserService struct {
    repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
    return &UserService{repo: repo}
}

func (s *UserService) Create(ctx context.Context, req *dto.CreateUserRequest) (*model.User, error) {
    existing, err := s.repo.FindByEmail(ctx, req.Email)
    if err != nil {
        return nil, errors.InternalErr(err)
    }
    if existing != nil {
        return nil, errors.ConflictErr("email already registered")
    }

    user := &model.User{
        Email:     req.Email,
        Password:  req.Password, // hash this
        FirstName: req.FirstName,
        LastName:  req.LastName,
    }

    if err := s.repo.Save(ctx, user); err != nil {
        return nil, errors.InternalErr(err)
    }

    return user, nil
}
```

Rules for services:
- Accept `context.Context` as the first parameter.
- Accept DTOs or primitives as input, return models.
- Wrap repository errors with `errors.InternalErr(err)`.
- Use 4xx constructors for domain-level validation failures.
- Never import `gin` — services are HTTP-agnostic.

---

## Repositories

Repositories handle database access using [GORM](https://gorm.io/). They live in `internal/repository/`.

```go
// internal/repository/user_repository.go
package repository

import (
    "context"

    "user-service/internal/model"

    "gorm.io/gorm"
)

type UserRepository struct {
    db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
    return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(ctx context.Context, id uint) (*model.User, error) {
    var user model.User
    result := r.db.WithContext(ctx).First(&user, id)
    if errors.Is(result.Error, gorm.ErrRecordNotFound) {
        return nil, nil
    }
    return &user, result.Error
}

func (r *UserRepository) FindByIDWithAccounts(ctx context.Context, id uint) (*model.User, error) {
    var user model.User
    result := r.db.WithContext(ctx).Preload("Accounts").First(&user, id)
    if errors.Is(result.Error, gorm.ErrRecordNotFound) {
        return nil, nil
    }
    return &user, result.Error
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
    var user model.User
    result := r.db.WithContext(ctx).Where("email = ?", email).First(&user)
    if errors.Is(result.Error, gorm.ErrRecordNotFound) {
        return nil, nil
    }
    return &user, result.Error
}

func (r *UserRepository) Save(ctx context.Context, user *model.User) error {
    return r.db.WithContext(ctx).Create(user).Error
}
```

### Preloading relations

GORM does not load relations by default. Use `Preload` to eagerly load them at query time:

```go
// Just the user — no relations loaded
r.db.First(&user, id)

// User + their accounts
r.db.Preload("Accounts").First(&user, id)

// User + accounts + cards
r.db.Preload("Accounts").Preload("Cards").First(&user, id)

// Accounts with a filter
r.db.Preload("Accounts", "status = ?", "active").First(&user, id)

// Nested relations — accounts and each account's transactions
r.db.Preload("Accounts.Transactions").First(&user, id)
```

Create separate repository methods for different preload needs (e.g. `FindByID`, `FindByIDWithAccounts`, `FindByIDFull`) so the service layer picks the right one per use case.

Rules for repositories:
- Always use `.WithContext(ctx)` for cancellation/timeout propagation.
- Work with models, not DTOs.
- Return `nil, nil` when a record is not found — the service layer decides whether that's an error.
- Return raw errors for everything else — the service layer wraps them in `AppError`.
- One repository per table.

---

## Routing

Routes are registered in `internal/server/rest.go` inside `SetupRoutes`. Group routes by handler:

```go
func SetupRoutes(r *gin.Engine, healthHandler *handler.HealthHandler, userHandler *handler.UserHandler) {
    r.GET("/health", healthHandler.Health)

    users := r.Group("/users")
    {
        users.POST("/", userHandler.Create)
        users.GET("/:id", userHandler.GetByID)
    }
}
```

When adding a new handler, also add it as a parameter to `SetupRoutes` and to `NewServer`, and register it in `fx.Provide` in `main.go`.

---

## Middleware

Middleware is registered in `InitRouter` in `internal/server/rest.go`. The current chain:

1. `gin.Recovery()` — catches panics and returns 500.
2. `logging.Logger()` — logs every request (method, path, status, duration, IP).
3. `errors.ErrorHandler()` — catches `AppError` from `c.Error()` and writes the JSON response.

The order matters — recovery is first so panics in other middleware are caught. The error handler is last so it can process errors from all handlers.

---

## Configuration

Configuration is loaded from environment variables. The `.env` file is automatically loaded via [godotenv](https://github.com/joho/godotenv) at the start of `Load()`. Defined in `internal/config/config.go`:

| Variable  | Required | Default       | Purpose                       |
|-----------|----------|---------------|-------------------------------|
| `ENV`     | No       | `development` | Logger mode (production = JSON) |
| `PORT`    | No       | `8080`        | HTTP server port              |
| `DB_HOST` | Yes      |               | PostgreSQL host               |
| `DB_PORT` | Yes      |               | PostgreSQL port               |
| `DB_USER` | Yes      |               | PostgreSQL user               |
| `DB_PASS` | Yes      |               | PostgreSQL password           |
| `DB_NAME` | Yes      |               | PostgreSQL database name      |

There are two helpers for reading env vars:
- `GetOrDefault(key, fallback)` — returns the fallback if the variable is not set.
- `GetOrThrow(key)` — kills the process with `log.Fatalf` if the variable is not set. Use this for required config like database credentials.

Each service has a `.env` file (gitignored) for local development:

```
ENV=development
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASS=postgres
DB_NAME=banka4
```

To add a new config value, add a field to `Configuration` (or a nested struct) and load it in `Load()`:

```go
type DBConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    DBName   string
}

func (c *DBConfig) DSN() string {
    return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        c.Host, c.Port, c.User, c.Password, c.DBName)
}

type Configuration struct {
    Env  string
    Port string
    DB   DBConfig
}

func Load() *Configuration {
    _ = godotenv.Load()

    return &Configuration{
        Env:  GetOrDefault("ENV", "development"),
        Port: GetOrDefault("PORT", "8080"),
        DB: DBConfig{
            Host:     GetOrThrow("DB_HOST"),
            Port:     GetOrThrow("DB_PORT"),
            User:     GetOrThrow("DB_USER"),
            Password: GetOrThrow("DB_PASS"),
            DBName:   GetOrThrow("DB_NAME"),
        },
    }
}
```
---

## Common Module

The `common/` module contains packages shared across all services. Currently:

| Package          | Purpose                           |
|------------------|-----------------------------------|
| `pkg/db`         | GORM PostgreSQL connection (`db.New(dsn)`) |
| `pkg/errors`     | `AppError` type + constructors + Gin error middleware |
| `pkg/logging`    | Zap logger init + Gin request logging middleware |

To add a new shared package, create `common/pkg/<name>/` and import it from any service as `common/pkg/<name>`. The Go workspace resolves it automatically.
