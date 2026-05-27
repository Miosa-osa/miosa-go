package miosa

import (
	"context"
	"fmt"
)

// ApiKeysService provides list, create, and delete operations for API keys.
// The plaintext token is returned only at create time; the server stores only a hash.
type ApiKeysService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// ApiKeyData is the API representation of an API key (token omitted after creation).
type ApiKeyData struct {
	ID         string   `json:"id"`
	TenantID   string   `json:"tenant_id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"prefix,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
	ExpiresAt  string   `json:"expires_at,omitempty"`
	LastUsedAt string   `json:"last_used_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

// ApiKeyCreateResult is returned by Create; Token is the one-time plaintext value.
type ApiKeyCreateResult struct {
	ApiKeyData
	Token string `json:"token,omitempty"`
	Key   string `json:"key,omitempty"` // some backends use "key" instead of "token"
}

// ApiKeyListResponse wraps GET /api-keys.
type ApiKeyListResponse struct {
	Data []ApiKeyData `json:"data"`
}

// CreateApiKeyInput is the request body for POST /api-keys.
type CreateApiKeyInput struct {
	Name           string   `json:"name"`
	Scopes         []string `json:"scopes,omitempty"`
	ExpiresAt      string   `json:"expires_at,omitempty"`
	IdempotencyKey string   `json:"-"`
}

// ListApiKeysInput holds optional query parameters for GET /api-keys.
type ListApiKeysInput struct {
	Limit  int
	Cursor string
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// List returns API keys for the authenticated tenant.
func (s *ApiKeysService) List(ctx context.Context, input ListApiKeysInput) (*ApiKeyListResponse, error) {
	params := map[string]string{}
	if input.Cursor != "" {
		params["cursor"] = input.Cursor
	}
	var out ApiKeyListResponse
	if err := s.client.getJSON(ctx, "/api-keys"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create mints a new API key. The Token field in the result is the one-time plaintext;
// store it immediately — it cannot be retrieved again.
func (s *ApiKeysService) Create(ctx context.Context, input CreateApiKeyInput) (*ApiKeyCreateResult, error) {
	var env apiResponse[ApiKeyCreateResult]
	if err := s.client.postJSONIdempotent(ctx, "/api-keys", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete revokes an API key by ID.
func (s *ApiKeysService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/api-keys/"+id, nil)
}

// CreateScoped creates an L2 delegation token bound to one external user.
// POST /api/v1/api-keys/scoped
func (s *ApiKeysService) CreateScoped(ctx context.Context, input CreateScopedApiKeyInput) (*ScopedApiKeyResult, error) {
	var out ScopedApiKeyResult
	if err := s.client.postJSON(ctx, "/api-keys/scoped", input, &out); err != nil {
		return nil, fmt.Errorf("ApiKeysService.CreateScoped: %w", err)
	}
	return &out, nil
}
