# PKG - Go Common Package Library

A comprehensive collection of reusable Go packages for building modern, scalable applications. This library provides standardized components for HTTP servers, database operations, logging, distributed tracing, alerting, and more.

## 🏗️ Architecture

This package collection follows Clean Architecture principles and provides building blocks for:

- **HTTP/API Servers** with comprehensive middleware
- **CQRS Pattern** implementation with command/query separation
- **Database Operations** with PostgreSQL/Bun ORM integration
- **Distributed Tracing** with OpenTelemetry
- **Structured Logging** with contextual metadata
- **Error Monitoring** with Sentinel integration
- **Configuration Management** with environment-based YAML configs

## 📦 Packages

### Core Infrastructure

#### `http/server`

HTTP server implementation built on Fiber framework with prioritized middleware system.

```go
server := server.NewHTTPServer(cfg, []server.Middleware{
    middleware.NewRecoveryMW(logger),           // Priority: 1000
    middleware.NewTracingMW(),                  // Priority: 900
    middleware.NewTimeoutMW(5 * time.Second),   // Priority: 800
    middleware.NewMetaInjectMW("api", "v1.0"),  // Priority: 700
    middleware.NewAlertingMW(logger, provider), // Priority: 600
    middleware.NewLoggerMW(logger),             // Priority: 500
    middleware.NewErrorHandlerMW(false),        // Priority: 400
})
```

**Available Middleware:**

- `RecoveryMW` - Panic recovery with graceful error handling
- `TracingMW` - OpenTelemetry distributed tracing
- `TimeoutMW` - Request timeout management
- `MetaInjectMW` - Request metadata injection (trace ID, user info, etc.)
- `AlertingMW` - Error alerting to monitoring systems
- `LoggerMW` - Structured request/response logging
- `ErrorHandlerMW` - Standardized error response formatting

#### `logger`

Structured logging interface built on Zap with contextual metadata support.

```go
log := logger.New(logger.Config{
    Level:    "info",
    Encoding: "console", // or "json" for production
})

// Context-aware logging with metadata
log.WithContext(ctx).Info("user created", "user_id", userID)
```

**Features:**

- Multiple log levels (debug, info, warn, error, fatal)
- Console mode with colors and pretty-printed JSON
- JSON mode for production log processing
- Context metadata extraction and injection
- Structured field logging

#### `tracing`

OpenTelemetry distributed tracing setup and configuration.

```go
shutdown, err := tracing.InitGlobalTracer(tracing.Config{
    ExporterHost: "localhost",
    ExporterPort: 4317,
    SampleRate:   1.0,
    Tags: map[string]string{
        "environment": "production",
    },
}, "my-service", "v1.0.0")
defer shutdown()
```

#### `alert`

Error alerting system with Sentinel integration for monitoring internal errors.

```go
provider, err := alert.NewSentinelProvider(cfg, "my-service", "v1.0.0")
err = provider.SendError(ctx, "DB_ERROR", "Connection failed", "user_login", details)
```

### Data Layer

#### `pg`

PostgreSQL integration with Bun ORM, connection pooling, and observability hooks.

```go
db, err := pg.NewBunDB(pg.Config{
    Host:     "localhost",
    Port:     5432,
    Database: "myapp",
    Username: "user",
    Password: "pass",
    Debug:    true,
})
```

**Features:**

- Connection pooling with configurable limits
- Automatic query logging in debug mode
- OpenTelemetry tracing integration
- PostgreSQL error handling utilities
- Model with automatic timestamp tracking

#### `repogen`

Generic repository pattern implementation with PostgreSQL backend.

```go
// Read-only repository
userRepo := repogen.NewPgReadOnlyRepo[User, UserFilters](
    db, "user", "USER_NOT_FOUND", applyUserFilters,
)

// Full CRUD repository
userRepo := repogen.NewPgRepo[User, UserFilters](
    db, "user", "USER_NOT_FOUND", "USER_CONFLICT", applyUserFilters,
)

// Usage
users, err := userRepo.List(ctx, UserFilters{Active: true})
user, err := userRepo.Create(ctx, &newUser)
```

**Repository Methods:**

- `Get`, `List`, `Count`, `FirstOrNil`, `Exists` (read operations)
- `Create`, `Update`, `Delete` (write operations)
- `BulkCreate`, `BulkUpdate`, `BulkDelete` (bulk operations)

### Business Logic

#### `cqrs`

Command Query Responsibility Segregation pattern with middleware support.

```go
// Command handler
type CreateUserCommand struct {
    Email string
    Name  string
}

func (c *CreateUserHandler) Execute(ctx context.Context, input CreateUserCommand) (*User, error) {
    // Business logic here
}

// With tracing wrapper
wrappedCmd := wrapper.NewTracingCommandWrapper[CreateUserCommand, *User]()(handler)
```

**Features:**

- Separate command and query interfaces
- Generic type-safe handlers
- Middleware wrappers (tracing, logging, metrics)
- Automatic span creation and error recording

#### `meta`

Request metadata management for context propagation.

```go
// Inject metadata into context
metadata := map[meta.ContextKey]string{
    meta.TraceID:      "trace-123",
    meta.RequestUserID: "user-456",
    meta.IPAddress:    "192.168.1.1",
}
ctx = meta.InjectMetaToContext(ctx, metadata)

// Extract metadata from context
metadata = meta.ExtractMetaFromContext(ctx)
```

**Available Metadata Keys:**

- `TraceID`, `RequestUserID`, `RequestUserType`, `RequestUserRole`
- `IPAddress`, `UserAgent`, `RemoteAddr`, `Referer`
- `ServiceName`, `ServiceVersion`
- Custom client headers (`XClientAppName`, `XClientAppOS`, etc.)

