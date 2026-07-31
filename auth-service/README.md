# auth-service

Identity for ms3: user accounts, JWT session auth, and S3 access-key/secret-key issuance.

See [`docs/design.md`](docs/design.md) for the full design and [`docs/openapi.yml`](docs/openapi.yml)
for the API surface.

## Quickstart

```bash
export AUTH_SERVICE_JWT_SECRET=dev-secret-change-me
export AUTH_SERVICE_MASTER_KEY=$(openssl rand -base64 32)
export AUTH_SERVICE_INTERNAL_TOKEN=dev-internal-token-change-me

make run
```

## Development

```bash
make help   # list all targets
make check  # vet + lint + test — run before every commit
```
