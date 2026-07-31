package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"auth-service/internal/model"
	"auth-service/internal/repository"
	"auth-service/internal/service"
)

const testInternalToken = "test-internal-token"

func testRouter(t *testing.T, users service.UserService, auth service.AuthService, credentials service.CredentialService) http.Handler {
	t.Helper()
	if users == nil {
		users = &fakeUserService{}
	}
	if auth == nil {
		auth = &fakeAuthService{}
	}
	if credentials == nil {
		credentials = &fakeCredentialService{}
	}
	return NewRouter(users, auth, credentials, testInternalToken, newTestLogger(t))
}

func TestHealthz(t *testing.T) {
	r := testRouter(t, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCreateUser_Success(t *testing.T) {
	users := &fakeUserService{
		registerFn: func(ctx context.Context, username, password string) (model.User, error) {
			return model.User{ID: "user-1", Username: username, CreatedAt: time.Now()}, nil
		},
	}
	r := testRouter(t, users, nil, nil)

	body := strings.NewReader(`{"username":"alice","password":"hunter22"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, present := resp["password_hash"]; present {
		t.Error("response must not include password_hash")
	}
	if _, present := resp["password"]; present {
		t.Error("response must not include password")
	}
	if resp["username"] != "alice" {
		t.Errorf("username = %v, want %q", resp["username"], "alice")
	}
}

func TestCreateUser_InvalidJSON(t *testing.T) {
	r := testRouter(t, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateUser_InvalidInput(t *testing.T) {
	users := &fakeUserService{
		registerFn: func(ctx context.Context, username, password string) (model.User, error) {
			return model.User{}, service.ErrInvalidInput
		},
	}
	r := testRouter(t, users, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(`{"username":"a","password":"x"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	users := &fakeUserService{
		registerFn: func(ctx context.Context, username, password string) (model.User, error) {
			return model.User{}, repository.ErrAlreadyExists
		},
	}
	r := testRouter(t, users, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(`{"username":"alice","password":"hunter22"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestGetUser_Success(t *testing.T) {
	users := &fakeUserService{
		getUserFn: func(ctx context.Context, id string) (model.User, error) {
			return model.User{ID: id, Username: "alice"}, nil
		},
	}
	r := testRouter(t, users, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/user-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	users := &fakeUserService{
		getUserFn: func(ctx context.Context, id string) (model.User, error) {
			return model.User{}, repository.ErrNotFound
		},
	}
	r := testRouter(t, users, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/missing", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestCreateCredential_Success(t *testing.T) {
	credentials := &fakeCredentialService{
		issueFn: func(ctx context.Context, userID string) (service.IssuedCredential, error) {
			return service.IssuedCredential{AccessKey: "AKIAEXAMPLE", SecretKey: "secret", UserID: userID}, nil
		},
	}
	r := testRouter(t, nil, nil, credentials)

	req := httptest.NewRequest(http.MethodPost, "/v1/users/user-1/credentials", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp credentialIssuedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.SecretKey != "secret" {
		t.Errorf("secret_key = %q, want %q", resp.SecretKey, "secret")
	}
}

func TestCreateCredential_UserNotFound(t *testing.T) {
	credentials := &fakeCredentialService{
		issueFn: func(ctx context.Context, userID string) (service.IssuedCredential, error) {
			return service.IssuedCredential{}, repository.ErrNotFound
		},
	}
	r := testRouter(t, nil, nil, credentials)

	req := httptest.NewRequest(http.MethodPost, "/v1/users/missing/credentials", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRevokeCredential_Success(t *testing.T) {
	var revoked string
	credentials := &fakeCredentialService{
		revokeFn: func(ctx context.Context, accessKey string) error {
			revoked = accessKey
			return nil
		},
	}
	r := testRouter(t, nil, nil, credentials)

	req := httptest.NewRequest(http.MethodDelete, "/v1/access-keys/AKIAEXAMPLE", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if revoked != "AKIAEXAMPLE" {
		t.Errorf("revoked access key = %q, want %q", revoked, "AKIAEXAMPLE")
	}
}

func TestRevokeCredential_NotFound(t *testing.T) {
	credentials := &fakeCredentialService{
		revokeFn: func(ctx context.Context, accessKey string) error {
			return repository.ErrNotFound
		},
	}
	r := testRouter(t, nil, nil, credentials)

	req := httptest.NewRequest(http.MethodDelete, "/v1/access-keys/missing", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestLogin_Success(t *testing.T) {
	auth := &fakeAuthService{
		loginFn: func(ctx context.Context, username, password string) (string, string, error) {
			return "access-token", "refresh-token", nil
		},
	}
	r := testRouter(t, nil, auth, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"username":"alice","password":"hunter22"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.AccessToken != "access-token" || resp.RefreshToken != "refresh-token" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	auth := &fakeAuthService{
		loginFn: func(ctx context.Context, username, password string) (string, string, error) {
			return "", "", service.ErrInvalidCredentials
		},
	}
	r := testRouter(t, nil, auth, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"username":"alice","password":"wrong"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	r := testRouter(t, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRefresh_Success(t *testing.T) {
	auth := &fakeAuthService{
		refreshFn: func(ctx context.Context, refreshToken string) (string, error) {
			return "new-access-token", nil
		},
	}
	r := testRouter(t, nil, auth, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader(`{"refresh_token":"rt"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRefresh_InvalidCredentials(t *testing.T) {
	auth := &fakeAuthService{
		refreshFn: func(ctx context.Context, refreshToken string) (string, error) {
			return "", service.ErrInvalidCredentials
		},
	}
	r := testRouter(t, nil, auth, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader(`{"refresh_token":"bad"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestInternalLookupCredential_MissingToken(t *testing.T) {
	r := testRouter(t, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/internal/credentials/AKIAEXAMPLE", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (missing X-Internal-Token must be rejected)", rec.Code, http.StatusUnauthorized)
	}
}

func TestInternalLookupCredential_WrongToken(t *testing.T) {
	r := testRouter(t, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/internal/credentials/AKIAEXAMPLE", nil)
	req.Header.Set("X-Internal-Token", "wrong-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestInternalLookupCredential_Success(t *testing.T) {
	credentials := &fakeCredentialService{
		lookupFn: func(ctx context.Context, accessKey string) (service.LookedUpCredential, error) {
			return service.LookedUpCredential{UserID: "user-1", SecretKey: "plaintext-secret"}, nil
		},
	}
	r := testRouter(t, nil, nil, credentials)

	req := httptest.NewRequest(http.MethodGet, "/internal/credentials/AKIAEXAMPLE", nil)
	req.Header.Set("X-Internal-Token", testInternalToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp internalCredentialResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.SecretKey != "plaintext-secret" {
		t.Errorf("secret_key = %q, want %q", resp.SecretKey, "plaintext-secret")
	}
}

func TestInternalLookupCredential_NotFoundWithValidToken(t *testing.T) {
	credentials := &fakeCredentialService{
		lookupFn: func(ctx context.Context, accessKey string) (service.LookedUpCredential, error) {
			return service.LookedUpCredential{}, repository.ErrNotFound
		},
	}
	r := testRouter(t, nil, nil, credentials)

	req := httptest.NewRequest(http.MethodGet, "/internal/credentials/missing", nil)
	req.Header.Set("X-Internal-Token", testInternalToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestInternalLookupCredential_NotReachableFromPublicPrefix(t *testing.T) {
	r := testRouter(t, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/internal/credentials/AKIAEXAMPLE", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (no route should alias /internal under /v1)", rec.Code, http.StatusNotFound)
	}
}
