package sigv4

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	testRegion  = "us-east-1"
	testService = "s3"
)

// sign mirrors what a real client does: build the same canonical
// request/string-to-sign/signing-key chain that Verify uses, so these
// tests catch a broken canonicalization as readily as a broken comparison.
func sign(r *http.Request, secretKey, date, signedHeaders string) string {
	canonicalRequest := buildCanonicalRequest(r, strings.Split(signedHeaders, ";"))
	hashed := sha256Hex([]byte(canonicalRequest))
	scope := date[:8] + "/" + testRegion + "/" + testService + "/aws4_request"
	stringToSign := Algorithm + "\n" + date + "\n" + scope + "\n" + hashed
	key := deriveSigningKey(secretKey, date[:8], testRegion, testService)
	sig := hmacSHA256(key, []byte(stringToSign))
	return hex.EncodeToString(sig)
}

func newSignedRequest(t *testing.T, method, target, accessKey, secretKey string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, target, nil)
	r.Host = "localhost:8080"
	date := time.Now().UTC().Format(DateFormat)
	r.Header.Set("X-Amz-Date", date)
	r.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	sig := sign(r, secretKey, date, signedHeaders)

	authHeader := Algorithm + " Credential=" + accessKey + "/" + date[:8] + "/" + testRegion + "/" + testService +
		"/aws4_request, SignedHeaders=" + signedHeaders + ", Signature=" + sig
	r.Header.Set("Authorization", authHeader)
	return r
}

func TestVerify_ValidSignatureSucceeds(t *testing.T) {
	r := newSignedRequest(t, http.MethodPut, "/buckets/my-bucket/objects/photo.png", "AKIAEXAMPLE", "s3cr3t")

	cred, err := ParseAuthorization(r.Header.Get("Authorization"))
	if err != nil {
		t.Fatalf("ParseAuthorization: %v", err)
	}
	if err := Verify(r, cred, "s3cr3t", 15*time.Minute); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerify_WrongSecretKeyFails(t *testing.T) {
	r := newSignedRequest(t, http.MethodPut, "/buckets/my-bucket/objects/photo.png", "AKIAEXAMPLE", "s3cr3t")

	cred, err := ParseAuthorization(r.Header.Get("Authorization"))
	if err != nil {
		t.Fatalf("ParseAuthorization: %v", err)
	}
	if err := Verify(r, cred, "wrong-secret", 15*time.Minute); err == nil {
		t.Fatal("expected signature mismatch, got nil")
	}
}

func TestVerify_TamperedPathFails(t *testing.T) {
	r := newSignedRequest(t, http.MethodGet, "/buckets/my-bucket/objects/photo.png", "AKIAEXAMPLE", "s3cr3t")

	cred, err := ParseAuthorization(r.Header.Get("Authorization"))
	if err != nil {
		t.Fatalf("ParseAuthorization: %v", err)
	}

	// Simulate a MITM changing the target object after signing.
	r.URL.Path = "/buckets/my-bucket/objects/other.png"

	if err := Verify(r, cred, "s3cr3t", 15*time.Minute); err == nil {
		t.Fatal("expected signature mismatch after path tampering, got nil")
	}
}

func TestVerify_ExpiredRequestFails(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.Host = "localhost:8080"
	old := time.Now().UTC().Add(-1 * time.Hour).Format(DateFormat)
	r.Header.Set("X-Amz-Date", old)
	r.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	sig := sign(r, "s3cr3t", old, signedHeaders)
	r.Header.Set("Authorization", Algorithm+" Credential=AKIAEXAMPLE/"+old[:8]+"/"+testRegion+"/"+testService+
		"/aws4_request, SignedHeaders="+signedHeaders+", Signature="+sig)

	cred, err := ParseAuthorization(r.Header.Get("Authorization"))
	if err != nil {
		t.Fatalf("ParseAuthorization: %v", err)
	}
	if err := Verify(r, cred, "s3cr3t", 15*time.Minute); err != ErrRequestExpired {
		t.Fatalf("Verify error = %v, want ErrRequestExpired", err)
	}
}

func TestParseAuthorization_Malformed(t *testing.T) {
	_, err := ParseAuthorization("Bearer abc123")
	if err != ErrMalformedAuthHeader {
		t.Fatalf("err = %v, want ErrMalformedAuthHeader", err)
	}
}
