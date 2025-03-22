# Load environment variables from .env if the file exists
include .env
export $(shell sed 's/=.*//' .env)

APP_NAME = user-service
MIGRATION_DIR = db/migrations
DOCKER_REGISTRY ?= local
TAG ?= latest

.PHONY: all migrate-up migrate-down sqlc-gen run docker-build docker-push docker-run-postgres docker-run-app docker-run docker-compose-dev docker-compose-prod

all: migrate-up sqlc-gen run

migrate-up:
	goose -dir ${MIGRATION_DIR} postgres "${DB_DSN}" up

migrate-down:
	goose -dir ${MIGRATION_DIR} postgres "${DB_DSN}" down

sqlc-gen:
	sqlc generate

run:
	go run main.go

# Docker targets
docker-build:
	docker build -t $(DOCKER_REGISTRY)/$(APP_NAME):$(TAG) .

docker-push:
	docker push $(DOCKER_REGISTRY)/$(APP_NAME):$(TAG)

docker-run-postgres:
	docker run --name user-service-postgres \
		-p 5432:5432 \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=Saadsaad1 \
		-e POSTGRES_DB=gomicro \
		-d postgres:15-alpine

docker-run-app:
	docker run --name $(APP_NAME) \
		-p 8080:8080 \
		--network host \
		-e DB_DSN="postgres://postgres:Saadsaad1@localhost:5432/gomicro?sslmode=disable" \
		-d $(DOCKER_REGISTRY)/$(APP_NAME):$(TAG)

docker-run: docker-run-postgres docker-run-app

# Docker Compose targets
docker-compose-dev:
	docker-compose -f docker-compose.dev.yml up -d

docker-compose-prod:
	docker-compose -f docker-compose.prod.yml up -d

# Clean up
clean:
	docker stop $(APP_NAME) user-service-postgres || true
	docker rm $(APP_NAME) user-service-postgres || true
	docker rmi $(DOCKER_REGISTRY)/$(APP_NAME):$(TAG) || true