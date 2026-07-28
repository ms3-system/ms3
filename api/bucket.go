package api

import (
	"crypto/sha256"
	"encoding/hex"
)

// Bucket should represent a row in the Metadata DB
type Bucket struct {
	ID   string
	Name string
}

func NewBucket(name string) *Bucket {
	id := sha256.Sum256([]byte(name))
	return &Bucket{
		Name: name,
		ID:   hex.EncodeToString(id[:]),
	}
}
