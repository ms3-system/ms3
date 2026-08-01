# 0001. Plaintext-recoverable secrets for SigV4, encrypted at rest with AES-GCM

## Status

Accepted

## Context

`auth-service` stores two kinds of credentials:

- **User passwords**, verified by `POST /v1/auth/login`.
- **S3 secret keys**, verified indirectly by `api-service` on every SigV4-signed request via
  `GET /internal/credentials/{access_key}`.

Passwords are hashed one-way with bcrypt — the server never needs the plaintext password again,
only a yes/no answer to "does this input hash to the same value." That's the textbook case for a
one-way hash, and it's what `model.User.PasswordHash` does.

The S3 secret key cannot use the same approach. SigV4 authenticates a request by having the
*client* compute an HMAC-SHA256 signature over the canonical request, keyed by the secret key, and
having the *server* recompute that same HMAC independently and compare. Recomputing an HMAC
requires the actual secret key as the HMAC key — there is no way to verify an HMAC against only a
hash of the key, the way `bcrypt.CompareHashAndPassword` verifies a password against only its
hash. If `auth-service` only ever stored `bcrypt(secret_key)`, `api-service` would have no way to
independently derive the same signature the client produced, and SigV4 verification would be
impossible.

So the secret key must be recoverable in plaintext by the server, on demand, for every signed
request. That's a fundamentally different requirement from "prove the caller knows a secret"
(passwords) — it's "reconstruct the secret to feed into a keyed HMAC" (SigV4). Storing it as
plaintext outright was rejected outright: a leaked `auth.db` file or a leaked bbolt backup would
hand over every S3 secret key in the system with zero additional work from an attacker.

## Decision

Encrypt the secret key at rest with AES-256-GCM before storing it, using a master key supplied via
the `AUTH_SERVICE_MASTER_KEY` environment variable (32 bytes, base64-encoded — never checked into
source, never logged). `Credential.SecretKeyEncrypted` stores `base64(nonce || ciphertext)`, with
a fresh random 12-byte nonce generated per encryption (see `internal/service/credential_service.go`).

This means the plaintext secret is only ever reconstructed in memory, transiently, in exactly two
places:

1. `POST /v1/users/{id}/credentials` — right after generation, before encrypting it for storage,
   to return it to the caller once.
2. `GET /internal/credentials/{access_key}` — decrypted on each lookup, to hand to `api-service`
   for SigV4 verification.

GCM was chosen over a non-authenticated mode (e.g. plain AES-CTR) because it's authenticated —
`gcm.Open` fails closed if the ciphertext has been tampered with (bit-flipped, truncated, or
swapped with a different record's ciphertext), rather than silently returning corrupted "secret
key" bytes that would just fail every subsequent signature check in a confusing way.

## Consequences

- **Anyone who obtains both `AUTH_SERVICE_MASTER_KEY` and the bbolt file can recover every secret
  key.** This is an inherent property of "the server can reconstruct the secret," not something
  AES-GCM avoids — it's the same shape of risk AWS itself accepts for IAM secret access keys. The
  mitigation is keeping the master key out of the data volume (a separate env var / secrets
  manager entry, never written to `auth.db`) and restricting who can read `auth.db` at all.
- **No key rotation in v1** (see `docs/design.md` §7) — one static master key, no key-version
  prefix on ciphertexts. If the master key is ever compromised, every stored credential must be
  re-issued (there's no way to re-encrypt in place with a new key without also decrypting with the
  old one first, which this design already supports — decrypting existing records with the
  original key remains possible for exactly that reason).
- **Future option, not built now**: a rotation scheme where each ciphertext is prefixed with a
  short key-version identifier, and the decrypt path looks up the corresponding historical key by
  that identifier. This lets new writes use a new key while old records stay readable until
  they're naturally re-issued or migrated. Deferred because v1 has exactly one key and building
  the versioning machinery for a rotation that doesn't happen yet is pure speculative complexity —
  add it when a real rotation need shows up.
- `GET /internal/credentials/{access_key}` is, by construction, the single highest-value target in
  the system — it's the only endpoint that ever returns decrypted secret material. Its network and
  application-level gating (§8 of `docs/design.md`) exists specifically because of this decision;
  the two documents should be read together.

## Alternatives considered

- **bcrypt/argon2 the secret key, like the password.** Rejected — see Context. SigV4 fundamentally
  cannot be verified against a one-way hash of the key.
- **Store plaintext, no encryption.** Rejected — turns any read access to the DB file (backup,
  misconfigured volume mount, stolen disk) into a full credential-material breach with no
  additional attacker effort.
- **Envelope encryption with a KMS-managed key instead of a static env-var key.** More appropriate
  once this runs somewhere with a real KMS available; overkill for the current single-container
  deployment target. The `AUTH_SERVICE_MASTER_KEY` env var is a placeholder for that upgrade, not
  a permanent design commitment.
