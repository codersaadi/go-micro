# Go Microservice Template

A lightweight, production-ready microservice template for Go applications with built-in support for common middleware, database connectivity, logging, metrics, and more.

## Features

- 🔒 **Security-focused**: CORS configuration, security headers, rate limiting
- 📊 **Observability**: Structured logging with Zap, Prometheus metrics
- 🔄 **Middleware stack**: Request ID, logging, metrics, recovery, timeout, etc.
- 🛣️ **Routing**: Built on Gorilla Mux with an improved API
- 💾 **Database**: PostgreSQL integration with pgx
- ⚡ **Performance**: Rate limiting and configurable timeouts
- 🏗️ **Clean Architecture**: Clear separation of concerns (handlers, services, repositories)
- 🧪 **Health Checks**: Built-in health check endpoint

## TODO Features

- 🔐 **Authentication**: User registration and login with bcrypt password hashing
- 🔑 **OIDC Support**: Integration with OpenID Connect providers
- 📧 **Email Service**: Templated email sending capabilities
- ⏰ **Job Scheduling**: Background task processing and scheduling
- 📱 **Push Notifications**: Mobile and web push notification support
- 🔍 **Full-text Search**: Integration with search engines
- 🌐 **Internationalization**: Multi-language support
- 📄 **API Documentation**: Auto-generated API docs with Swagger/OpenAPI
- 🧩 **Plugin System**: Extensible architecture for custom plugins
- 🔄 **Event Bus**: Internal publish/subscribe messaging

## Getting Started
```env
git clone github.com/codersaadi/go-micro.git 
cd ./go-micro
```

### Prerequisites

- Go 1.18 or newer
- PostgreSQL database
- Make (for running Makefile commands)

### Environment Setup

Create a `.env` file in the root directory:

```env
APP_NAME=user-service
PORT=8080
LOG_LEVEL=info
DB_DSN=postgres://postgres:Saadsaad1@localhost:5432/gomicro?sslmode=disable
```

### Database Migrations

Run migrations to set up your database schema:

```bash
make migrate-up
```

### Running the Application

Start the application with:

```bash
make run
```

Or use Docker:

```bash
make docker-build
make docker-run
```

## Project Structure

```
├── cmd/                  # Application entry points
├── db/                   # Database migrations and connection
│   └── migrations/       # SQL migration files
├── internal/             # Private application code
│   ├── handler/          # HTTP handlers
│   ├── models/           # Data models and database queries
│   ├── repository/       # Data access layer
│   └── service/          # Business logic layer
├── pkg/                  # Public libraries
│   └── micro/            # Micro template components
└── main.go               # Application entry point
```

## Core Components

### Micro App

The core `App` struct provides all the necessary functionality:

```go
app, err := micro.NewApp(cfg)
if err != nil {
    panic("Failed to create application: " + err.Error())
}

// Register routes
app.POST("/register", micro.Handler(userHandler.Register))
app.GET("/users/{id}", micro.Handler(userHandler.GetUser))

// Start the server
if err := app.Start(); err != nil {
    app.Logger.Error("Server failed to start", zap.Error(err))
}
```

### Middleware

The template includes several built-in middleware components:
- Request ID generation
- Logging
- Metrics collection
- Rate limiting
- Security headers
- Timeout handling
- Recovery (panic handling)
- CORS support

### Error Handling

Structured API error handling:

```go
func (h *UserHandler) GetUser(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
    userID, err := h.app.URLParamInt(r, "id")
    if err != nil {
        return micro.NewAPIError(http.StatusBadRequest, "invalid user ID")
    }
    
    // Business logic...
    
    if someError != nil {
        return micro.NewAPIError(http.StatusNotFound, "user not found")
    }
    
    return h.app.JSON(w, http.StatusOK, user)
}
```

## Configuration

The template can be configured through environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| APP_NAME | Application name | "micro-service" |
| PORT | HTTP server port | 8080 |
| LOG_LEVEL | Log level (debug, info, warn, error) | "info" |
| DB_DSN | Database connection string | Required |
| READ_TIMEOUT | HTTP read timeout | "5s" |
| WRITE_TIMEOUT | HTTP write timeout | "10s" |
| METRICS_ENABLED | Enable Prometheus metrics | true |
| HANDLER_TIMEOUT | Request timeout | "30s" |
| CORS_ENABLED | Enable CORS | true |
| CORS_ALLOWED_ORIGINS | Allowed origins | "*" |
| CORS_ALLOWED_METHODS | Allowed HTTP methods | "GET,POST,PUT,DELETE,OPTIONS,HEAD" |
| CORS_ALLOWED_HEADERS | Allowed headers | "Content-Type,Authorization,X-Requested-With" |

## Docker Support

The template includes Docker and docker-compose support:

```bash
# Run with Docker
make docker-build
make docker-run

# Run with Docker Compose for development
make docker-compose-dev

# Run with Docker Compose for production
make docker-compose-prod
```

## Development Commands

```bash
# Run database migrations up
make migrate-up

# Run database migrations down
make migrate-down

# Generate SQL code (requires sqlc)
make sqlc-gen

# Run the application
make run

# Clean up Docker resources
make clean
```

## Contributing

Contributions are welcome! Whether it's bug fixes, feature additions, or documentation improvements, feel free to fork the repository and submit a pull request.

## About

Developed by [Saad Bukhari](https://github.com/codersaadi)

## License

[MIT License](LICENSE)
