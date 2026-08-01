package bolt

import (
	"bytes"
	"testing"
)

func TestCredentialKey(t *testing.T) {
	got, err := CredentialKey("AKIAIOSFODNN7EXAMPLE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []byte("AKIAIOSFODNN7EXAMPLE"); !bytes.Equal(got, want) {
		t.Fatalf("CredentialKey() = %q, want %q", got, want)
	}
}

func TestCredentialKey_RejectsSeparator(t *testing.T) {
	if _, err := CredentialKey("AKIA\x00EXAMPLE"); err == nil {
		t.Fatal("expected error for access_key containing separator, got nil")
	}
}

func TestCredentialKey_DoesNotCollideAcrossDifferentAccessKeys(t *testing.T) {
	a, err := CredentialKey("AKIAAAAAAAAAAAAAAAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := CredentialKey("AKIABBBBBBBBBBBBBBBB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatalf("CredentialKey() unexpectedly collided: %q", a)
	}
}
