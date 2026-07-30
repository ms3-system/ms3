package model

import "time"

type Bucket struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	OwnerID     string     `json:"owner_id"`
	IsVersioned bool       `json:"is_versioned"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
