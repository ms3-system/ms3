package api

import (
	"context"
	"encoding/json"
	"errors"
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
	return NewRouter(users, auth, credentials, nil, testInternalToken, newTestLogger(t))
}

// fakeAuthAs builds a fakeAuthService whose VerifyAccessToken always
// authenticates as the given principal, regardless of the token string
// presented — JWT parsing itself is covered by the service package's own
// tests, so handler tests only need to exercise what the API layer does
// with the resulting principal (self-or-admin authorization).
func fakeAuthAs(userID string, isAdmin bool) *fakeAuthService {
	return &fakeAuthService{
		verifyAccessTokenFn: func(_ string) (service.Principal, error) {
			return service.Principal{UserID: userID, IsAdmin: isAdmin}, nil
		},
	}
}

func withBearer(req *http.Request, token string) *http.Request {
	req.Header.Set("Authorization", "Bearer "+token)
	return req
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

func TestHealthzProbes(t *testing.T) {
	for _, path := range []string{"/healthz/live", "/healthz/ready", "/healthz/startup"} {
		t.Run(path, func(t *testing.T) {
			r := testRouter(t, nil, nil, nil)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestHealthzReady_DependencyDown(t *testing.T) {
	users := &fakeUserService{}
	auth := &fakeAuthService{}
	credentials := &fakeCredentialService{}
	r := NewRouter(users, auth, credentials, failingReadinessChecker{}, testInternalToken, newTestLogger(t))

	req := httptest.NewRequest(http.MethodGet, "/healthz/ready", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

type failingReadinessChecker struct{}

func (failingReadinessChecker) Ready(_ context.Context) error {
	return errors.New("store unavailable")
}

func TestCreateUser_Success(t *testing.T) {
	users := &fakeUserService{
		registerFn: func(_ context.Context, username, _ string) (model.User, error) {
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
		registerFn: func(_ context.Context, _, _ string) (model.User, error) {
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
		registerFn: func(_ context.Context, _, _ string) (model.User, error) {
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
		getUserFn: func(_ context.Context, id string) (model.User, error) {
			return model.User{ID: id, Username: "alice"}, nil
		},
	}
	r := testRouter(t, users, fakeAuthAs("user-1", false), nil)

	req := withBearer(httptest.NewRequest(http.MethodGet, "/v1/users/user-1", nil), "valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetUser_Success_AsAdminForAnotherUser(t *testing.T) {
	users := &fakeUserService{
		getUserFn: func(_ context.Context, id string) (model.User, error) {
			return model.User{ID: id, Username: "alice"}, nil
		},
	}
	r := testRouter(t, users, fakeAuthAs("admin-1", true), nil)

	req := withBearer(httptest.NewRequest(http.MethodGet, "/v1/users/user-1", nil), "valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	users := &fakeUserService{
		getUserFn: func(_ context.Context, _ string) (model.User, error) {
			return model.User{}, repository.ErrNotFound
		},
	}
	r := testRouter(t, users, fakeAuthAs("user-1", false), nil)

	req := withBearer(httptest.NewRequest(http.MethodGet, "/v1/users/user-1", nil), "valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetUser_Unauthorized_NoToken(t *testing.T) {
	r := testRouter(t, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/user-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetUser_Forbidden_MismatchedSubject(t *testing.T) {
	r := testRouter(t, nil, fakeAuthAs("user-2", false), nil)

	req := withBearer(httptest.NewRequest(http.MethodGet, "/v1/users/user-1", nil), "valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCreateCredential_Success(t *testing.T) {
	credentials := &fakeCredentialService{
		issueFn: func(_ context.Context, userID string) (service.IssuedCredential, error) {
			return service.IssuedCredential{AccessKey: "AKIAEXAMPLE", SecretKey: "secret", UserID: userID}, nil
		},
	}
	r := testRouter(t, nil, fakeAuthAs("user-1", false), credentials)

	req := withBearer(httptest.NewRequest(http.MethodPost, "/v1/users/user-1/credentials", nil), "valid-token")
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
		issueFn: func(_ context.Context, _ string) (service.IssuedCredential, error) {
			return service.IssuedCredential{}, repository.ErrNotFound
		},
	}
	r := testRouter(t, nil, fakeAuthAs("missing", false), credentials)

	req := withBearer(httptest.NewRequest(http.MethodPost, "/v1/users/missing/credentials", nil), "valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestCreateCredential_Unauthorized_NoToken(t *testing.T) {
	r := testRouter(t, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/users/user-1/credentials", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateCredential_Forbidden_MismatchedSubject(t *testing.T) {
	r := testRouter(t, nil, fakeAuthAs("user-2", false), nil)

	req := withBearer(httptest.NewRequest(http.MethodPost, "/v1/users/user-1/credentials", nil), "valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRevokeCredential_Success_Owner(t *testing.T) {
	var revoked string
	credentials := &fakeCredentialService{
		getOwnerFn: func(_ context.Context, _ string) (string, error) {
			return "user-1", nil
		},
		revokeFn: func(_ context.Context, accessKey string) error {
			revoked = accessKey
			return nil
		},
	}
	r := testRouter(t, nil, fakeAuthAs("user-1", false), credentials)

	req := withBearer(httptest.NewRequest(http.MethodDelete, "/v1/access-keys/AKIAEXAMPLE", nil), "valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if revoked != "AKIAEXAMPLE" {
		t.Errorf("revoked access key = %q, want %q", revoked, "AKIAEXAMPLE")
	}
}

func TestRevokeCredential_Success_Admin(t *testing.T) {
	var revoked string
	credentials := &fakeCredentialService{
		// getOwnerFn deliberately left unset: an admin must not need an
		// ownership lookup at all.
		revokeFn: func(_ context.Context, accessKey string) error {
			revoked = accessKey
			return nil
		},
	}
	r := testRouter(t, nil, fakeAuthAs("admin-1", true), credentials)

	req := withBearer(httptest.NewRequest(http.MethodDelete, "/v1/access-keys/AKIAEXAMPLE", nil), "valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if revoked != "AKIAEXAMPLE" {
		t.Errorf("revoked access key = %q, want %q", revoked, "AKIAEXAMPLE")
	}
}

func TestRevokeCredential_NotFound(t *testing.T) {
	credentials := &fakeCredentialService{
		getOwnerFn: func(_ context.Context, _ string) (string, error) {
			return "", repository.ErrNotFound
		},
	}
	r := testRouter(t, nil, fakeAuthAs("user-1", false), credentials)

	req := withBearer(httptest.NewRequest(http.MethodDelete, "/v1/access-keys/missing", nil), "valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRevokeCredential_Unauthorized_NoToken(t *testing.T) {
	r := testRouter(t, nil, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/v1/access-keys/AKIAEXAMPLE", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRevokeCredential_Forbidden_MismatchedOwner(t *testing.T) {
	credentials := &fakeCredentialService{
		getOwnerFn: func(_ context.Context, _ string) (string, error) {
			return "user-1", nil
		},
	}
	r := testRouter(t, nil, fakeAuthAs("user-2", false), credentials)

	req := withBearer(httptest.NewRequest(http.MethodDelete, "/v1/access-keys/AKIAEXAMPLE", nil), "valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLogin_Success(t *testing.T) {
	auth := &fakeAuthService{
		loginFn: func(_ context.Context, _, _ string) (string, string, error) {
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
		loginFn: func(_ context.Context, _, _ string) (string, string, error) {
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
		refreshFn: func(_ context.Context, _ string) (string, error) {
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
		refreshFn: func(_ context.Context, _ string) (string, error) {
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
		lookupFn: func(_ context.Context, _ string) (service.LookedUpCredential, error) {
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
		lookupFn: func(_ context.Context, _ string) (service.LookedUpCredential, error) {
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
