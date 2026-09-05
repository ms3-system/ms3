package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"metadata-service/internal/model"
	"metadata-service/internal/repository"
	"metadata-service/internal/service"
)

func newTestRouter(t *testing.T, buckets service.BucketService, objects service.ObjectService) http.Handler {
	t.Helper()
	return NewRouter(buckets, objects, nil, newTestLogger(t))
}

func doRequest(t *testing.T, r http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(encoded)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, target, reqBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestHealthz(t *testing.T) {
	r := newTestRouter(t, &fakeBucketService{}, &fakeObjectService{})

	rec := doRequest(t, r, http.MethodGet, "/healthz", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHealthzProbes(t *testing.T) {
	for _, path := range []string{"/healthz/live", "/healthz/ready", "/healthz/startup"} {
		t.Run(path, func(t *testing.T) {
			r := newTestRouter(t, &fakeBucketService{}, &fakeObjectService{})

			rec := doRequest(t, r, http.MethodGet, path, nil)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestHealthzReady_DependencyDown(t *testing.T) {
	r := NewRouter(&fakeBucketService{}, &fakeObjectService{}, failingReadinessChecker{}, newTestLogger(t))

	rec := doRequest(t, r, http.MethodGet, "/healthz/ready", nil)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

type failingReadinessChecker struct{}

func (failingReadinessChecker) Ready(ctx context.Context) error {
	return errors.New("store unavailable")
}

func TestCreateBucket_Success(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	buckets := &fakeBucketService{
		createFn: func(_ context.Context, name, ownerID string) (model.Bucket, error) {
			return model.Bucket{ID: "b-1", Name: name, OwnerID: ownerID, CreatedAt: created}, nil
		},
	}
	r := newTestRouter(t, buckets, &fakeObjectService{})

	rec := doRequest(t, r, http.MethodPost, "/buckets", createBucketRequest{Name: "my-bucket", OwnerID: "owner-1"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	got := decodeBody[bucketResponse](t, rec)
	if got.ID != "b-1" || got.Name != "my-bucket" || got.OwnerID != "owner-1" {
		t.Errorf("unexpected body: %+v", got)
	}
}

func TestCreateBucket_InvalidInput(t *testing.T) {
	buckets := &fakeBucketService{
		createFn: func(_ context.Context, _, _ string) (model.Bucket, error) {
			return model.Bucket{}, service.ErrInvalidInput
		},
	}
	r := newTestRouter(t, buckets, &fakeObjectService{})

	rec := doRequest(t, r, http.MethodPost, "/buckets", createBucketRequest{Name: "AB", OwnerID: "owner-1"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreateBucket_AlreadyExists(t *testing.T) {
	buckets := &fakeBucketService{
		createFn: func(_ context.Context, _, _ string) (model.Bucket, error) {
			return model.Bucket{}, repository.ErrAlreadyExists
		},
	}
	r := newTestRouter(t, buckets, &fakeObjectService{})

	rec := doRequest(t, r, http.MethodPost, "/buckets", createBucketRequest{Name: "my-bucket", OwnerID: "owner-1"})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestListBuckets_Success(t *testing.T) {
	buckets := &fakeBucketService{
		listFn: func(_ context.Context, ownerID string) ([]model.Bucket, error) {
			if ownerID != "owner-1" {
				t.Errorf("listFn called with ownerID = %q, want %q", ownerID, "owner-1")
			}
			return []model.Bucket{{ID: "b-1", Name: "bucket-a"}, {ID: "b-2", Name: "bucket-b"}}, nil
		},
	}
	r := newTestRouter(t, buckets, &fakeObjectService{})

	rec := doRequest(t, r, http.MethodGet, "/buckets?owner_id=owner-1", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := decodeBody[[]bucketResponse](t, rec)
	if len(got) != 2 {
		t.Fatalf("got %d buckets, want 2", len(got))
	}
}

func TestGetBucket_Success(t *testing.T) {
	buckets := &fakeBucketService{
		getFn: func(_ context.Context, name string) (model.Bucket, error) {
			return model.Bucket{ID: "b-1", Name: name}, nil
		},
	}
	r := newTestRouter(t, buckets, &fakeObjectService{})

	rec := doRequest(t, r, http.MethodGet, "/buckets/my-bucket", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := decodeBody[bucketResponse](t, rec)
	if got.Name != "my-bucket" {
		t.Errorf("got.Name = %q, want %q", got.Name, "my-bucket")
	}
}

func TestGetBucket_NotFound(t *testing.T) {
	buckets := &fakeBucketService{
		getFn: func(_ context.Context, _ string) (model.Bucket, error) {
			return model.Bucket{}, repository.ErrNotFound
		},
	}
	r := newTestRouter(t, buckets, &fakeObjectService{})

	rec := doRequest(t, r, http.MethodGet, "/buckets/does-not-exist", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestGetBucket_InternalError(t *testing.T) {
	buckets := &fakeBucketService{
		getFn: func(_ context.Context, _ string) (model.Bucket, error) {
			return model.Bucket{}, errors.New("boltdb: unexpected disk failure")
		},
	}
	r := newTestRouter(t, buckets, &fakeObjectService{})

	rec := doRequest(t, r, http.MethodGet, "/buckets/my-bucket", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	got := decodeBody[errorResponse](t, rec)
	if got.Error != "internal server error" {
		t.Errorf("got.Error = %q, want a generic message that does not leak internals", got.Error)
	}
}

func TestDeleteBucket_Success(t *testing.T) {
	var calledWith string
	buckets := &fakeBucketService{
		deleteFn: func(_ context.Context, name string) error {
			calledWith = name
			return nil
		},
	}
	r := newTestRouter(t, buckets, &fakeObjectService{})

	rec := doRequest(t, r, http.MethodDelete, "/buckets/my-bucket", nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if calledWith != "my-bucket" {
		t.Errorf("DeleteBucket called with %q, want %q", calledWith, "my-bucket")
	}
}

func TestDeleteBucket_NotEmpty(t *testing.T) {
	buckets := &fakeBucketService{
		deleteFn: func(_ context.Context, _ string) error {
			return repository.ErrBucketNotEmpty
		},
	}
	r := newTestRouter(t, buckets, &fakeObjectService{})

	rec := doRequest(t, r, http.MethodDelete, "/buckets/my-bucket", nil)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestPutObject_Success(t *testing.T) {
	objects := &fakeObjectService{
		putFn: func(_ context.Context, bucketName, key string, sizeBytes int64, etag, contentType, storageRef string) (model.Object, error) {
			return model.Object{
				ID:          "o-1",
				BucketName:  bucketName,
				ObjectKey:   key,
				SizeBytes:   sizeBytes,
				ETag:        etag,
				ContentType: contentType,
				StorageRef:  storageRef,
				VersionID:   "null",
			}, nil
		},
	}
	r := newTestRouter(t, &fakeBucketService{}, objects)

	rec := doRequest(t, r, http.MethodPost, "/buckets/my-bucket/objects", putObjectRequest{
		Key:         "photo.png",
		SizeBytes:   2048,
		ETag:        "etag-1",
		ContentType: "image/png",
		StorageRef:  "storage-ref-1",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	got := decodeBody[objectResponse](t, rec)
	if got.BucketName != "my-bucket" || got.Key != "photo.png" {
		t.Errorf("unexpected body: %+v", got)
	}
}

func TestListObjects_Success(t *testing.T) {
	objects := &fakeObjectService{
		listFn: func(_ context.Context, bucketName, prefix string, _ int) ([]model.Object, error) {
			if bucketName != "my-bucket" || prefix != "photos/" {
				t.Errorf("listFn called with (%q, %q), want (%q, %q)", bucketName, prefix, "my-bucket", "photos/")
			}
			return []model.Object{{ID: "o-1", ObjectKey: "photos/a.png"}}, nil
		},
	}
	r := newTestRouter(t, &fakeBucketService{}, objects)

	rec := doRequest(t, r, http.MethodGet, "/buckets/my-bucket/objects?prefix=photos/", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := decodeBody[[]objectResponse](t, rec)
	if len(got) != 1 {
		t.Fatalf("got %d objects, want 1", len(got))
	}
}

func TestGetObject_WithSlashKey(t *testing.T) {
	var capturedKey string
	objects := &fakeObjectService{
		getFn: func(_ context.Context, bucketName, key string) (model.Object, error) {
			capturedKey = key
			return model.Object{ID: "o-1", BucketName: bucketName, ObjectKey: key}, nil
		},
	}
	r := newTestRouter(t, &fakeBucketService{}, objects)

	rec := doRequest(t, r, http.MethodGet, "/buckets/my-bucket/objects/photos/2024/img.png", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if capturedKey != "photos/2024/img.png" {
		t.Errorf("GetObject called with key = %q, want %q", capturedKey, "photos/2024/img.png")
	}
	got := decodeBody[objectResponse](t, rec)
	if got.Key != "photos/2024/img.png" {
		t.Errorf("got.Key = %q, want %q", got.Key, "photos/2024/img.png")
	}
}

func TestDeleteObject_WithSlashKey(t *testing.T) {
	var capturedKey string
	objects := &fakeObjectService{
		deleteFn: func(_ context.Context, _, key string) error {
			capturedKey = key
			return nil
		},
	}
	r := newTestRouter(t, &fakeBucketService{}, objects)

	rec := doRequest(t, r, http.MethodDelete, "/buckets/my-bucket/objects/photos/2024/img.png", nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if capturedKey != "photos/2024/img.png" {
		t.Errorf("DeleteObject called with key = %q, want %q", capturedKey, "photos/2024/img.png")
	}
}
