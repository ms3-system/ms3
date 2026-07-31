package bolt

import (
	"bytes"
	"testing"
)

func TestOwnerIndexKey(t *testing.T) {
	got, err := ownerIndexKey("user-1", "my-bucket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []byte("user-1\x00my-bucket")
	if !bytes.Equal(got, want) {
		t.Fatalf("ownerIndexKey() = %q, want %q", got, want)
	}
}

func TestOwnerIndexKey_RejectsSeparatorInComponents(t *testing.T) {
	if _, err := ownerIndexKey("user\x001", "my-bucket"); err == nil {
		t.Fatal("expected error for owner_id containing separator, got nil")
	}
	if _, err := ownerIndexKey("user-1", "my\x00bucket"); err == nil {
		t.Fatal("expected error for bucket_name containing separator, got nil")
	}
}

func TestOwnerIndexPrefix(t *testing.T) {
	got, err := ownerIndexPrefix("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []byte("user-1\x00")
	if !bytes.Equal(got, want) {
		t.Fatalf("ownerIndexPrefix() = %q, want %q", got, want)
	}
}

func TestOwnerIndexPrefix_IsPrefixOfOwnerIndexKey(t *testing.T) {
	prefix, err := ownerIndexPrefix("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fullKey, err := ownerIndexKey("user-1", "my-bucket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.HasPrefix(fullKey, prefix) {
		t.Fatalf("ownerIndexKey() %q does not have prefix %q", fullKey, prefix)
	}
}

func TestOwnerIndexPrefix_DoesNotMatchDifferentOwner(t *testing.T) {
	prefix, err := ownerIndexPrefix("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	otherOwnerKey, err := ownerIndexKey("user-2", "my-bucket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bytes.HasPrefix(otherOwnerKey, prefix) {
		t.Fatalf("owner-1 prefix unexpectedly matched owner-2's key %q", otherOwnerKey)
	}
}
