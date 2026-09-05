DB_URL ?= postgres://resqio_user:resqio_password@localhost:5433/auth_db?sslmode=disable

.PHONY: help sqlc migrate-up migrate-down migrate-status run yolo-install yolo-run test build

help:
	@echo "Available commands:"
	@echo "  make sqlc          - Generate Go database models and queries via sqlc"
	@echo "  make migrate-up    - Run all pending goose migrations"
	@echo "  make migrate-down  - Rollback the most recent goose migration"
	@echo "  make migrate-status- Show migration status"
	@echo "  make run           - Run the auth server locally"
	@echo "  make yolo-install  - Install the local YOLO service dependencies"
	@echo "  make yolo-run      - Start YOLO issue image classification on port 8086"
	@echo "  make test          - Run all test suites"
	@echo "  make build         - Build server binary"

sqlc:
	export PATH="$$PATH:$$HOME/go/bin" && sqlc generate

migrate-up:
	export PATH="$$PATH:$$HOME/go/bin" && goose -dir db/migrations postgres "$(DB_URL)" up

migrate-down:
	export PATH="$$PATH:$$HOME/go/bin" && goose -dir db/migrations postgres "$(DB_URL)" down

migrate-status:
	export PATH="$$PATH:$$HOME/go/bin" && goose -dir db/migrations postgres "$(DB_URL)" status

run:
	go run main.go

yolo-install:
	python -m pip install torch torchvision --index-url https://download.pytorch.org/whl/cpu
	python -m pip install -r requirements.txt

yolo-run:
	python ml/yolo_service.py

test:
	go test -v ./...

build:
	go build -o bin/server main.go
