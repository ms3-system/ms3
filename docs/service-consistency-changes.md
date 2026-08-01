# Service consistency changes

This documents every change made while getting `api-service`, `auth-service`,
`metadata-service`, and `data-service` running together locally for the
first time (no Docker/compose — plain `go run`/`make run`). It covers the
Part 1 contract audit (api-service's calls into the three backing services)
and the Part 3 startup/port/config consistency audit. Nothing here adds new
product features — everything below either fixes a broken call site or
brings a service in line with how the other three already behave.

## 1. Contract fixes (api-service ↔ backing services)

### 1.1 Critical: object key field name mismatch (`key` vs `object_key`)

**Symptom:** every object uploaded through `api-service` would fail, or come
back with an empty key.

`api-service`'s `clients.ObjectInfo` struct is tagged `object_key` (that's
api-service's own public response vocabulary). `api-service/clients/metadata`
was marshaling that same struct directly as the request body to
metadata-service's `POST /buckets/{bucket}/objects`, which decodes a
`putObjectRequest` expecting the field `key`. Since Go's JSON decoder
silently ignores unknown fields and zero-fills missing ones,
`req.Key` was always `""` — metadata-service's own validation
(`object key is required`) then rejected every upload with 400. The same
mismatch affected the *response* side: `objectResponse.Key` (`"key"` on the
wire) never landed in `api.ObjectInfo.Key` (tagged `"object_key"`), so any
object metadata that came back from `PutObjectMeta`/`GetObjectMeta`/
`ListObjects` had an empty key even where the call otherwise succeeded.

**Fix:** `api-service/clients/metadata/client_metadata.go` now has its own
private `putObjectRequest`/`objectResponse`/`bucketResponse` types that
mirror metadata-service's actual wire shapes exactly, and translates to/from
`api.ObjectInfo` explicitly. `api.ObjectInfo`'s public tags were left
unchanged — they're api-service's own contract with its callers, not
metadata-service's.

Verified end-to-end: uploaded `photos/2024/img.png`, listed it back, and the
key round-trips correctly (previously it round-tripped as `""`).

### 1.2 Object routes couldn't represent keys containing `/`

**Symptom:** the task's own required scenario — an object key like
`photos/2024/img.png` — 404'd immediately at api-service.

`api-service/api/router.go` registered object routes as
`/buckets/{bucket}/objects/{object}`. Chi's `{object}` is a single path
segment and does not match across `/`. metadata-service's own object routes
already use a wildcard (`/objects/*`) for exactly this reason. api-service's
public router never followed suit.

**Fix:** changed all three object routes to
`/buckets/{bucket}/objects/*` and switched the handlers from
`chi.URLParam(r, "object")` to `chi.URLParam(r, "*")`. Verified with a live
`PUT`/`GET` round-trip on `photos/2024/img.png`.

### 1.3 409/400 from metadata-service were surfaced as 502

**Symptom:** duplicate bucket name and non-empty bucket delete — both
required Part 2 scenarios — returned `502 Bad Gateway` instead of `409
Conflict`. Malformed/invalid input returned 502 instead of 400.

`api-service`'s metadata client (`do()`) only special-cased HTTP 404 into a
typed error; every other non-2xx status (409, 400, 500, ...) fell through to
a generic `fmt.Errorf`, which `utils.WriteUpstreamError` always mapped to
502 regardless of the real upstream status.

**Fix:** added `ConflictError` and `InvalidInputError` types alongside the
existing `NotFoundError` in `api-service/clients/clients.go`. The metadata
client now reads metadata-service's `{"error": "..."}` body and returns the
right typed error for 404/409/400. `utils.WriteUpstreamError` now maps:
`NotFoundError → 404`, `ConflictError → 409`, `InvalidInputError → 400`,
anything else → 502. Verified live: duplicate bucket create → 409, delete
non-empty bucket → 409 with `{"error":"bucket is not empty"}`.

### 1.4 `ListObjects` never forwarded `?prefix=`

metadata-service's `listObjects` handler already supports `?prefix=` and
`?limit=`, and the original design doc's api-service surface
(`GET /{bucket}?list-type=2&prefix=...`) calls for prefix filtering, but
`api-service`'s `handleListObjects` dropped the query param entirely — the
`MetadataClient.ListObjects` interface didn't even have a parameter for it.

**Fix:** `handleListObjects` now reads `r.URL.Query().Get("prefix")` and
forwards it; `MetadataClient.ListObjects` and `HTTPMetadataClient.ListObjects`
both take a `prefix` argument. Verified live:
`GET /buckets/my-bucket?prefix=photos/` returns only the matching object.

### 1.5 Error response bodies were plain text, not JSON

`api-service` (`utils.WriteUpstreamError`, and two `http.Error()` calls in
`handlers.go` for data-service failures) and `data-service` (three
`http.Error()` calls in `handler.go`) wrote plain-text bodies via
`http.Error`, while `metadata-service` and `auth-service` both use a
`{"error": "..."}` JSON envelope. See §3 below — fixed to match.

### 1.6 `CreateBucket` never sent `owner_id` — now fixed by the auth integration (§2)

metadata-service's `createBucketRequest` has a required `owner_id` field,
and `bucketService.CreateBucket` rejects an empty one with 400.
`handleCreateBucket` previously had no way to populate it, because
api-service had no auth-service integration at all — no notion of "who is
calling." **This is now implemented (§2):** api-service verifies a SigV4
signature on every request, resolves it to a `user_id` via auth-service, and
passes that as `owner_id` on `CreateBucket`. Verified live: `PUT
/buckets/{name}` signed with a real credential now returns `201` with
`"owner_id"` set to the signing user's id (previously always `400`).

### 1.7 "List buckets" and object `HEAD` from the original design — now built

`docs/design-arch.md` §6 lists `GET /` (list buckets) and
`HEAD /{bucket}/{key}` (object metadata) as part of api-service's intended
public surface. Neither existed in `api-service/api/router.go`. Both are now
implemented:

- `GET /` → `handleListBuckets`, calls the new `MetadataClient.ListBucketsByOwner`
  (`GET /buckets?owner_id=...` on metadata-service) scoped to the
  SigV4-authenticated caller — there's no "list all buckets" surface, since
  S3's own `ListBuckets` is likewise implicitly scoped to the caller's
  identity, not an open parameter.
- `HEAD /buckets/{bucket}/objects/*` → `handleHeadObject`, calls the
  existing `MetadataClient.GetObjectMeta` and writes `Content-Length`,
  `Content-Type`, and `ETag` (when set) with no body.

Note also that `GET /{bucket}` in the design is "list objects" (matches
current api-service behavior), not "get bucket metadata" — api-service
still has no public "get a single bucket's metadata" endpoint, since that
was never part of its own design (see §6 of `docs/design-arch.md` — only
list/create/delete are listed for buckets). What *is* now reachable is
metadata-service's `GetBucket` internally, via the new authorization check
in §2 — every bucket/object operation fetches the bucket to check
ownership, which exercises that capability even without a dedicated public
endpoint for it.

### 1.8 `POST /v1/...` vs `POST /...` — no actual drift

The original design doc (`docs/design-arch.md` §6) specifies `/v1/`-prefixed
paths for metadata-service and auth-service internal routes.
metadata-service's actual router has no `/v1` prefix at all
(`/buckets`, not `/v1/buckets`), and `api-service`'s metadata client was
already built against that same un-prefixed shape. Since both real
implementations already agree with each other, this is a documentation-only
drift from the design doc, not a bug between the two services — left as-is.

## 2. Auth-service integration — now implemented

This was previously flagged as a full-size gap (no `AuthClient`, no SigV4
verification, no call to `GET /internal/credentials/{access_key}`, no
`X-Internal-Token` usage — every api-service request was completely
unauthenticated). It's now built:

### 2.1 SigV4 verification (`api-service/internal/sigv4`)

A from-scratch implementation of the core AWS Signature Version 4
algorithm: parses the `Authorization: AWS4-HMAC-SHA256 Credential=.../
SignedHeaders=..., Signature=...` header, builds the canonical request
(method, URI, query string, signed headers, payload hash placeholder),
derives the signing key (`HMAC(HMAC(HMAC(HMAC("AWS4"+secret, date), region),
service), "aws4_request")`), and compares signatures with `hmac.Equal`
(constant-time). Requests more than 15 minutes off the server's clock
(`X-Amz-Date`) are rejected as expired. Region/service are fixed to
`us-east-1`/`s3` — this is a local single-region system, not a real
multi-region deployment, so there's nothing else for a client to pick.
Covered by `api-service/internal/sigv4/sigv4_test.go` (valid signature,
wrong secret, tampered path, expired timestamp, malformed header).

**Deliberate simplification:** the request payload hash
(`X-Amz-Content-Sha256`) is taken as given and folded into the canonical
request/signature like any other signed value, but never independently
recomputed from the body. Recomputing it would mean buffering upload
bodies before forwarding them, which breaks the pass-through streaming
`docs/design-arch.md` §5.2 calls for ("api-service must never read the
client's upload into a `[]byte` before forwarding it"). This mirrors AWS's
own `UNSIGNED-PAYLOAD` mode, applied uniformly rather than as a special
case for uploads only — the signature still covers method, path, query,
and every other signed header, so a MITM can't alter *those* without
detection; body integrity for streamed uploads is left to the client's own
`X-Amz-Content-Sha256` claim, same as real S3 clients using that mode.

### 2.2 `AuthClient` + credential lookup

`api-service/clients/auth/client_auth.go` (`HTTPAuthClient`) calls
`GET {auth-service}/internal/credentials/{access_key}` with
`X-Internal-Token: <API_SERVICE_INTERNAL_TOKEN>`, and returns the
`{user_id, secret_key}` pair `sigv4.Verify` needs. A 404 from auth-service
(unknown access key) maps to a 401 at api-service; any other failure (bad
internal token, auth-service down) maps to 502, distinguishing "you signed
with an unknown key" from "the system is broken" for the caller.

### 2.3 `requireSigV4` middleware + ownership authorization

`api-service/api/auth_middleware.go` wraps every route except `/healthz`:
parses the signature, looks up the credential, verifies it, and stores a
`Principal{UserID, AccessKey}` in the request context. A separate
`authorizeBucketOwner` helper (also in `auth_middleware.go`) fetches the
target bucket via the new `MetadataClient.GetBucket` and compares its
`owner_id` against the principal — authentication (valid signature, known
key) and authorization (owns this specific resource) are deliberately
separate checks, returning 401 vs. 403 respectively. `CreateBucket` uses
the principal's `user_id` directly as `owner_id` (§1.6); every other
bucket/object handler calls `authorizeBucketOwner` before doing anything
else — for `PutObject` specifically, authorization happens *before* the
upload body is touched, so an unauthorized caller can't push bytes into
data-service at all.

There's no `public_read`/admin concept in the current bucket model (see
`metadata-service/internal/model/bucket.go`), so authorization is strictly
owner-only for every operation — no anonymous or shared-bucket access.
Adding a public-read flag would mean a metadata-service schema change,
which is out of scope here.

Verified live end-to-end (see `docs/local-integration-testing.md` §3–4):
registered two real users (alice, bob) via auth-service, issued each a
credential, and confirmed: alice's signed `PUT /buckets/my-bucket` returns
`201` with `owner_id` set to her user id (previously always `400`); an
unsigned request gets `401`; bob's signed requests against alice's bucket
get `403 {"error":"not the bucket owner"}`; a tampered signature (path
changed post-signing) gets `401 {"error":"signature verification failed"}`.

### 2.4 New routes: `GET /` (list buckets) and `HEAD` (object metadata)

See §1.7.

### 2.5 What's still not built

Presigned-URL authentication (query-string SigV4, as opposed to the
`Authorization` header form) is not implemented — `docs/design-arch.md` §1
mentions it as part of api-service's eventual responsibilities, but nothing
in the original gap list called it out specifically, and it's a distinct
mechanism from header-based SigV4. Flagging it as a follow-up rather than
building it speculatively. There's also no credential caching in
api-service (`docs/design-arch.md` §3 suggests a short-TTL memory cache for
credential lookups, since they're on the hot path of every request) — every
request currently does a live round-trip to auth-service; a reasonable
next step but not part of this change.

## 3. Part 3: startup, port, and configuration consistency

### 3.1 metadata-service didn't actually run

`metadata-service/cmd/server/main.go` opened the bbolt store and then called
`os.Exit(0)` — it never built the router, repositories, or services, and
never started an HTTP server, despite a fully implemented and unit-tested
`internal/api`/`internal/service`/`internal/repository` layer sitting right
next to it. This was the single largest blocker to "run all four services
together" — metadata-service could not be run at all.

**Fix:** rewrote `main.go` to wire `store → repositories → services → router
→ http.Server`, following the same shape as the other three services (see
§3.3–§3.5).

### 3.2 Port assignment

Before this task: `auth-service` hardcoded `:8082`, `data-service` hardcoded
`:8081`, `metadata-service` bound to nothing, and `api-service` hardcoded
`:8080` — but its own `metadataURL` constant pointed at
`http://metadata-service:8082` (auth-service's port, not metadata-service's;
metadata-service didn't have one) and used Docker Compose-style hostnames
that don't resolve outside a compose network.

**Final local port assignment** (all configurable, see §3.4):

| Service            | Port   | Notes                                              |
|--------------------|--------|----------------------------------------------------|
| `api-service`      | `8080` | public gateway, unchanged                          |
| `data-service`     | `8081` | unchanged                                          |
| `auth-service`     | `8082` | unchanged                                          |
| `metadata-service` | `8083` | **newly assigned** — previously had no port at all |

`api-service`'s default `API_SERVICE_METADATA_URL` /
`API_SERVICE_DATA_URL` now point at `http://localhost:8083` /
`http://localhost:8081` respectively, so all four resolve correctly with
plain `go run` on one machine (no Docker DNS involved).

### 3.3 Env var naming pattern

auth-service already used a consistent `AUTH_SERVICE_*` prefix
(`AUTH_SERVICE_JWT_SECRET`, `AUTH_SERVICE_INTERNAL_TOKEN`,
`AUTH_SERVICE_MASTER_KEY`, `AUTH_SERVICE_DB_PATH`). The same
`<SERVICE>_*` pattern is now applied to all four:

| Service            | New env vars                                       | Default                 |
|--------------------|----------------------------------------------------|-------------------------|
| `metadata-service` | `METADATA_SERVICE_SERVER_PORT`                     | `8083`                  |
|                    | `METADATA_SERVICE_DB_PATH`                         | `data/metadata.db`      |
| `data-service`     | `DATA_SERVICE_SERVER_PORT`                         | `8081`                  |
|                    | `DATA_SERVICE_DATA_DIR`                            | `data`                  |
| `auth-service`     | `AUTH_SERVICE_SERVER_PORT` (new)                   | `8082`                  |
|                    | `AUTH_SERVICE_DB_PATH` (existing)                  | `data/auth.db`          |
|                    | `AUTH_SERVICE_JWT_SECRET` (existing, required)     | none                    |
|                    | `AUTH_SERVICE_INTERNAL_TOKEN` (existing, required) | none                    |
|                    | `AUTH_SERVICE_MASTER_KEY` (existing, required)     | none                    |
| `api-service`      | `API_SERVICE_SERVER_PORT` (new)                    | `8080`                  |
|                    | `API_SERVICE_METADATA_URL` (new)                   | `http://localhost:8083` |
|                    | `API_SERVICE_DATA_URL` (new)                       | `http://localhost:8081` |
|                    | `API_SERVICE_AUTH_URL` (new)                       | `http://localhost:8082` |
|                    | `API_SERVICE_INTERNAL_TOKEN` (new, required)       | none                    |

`metadata-service`, `data-service`, and `api-service` needed an
`internal/config` package added from scratch (none existed). `metadata-service`
and `data-service` run with zero configuration for a first run — sane
defaults everywhere, including the bbolt file (`metadata-service`) and the
local storage directory (`data-service`, see §3.6). `api-service` needs one
required value, `API_SERVICE_INTERNAL_TOKEN` — added along with the SigV4
auth integration (§2); it's a shared secret with auth-service (must match
`AUTH_SERVICE_INTERNAL_TOKEN`) and, like `auth-service`'s three secrets
(§3.7), has no safe default. `auth-service`'s three security secrets remain
*required* (not defaulted) — see §3.7 for why that wasn't changed.

### 3.4 Graceful shutdown

None of the four services previously handled `SIGINT`/`SIGTERM` — `auth-service`
called `srv.ListenAndServe()` directly with no signal handling;
`data-service` and (pre-fix) `metadata-service` used the bare
`http.ListenAndServe` package function, which offers no shutdown hook at
all.

**Fix:** all four `cmd/server/main.go` now follow the same pattern: build an
`*http.Server` with `ReadTimeout`/`ReadHeaderTimeout`/`WriteTimeout`/
`IdleTimeout` (30s/5s/30s/120s), run `ListenAndServe` in a goroutine, block
on `SIGINT`/`SIGTERM` via `signal.Notify`, then call `srv.Shutdown(ctx)` with
a 10s timeout. Verified live: sent `SIGTERM` to all four running processes
and confirmed each logged `"shutting down"` and released its port within
the shutdown window.

### 3.5 `/healthz`

`metadata-service` and `auth-service` already had `GET /healthz` returning
`200 {"status":"ok"}`. `data-service` had none at all — added
`h.healthz` + `r.Get("/healthz", h.healthz)`, same shape.
`api-service`'s router had no `/healthz` either — added the same handler
directly on `Server`.

### 3.6 Local storage sane defaults

`data-service/cmd/server/main.go` hardcoded `localfs.NewLocalStore("/data")`
— an absolute, typically unwritable path outside a container, and one that
would fail on first run even if writable: `Write()` calls
`os.CreateTemp(filepath.Join(baseDir, "tmp"), ...)`, but nothing ever
created that `tmp` subdirectory, so the very first upload would fail with
`no such file or directory`.

**Fix:** `DATA_SERVICE_DATA_DIR` now defaults to a relative `data/` dir
(matching the other services' relative-path conventions), and
`localfs.NewLocalStore` now returns an error and creates
`<dir>/tmp` (`os.MkdirAll`) up front, so a fresh checkout works with zero
manual setup — verified with a clean scratch dir and a real upload.

### 3.7 auth-service's required secrets were left required

`auth-service/internal/config/config_test.go` has explicit tests asserting
`Load()` errors when `AUTH_SERVICE_JWT_SECRET` /
`AUTH_SERVICE_INTERNAL_TOKEN` / `AUTH_SERVICE_MASTER_KEY` are unset. Rather
than weaken that (auto-generating ephemeral secrets would contradict tested,
intentional behavior, and is a bigger call than this task should make
unilaterally for a security-sensitive service), these stay required.
`auth-service/.env.example` documents copy-pasteable throwaway local-dev
values instead — this is the "or equivalent" to a zero-config first run for
this one service, since a security-sensitive service that starts with no
secrets configured at all isn't actually a safe default.

### 3.8 Logging

`metadata-service` and `auth-service` already used `slog.NewJSONHandler` with
a consistent per-request logging middleware (`request_id`, `method`, `path`,
`status`, `bytes`, `duration`). `data-service` used the stdlib `log` package
plus chi's plain-text `middleware.Logger`; `api-service` used stdlib `log`
plus the same plain-text `middleware.Logger`. Both now use
`slog.NewJSONHandler(os.Stdout, nil)` and the identical
`requestLogger` middleware shape as metadata-service/auth-service.

### 3.9 `config.example.yaml` equivalent

None of the four services load configuration from a YAML file (all four use
env vars via `internal/config`), so introducing YAML parsing would be the
kind of larger architectural change this task explicitly avoids. Each
service now has a `.env.example` at its root instead, documenting every env
var, its default, and (for auth-service) working throwaway local-dev
secrets.

### 3.10 Makefiles

`auth-service` and `metadata-service` already had a `Makefile` with
`build`/`run`/`test`/`vet`/`lint`/`check`/`clean` targets.
`api-service` and `data-service` had none. Added matching Makefiles to both
(same targets; `data-service`'s `clean` removes its data dir instead of a DB
file). `make check` (`vet` + `lint` + `test`) passes clean on all four.

## 4. Part 4 cross-cutting checks

### 4.1 Request ID propagation — gap, not fixed

`api-service` generates a request ID for its own logs via chi's
`middleware.RequestID`, but never forwards it (e.g. as `X-Request-ID`) on
its outbound calls to metadata-service/data-service. Each backing service
generates its own independent request ID instead, so a single client
request currently produces three unrelated IDs across the three services'
logs, with no way to correlate them. Per the task's guidance, this is
flagged as a gap for a follow-up issue rather than built here (would mean
introducing real correlation-ID propagation, a bigger change than an
in-scope fix).

### 4.2 Error envelope — fixed

All four services now return `{"error": "<message>"}` on every non-2xx
response. Before this task, `api-service` and `data-service` used
`http.Error` (plain text, `Content-Type: text/plain`); `metadata-service`
and `auth-service` already used the JSON envelope. See §1.5.

### 4.3 Timeouts on api-service's outbound calls — fixed

Before this task, `api-service`'s HTTP clients used `http.DefaultClient`
with no timeout, and passed `r.Context()` straight through with no deadline
— a hung metadata-service or data-service call would hang the client
request indefinitely. `HTTPMetadataClient` now wraps every call in a 10s
`context.WithTimeout` (all metadata operations are small CRUD calls).
`HTTPDataClient.Delete` uses the same 10s deadline; `Write`/`Read` use a
5-minute deadline instead, since they stream request/response bodies of
unknown size (a short deadline would break large uploads/downloads) — the
`Read` deadline is released as soon as the caller finishes reading/closing
the streamed body rather than held for the full 5 minutes regardless of
actual transfer time.
