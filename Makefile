.PHONY: help setup fmt test vet typecheck check up down migrate migrate-down logs

GO_CACHE_DIR ?= /tmp/urock-go-build

help:
	@echo "setup         Install local dependencies"
	@echo "check         Run formatting checks, tests, vet, and TypeScript checks"
	@echo "up            Start MySQL, API, and Nginx"
	@echo "migrate       Apply all database migrations"
	@echo "migrate-down  Roll back one database migration"
	@echo "down          Stop local containers"

setup:
	cd backend && go mod download
	cd miniprogram && npm install
	cd admin-web && npm install

fmt:
	cd backend && test -z "$$(gofmt -l .)"

test:
	cd backend && GOCACHE=$(GO_CACHE_DIR) go test -race ./...

vet:
	cd backend && GOCACHE=$(GO_CACHE_DIR) go vet ./...

typecheck:
	cd miniprogram && npm run typecheck
	cd admin-web && npm run build

check: fmt test vet typecheck

up:
	docker compose up -d --build mysql api nginx

down:
	docker compose down

migrate:
	docker compose --profile tools run --rm migrate

migrate-down:
	docker compose --profile tools run --rm migrate -path=/migrations -database='mysql://$${MYSQL_USER}:$${MYSQL_PASSWORD}@tcp(mysql:3306)/$${MYSQL_DATABASE}' down 1

logs:
	docker compose logs -f api nginx
