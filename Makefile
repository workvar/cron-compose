.PHONY: help db-up db-down migrate migrate-tool proto control-plane agent cli web dev tidy install

help:
	@echo "Targets:"
	@echo "  db-up           Start Postgres in docker-compose"
	@echo "  db-down         Stop Postgres"
	@echo "  migrate         Apply SQL migrations to local Postgres"
	@echo "  proto           Regenerate Go code from proto/agent.proto"
	@echo "  control-plane   Build the control-plane binary"
	@echo "  agent           Build the agent binary"
	@echo "  web             Install web deps and run Next.js dev server"
	@echo "  dev             db-up + control-plane + web (separate shells expected)"
	@echo "  tidy            go mod tidy in each Go module"

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

migrate:
	@for f in migrations/*.sql; do \
		echo "applying $$f"; \
		PGPASSWORD=croncompose psql -h localhost -U croncompose -d croncompose -f $$f || exit 1; \
	done

proto:
	cd proto && protoc \
		--go_out=. --go_opt=module=github.com/croncompose/croncompose/proto \
		--go-grpc_out=. --go-grpc_opt=module=github.com/croncompose/croncompose/proto \
		agent.proto

control-plane:
	cd control-plane && go build -o bin/control-plane ./cmd/server

agent:
	cd agent && go build -o bin/agent ./cmd/agent

cli:
	cd cli && go build -o bin/cc ./cmd/cc

# Cross-platform migration runner used by the installer (no psql required).
migrate-tool:
	cd control-plane && go build -o bin/migrate ./cmd/migrate

# Interactive, from-source install of the whole control plane (Linux/macOS).
install:
	./install/install.sh

web:
	cd web && npm install && npm run dev

tidy:
	cd control-plane && go mod tidy
	cd agent && go mod tidy
	cd cli && go mod tidy
