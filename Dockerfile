# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache git ca-certificates tzdata && \
    update-ca-certificates

# Install dependencies first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /go/bin/app ./cmd/server.go

# Runtime stage
FROM alpine:3.18

# Add non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Import from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/bin/app /app

# Use non-root user
USER appuser

# Default environment variables
ENV PORT=8080 \
    APP_NAME="user-service" \
    LOG_LEVEL="info" \
    METRICS_ENABLED="true" \
    CORS_ENABLED="true" \
    CORS_ALLOWED_ORIGINS="https://yourdomain.com,https://app.yourdomain.com" \
    CORS_ALLOWED_METHODS="GET,POST,PUT,DELETE,OPTIONS" \
    CORS_ALLOWED_HEADERS="Content-Type,Authorization,X-API-Key" \
    CORS_EXPOSED_HEADERS="X-Request-ID,X-Rate-Limit-Remaining" \
    CORS_ALLOW_CREDENTIALS="true" \
    CORS_MAX_AGE="600" \
    RATE_LIMITER_ENABLED="true" \
    RATE_LIMITER_REQUESTS_PER_SECOND="10" \
    RATE_LIMITER_BURST="20" \
    RATE_LIMITER_TTL="3600s" \
    RATE_LIMITER_STRATEGY="ip"

# Expose ports
EXPOSE 8080

# Run the application
ENTRYPOINT ["/app"]