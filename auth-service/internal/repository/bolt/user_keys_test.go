package bolt

import (
	"bytes"
	"testing"
)

func TestUserKey(t *testing.T) {
	got, err := UserKey("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []byte("user-1"); !bytes.Equal(got, want) {
		t.Fatalf("UserKey() = %q, want %q", got, want)
	}
}

func TestUserKey_RejectsSeparator(t *testing.T) {
	if _, err := UserKey("user\x001"); err == nil {
		t.Fatal("expected error for user_id containing separator, got nil")
	}
}

func TestUsernameKey(t *testing.T) {
	got, err := UsernameKey("alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []byte("alice"); !bytes.Equal(got, want) {
		t.Fatalf("UsernameKey() = %q, want %q", got, want)
	}
}

func TestUsernameKey_RejectsSeparator(t *testing.T) {
	if _, err := UsernameKey("ali\x00ce"); err == nil {
		t.Fatal("expected error for username containing separator, got nil")
	}
}

func TestUserKey_DoesNotCollideAcrossDifferentUsers(t *testing.T) {
	a, err := UserKey("user-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := UserKey("user-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatalf("UserKey(user-a) unexpectedly equals UserKey(user-b): %q", a)
	}
}
