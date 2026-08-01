package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"auth-service/internal/model"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour

	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"

	// minJWTSecretLength matches HS256's 256-bit output size (RFC 2104:
	// an HMAC key shorter than the hash output doesn't add security
	// margin). Enforced in NewAuthService, not here — this file stays
	// focused on constructing jwtIssuer from an already-validated secret.
	minJWTSecretLength = 32
)

type tokenClaims struct {
	Username string `json:"username,omitempty"`
	IsAdmin  bool   `json:"is_admin,omitempty"`
	Type     string `json:"typ"`
	jwt.RegisteredClaims
}

type jwtIssuer struct {
	secret []byte
}

func newJWTIssuer(secret []byte) jwtIssuer {
	return jwtIssuer{secret: secret}
}

func (j jwtIssuer) mintAccessToken(u model.User) (string, error) {
	now := time.Now().UTC()
	claims := tokenClaims{
		Username: u.Username,
		IsAdmin:  u.IsAdmin,
		Type:     tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
}

func (j jwtIssuer) mintRefreshToken(u model.User) (string, error) {
	now := time.Now().UTC()
	claims := tokenClaims{
		Type: tokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
}

func (j jwtIssuer) parse(tokenString, wantType string) (*tokenClaims, error) {
	claims := &tokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.Type != wantType {
		return nil, fmt.Errorf("unexpected token type %q, want %q", claims.Type, wantType)
	}
	return claims, nil
}
