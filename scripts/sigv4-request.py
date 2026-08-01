#!/usr/bin/env python3
"""Send a SigV4-signed request to a locally-running api-service.

Stdlib only (hashlib/hmac/urllib) — no dependencies to install. Used for
curl-style manual testing of api-service, which requires every request to
carry a valid "Authorization: AWS4-HMAC-SHA256 ..." header (see
docs/local-integration-testing.md). For interactive testing, Postman/Apidog's
built-in "AWS Signature" auth type is usually more convenient — this script
exists for scripted/CLI testing and for exercises that need a large or
binary request body.

Region/service are fixed to "us-east-1"/"s3", matching api-service's
internal/sigv4 verifier (see api-service/api/auth_middleware.go) — this is a
small local system, not a multi-region deployment, so there's nothing else
for a client to legitimately choose.

Example:
    python3 scripts/sigv4-request.py \\
        --access-key AKIA... --secret-key ... \\
        --method PUT --url http://localhost:8080/buckets/my-bucket/objects/hello.txt \\
        --content-type text/plain --data "hello world"
"""

import argparse
import datetime
import hashlib
import hmac
import sys
import urllib.error
import urllib.parse
import urllib.request

REGION = "us-east-1"
SERVICE = "s3"
ALGORITHM = "AWS4-HMAC-SHA256"


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def hmac_sha256(key: bytes, data: str) -> bytes:
    return hmac.new(key, data.encode("utf-8"), hashlib.sha256).digest()


def uri_encode(s: str, encode_slash: bool) -> str:
    unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.~"
    out = []
    for ch in s.encode("utf-8"):
        c = chr(ch)
        if c in unreserved or (c == "/" and not encode_slash):
            out.append(c)
        else:
            out.append("%%%02X" % ch)
    return "".join(out)


def canonical_query_string(query: str) -> str:
    pairs = urllib.parse.parse_qsl(query, keep_blank_values=True)
    encoded = sorted((uri_encode(k, True), uri_encode(v, True)) for k, v in pairs)
    return "&".join(f"{k}={v}" for k, v in encoded)


def sign_request(method, url, access_key, secret_key, headers, body: bytes):
    parsed = urllib.parse.urlsplit(url)
    now = datetime.datetime.now(datetime.timezone.utc)
    amz_date = now.strftime("%Y%m%dT%H%M%SZ")
    date_stamp = now.strftime("%Y%m%d")

    headers = dict(headers)
    headers["host"] = parsed.netloc
    headers["x-amz-date"] = amz_date
    headers.setdefault("x-amz-content-sha256", "UNSIGNED-PAYLOAD")

    signed_header_names = sorted(headers.keys())
    canonical_headers = "".join(f"{h}:{headers[h].strip()}\n" for h in signed_header_names)
    signed_headers = ";".join(signed_header_names)

    canonical_uri = uri_encode(parsed.path or "/", False)
    canonical_qs = canonical_query_string(parsed.query)
    content_sha256 = headers["x-amz-content-sha256"]

    canonical_request = "\n".join([
        method.upper(), canonical_uri, canonical_qs, canonical_headers, signed_headers, content_sha256,
    ])

    credential_scope = f"{date_stamp}/{REGION}/{SERVICE}/aws4_request"
    string_to_sign = "\n".join([
        ALGORITHM, amz_date, credential_scope, sha256_hex(canonical_request.encode("utf-8")),
    ])

    k_date = hmac_sha256(("AWS4" + secret_key).encode("utf-8"), date_stamp)
    k_region = hmac_sha256(k_date, REGION)
    k_service = hmac_sha256(k_region, SERVICE)
    k_signing = hmac_sha256(k_service, "aws4_request")
    signature = hmac.new(k_signing, string_to_sign.encode("utf-8"), hashlib.sha256).hexdigest()

    auth_header = (
        f"{ALGORITHM} Credential={access_key}/{credential_scope}, "
        f"SignedHeaders={signed_headers}, Signature={signature}"
    )
    headers["authorization"] = auth_header
    return headers


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--access-key", required=True)
    p.add_argument("--secret-key", required=True)
    p.add_argument("--method", default="GET")
    p.add_argument("--url", required=True)
    p.add_argument("--content-type", default="")
    p.add_argument("--data", default=None, help="request body as a string")
    p.add_argument("--data-file", default=None, help="path to a file to use as the request body")
    p.add_argument("--unsigned-payload", action="store_true", default=True,
                    help="set X-Amz-Content-Sha256: UNSIGNED-PAYLOAD (default; safe for streaming uploads)")
    args = p.parse_args()

    body = b""
    if args.data_file:
        with open(args.data_file, "rb") as f:
            body = f.read()
    elif args.data is not None:
        body = args.data.encode("utf-8")

    extra_headers = {}
    if args.content_type:
        extra_headers["content-type"] = args.content_type

    signed = sign_request(args.method, args.url, args.access_key, args.secret_key, extra_headers, body)

    req = urllib.request.Request(args.url, data=body if body else None, method=args.method.upper())
    for k, v in signed.items():
        req.add_header(k, v)

    try:
        with urllib.request.urlopen(req) as resp:
            print(f"status: {resp.status}", file=sys.stderr)
            sys.stdout.buffer.write(resp.read())
    except urllib.error.HTTPError as e:
        print(f"status: {e.code}", file=sys.stderr)
        sys.stdout.buffer.write(e.read())
        sys.exit(1)


if __name__ == "__main__":
    main()
