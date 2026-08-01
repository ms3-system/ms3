# auth-service — Design Document

`auth-service` owns identity for ms3: user accounts, JWT session auth for future human-facing
clients, and S3 access-key/secret-key issuance for SigV4 request signing. It follows
`metadata-service`'s conventions exactly (bbolt store, layered `store → repository/bolt →
repository → service → api`, no DI framework) — read that service first if anything here is
under-explained, since this doc only calls out where `auth-service` differs.

---

## 1. Architecture

```mermaid
graph TB
    Client[Client / CLI / aws-cli]
    API[api-service]
    Auth[auth-service]
    Store[(bbolt file<br/>auth.db<br/>single process only)]

    Client -->|"REST: login, register, issue keys"| API
    API -->|"REST: /v1/auth/*, /v1/users/*, /v1/access-keys/*"| Auth
    API -->|"REST, internal-token gated:<br/>GET /internal/credentials/{access_key}"| Auth
    Auth --> Store

    subgraph auth-service
        Handlers[api: chi handlers]
        Svc["service: bcrypt, JWT, AES-GCM"]
        Repo[repository/bolt: bolt transactions]
        Handlers --> Svc --> Repo
    end
    Repo --> Store
```

`auth-service` never calls another service — same leaf-service constraint `metadata-service`
follows. It is on the hot path of every signed S3 request (via `api-service`'s SigV4
verification), so §9 below on the internal lookup endpoint is the most consequential part of
this document.

---

## 2. Responsibilities

- User account management: register, fetch by ID.
- Password authentication for a future console/human client (bcrypt + JWT), not consumed by
  anything today.
- S3 access-key / secret-key issuance, soft-revocation, and decrypted lookup for SigV4
  verification — this is the part `api-service`'s future signature-verification work depends on.

`auth-service` does **not** authorize actions (bucket ownership, policy) — that's
`api-service` + `metadata-service`'s job once it has resolved a caller's identity through here.

---

## 3. Domain Model

```go
type User struct {
    ID           string     `json:"id"`
    Username     string     `json:"username"`
    PasswordHash string     `json:"password_hash"` // bcrypt, cost 10
    IsAdmin      bool       `json:"is_admin"`
    CreatedAt    time.Time  `json:"created_at"`
}

