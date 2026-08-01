package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	api "github.com/naseyro/ms3/api-service/clients"
)

// requestTimeout bounds every call to metadata-service. Metadata operations
// are small CRUD calls, so a short deadline is safe here (unlike the data
// client, which must tolerate large streaming bodies).
const requestTimeout = 10 * time.Second

type HTTPMetadataClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewHTTPMetadataClient(baseURL string) *HTTPMetadataClient {
	return &HTTPMetadataClient{BaseURL: baseURL, HTTP: http.DefaultClient}
}

// createBucketRequest mirrors metadata-service's internal/api.createBucketRequest.
type createBucketRequest struct {
	Name    string `json:"name"`
	OwnerID string `json:"owner_id"`
}

// bucketResponse mirrors metadata-service's internal/api.bucketResponse.
type bucketResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	OwnerID    string    `json:"owner_id"`
	Versioning bool      `json:"versioning"`
	CreatedAt  time.Time `json:"created_at"`
}

func (b bucketResponse) toBucketInfo() *api.BucketInfo {
	return &api.BucketInfo{Name: b.Name, OwnerID: b.OwnerID, CreatedAt: b.CreatedAt}
}

// putObjectRequest mirrors metadata-service's internal/api.putObjectRequest.
// Note the field is "key", not "object_key" — api.ObjectInfo uses
// "object_key" for api-service's own public response shape, which is not
// the same wire shape metadata-service expects.
type putObjectRequest struct {
	Key         string `json:"key"`
	SizeBytes   int64  `json:"size_bytes"`
	ETag        string `json:"etag"`
	ContentType string `json:"content_type"`
	StorageRef  string `json:"storage_ref"`
}

// objectResponse mirrors metadata-service's internal/api.objectResponse.
type objectResponse struct {
	ID          string    `json:"id"`
	BucketName  string    `json:"bucket_name"`
	Key         string    `json:"key"`
	SizeBytes   int64     `json:"size_bytes"`
	ETag        string    `json:"etag"`
	ContentType string    `json:"content_type"`
	StorageRef  string    `json:"storage_ref"`
	VersionID   string    `json:"version_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (o objectResponse) toObjectInfo() api.ObjectInfo {
	return api.ObjectInfo{
		Key:         o.Key,
		Bucket:      o.BucketName,
		SizeBytes:   o.SizeBytes,
		ETag:        o.ETag,
		ContentType: o.ContentType,
		StorageRef:  o.StorageRef,
		CreatedAt:   o.CreatedAt,
	}
}

func (c *HTTPMetadataClient) CreateBucket(ctx context.Context, name, ownerID string) (*api.BucketInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	body, err := json.Marshal(createBucketRequest{Name: name, OwnerID: ownerID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/buckets", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out bucketResponse
	if err := c.do(req, http.StatusCreated, &out); err != nil {
		return nil, err
	}
	return out.toBucketInfo(), nil
}

func (c *HTTPMetadataClient) GetBucket(ctx context.Context, name string) (*api.BucketInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/buckets/"+name, nil)
	if err != nil {
		return nil, err
	}
	var out bucketResponse
	if err := c.do(req, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return out.toBucketInfo(), nil
}

func (c *HTTPMetadataClient) ListBucketsByOwner(ctx context.Context, ownerID string) ([]api.BucketInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	u := c.BaseURL + "/buckets?" + url.Values{"owner_id": {ownerID}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	var out []bucketResponse
	if err := c.do(req, http.StatusOK, &out); err != nil {
		return nil, err
	}

	buckets := make([]api.BucketInfo, len(out))
	for i, b := range out {
		buckets[i] = *b.toBucketInfo()
	}
	return buckets, nil
}

func (c *HTTPMetadataClient) DeleteBucket(ctx context.Context, name string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.BaseURL+"/buckets/"+name, nil)
	if err != nil {
		return err
	}
	return c.do(req, http.StatusNoContent, nil)
}

func (c *HTTPMetadataClient) ListObjects(ctx context.Context, bucket, prefix string) ([]api.ObjectInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	u := c.BaseURL + "/buckets/" + bucket + "/objects"
	if prefix != "" {
		u += "?" + url.Values{"prefix": {prefix}}.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	var out []objectResponse
	if err := c.do(req, http.StatusOK, &out); err != nil {
		return nil, err
	}

	objs := make([]api.ObjectInfo, len(out))
	for i, o := range out {
		objs[i] = o.toObjectInfo()
	}
	return objs, nil
}

func (c *HTTPMetadataClient) PutObjectMeta(ctx context.Context, obj api.ObjectInfo) (*api.ObjectInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	body, err := json.Marshal(putObjectRequest{
		Key:         obj.Key,
		SizeBytes:   obj.SizeBytes,
		ETag:        obj.ETag,
		ContentType: obj.ContentType,
		StorageRef:  obj.StorageRef,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/buckets/"+obj.Bucket+"/objects", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out objectResponse
	if err := c.do(req, http.StatusCreated, &out); err != nil {
		return nil, err
	}
	saved := out.toObjectInfo()
	return &saved, nil
}

func (c *HTTPMetadataClient) GetObjectMeta(ctx context.Context, bucket, key string) (*api.ObjectInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/buckets/"+bucket+"/objects/"+key, nil)
	if err != nil {
		return nil, err
	}
	var out objectResponse
	if err := c.do(req, http.StatusOK, &out); err != nil {
		return nil, err
	}
	got := out.toObjectInfo()
	return &got, nil
}

func (c *HTTPMetadataClient) DeleteObjectMeta(ctx context.Context, bucket, key string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.BaseURL+"/buckets/"+bucket+"/objects/"+key, nil)
	if err != nil {
		return err
	}
	return c.do(req, http.StatusNoContent, nil)
}

type errorResponse struct {
	Error string `json:"error"`
}

func (c *HTTPMetadataClient) do(req *http.Request, wantStatus int, out any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		var errBody errorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errBody)

		switch resp.StatusCode {
		case http.StatusNotFound:
			return &api.NotFoundError{Resource: "metadata record"}
		case http.StatusConflict:
			return &api.ConflictError{Message: errBody.Error}
		case http.StatusBadRequest:
			return &api.InvalidInputError{Message: errBody.Error}
		default:
			return fmt.Errorf("metadata-service returned %s", resp.Status)
		}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
