package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	api "github.com/naseyro/ms3/api-service/clients"
)

// requestTimeout bounds Delete calls to data-service. Write/Read stream
// request/response bodies whose size isn't known up front (uploads/
// downloads), so they get a longer, more generous deadline instead.
const requestTimeout = 10 * time.Second
const streamingRequestTimeout = 5 * time.Minute

type HTTPDataClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewHTTPDataClient(baseURL string) *HTTPDataClient {
	return &HTTPDataClient{BaseURL: baseURL, HTTP: http.DefaultClient}
}

func (c *HTTPDataClient) Write(ctx context.Context, namespace string, r io.Reader) (string, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, streamingRequestTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/namespaces/%s/objects", c.BaseURL, namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, r)
	if err != nil {
		return "", 0, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", 0, dataServiceError(resp, "data-service upload failed")
	}

	var out struct {
		Hash string `json:"hash"`
		Size int64  `json:"size"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", 0, err
	}
	return out.Hash, out.Size, nil
}

func (c *HTTPDataClient) Read(ctx context.Context, namespace, hash string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(ctx, streamingRequestTimeout)
	url := fmt.Sprintf("%s/namespaces/%s/objects/%s", c.BaseURL, namespace, hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		cancel()
		return nil, &api.NotFoundError{Resource: "object data"}
	}
	if resp.StatusCode != http.StatusOK {
		err := dataServiceError(resp, "data-service download failed")
		resp.Body.Close()
		cancel()
		return nil, err
	}
	return &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}, nil
}

func (c *HTTPDataClient) Delete(ctx context.Context, namespace, hash string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/namespaces/%s/objects/%s", c.BaseURL, namespace, hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return dataServiceError(resp, "data-service delete failed")
	}
	return nil
}

type errorResponse struct {
	Error string `json:"error"`
}

func dataServiceError(resp *http.Response, fallback string) error {
	var errBody errorResponse
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody.Error != "" {
		return fmt.Errorf("%s: %s", fallback, errBody.Error)
	}
	return fmt.Errorf("%s: %s", fallback, resp.Status)
}

// cancelOnCloseBody releases the request context's timeout once the caller
// finishes reading the streamed response, instead of holding it open for
// the lifetime of the deadline regardless of how long the download takes.
type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	defer b.cancel()
	return b.ReadCloser.Close()
}