type Credential struct {
    AccessKey          string     `json:"access_key"`           // primary key, e.g. AKIAJH3K9QZX7YMN2P4R
    UserID              string     `json:"user_id"`
    SecretKeyEncrypted string     `json:"secret_key_encrypted"` // base64(nonce || AES-GCM ciphertext)
    CreatedAt           time.Time  `json:"created_at"`
    RevokedAt            *time.Time `json:"revoked_at,omitempty"` // soft-revoke, same pattern as metadata-service's DeletedAt
}
```

Both are JSON-encoded as bolt values, exactly like `model.Bucket` / `model.Object` in
`metadata-service`.

---

## 4. bbolt Keyspace Design

Four bolt buckets (namespaces) inside the single `auth.db` file:

```mermaid
graph LR
subgraph "auth.store (one bbolt file)"
U[bolt bucket: users<br/>id → User JSON]
UN[bolt bucket: usernames<br/>username → id]
C[bolt bucket: credentials<br/>access_key → Credential JSON]
Meta[bolt bucket: meta<br/>schema_version → int]
end
UN -. resolves to .-> U
```

| Bolt bucket | Key                      | Value                            | Purpose                                                                                     |
|-------------|--------------------------|-----------------------------------|-----------------------------------------------------------------------------------------------|
| `users`       | `<user-id>` (UUID)        | JSON-encoded `model.User`         | Primary record, keyed by immutable ID — same shape as `metadata-service`'s `buckets` bucket |
| `usernames`   | `<username>` (raw string) | `<user-id>`                       | Username → ID lookup **and** the uniqueness constraint, same role as `bucket_names`         |
| `credentials` | `<access_key>` (string)   | JSON-encoded `model.Credential`   | Primary record, keyed by the access key itself — it's already a globally unique natural ID  |
| `meta`        | `schema_version`          | integer (as bytes)                | Schema version marker, identical mechanism to `metadata-service`                             |

### Why no composite keys / secondary index for credentials

`metadata-service` needs `bucket_owner_index` and a `0x00`-joined `objects` key because it has
two access patterns per entity: get-by-name *and* prefix-scan (list-by-owner, list-by-prefix).
`auth-service`'s v1 API has exactly one lookup per entity — get-user-by-id, get-user-by-username,
get-credential-by-access-key — so every bolt bucket here is a flat `key → value` map with no
scans. If a future endpoint needs "list credentials for user X", add a
`credential_user_index` bolt bucket keyed `<user-id>0x00<access_key> → access_key`, built the
same way as `bucket_owner_index`; there's no reason to build it now for an endpoint that doesn't
exist (see `docs/decisions/` for whether/when that lands).

Every key-encoding helper still runs through `ValidateKeyComponent` (rejects embedded `0x00`
bytes) even though no key here is composite today — a username or access key that somehow
contained a null byte would be a bug worth surfacing at the boundary either way, and it keeps
`repository/bolt` uniform with `metadata-service` if a composite key is added later.

### Transactions

Same pattern as `metadata-service` §5: every write that touches more than one bolt bucket runs
inside a single `db.Update(...)`.

**Register user** — one transaction:
1. `usernames.Get(username)` — if found, `ErrAlreadyExists` (transaction aborts)
2. `users.Put(id, json(user))`
3. `usernames.Put(username, id)`

**Issue credential** — one transaction (access key collisions are astronomically unlikely at 16
random base32 chars, but checked anyway, the same defensive habit as bucket-name uniqueness):
1. `credentials.Get(accessKey)` — if found, regenerate and retry (bounded retries) rather than
   fail the request
2. `credentials.Put(accessKey, json(credential))`

**Revoke credential** — one transaction:
1. `credentials.Get(accessKey)` — not found → `ErrNotFound`; already revoked → `ErrNotFound`
   (revoked keys behave as gone, same as soft-deleted objects in `metadata-service`)
2. Set `RevokedAt`, `credentials.Put(accessKey, json(credential))`

---

## 5. Password Authentication

- `golang.org/x/crypto/bcrypt`, cost `bcrypt.DefaultCost` (10).
- `POST /v1/users` registers a user with a bcrypt-hashed password; the plaintext password is
  never stored or logged.
- `POST /v1/auth/login` verifies with `bcrypt.CompareHashAndPassword`; on mismatch returns a
  generic 401 (never reveals whether the username or the password was wrong).

---

## 6. JWT Session Auth (console/human use — not consumed by anything yet)

- `github.com/golang-jwt/jwt/v5`, `HS256`, shared secret from env var `AUTH_SERVICE_JWT_SECRET`.
- Stateless — no server-side session store, no refresh-token revocation list in v1 (see
  `docs/decisions/` for the option to add one later; this mirrors the "skip it for v1, document
  as future work" call already made for this service).

| Token   | Lifetime | Claims                                                              |
|---------|----------|----------------------------------------------------------------------|
| Access  | 15m      | `sub` (user id), `username`, `is_admin`, `typ: "access"`, `iat`, `exp` |
| Refresh | 7d       | `sub` (user id), `typ: "refresh"`, `iat`, `exp`                       |

The refresh token deliberately carries no `username`/`is_admin` — if those change, the client
gets fresh values on the next access-token mint rather than a 7-day-stale copy. `typ` prevents an
access token being replayed as a refresh token or vice versa (both are otherwise structurally
valid JWTs signed with the same secret).

```mermaid
sequenceDiagram
    participant C as Client
    participant Api as api (handler)
    participant Svc as service
    participant Repo as repository

    C->>Api: POST /v1/auth/login {username, password}
    Api->>Svc: Login(username, password)
    Svc->>Repo: GetByUsername(username)
    Repo-->>Svc: model.User
    Svc->>Svc: bcrypt.CompareHashAndPassword
    Svc->>Svc: mint access (15m) + refresh (7d) JWTs
    Svc-->>Api: access_token, refresh_token
    Api-->>C: 200 {access_token, refresh_token}
