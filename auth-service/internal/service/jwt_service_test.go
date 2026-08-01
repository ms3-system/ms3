package service

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"auth-service/internal/model"
)

func TestJWTIssuer_MintAndParseAccessToken(t *testing.T) {
	issuer := newJWTIssuer([]byte("test-secret"))
	u := model.User{ID: "user-1", Username: "alice", IsAdmin: true}

	token, err := issuer.mintAccessToken(u)
	if err != nil {
		t.Fatalf("mintAccessToken() error = %v", err)
	}

	claims, err := issuer.parse(token, tokenTypeAccess)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	if claims.Subject != u.ID {
		t.Errorf("Subject = %q, want %q", claims.Subject, u.ID)
	}
	if claims.Type != tokenTypeAccess {
		t.Errorf("Type = %q, want %q", claims.Type, tokenTypeAccess)
	}
}

func TestJWTIssuer_Parse_RejectsExpiredToken(t *testing.T) {
	secret := []byte("test-secret")
	past := time.Now().Add(-time.Hour)
	claims := tokenClaims{
		Type: tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			IssuedAt:  jwt.NewNumericDate(past.Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(past),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	issuer := newJWTIssuer(secret)
	if _, err := issuer.parse(token, tokenTypeAccess); err == nil {
		t.Fatal("parse() should reject an expired token")
	}
}

func TestJWTIssuer_Parse_RejectsWrongType(t *testing.T) {
	issuer := newJWTIssuer([]byte("test-secret"))
	u := model.User{ID: "user-1"}

	token, err := issuer.mintRefreshToken(u)
	if err != nil {
		t.Fatalf("mintRefreshToken() error = %v", err)
	}

	if _, err := issuer.parse(token, tokenTypeAccess); err == nil {
		t.Fatal("parse() should reject a refresh token when access is wanted")
	}
}

func TestJWTIssuer_Parse_RejectsUnsignedAlg(t *testing.T) {
	claims := tokenClaims{
		Type: tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	issuer := newJWTIssuer([]byte("test-secret"))
	if _, err := issuer.parse(token, tokenTypeAccess); err == nil {
		t.Fatal("parse() should reject a token signed with alg=none")
	}
}
