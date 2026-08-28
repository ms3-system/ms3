package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/naseyro/ms3/data-service/internal/storage"
)

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeBackend struct {
	readyFn func(ctx context.Context) error
}

var _ storage.Backend = (*fakeBackend)(nil)

func (f *fakeBackend) Write(ctx context.Context, namespace string, r io.Reader) (string, int64, error) {
	return "", 0, nil
}

func (f *fakeBackend) Read(ctx context.Context, namespace, hash string) (io.ReadCloser, error) {
	return nil, nil
}

func (f *fakeBackend) Delete(ctx context.Context, namespace, hash string) error {
	return nil
}

func (f *fakeBackend) Ready(ctx context.Context) error {
	if f.readyFn != nil {
		return f.readyFn(ctx)
	}
	return nil
}

func testRouter(t *testing.T, store storage.Backend) http.Handler {
	t.Helper()
	if store == nil {
		store = &fakeBackend{}
	}
	return NewRouter(NewHandler(store, newTestLogger(t)), newTestLogger(t))
}

func TestHealthz(t *testing.T) {
	r := testRouter(t, nil)

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
			r := testRouter(t, nil)

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
	store := &fakeBackend{readyFn: func(ctx context.Context) error {
		return errors.New("data dir not writable")
	}}
	r := testRouter(t, store)

	req := httptest.NewRequest(http.MethodGet, "/healthz/ready", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
