.PHONY: run

# Shared local-dev secrets — auth-service and api-service must agree on
# AUTH_SERVICE_INTERNAL_TOKEN / API_SERVICE_INTERNAL_TOKEN. Override any of
# these on the command line, e.g. `make run AUTH_SERVICE_JWT_SECRET=...`.
AUTH_SERVICE_JWT_SECRET     ?= <GENERATE_ME_openssl_rand_hex_32>
AUTH_SERVICE_INTERNAL_TOKEN ?= <GENERATE_ME_openssl_rand_hex_32>
AUTH_SERVICE_MASTER_KEY     ?= 4/NI5ZVXCJjvLHiTVfPB1NSPThY5ESxTBhXJiE1Y8mk=
API_SERVICE_INTERNAL_TOKEN  ?= $(AUTH_SERVICE_INTERNAL_TOKEN)

run: ## Start all four services locally in the background; Ctrl+C stops all of them
	@trap 'kill 0' EXIT INT TERM; \
	(cd metadata-service && go run ./cmd/server) & \
	(cd data-service && go run ./cmd/server) & \
	(cd auth-service && \
		AUTH_SERVICE_JWT_SECRET=$(AUTH_SERVICE_JWT_SECRET) \
		AUTH_SERVICE_INTERNAL_TOKEN=$(AUTH_SERVICE_INTERNAL_TOKEN) \
		AUTH_SERVICE_MASTER_KEY=$(AUTH_SERVICE_MASTER_KEY) \
		go run ./cmd/server) & \
	sleep 2; \
	(cd api-service && \
		API_SERVICE_INTERNAL_TOKEN=$(API_SERVICE_INTERNAL_TOKEN) \
		go run ./cmd/server) & \
	wait
