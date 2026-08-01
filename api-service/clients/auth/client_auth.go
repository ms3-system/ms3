package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	api "github.com/naseyro/ms3/api-service/clients"
)

// requestTimeout bounds every call to auth-service. Credential lookups are
// small, single-record reads.
const requestTimeout = 10 * time.Second

type HTTPAuthClient struct {
	BaseURL       string
	InternalToken string
	HTTP          *http.Client
}

func NewHTTPAuthClient(baseURL, internalToken string) *HTTPAuthClient {
	return &HTTPAuthClient{BaseURL: baseURL, InternalToken: internalToken, HTTP: http.DefaultClient}
}

// internalCredentialResponse mirrors auth-service's internal/api.internalCredentialResponse.
type internalCredentialResponse struct {
	UserID    string `json:"user_id"`
	SecretKey string `json:"secret_key"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (c *HTTPAuthClient) LookupCredential(ctx context.Context, accessKey string) (*api.Credential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/internal/credentials/"+accessKey, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Token", c.InternalToken)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody errorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errBody)

		if resp.StatusCode == http.StatusNotFound {
			return nil, &api.NotFoundError{Resource: "access key"}
		}
		return nil, fmt.Errorf("auth-service returned %s: %s", resp.Status, errBody.Error)
	}

	var out internalCredentialResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &api.Credential{UserID: out.UserID, SecretKey: out.SecretKey}, nil
}
