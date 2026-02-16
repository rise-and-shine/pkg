# PKG - Go Common Package Library

A comprehensive collection of reusable Go packages for building modern, scalable applications. This library provides standardized components for HTTP servers, database operations, logging, distributed tracing, alerting, and more.

## 🏗️ Architecture

This package collection follows Clean Architecture principles and provides building blocks for:

- **HTTP/API Servers** with comprehensive middleware
- **Database Operations** with PostgreSQL/Bun ORM integration
- **Distributed Tracing** with OpenTelemetry
- **Structured Logging** with contextual metadata
- **Error Monitoring** with Discord/Telegram alerting
- **Configuration Management** with environment-based YAML configs
- **and more...**

## 📋 Requirements

- **Go**: 1.23.3+
- **PostgreSQL**: 12+ (for database operations)
- **OpenTelemetry Collector**: (for tracing)
- **Discord/Telegram Bot**: (for error alerting)

## 🧪 Testing

```bash
# Run linting
make lint

# Run tests
make test
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
