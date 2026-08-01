package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func validMasterKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, masterKeySize))
}

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AUTH_SERVICE_JWT_SECRET", "test-jwt-secret")
	t.Setenv("AUTH_SERVICE_INTERNAL_TOKEN", "test-internal-token")
	t.Setenv("AUTH_SERVICE_MASTER_KEY", validMasterKey())
	t.Setenv("AUTH_SERVICE_DB_PATH", "")
}

func TestLoad_Success(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.JWTSecret != "test-jwt-secret" {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, "test-jwt-secret")
	}
	if cfg.InternalToken != "test-internal-token" {
		t.Errorf("InternalToken = %q, want %q", cfg.InternalToken, "test-internal-token")
	}
	if len(cfg.MasterKey) != masterKeySize {
		t.Errorf("len(MasterKey) = %d, want %d", len(cfg.MasterKey), masterKeySize)
	}
	if cfg.DBPath == "" {
		t.Error("DBPath should default to a non-empty value")
	}
}

func TestLoad_CustomDBPath(t *testing.T) {
	setValidEnv(t)
	t.Setenv("AUTH_SERVICE_DB_PATH", "custom/path.db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DBPath != "custom/path.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "custom/path.db")
	}
}

func TestLoad_MissingRequiredVars(t *testing.T) {
	tests := []struct {
		name   string
		unset  string
		errSub string
	}{
		{name: "missing jwt secret", unset: "AUTH_SERVICE_JWT_SECRET", errSub: "AUTH_SERVICE_JWT_SECRET"},
		{name: "missing internal token", unset: "AUTH_SERVICE_INTERNAL_TOKEN", errSub: "AUTH_SERVICE_INTERNAL_TOKEN"},
		{name: "missing master key", unset: "AUTH_SERVICE_MASTER_KEY", errSub: "AUTH_SERVICE_MASTER_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv(tt.unset, "")

			_, err := Load()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestLoad_MasterKeyNotBase64(t *testing.T) {
	setValidEnv(t)
	t.Setenv("AUTH_SERVICE_MASTER_KEY", "not-valid-base64!!!")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoad_MasterKeyWrongSize(t *testing.T) {
	setValidEnv(t)
	t.Setenv("AUTH_SERVICE_MASTER_KEY", base64.StdEncoding.EncodeToString(make([]byte, 16)))

	_, err := Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "32 bytes") {
		t.Errorf("error = %q, want substring %q", err.Error(), "32 bytes")
	}
}
