# Local integration testing

How to run all four `ms3` services together on one machine (no Docker/
compose — plain `go run`/`make run`), and a plain-markdown reference of
requests to run by hand in Postman/Apidog/`curl` against the locally-running
services, to prove the whole system actually works end-to-end.

See `docs/service-consistency-changes.md` for everything that had to be
fixed to make this possible.

> **api-service now requires every request (except `/healthz`) to carry a
> valid AWS SigV4 signature.** This used to be a flagged gap — no
> `auth-service` integration existed at all, so bucket creation always
> failed and nothing was authorized. That's now implemented: api-service
> looks up the signing credential's secret key from auth-service
> (`GET /internal/credentials/{access_key}`, protected by
> `X-Internal-Token`), verifies the SigV4 signature against the live
> request, and authorizes bucket/object operations by comparing the
> caller's user id against the bucket's `owner_id`. See "Signing requests"
> below for how to actually send a signed request.

---

## Running all four services locally

### Prerequisites

- Go (matching the version in each service's `go.mod`)
- Python 3 (stdlib only) if you want to sign requests via
  `scripts/sigv4-request.py` — see "Signing requests" below
- The `bbolt` CLI (`go install go.etcd.io/bbolt/cmd/bbolt@latest`) if you
  want to inspect `metadata-service`/`auth-service` data directly — see
  "Inspecting the databases with `bbolt`" below. Optional; nothing else in
  this doc needs it.
- Nothing else — no database server, no Docker. Each service uses an
  embedded bbolt file or the local filesystem, created on demand.

### Port assignment

| Service            | Port   | Health check                        |
|---------------------|--------|--------------------------------------|
| `metadata-service` | `8083` | `GET http://localhost:8083/healthz` |
| `data-service`     | `8081` | `GET http://localhost:8081/healthz` |
| `auth-service`     | `8082` | `GET http://localhost:8082/healthz` |
| `api-service`      | `8080` | `GET http://localhost:8080/healthz` |

All four return `200 {"status":"ok"}` when healthy.

### Startup order

Start the three backing services first, then `api-service` last (it's the
only one that calls the others). Each command below is run from that
service's own directory, in its own terminal.

**1. `metadata-service`** — no required env vars, uses sane defaults:

```sh
cd metadata-service
make run
# or: go run ./cmd/server
```

Confirm: `curl http://localhost:8083/healthz` → `{"status":"ok"}`

**2. `data-service`** — no required env vars:

```sh
cd data-service
make run
```

Confirm: `curl http://localhost:8081/healthz` → `{"status":"ok"}`

**3. `auth-service`** — **requires** three secrets (no defaults — see
`docs/service-consistency-changes.md` §3.7 for why). Copy
`auth-service/.env.example` to `.env` and source it, or export directly.
**Note the internal token — api-service needs the exact same value:**

```sh
cd auth-service
export AUTH_SERVICE_JWT_SECRET=local-dev-jwt-secret-change-me
export AUTH_SERVICE_INTERNAL_TOKEN=local-dev-internal-token-change-me
export AUTH_SERVICE_MASTER_KEY=4/NI5ZVXCJjvLHiTVfPB1NSPThY5ESxTBhXJiE1Y8mk=
make run
```

Confirm: `curl http://localhost:8082/healthz` → `{"status":"ok"}`

**4. `api-service`** — **requires** `API_SERVICE_INTERNAL_TOKEN`, which
must exactly match `AUTH_SERVICE_INTERNAL_TOKEN` above (the URL settings
are optional and already default to the other three services' default
ports):

```sh
cd api-service
export API_SERVICE_INTERNAL_TOKEN=local-dev-internal-token-change-me
make run
```

Confirm: `curl http://localhost:8080/healthz` → `{"status":"ok"}`

### Stopping

`Ctrl+C` in each terminal — all four handle `SIGINT`/`SIGTERM` with a
graceful shutdown (10s timeout).

### Resetting local state

Each service's `make clean` removes its local data (bbolt file or storage
directory) so the next run starts from empty.

