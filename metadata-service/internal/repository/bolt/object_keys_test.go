package bolt

import (
	"bytes"
	"testing"
)

func TestObjectKey(t *testing.T) {
	got, err := ObjectKey("my-bucket", "photos/2024/img.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []byte("my-bucket\x00photos/2024/img.png")
	if !bytes.Equal(got, want) {
		t.Fatalf("objectKey() = %q, want %q", got, want)
	}
}

func TestObjectKey_RejectsSeparatorInComponents(t *testing.T) {
	if _, err := ObjectKey("my\x00bucket", "key"); err == nil {
		t.Fatal("expected error for bucket_name containing separator, got nil")
	}
	if _, err := ObjectKey("my-bucket", "evil\x00key"); err == nil {
		t.Fatal("expected error for object_key containing separator, got nil")
	}
}

func TestObjectListPrefix_EmptyPrefixListsWholeBucket(t *testing.T) {
	got, err := ObjectListPrefix("my-bucket", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []byte("my-bucket\x00")
	if !bytes.Equal(got, want) {
		t.Fatalf("objectListPrefix() = %q, want %q", got, want)
	}
}

func TestObjectListPrefix_WithUserPrefix(t *testing.T) {
	got, err := ObjectListPrefix("my-bucket", "photos/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []byte("my-bucket\x00photos/")
	if !bytes.Equal(got, want) {
		t.Fatalf("objectListPrefix() = %q, want %q", got, want)
	}
}

func TestObjectListPrefix_IsPrefixOfObjectKey(t *testing.T) {
	prefix, err := ObjectListPrefix("my-bucket", "photos/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fullKey, err := ObjectKey("my-bucket", "photos/2024/img.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.HasPrefix(fullKey, prefix) {
		t.Fatalf("objectKey() %q does not have prefix %q", fullKey, prefix)
	}
}

func TestObjectListPrefix_DoesNotMatchDifferentBucket(t *testing.T) {
	prefix, err := ObjectListPrefix("bucket-a", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	otherBucketKey, err := ObjectKey("bucket-b", "img.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bytes.HasPrefix(otherBucketKey, prefix) {
		t.Fatalf("bucket-a prefix unexpectedly matched bucket-b's key %q", otherBucketKey)
	}
}

func TestObjectListPrefix_DoesNotMatchSimilarBucketName(t *testing.T) {
	prefix, err := ObjectListPrefix("my-bucket", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	similarBucketKey, err := ObjectKey("my-bucket-2", "img.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bytes.HasPrefix(similarBucketKey, prefix) {
		t.Fatalf("my-bucket prefix unexpectedly matched my-bucket-2's key %q", similarBucketKey)
	}
}
