package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	api "github.com/naseyro/ms3/api-service/clients"
)

type HTTPDataClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewHTTPDataClient(baseURL string) *HTTPDataClient {
	return &HTTPDataClient{BaseURL: baseURL, HTTP: http.DefaultClient}
}

func (c *HTTPDataClient) Write(ctx context.Context, namespace string, r io.Reader) (string, int64, error) {
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
		return "", 0, fmt.Errorf("data-service upload failed: %s", resp.Status)
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
	url := fmt.Sprintf("%s/namespaces/%s/objects/%s", c.BaseURL, namespace, hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, &api.NotFoundError{Resource: "object data"}
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("data-service download failed: %s", resp.Status)
	}
	return resp.Body, nil
}

func (c *HTTPDataClient) Delete(ctx context.Context, namespace, hash string) error {
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
		return fmt.Errorf("data-service delete failed: %s", resp.Status)
	}
	return nil
}