### Utilities

#### `cfgloader`

Environment-based configuration loading with validation and defaults.

```go
type Config struct {
    Host     string `yaml:"host" validate:"required"`
    Port     int    `yaml:"port" default:"8080"`
    LogLevel string `yaml:"log_level" default:"info"`
}

cfg := cfgloader.MustLoad[Config]() // Loads from config/{ENVIRONMENT}.yaml
```

**Features:**

- Environment-based config files (`dev.yaml`, `prod.yaml`, etc.)
- Struct tag validation with `go-playground/validator`
- Default value injection with `creasty/defaults`
- Environment variable substitution in YAML files

#### `sorter`

Query parameter sorting utilities for APIs.

```go
// Parse from query string
sortOpts := sorter.MakeFromStr("name:asc,created_at:desc", "name", "created_at", "status")

// Convert to SQL
for _, opt := range sortOpts {
    orderBy += opt.ToSQL() // "name asc", "created_at desc"
}
```

## 🚀 Quick Start

### 1. HTTP Server with Middleware

```go
package main

import (
    "github.com/code19m/pkg/http/server"
    "github.com/code19m/pkg/http/server/middleware"
    "github.com/code19m/pkg/logger"
    "github.com/gofiber/fiber/v2"
)

func main() {
    log := logger.New(logger.Config{Level: "info"})

    httpServer := server.NewHTTPServer(server.Config{
        Port: 8080,
    }, []server.Middleware{
        middleware.NewRecoveryMW(log),
        middleware.NewLoggerMW(log),
        middleware.NewMetaInjectMW("api", "v1.0.0"),
    })

    httpServer.RegisterRouter(func(r fiber.Router) {
        r.Get("/health", func(c *fiber.Ctx) error {
            return c.JSON(fiber.Map{"status": "ok"})
        })
    })

    log.Fatal(httpServer.Start())
}
```

### 2. Repository with Database

```go
package main

import (
    "context"
    "github.com/code19m/pkg/pg"
    "github.com/code19m/pkg/repogen"
)

type User struct {
    ID    int64  `bun:"id,pk,autoincrement"`
    Email string `bun:"email,notnull"`
    Name  string `bun:"name,notnull"`
    pg.Model // Adds created_at, updated_at
}

type UserFilters struct {
    Email string
}

func main() {
    db, _ := pg.NewBunDB(pg.Config{...})

    userRepo := repogen.NewPgRepo[User, UserFilters](
        db, "user", "USER_NOT_FOUND", "USER_CONFLICT",
        func(q *bun.SelectQuery, f UserFilters) *bun.SelectQuery {
            if f.Email != "" {
                q = q.Where("email = ?", f.Email)
            }
            return q
        },
    )

    user, err := userRepo.Create(context.Background(), &User{
        Email: "user@example.com",
        Name:  "John Doe",
    })
}
```

### 3. CQRS Command Handler

```go
package main

import (
    "context"
    "github.com/code19m/pkg/cqrs/command"
    "github.com/code19m/pkg/cqrs/command/wrapper"
)

type CreateUserInput struct {
    Email string
    Name  string
}

type CreateUserHandler struct {
    userRepo UserRepository
}

func (h *CreateUserHandler) Execute(ctx context.Context, input CreateUserInput) (*User, error) {
    user := &User{Email: input.Email, Name: input.Name}
    return h.userRepo.Create(ctx, user)
}

func main() {
    handler := &CreateUserHandler{userRepo: userRepo}

    // Add tracing wrapper
    tracedHandler := wrapper.NewTracingCommandWrapper[CreateUserInput, *User]()(handler)

    user, err := tracedHandler.Execute(context.Background(), CreateUserInput{
        Email: "user@example.com",
        Name:  "John Doe",
    })
}
```

## 🔧 Configuration

### Environment Setup

Set the `ENVIRONMENT` variable to load the appropriate config:

```bash
export ENVIRONMENT=dev     # Loads config/dev.yaml
export ENVIRONMENT=prod    # Loads config/prod.yaml
```

### Typical Configuration Structure

```
config/
├── dev.yaml
├── prod.yaml
└── local.yaml
```

Example `config/dev.yaml`:

```yaml
logger:
  level: debug
  encoding: console

database:
  host: localhost
  port: 5432
  database: myapp_dev
  username: ${DB_USER}
  password: ${DB_PASS}

tracing:
  disable: false
  exporter_host: localhost
  exporter_port: 4317
  sample_rate: 1.0

alert:
  disable: false
  sentinel_host: localhost
  sentinel_port: 9090
```

## 📋 Requirements

- **Go**: 1.23.3+
- **PostgreSQL**: 12+ (for database operations)
- **OpenTelemetry Collector**: (for tracing)
- **Sentinel Service**: (for error alerting)

## 🧪 Testing

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run linting
make lint
```

## 📚 Dependencies

### Core Dependencies

- **Fiber v2** - HTTP web framework
- **Zap** - Structured logging
- **Bun** - PostgreSQL ORM
- **OpenTelemetry** - Distributed tracing
- **Validator v10** - Struct validation
- **errx** - Enhanced error handling

### Development

- **golangci-lint** - Comprehensive linting
- **testify** - Testing assertions

## 🤝 Contributing

1. Follow the existing code patterns and architecture
2. Add tests for new functionality
3. Run linting before submitting: `make lint`
4. Ensure all tests pass: `go test ./...`

## 📄 License

This package collection is designed for internal use and follows the project's coding standards and architectural patterns.

---

For detailed package documentation, see the individual package directories and their respective README files.