```

```mermaid
sequenceDiagram
    participant C as Client
    participant Api as api (handler)
    participant Svc as service
    participant Repo as repository

    C->>Api: POST /v1/auth/refresh {refresh_token}
    Api->>Svc: Refresh(refreshToken)
    Svc->>Svc: parse + verify signature, exp, typ=="refresh"
    Svc->>Repo: GetByID(sub) — not found → invalid credentials
    Repo-->>Svc: model.User
    Svc->>Svc: mint new access token (15m) with current username/is_admin
    Svc-->>Api: access_token
    Api-->>C: 200 {access_token}
```

`Refresh` needs one repository round-trip: the refresh token's claims only carry `sub`
(deliberately, per above), so minting a new access token with a *current* `username`/`is_admin`
means re-fetching the user by ID. This is also what catches a user that no longer exists. There's
still no revocation state to check — the round-trip is purely to source fresh claim data, not to
validate the refresh token itself. It only re-mints the access token; the original refresh token
stays valid until it naturally expires.

---

## 7. Authorization on Public Endpoints

Everything under `/v1` except registration and the two auth endpoints requires a valid access
token, and requires the caller to be either the resource's own user or an admin (`is_admin: true`
on the token). This closes an obvious gap: without it, any unauthenticated caller could fetch any
user's record, mint S3 credentials for any user, or revoke anyone's access key.

| Endpoint | Auth required | Who's allowed |
|---|---|---|
| `POST /v1/users` | none | anyone (open registration) |
| `POST /v1/auth/login`, `POST /v1/auth/refresh` | none | anyone with valid credentials/a valid refresh token |
| `GET /v1/users/{id}` | `Authorization: Bearer <access token>` | that user, or an admin |
| `POST /v1/users/{id}/credentials` | `Authorization: Bearer <access token>` | that user, or an admin |
| `DELETE /v1/access-keys/{access_key}` | `Authorization: Bearer <access token>` | the credential's owner, or an admin |

Two-stage enforcement in `internal/api`:

1. `requireAuth` middleware (`internal/api/auth_middleware.go`) parses the `Bearer` token via
   `AuthService.VerifyAccessToken`, rejects with 401 if it's missing, malformed, expired, or not
   an access token (a refresh token cannot be used here — same `typ` check as §6), and stores the
   resulting `service.Principal{UserID, IsAdmin}` on the request context.
2. Each handler checks self-or-admin against the specific resource: `GET`/`POST .../credentials`
   compare the principal against the `{id}` path param directly; `DELETE /v1/access-keys/{access_key}`
   has no user ID in the path, so the handler resolves the credential's owner first
   (`CredentialService.GetCredentialOwner`, which never decrypts the secret) and compares against
   that. A mismatch is 403, not 404 — the caller already proved they hold *some* valid token, so
   there's no ambiguity to hide behind a 404 the way an unauthenticated caller might warrant.

This only governs `auth-service`'s own public surface. It's independent of `api-service`'s SigV4
verification (§8, §9), which authenticates S3 clients, not console/admin callers of this API.

---

## 8. S3 Credential Issuance (this is the part `api-service`'s SigV4 verification depends on)

### Access key format

`AKIA` + 16 random base32 characters (RFC 4648 alphabet, `A-Z2-7`), 20 characters total,
uppercase — the exact shape of a real AWS IAM access key ID (e.g. `AKIAIOSFODNN7EXAMPLE`).
Chosen deliberately so anything that pattern-matches "looks like an AWS access key" (regexes in
`api-service`'s future SigV4 parser, log scanners, etc.) works unmodified.

### Secret key format

30 random bytes from `crypto/rand`, standard base64-encoded → 40 characters, matching the shape
of a real AWS secret access key. Generated fresh per credential, never derived from the access
key.

### Encryption at rest

- AES-256-GCM, key from env var `AUTH_SERVICE_MASTER_KEY` (32 bytes, base64-encoded in the env
  var).
- Stored value is `base64(nonce || ciphertext)` in `Credential.SecretKeyEncrypted` — a fresh
  random 12-byte nonce per encryption, prepended so decryption doesn't need a separate nonce
  store.
- **No key rotation in v1** — one static master key, no key-version prefix on ciphertexts. If
  rotation is ever needed, decrypting existing records requires the original key; see
  `docs/decisions/0001-plaintext-recoverable-secrets-for-sigv4.md` for why this can't just be a
  one-way hash like the password, and for rotation as a documented future option.

### Issuance flow

```mermaid
sequenceDiagram
    participant C as Client (admin)
    participant Api as api (handler)
    participant Svc as service
    participant Repo as repository

    C->>Api: POST /v1/users/{id}/credentials
    Api->>Svc: IssueCredential(userID)
    Svc->>Repo: GetByID(userID)
    Repo-->>Svc: model.User (confirms user exists)
    Svc->>Svc: generate access_key (AKIA…), secret_key (random 30B)
    Svc->>Svc: AES-GCM encrypt secret_key with master key
    Svc->>Repo: Create(credential)
    Repo-->>Svc: ok
    Svc-->>Api: {access_key, secret_key} — plaintext secret, this call only
    Api-->>C: 201 {access_key, secret_key}
