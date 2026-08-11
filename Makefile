.PHONY: help db-up db-down migrate migrate-tool proto control-plane agent cli web dev tidy install \
        pm2-start pm2-stop pm2-restart pm2-reload pm2-status pm2-logs pm2-save pm2-delete

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
	@echo "  pm2-start       Start the stack under pm2 (needs a built tree + .env)"
	@echo "  pm2-restart     Restart pm2 processes, re-reading .env"
	@echo "  pm2-status      Show pm2 process status"
	@echo "  pm2-logs        Tail pm2 logs"
	@echo "  pm2-save        Persist the pm2 process list for boot"

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

# --- pm2 -------------------------------------------------------------------
# Run ./install/install.sh first: it builds the binaries and the standalone web
# bundle and writes the .env that ecosystem.config.js reads.

pm2-start:
	@test -f .env || { echo "no .env; run ./install/install.sh first"; exit 1; }
	@test -x control-plane/bin/control-plane || { echo "no control-plane binary; run make control-plane"; exit 1; }
	pm2 start ecosystem.config.js

pm2-stop:
	pm2 stop ecosystem.config.js

# --update-env re-reads .env; plain restart would keep the old values.
pm2-restart:
	pm2 restart ecosystem.config.js --update-env

# Zero-downtime where the process supports it; falls back to restart in fork mode.
pm2-reload:
	pm2 reload ecosystem.config.js --update-env

pm2-status:
	pm2 status

pm2-logs:
	pm2 logs --lines 100

pm2-save:
	pm2 save
	@echo "run 'pm2 startup' once (as printed) to resurrect on boot"

pm2-delete:
	pm2 delete ecosystem.config.js
