package api

import (
	"time"

	"metadata-service/internal/model"
)

type createBucketRequest struct {
	Name    string `json:"name"`
	OwnerID string `json:"owner_id"`
}

type bucketResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	OwnerID    string    `json:"owner_id"`
	Versioning bool      `json:"versioning"`
	CreatedAt  time.Time `json:"created_at"`
}

func toBucketResponse(b model.Bucket) bucketResponse {
	return bucketResponse{
		ID:         b.ID,
		Name:       b.Name,
		OwnerID:    b.OwnerID,
		Versioning: b.IsVersioned,
		CreatedAt:  b.CreatedAt,
	}
}

type putObjectRequest struct {
	Key         string `json:"key"`
	SizeBytes   int64  `json:"size_bytes"`
	ETag        string `json:"etag"`
	ContentType string `json:"content_type"`
	StorageRef  string `json:"storage_ref"`
}

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

func toObjectResponse(o model.Object) objectResponse {
	return objectResponse{
		ID:          o.ID,
		BucketName:  o.BucketName,
		Key:         o.ObjectKey,
		SizeBytes:   o.SizeBytes,
		ETag:        o.ETag,
		ContentType: o.ContentType,
		StorageRef:  o.StorageRef,
		VersionID:   o.VersionID,
		CreatedAt:   o.CreatedAt,
	}
}
