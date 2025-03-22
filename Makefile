# Load environment variables from .env if the file exists
include .env
export $(shell sed 's/=.*//' .env)

APP_NAME = user-service
MIGRATION_DIR = db/migrations
DOCKER_REGISTRY ?= local
TAG ?= latest

.PHONY: all build migrate-up migrate-down sqlc-gen run run-binary docker-build docker-push docker-run-postgres docker-run-app docker-run docker-compose-dev docker-compose-prod clean

all: build migrate-up sqlc-gen run

# Build the Go binary
build:
	@echo "🔨 Building the Go binary..."
	go build -o bin/$(APP_NAME) main.go
	@echo "✅ Build completed: bin/$(APP_NAME)"

# Run the built binary
run-binary:
	@echo "🚀 Running the built binary..."
	./bin/$(APP_NAME)

migrate-up:
	@echo "📥 Running database migrations (up)..."
	goose -dir ${MIGRATION_DIR} postgres "${DB_DSN}" up
	@echo "✅ Migrations applied successfully!"

migrate-down:
	@echo "📤 Reverting database migrations (down)..."
	goose -dir ${MIGRATION_DIR} postgres "${DB_DSN}" down
	@echo "✅ Migrations reverted successfully!"

sqlc-gen:
	@echo "📜 Generating SQLC code..."
	sqlc generate
	@echo "✅ SQLC code generation completed!"

run:
	@echo "🚀 Running the application..."
	go run main.go

# Docker targets
docker-build:
	@echo "🐳 Building Docker image: $(DOCKER_REGISTRY)/$(APP_NAME):$(TAG)..."
	docker build -t $(DOCKER_REGISTRY)/$(APP_NAME):$(TAG) .
	@echo "✅ Docker image built successfully!"

docker-push:
	@echo "📤 Pushing Docker image to registry..."
	docker push $(DOCKER_REGISTRY)/$(APP_NAME):$(TAG)
	@echo "✅ Docker image pushed successfully!"

docker-run-postgres:
	@echo "🐘 Starting PostgreSQL container..."
	docker run --name user-service-postgres \
		-p 5432:5432 \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=Saadsaad1 \
		-e POSTGRES_DB=gomicro \
		-d postgres:15-alpine
	@echo "✅ PostgreSQL container started!"

docker-run-app:
	@echo "🚀 Running the application in a Docker container..."
	docker run --name $(APP_NAME) \
		-p 8080:8080 \
		--network host \
		-e DB_DSN="postgres://postgres:Saadsaad1@localhost:5432/gomicro?sslmode=disable" \
		-d $(DOCKER_REGISTRY)/$(APP_NAME):$(TAG)
	@echo "✅ Application container started!"

docker-run: docker-run-postgres docker-run-app

# Docker Compose targets
docker-compose-dev:
	@echo "🚀 Starting services with docker-compose (dev)..."
	docker-compose -f docker-compose.dev.yml up -d
	@echo "✅ Dev environment started!"

docker-compose-prod:
	@echo "🚀 Starting services with docker-compose (prod)..."
	docker-compose -f docker-compose.prod.yml up -d
	@echo "✅ Production environment started!"

# Clean up
clean:
	@echo "🗑️ Stopping and removing containers..."
	docker stop $(APP_NAME) user-service-postgres || true
	docker rm $(APP_NAME) user-service-postgres || true
	@echo "🗑️ Removing Docker image..."
	docker rmi $(DOCKER_REGISTRY)/$(APP_NAME):$(TAG) || true
	@echo "🗑️ Removing built binary..."
	rm -rf bin/$(APP_NAME)
	@echo "✅ Cleanup completed!"
