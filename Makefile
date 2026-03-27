.PHONY: build run dev frontend all clean deps install-services uninstall-services

# Build the Go router
build:
	go build -o bin/polymr ./cmd/polymr

# Run the Go router
run: build
	./bin/polymr

# Run Go router in dev mode with hot reload (requires air)
dev:
	air -c .air.toml || go run ./cmd/polymr

# Run the Next.js frontend
frontend:
	cd web && npm run dev

# Run both router and frontend
all:
	@echo "Starting Polymr..."
	@make run &
	@make frontend

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf web/.next/

# Install dependencies
deps:
	go mod tidy
	cd web && npm install

# Install background services (macOS launchd or Linux systemd)
install-services: build
	@bash scripts/install-services.sh

# Uninstall background services
uninstall-services:
	@bash scripts/uninstall-services.sh
