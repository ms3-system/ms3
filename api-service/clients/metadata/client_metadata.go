package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	api "github.com/naseyro/ms3/api-service/clients"
)

type HTTPMetadataClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewHTTPMetadataClient(baseURL string) *HTTPMetadataClient {
	return &HTTPMetadataClient{BaseURL: baseURL, HTTP: http.DefaultClient}
}

func (c *HTTPMetadataClient) CreateBucket(ctx context.Context, name string) (*api.BucketInfo, error) {
	body, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/buckets", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out api.BucketInfo
	if err := c.do(req, http.StatusCreated, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *HTTPMetadataClient) DeleteBucket(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.BaseURL+"/buckets/"+name, nil)
	if err != nil {
		return err
	}
	return c.do(req, http.StatusNoContent, nil)
}

func (c *HTTPMetadataClient) ListObjects(ctx context.Context, bucket string) ([]api.ObjectInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/buckets/"+bucket+"/objects", nil)
	if err != nil {
		return nil, err
	}
	var out []api.ObjectInfo
	if err := c.do(req, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *HTTPMetadataClient) PutObjectMeta(ctx context.Context, obj api.ObjectInfo) (*api.ObjectInfo, error) {
	body, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/buckets/"+obj.Bucket+"/objects", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out api.ObjectInfo
	if err := c.do(req, http.StatusCreated, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *HTTPMetadataClient) GetObjectMeta(ctx context.Context, bucket, key string) (*api.ObjectInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/buckets/"+bucket+"/objects/"+key, nil)
	if err != nil {
		return nil, err
	}
	var out api.ObjectInfo
	if err := c.do(req, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *HTTPMetadataClient) DeleteObjectMeta(ctx context.Context, bucket, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.BaseURL+"/buckets/"+bucket+"/objects/"+key, nil)
	if err != nil {
		return err
	}
	return c.do(req, http.StatusNoContent, nil)
}

func (c *HTTPMetadataClient) do(req *http.Request, wantStatus int, out interface{}) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &api.NotFoundError{Resource: "metadata record"}
	}
	if resp.StatusCode != wantStatus {
		return fmt.Errorf("metadata-service returned %s", resp.Status)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
