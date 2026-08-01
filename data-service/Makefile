.PHONY: help build run test test-race coverage vet lint fmt tidy check clean install-tools

BINARY_NAME := data-service
BUILD_DIR   := bin
CMD_DIR     := ./cmd/server
DATA_DIR    := data

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Build the server binary into bin/
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

run: ## Run the server locally (go run, no binary produced)
	go run $(CMD_DIR)

test: ## Run all unit tests
	go test ./... -v

test-race: ## Run all unit tests with the race detector
	go test ./... -v -race

coverage: ## Run tests and print a per-function coverage report
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format all Go source files in place
	gofmt -l -w .

tidy: ## Sync go.mod/go.sum with actual imports
	go mod tidy

check: vet lint test ## The pre-commit/pre-push gate — run before every commit

clean: ## Remove build artifacts and the local dev data directory
	rm -rf $(BUILD_DIR)
	rm -rf $(DATA_DIR)

install-tools: ## Install golangci-lint
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