```

The plaintext `secret_key` is returned exactly once, in this response, and is never retrievable
again through any public endpoint — identical to AWS's own behavior. Only the encrypted form is
persisted.

### Revocation

`DELETE /v1/access-keys/{access_key}` soft-revokes: sets `RevokedAt`, leaves the record in place.
A revoked credential behaves as not-found for issuance/lookup purposes but the record (and its
audit trail — who had this key, when it was created/revoked) survives, same rationale as
`metadata-service`'s soft-deleted objects.

---

## 9. `GET /internal/credentials/{access_key}` — internal-only, decrypted lookup

This is the endpoint `api-service` calls on every signed request to verify a SigV4 signature: it
needs the actual secret to recompute the same HMAC the client used, not a hash of it (see the ADR
for the full reasoning). That means **this is the one place in the entire system that returns
decrypted secret material**, which makes it the highest-value target in the service.

```mermaid
sequenceDiagram
    participant C as Client
    participant Api as api-service
    participant Auth as auth-service (handler)
    participant Svc as service
    participant Repo as repository

    C->>Api: PUT /my-bucket/key (SigV4 signed)
    Api->>Auth: GET /internal/credentials/{access_key}<br/>X-Internal-Token: shared secret
    Auth->>Auth: middleware: reject if token missing/wrong (401)
    Auth->>Svc: LookupCredential(accessKey)
    Svc->>Repo: GetByAccessKey(accessKey)
    Repo-->>Svc: model.Credential (revoked → ErrNotFound)
    Svc->>Svc: AES-GCM decrypt secret_key_encrypted
    Svc-->>Auth: {user_id, secret_key}
    Auth-->>Api: 200 {user_id, secret_key}
    Api->>Api: recompute SigV4 HMAC with secret_key, compare
    Api-->>C: 200 OK (or 403 if signature mismatch)
