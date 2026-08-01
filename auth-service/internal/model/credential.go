package model

import "time"

type Credential struct {
	AccessKey          string     `json:"access_key"`
	UserID             string     `json:"user_id"`
	SecretKeyEncrypted string     `json:"secret_key_encrypted"`
	CreatedAt          time.Time  `json:"created_at"`
	RevokedAt          *time.Time `json:"revoked_at,omitempty"`
}
