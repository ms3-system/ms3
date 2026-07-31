.PHONY: help build run test test-race coverage vet lint fmt tidy check clean inspect-db install-tools

BINARY_NAME := metadata-service
BUILD_DIR   := bin
CMD_DIR     := ./cmd/server
DB_PATH     := data/metadata.db

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

run:
	go run $(CMD_DIR)

test:
	go test ./... -v

test-race:
	go test ./... -v -race

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -l -w .

tidy:
	go mod tidy

check: vet lint test

clean:
	rm -rf $(BUILD_DIR)
	rm -f $(DB_PATH)

inspect-db:
	bbolt buckets $(DB_PATH)

install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install go.etcd.io/bbolt/cmd/bbolt@latest
