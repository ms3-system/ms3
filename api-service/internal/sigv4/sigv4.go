// Package sigv4 implements enough of AWS Signature Version 4 to
// authenticate requests against api-service: parsing the "Authorization:
// AWS4-HMAC-SHA256 ..." header, building the canonical request from the
// signed headers, deriving the signing key, and comparing signatures.
//
// One deliberate simplification: the request payload hash
// (X-Amz-Content-Sha256) is taken as given and folded into the canonical
// request like any other signed value, but is never independently
// recomputed from the body. api-service streams upload bodies straight
// through to data-service without buffering (see docs/design-arch.md §5.2),
// so re-hashing the body here would defeat that. This mirrors AWS's own
// UNSIGNED-PAYLOAD mode, just applied uniformly rather than as a special
// case — the signature still covers the method, path, query, and headers.
package sigv4

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	Algorithm  = "AWS4-HMAC-SHA256"
	DateFormat = "20060102T150405Z"
)

var (
	ErrMalformedAuthHeader = errors.New("malformed Authorization header")
	ErrMissingDate         = errors.New("missing X-Amz-Date header")
	ErrRequestExpired      = errors.New("request timestamp outside the allowed window")
	ErrSignatureMismatch   = errors.New("signature does not match")
)

// Credential is the parsed "Credential=" + "SignedHeaders=" + "Signature="
// portion of an Authorization header.
type Credential struct {
	AccessKey     string
	Date          string // YYYYMMDD
	Region        string
	Service       string
	SignedHeaders []string
	Signature     string
}

// ParseAuthorization parses:
//
//	AWS4-HMAC-SHA256 Credential=<key>/<date>/<region>/<service>/aws4_request, SignedHeaders=<h1;h2>, Signature=<hex>
func ParseAuthorization(header string) (*Credential, error) {
	if !strings.HasPrefix(header, Algorithm+" ") {
		return nil, ErrMalformedAuthHeader
	}
	rest := strings.TrimPrefix(header, Algorithm+" ")

	fields := map[string]string{}
	for part := range strings.SplitSeq(rest, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return nil, ErrMalformedAuthHeader
		}
		fields[kv[0]] = kv[1]
	}

	credential, ok := fields["Credential"]
	signedHeaders, ok2 := fields["SignedHeaders"]
	signature, ok3 := fields["Signature"]
	if !ok || !ok2 || !ok3 || credential == "" || signedHeaders == "" || signature == "" {
		return nil, ErrMalformedAuthHeader
	}

	credParts := strings.Split(credential, "/")
	if len(credParts) != 5 || credParts[4] != "aws4_request" {
		return nil, ErrMalformedAuthHeader
	}

	headers := strings.Split(signedHeaders, ";")
	for i, h := range headers {
		headers[i] = strings.ToLower(strings.TrimSpace(h))
	}
	sort.Strings(headers)

	return &Credential{
		AccessKey:     credParts[0],
		Date:          credParts[1],
		Region:        credParts[2],
		Service:       credParts[3],
		SignedHeaders: headers,
		Signature:     signature,
	}, nil
}

// Verify checks a parsed credential against the live request and the
// caller's secret key. It never reads r.Body, so it's safe to call before
// a streaming request body has been consumed.
func Verify(r *http.Request, cred *Credential, secretKey string, maxSkew time.Duration) error {
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return ErrMissingDate
	}
	ts, err := time.Parse(DateFormat, amzDate)
	if err != nil {
		return fmt.Errorf("invalid X-Amz-Date: %w", err)
	}
	if skew := time.Since(ts); skew > maxSkew || skew < -maxSkew {
		return ErrRequestExpired
	}

	canonicalRequest := buildCanonicalRequest(r, cred.SignedHeaders)
	hashedCanonicalRequest := sha256Hex([]byte(canonicalRequest))

	credentialScope := strings.Join([]string{cred.Date, cred.Region, cred.Service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{Algorithm, amzDate, credentialScope, hashedCanonicalRequest}, "\n")

	signingKey := deriveSigningKey(secretKey, cred.Date, cred.Region, cred.Service)
	expected := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(cred.Signature))) {
		return ErrSignatureMismatch
	}
	return nil
}

func buildCanonicalRequest(r *http.Request, signedHeaders []string) string {
	var headerLines []string
	for _, h := range signedHeaders {
		var values []string
		if h == "host" {
			host := r.Host
			if host == "" {
				host = r.Header.Get("Host")
			}
			values = []string{host}
		} else {
			values = r.Header.Values(h)
		}
		normalized := make([]string, len(values))
		for i, v := range values {
			normalized[i] = collapseWhitespace(strings.TrimSpace(v))
		}
		headerLines = append(headerLines, h+":"+strings.Join(normalized, ",")+"\n")
	}
	canonicalHeaders := strings.Join(headerLines, "")

	contentSha256 := r.Header.Get("X-Amz-Content-Sha256")
	if contentSha256 == "" {
		contentSha256 = "UNSIGNED-PAYLOAD"
	}

	return strings.Join([]string{
		r.Method,
		canonicalURI(r.URL.Path),
		canonicalQueryString(r.URL.Query()),
		canonicalHeaders,
		strings.Join(signedHeaders, ";"),
		contentSha256,
	}, "\n")
}

func canonicalURI(path string) string {
	if path == "" {
		return "/"
	}
	return uriEncode(path, false)
}

func canonicalQueryString(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		values := append([]string(nil), q[k]...)
		sort.Strings(values)
		for _, v := range values {
			parts = append(parts, uriEncode(k, true)+"="+uriEncode(v, true))
		}
	}
	return strings.Join(parts, "&")
}

// uriEncode implements AWS's canonical URI-encoding: unreserved characters
// (A-Z a-z 0-9 - _ . ~) are left alone, everything else is percent-encoded
// with uppercase hex. '/' is preserved when encodeSlash is false (used for
// paths; S3 does not normalize or double-encode object key path segments).
func uriEncode(s string, encodeSlash bool) string {
	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch {
		case isUnreserved(b):
			buf.WriteByte(b)
		case b == '/' && !encodeSlash:
			buf.WriteByte(b)
		default:
			fmt.Fprintf(&buf, "%%%02X", b)
		}
	}
	return buf.String()
}

func isUnreserved(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') ||
		b == '-' || b == '_' || b == '.' || b == '~'
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func deriveSigningKey(secretKey, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
