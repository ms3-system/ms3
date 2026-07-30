package model

import "time"

type Object struct {
	ID          string     `json:"id"`
	ObjectKey   string     `json:"object_key"`
	BucketName  string     `json:"bucket_name"`
	SizeBytes   int64      `json:"size_bytes"`
	ETag        string     `json:"etag"`
	ContentType string     `json:"content_type"`
	StorageRef  string     `json:"storage_ref"`
	VersionID   string     `json:"version_id"`
	IsLatest    bool       `json:"is_latest"`
	CreatedAt   time.Time  `json:"created_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}