---

## Inspecting the databases with `bbolt`

`metadata-service` and `auth-service` both store their data in an embedded
[bbolt](https://github.com/etcd-io/bbolt) file — you can open it directly
with the `bbolt` CLI to see exactly what a request wrote, independent of
whatever the API says. `data-service` isn't bbolt-backed (see the bottom of
this section).

**Install the CLI** (once): `go install go.etcd.io/bbolt/cmd/bbolt@latest`

**⚠️ The service must not be running when you inspect its file.** bbolt
takes an exclusive lock on the file for as long as a process has it open —
`bbolt get`/`keys`/`buckets` against a live service's file will hang
indefinitely (verified: it doesn't even time out) rather than error out.
The practical pattern used below: run a batch of requests, `Ctrl+C` the
service, inspect, then `make run` again to keep going — bbolt state is a
durable file, so stopping and restarting doesn't lose anything you already
wrote.

### DB paths and schema

| Service | Default path | Buckets (bbolt's own term for a keyspace, not an S3 bucket) |
|---|---|---|
| `metadata-service` | `metadata-service/data/metadata.db` | `buckets`, `bucket_owner_index`, `objects`, `meta` |
| `auth-service` | `auth-service/data/auth.db` | `users`, `usernames`, `credentials`, `meta` |

| Bucket | Key | Value |
|---|---|---|
| `metadata-service` → `buckets` | S3 bucket name (e.g. `my-bucket`) | JSON `model.Bucket` (`id`, `name`, `owner_id`, `is_versioned`, `deleted_at`, `created_at`) |
| `metadata-service` → `bucket_owner_index` | `<owner_id>\x00<bucket_name>` | the bucket name (plain bytes) |
| `metadata-service` → `objects` | `<bucket_name>\x00<object_key>` | JSON `model.Object` (`id`, `object_key`, `bucket_name`, `size_bytes`, `etag`, `content_type`, `storage_ref`, `version_id`, `is_latest`, `deleted_at`, `created_at`) |
| `auth-service` → `users` | user id (UUID) | JSON `model.User` (`id`, `username`, `password_hash`, `is_admin`, `created_at`) |
| `auth-service` → `usernames` | username | the user id (plain bytes) — a lookup index |
| `auth-service` → `credentials` | access key | JSON `model.Credential` (`access_key`, `user_id`, `secret_key_encrypted`, `created_at`, `revoked_at`) |

Both services soft-delete: a deleted bucket/object row stays in bbolt with
`deleted_at` set rather than being removed — the API correctly 404s it, but
it's still there if you look directly. Scenarios 3.10 and 3.13 below show
this.

### Command cookbook

```sh
# list bbolt's top-level buckets in a file
bbolt buckets metadata-service/data/metadata.db

# list keys in one bucket — plain string keys read fine with defaults
bbolt keys auth-service/data/auth.db users

# composite keys (bucket_owner_index, objects) contain a \x00 separator;
# -f ascii-encoded renders it readably instead of a raw control character
bbolt keys metadata-service/data/metadata.db objects -f ascii-encoded
# → "my-bucket\x00photos/2024/img.png"

# get a value by a plain string key
bbolt get metadata-service/data/metadata.db buckets my-bucket

# get a value by a composite (null-byte-containing) key: a raw \x00 doesn't
# survive as a shell argument, so hex-encode the key and pass --parse-format hex
bbolt get metadata-service/data/metadata.db objects \
  "$(printf 'my-bucket\x00photos/2024/img.png' | xxd -p | tr -d '\n')" \
  --parse-format hex
```

### `data-service`: not bbolt — plain files on disk

`data-service` stores bytes directly in the filesystem, content-addressed
by the sha256 hash `storage_ref` returned from an upload, sharded two
levels deep to avoid huge flat directories:

```
data-service/data/<bucket>/objects/<hash[0:2]>/<hash[2:4]>/<hash>
```

No lock issue here — the service can be left running. To verify an upload
landed correctly:

```sh
find data-service/data/my-bucket -type f
sha256sum data-service/data/my-bucket/objects/b9/4d/b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9
# no sha256sum on your machine? macOS also has: shasum -a 256 <path>
# compare against the storage_ref the upload response returned, and/or the
# original file's own sha256sum
```

---

## Signing requests

Every api-service request below except the health checks needs an
`Authorization: AWS4-HMAC-SHA256 ...` header, an `X-Amz-Date` header, and
(implicitly) an `X-Amz-Content-Sha256` header. Two ways to produce that:

**Postman/Apidog (recommended for interactive testing):** both have a
built-in "AWS Signature" auth type. Set:
- Access Key / Secret Key: from scenario 2.7 below
- AWS Region: `us-east-1`
- Service Name: `s3`

and the client signs each request for you.

**`scripts/sigv4-request.py` (for `curl`-less CLI testing):** a small
stdlib-only Python script in this repo:

```sh
python3 scripts/sigv4-request.py \
  --access-key AKIA... --secret-key ... \
  --method PUT --url http://localhost:8080/buckets/my-bucket
```

It prints `status: <code>` to stderr and the response body to stdout. Use
`--content-type` and `--data`/`--data-file` for request bodies.

Region/service are fixed to `us-east-1`/`s3` — this is a small local
system, not a multi-region deployment, so there's nothing else to pick.

---

## Test scenarios

All requests below hit the locally-running services directly by port, as
started above. Organized by user journey. Requests through api-service are
shown as `curl`-style method/URL/headers for readability; substitute your
signing method from above for the `Authorization`/`X-Amz-Date` headers.

### 1. Health checks

| #   | Request                             | Expect                | Proves                 |
|-----|--------------------------------------|------------------------|-------------------------|
| 1.1 | `GET http://localhost:8083/healthz` | `200 {"status":"ok"}` | metadata-service is up |
| 1.2 | `GET http://localhost:8081/healthz` | `200 {"status":"ok"}` | data-service is up     |
| 1.3 | `GET http://localhost:8082/healthz` | `200 {"status":"ok"}` | auth-service is up     |
| 1.4 | `GET http://localhost:8080/healthz` | `200 {"status":"ok"}` | api-service is up      |

### 2. Auth-service: register, login, refresh, issue credential

These hit `auth-service` **directly** (port 8082) — this is where identity
and credentials live; api-service only ever reads a credential's secret key
back via the internal `GET /internal/credentials/{access_key}` endpoint
(see `docs/service-consistency-changes.md` §2.2), it doesn't manage users
itself.

**2.1 Register a user**

```
POST http://localhost:8082/v1/users/
Content-Type: application/json

{"username": "alice", "password": "correct horse battery staple"}
```

Expect `201`:

```json
{
  "id": "<uuid>",
  "username": "alice",
  "is_admin": false,
  "created_at": "<time>"
}
```

Proves: user creation + password hashing works. Save `id` as `ALICE_ID`.

🔎 **Verify in bbolt** (stop `auth-service` first):
```sh
bbolt get auth-service/data/auth.db users <ALICE_ID>
bbolt get auth-service/data/auth.db usernames alice
```
Expect the first to return the same JSON as the API response, plus a
`password_hash` field (a bcrypt hash, e.g.
`$2a$10$R04jqQvphGd48lITTNY09er...` — never the plaintext password). Expect
the second to return `<ALICE_ID>` — confirms the username index points at
the right user.

**2.2 Register with a too-short password (expect 400)**

```
POST http://localhost:8082/v1/users/
Content-Type: application/json

{"username": "eve"}
```

Expect `400 {"error":"invalid input: password must be at least 8 characters"}`.
Proves: input validation on a write endpoint.

**2.3 Login**

```
POST http://localhost:8082/v1/auth/login
Content-Type: application/json

{"username": "alice", "password": "correct horse battery staple"}
```

Expect `200 {"access_token": "<jwt>", "refresh_token": "<jwt>"}`.
Save both as `ALICE_ACCESS_TOKEN` / `ALICE_REFRESH_TOKEN`.

**2.4 Refresh the access token**

```
POST http://localhost:8082/v1/auth/refresh
Content-Type: application/json

{"refresh_token": "<ALICE_REFRESH_TOKEN>"}
```

Expect `200 {"access_token": "<jwt>"}`.

**2.5 Get own user (authenticated)**

```
GET http://localhost:8082/v1/users/<ALICE_ID>
Authorization: Bearer <ALICE_ACCESS_TOKEN>
```

Expect `200` with the same user shape as 2.1.

**2.6 Get user without a token (expect 401)**

```
GET http://localhost:8082/v1/users/<ALICE_ID>
```

Expect `401 {"error":"unauthorized"}`.

**2.7 Issue an S3-style credential for alice**

```
POST http://localhost:8082/v1/users/<ALICE_ID>/credentials
Authorization: Bearer <ALICE_ACCESS_TOKEN>
```

Expect `201`:

```json
{
  "access_key": "AKIA...",
  "secret_key": "...",
  "user_id": "<ALICE_ID>",
  "created_at": "<time>"
}
```

Save `access_key`/`secret_key` as `ALICE_AK`/`ALICE_SK` — this is what
signs every api-service request in section 3 below.

🔎 **Verify in bbolt** (stop `auth-service` first):
```sh
bbolt get auth-service/data/auth.db credentials <ALICE_AK>
```
Expect `{"access_key":"<ALICE_AK>","user_id":"<ALICE_ID>","secret_key_encrypted":"<base64 ciphertext>","created_at":"<time>"}`
— note `secret_key_encrypted` is ciphertext, **not** the `secret_key` the
API handed back. Proves the secret is actually encrypted at rest, not just
hidden by the API response shape.

**2.8 Repeat 2.1–2.7 for a second user, "bob"** — save `BOB_AK`/`BOB_SK`.
Used in section 4 to prove one user can't touch another's bucket.

### 3. Bucket + object journey — via api-service, signed as alice

**3.1 Create a bucket**

```
PUT http://localhost:8080/buckets/my-bucket
Authorization: AWS4-HMAC-SHA256 Credential=<ALICE_AK>/... (signed with ALICE_SK)
X-Amz-Date: <timestamp>
```

Expect `201`:

```json
{"name": "my-bucket", "owner_id": "<ALICE_ID>", "created_at": "<time>"}
```

Proves: api-service now derives `owner_id` from the SigV4-authenticated
caller and forwards it to metadata-service — this always returned `400`
before the auth integration existed.

🔎 **Verify in bbolt** (stop `metadata-service` first):
```sh
bbolt get metadata-service/data/metadata.db buckets my-bucket
bbolt keys metadata-service/data/metadata.db bucket_owner_index -f ascii-encoded
```
Expect the first to return
`{"id":"<uuid>","name":"my-bucket","owner_id":"<ALICE_ID>","is_versioned":false,"created_at":"<time>"}`.
Expect the second to include `"<ALICE_ID>\x00my-bucket"` — the owner index
entry `ListBucketsByOwner` (3.3) actually scans.

**3.2 Create the same bucket name again (expect 409)**

Same request as 3.1. Expect `409 {"error":"already exists"}`.

**3.3 List buckets (new: `GET /`)**

```
GET http://localhost:8080/
```

Expect `200 [{"name": "my-bucket", "owner_id": "<ALICE_ID>", "created_at": "<time>"}]`
— only alice's buckets, scoped to the signing caller.

**3.4 Create a bucket without signing it (expect 401)**

```
PUT http://localhost:8080/buckets/unsigned-bucket
```
(no `Authorization` header)

Expect `401 {"error":"missing Authorization header"}`.

**3.5 Upload an object**

```
PUT http://localhost:8080/buckets/my-bucket/objects/hello.txt
Content-Type: text/plain

hello world
```

Expect `201`:

```json
{
  "object_key": "hello.txt",
  "bucket_name": "my-bucket",
  "size_bytes": 11,
  "etag": "",
  "content_type": "text/plain",
  "storage_ref": "<sha256>",
  "created_at": "<time>"
}
```

**3.6 Object key containing slashes — the wildcard route**

```
PUT http://localhost:8080/buckets/my-bucket/objects/photos/2024/img.png
Content-Type: image/png

<binary body>
```

Expect `201` with `"object_key": "photos/2024/img.png"`.

🔎 **Verify in bbolt** (stop `metadata-service` first):
```sh
bbolt keys metadata-service/data/metadata.db objects -f ascii-encoded
# → "my-bucket\x00hello.txt" and "my-bucket\x00photos/2024/img.png"

bbolt get metadata-service/data/metadata.db objects \
  "$(printf 'my-bucket\x00photos/2024/img.png' | xxd -p | tr -d '\n')" \
  --parse-format hex
```
Expect the composite key for each object (bucket name + object key,
null-byte separated — this is exactly how metadata-service supports
listing by prefix without a secondary index), and the `get` to return the
full record with `"object_key":"photos/2024/img.png"` and the same
`storage_ref` the API returned — that ref is what you'd look up on disk
under `data-service` (see the cookbook section above).

**3.7 HEAD an object (new route)**

```
HEAD http://localhost:8080/buckets/my-bucket/objects/photos/2024/img.png
```

Expect `200`, empty body, with `Content-Length`, `Content-Type: image/png`
headers set — same metadata `GET` would use, without downloading the bytes.

**3.8 Download the object back and confirm byte-for-byte match**

```
GET http://localhost:8080/buckets/my-bucket/objects/photos/2024/img.png
```

Expect `200` with the exact bytes uploaded in 3.6 and
`Content-Type: image/png`. Compare a checksum of the downloaded body
against the original file.

**3.9 List objects, and list with a prefix**

```
GET http://localhost:8080/buckets/my-bucket
GET http://localhost:8080/buckets/my-bucket?prefix=photos/
```

The first returns both objects; the second only `photos/2024/img.png`.

**3.10 Delete an object, then confirm it's gone**

```
DELETE http://localhost:8080/buckets/my-bucket/objects/hello.txt
```
Expect `204`. Then:
```
GET http://localhost:8080/buckets/my-bucket/objects/hello.txt
```
Expect `404 {"error":"metadata record not found"}`.

🔎 **Verify in bbolt** (stop `metadata-service` first) — this is the
interesting one, since the API's 404 hides what actually happened:
```sh
bbolt get metadata-service/data/metadata.db objects \
  "$(printf 'my-bucket\x00hello.txt' | xxd -p | tr -d '\n')" \
  --parse-format hex
```
Expect the record to **still be there**, now with `"deleted_at":"<time>"`
set. Deletes are soft — the row is never actually removed, just marked;
the API layer is what turns a `deleted_at`-set row into a 404. (This is
also how `DeleteBucket`'s non-empty check in 3.12 below can tell live
objects from deleted ones.)

**3.11 Upload to a bucket that doesn't exist (expect 404)**

```
PUT http://localhost:8080/buckets/does-not-exist/objects/foo.txt
Content-Type: text/plain

x
```
Expect `404 {"error":"metadata record not found"}`.

**3.12 Delete a non-empty bucket (expect 409)**

With `photos/2024/img.png` still in `my-bucket`:
```
DELETE http://localhost:8080/buckets/my-bucket
```
Expect `409 {"error":"bucket is not empty"}`.

**3.13 Delete the remaining object, then delete the now-empty bucket**

```
DELETE http://localhost:8080/buckets/my-bucket/objects/photos/2024/img.png
```
Expect `204`, then:
```
DELETE http://localhost:8080/buckets/my-bucket
```
Expect `204`.

🔎 **Verify in bbolt** (stop `metadata-service` first):
```sh
bbolt get metadata-service/data/metadata.db buckets my-bucket
bbolt keys metadata-service/data/metadata.db bucket_owner_index -f ascii-encoded
```
Expect the bucket record to still exist (soft-deleted, `"deleted_at"` set —
same pattern as objects above), but the `bucket_owner_index` key for it to
be **gone entirely** (not soft-deleted, actually removed) — that's what
keeps a deleted bucket from showing up in a future `ListBucketsByOwner`
(3.3) without needing to filter every scanned row for `deleted_at`.

### 4. Authorization boundary — bob vs. alice's bucket

Recreate `my-bucket` as alice (3.1) before running these.

**4.1 Bob (a different, validly-authenticated user) reads alice's bucket (expect 403)**

```
GET http://localhost:8080/buckets/my-bucket
```
Signed with `BOB_AK`/`BOB_SK`. Expect `403 {"error":"not the bucket owner"}`.
Proves: authentication (valid signature) and authorization (owns the
resource) are checked separately — bob is a real, known user, just not
this bucket's owner.

**4.2 Bob tries to delete alice's object (expect 403)**

```
DELETE http://localhost:8080/buckets/my-bucket/objects/hello.txt
```
Signed as bob. Expect `403 {"error":"not the bucket owner"}`.

**4.3 Bob's own bucket list never shows alice's buckets**

```
GET http://localhost:8080/
```
Signed as bob. Expect `200 []` (or only bob's own buckets) — never
`my-bucket`.

**4.4 Tampered signature (expect 401)**

Sign a request as alice, then change the URL or body after signing (e.g.
edit the bucket name in the path but keep the original signature). Expect
`401 {"error":"signature verification failed"}`.

### 5. Cross-cutting edge cases

**5.1 Large-ish object upload (exercises streaming, not a tiny in-memory payload)**

Upload a ~10MB file to `my-bucket` (signed as alice, its owner). Download
it back and diff/checksum against the original — should match exactly.
(Verified during this task with a 10MB random file; sha256 matched on both
ends.) Note: api-service's SigV4 verifier does not hash streaming request
bodies itself (see `api-service/internal/sigv4/sigv4.go` package doc) —
this keeps uploads flowing straight through to data-service without being
buffered in memory, matching `docs/design-arch.md` §5.2's pass-through
streaming requirement.

🔎 **Verify on disk** (no lock issue — `data-service` doesn't use bbolt, see
the cookbook section above; it's fine to leave it running):
```sh
find data-service/data/my-bucket -type f
sha256sum data-service/data/my-bucket/objects/<hash[0:2]>/<hash[2:4]>/<hash>
```
using the `storage_ref` from the upload response (also present in
metadata-service's `objects` record — see 3.6's bbolt check) to fill in
`<hash>`. Expect the sha256 of the on-disk file to equal `storage_ref`
itself (it's content-addressed) and to equal the original file's own
sha256.

**5.2 Concurrent upload to the same key (last-write-wins, no corruption)**

Fire several concurrent signed `PUT` requests at the same object key with
different bodies. Then `GET` it back — expect the full, uncorrupted body of
exactly one payload, never a mix or a truncation. data-service writes
atomically (temp file → rename), so this is safe by construction.

**5.3 Malformed/missing required fields (expect 400)**

```
POST http://localhost:8082/v1/users/
Content-Type: application/json

{"username": "eve"}
```
See 2.2 — `400 {"error":"invalid input: password must be at least 8 characters"}`.

**5.4 Unauthenticated request (expect 401)**

See 3.4 (write) or issue any `GET` against api-service with no
`Authorization` header — every non-`/healthz` route requires a valid
signature.

---

## Summary of what this proves

Running through sections 1–4 above end-to-end demonstrates: all four
services start cleanly (auth-service and api-service each need one shared
secret configured; the other two need zero config), discover each other on
fixed local ports, and correctly handle the full authenticated bucket/object
lifecycle through api-service — including SigV4 signature verification,
owner-derived bucket creation, owner-only authorization between two
different real users, slash-containing keys, prefix filtering, streaming
uploads, and correct 401/403/404/409 propagation.