```

**Hard requirements, not suggestions:**

- Routed under the literal `/internal/` prefix so it's grep-able and impossible to confuse with
  the public surface.
- Gated behind a shared internal token: every request under `/internal/*` must carry a valid
  `X-Internal-Token` header, checked by dedicated middleware against `AUTH_SERVICE_INTERNAL_TOKEN`
  (env var) before it ever reaches the handler. A missing/wrong token is a 401, logged at `Warn`,
  before any repository call happens.
- **Must never be published outside the Docker/compose network.** Per the root
  `docs/design-arch.md` §7.2: don't publish `auth-service`'s port in `docker-compose.yml` at all
  — it should only be reachable from other containers on the same compose network. The internal
  token is defense-in-depth for a compromised container on that network, not the only layer.
- If `AUTH_SERVICE_INTERNAL_TOKEN` is unset at startup, refuse to start rather than silently
  running the internal routes unauthenticated.

---

## 10. Errors

Same sentinel-error-mapped-to-HTTP-status pattern as `metadata-service`:

- `repository.ErrNotFound` → 404
- `repository.ErrAlreadyExists` → 409 (duplicate username)
- `service.ErrInvalidInput` → 400 (validation failures: empty username, weak password, etc.)
- `service.ErrInvalidCredentials` → 401 (login: wrong username/password — deliberately not
  merged into `ErrInvalidInput`, since 400 vs 401 matters here and callers may want to
  distinguish "malformed request" from "wrong password")
- Unhandled internal token failure → 401, before hitting the service layer at all
- Anything else → 500, logged at `Error`

`Debug` for expected client-caused rejections (bad password, duplicate username, invalid input),
`Info` for real state changes (user created, credential issued/revoked), `Error` only for genuine
failures — identical levels to `metadata-service`.

---

## 11. Project Layout

```
auth-service/
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── .gitignore
├── docs/
│   ├── design.md
│   ├── openapi.yml
│   └── decisions/
│       └── 0001-plaintext-recoverable-secrets-for-sigv4.md
├── cmd/
│   └── server/
│       └── main.go
└── internal/
    ├── config/
    │   └── config.go        # env var loading: JWT secret, master key, internal token, DB path
    ├── model/
    │   ├── user.go
    │   └── credential.go
    ├── store/
    │   ├── store.go         # Open/Close, bolt-bucket bootstrap
    │   ├── buckets.go       # bolt-bucket name constants
    │   └── schema.go        # schema_version check/stamp
    ├── repository/
    │   ├── repository.go    # interfaces + sentinel errors
    │   └── bolt/
    │       ├── keys.go              # ValidateKeyComponent (shared)
    │       ├── user_keys.go         # UserKey, UsernameKey
    │       ├── credential_keys.go   # CredentialKey
    │       └── *_test.go
    ├── service/
    │   ├── service.go       # UserService / CredentialService interfaces
    │   ├── user_service.go  # register, get, login (bcrypt)
    │   ├── jwt_service.go   # mint/verify access + refresh tokens
    │   ├── credential_service.go  # generate, encrypt, issue, revoke, decrypt-lookup
    │   └── errors.go
    └── api/
        ├── router.go        # public routes + /internal routes with token middleware
        ├── handlers.go
        ├── dto.go
        └── errors.go
```

`internal/config` is the one layer `metadata-service` doesn't have (it has no secrets to load) —
`auth-service` does, so config loading gets its own small package rather than being inlined in
`main.go`: read `AUTH_SERVICE_JWT_SECRET`, `AUTH_SERVICE_MASTER_KEY`,
`AUTH_SERVICE_INTERNAL_TOKEN`, and an optional DB path override, and fail fast (`log.Fatal` in
`main.go`, not a panic buried in a handler) if any required secret is missing.

---

## 12. Tooling & Install Commands

```bash
# HTTP router — same as metadata-service
go get github.com/go-chi/chi/v5

# embedded KV store — same as metadata-service
go get go.etcd.io/bbolt

# IDs — same as metadata-service
go get github.com/google/uuid

# password hashing
go get golang.org/x/crypto/bcrypt

# JWT
go get github.com/golang-jwt/jwt/v5

go mod tidy
```

`crypto/aes`, `crypto/cipher`, `crypto/rand`, and `log/slog` are all standard library — no extra
dependency for AES-GCM or structured logging.

---

## 13. Suggested Build Order

1. `internal/config` — env var loading, fail-fast validation. Small, but everything downstream
   needs it.
2. `internal/store` — `Open`, bolt-bucket bootstrap, schema version check (copy
   `metadata-service`'s pattern directly).
3. `internal/repository/bolt/keys.go` + per-entity key builders — unit tested in isolation first.
4. `internal/repository` — `UserRepository`, `CredentialRepository` against real bbolt (
   `t.TempDir()` instances).
5. `internal/service` — bcrypt, JWT mint/verify, AES-GCM encrypt/decrypt, credential generation;
   fakes for repositories in tests, no bbolt involved here.
6. `internal/api` — public routes first, then the `/internal` route + token middleware last, with
   `httptest`-based handler tests including a case that asserts the internal route 401s without
   the token header.

Run `make check` after each layer before moving to the next, same gate as `metadata-service`.

---

## 14. Cross-Service Notes

- `api-service` will need a small HTTP client for `auth-service`, mirroring
  `api-service/clients/metadata/client_metadata.go` — out of scope for this service, flagged here
  so it isn't forgotten when `api-service`'s SigV4 work starts.
- `docker-compose.yml` (owned elsewhere, not yet written) must not publish `auth-service`'s port —
  see §9.
