.PHONY: serve sim watch

GOPATH := $(shell go env GOPATH)
export PATH := $(GOPATH)/bin:$(PATH)

# Start the web server and automatically kill any process using port 8085
serve:
	@echo "Cleaning up port 8085..."
	-fuser -k 8085/tcp 2>/dev/null || true
	@echo "Starting server on http://localhost:8085..."
	go run cmd/server/main.go

# Run the CLI simulation for default symbols
sim:
	go run cmd/cli/main.go --symbol=NIFTY24MAR2622700CE,NIFTY24MAR2622650PE

# Start development server with Hot Reload (requires 'air')
watch:
	@echo "Checking for 'air'..."
	@command -v air >/dev/null 2>&1 || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	@echo "Killing any process on port 8085..."
	-fuser -k 8085/tcp 2>/dev/null || true
	air -c .air.toml
