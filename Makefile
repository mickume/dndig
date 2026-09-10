.DEFAULT_GOAL := help

BIN     ?= $(HOME)/go/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -X github.com/mickume/dndig/internal/cli.version=$(VERSION)

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*##"}; {printf "  %-14s %s\n", $$1, $$2}'

.PHONY: build
build: ## Build ./bin/dndig
	go build -ldflags "$(LDFLAGS)" -o bin/dndig ./cmd/dndig

.PHONY: install
install: ## Install dndig into $(BIN)
	go build -ldflags "$(LDFLAGS)" -o $(BIN)/dndig ./cmd/dndig

.PHONY: test
test: ## Run all tests (offline, no API key needed)
	go test ./...

.PHONY: fmt
fmt: ## gofmt all source files in place
	gofmt -l -w .

.PHONY: vet
vet: ## go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint if installed, otherwise skip
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed; skipping (see https://golangci-lint.run)"; \
	fi

.PHONY: check
check: fmt vet lint test ## Run fmt, vet, lint and test - use before committing

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy

.PHONY: kit-bump
kit-bump: ## Point the AgentKit replace at a pseudo-version: make kit-bump REV=v0.0.0-<date>-<sha12>
	@test -n "$(REV)" || (echo "usage: make kit-bump REV=v0.0.0-yyyymmddhhmmss-<sha12>  (see docs/adr/01)"; exit 2)
	go mod edit -replace github.com/agentfox/agentkit-go=github.com/agent-fox-dev/coder@$(REV)
	GOPROXY=direct go mod tidy
