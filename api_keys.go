package miosa

import "context"

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

	// Fields the API returns beyond the original set.
	KeyPrefix string `json:"key_prefix,omitempty"`
	LastFour  string `json:"last_four,omitempty"`
	// KeyType is user, platform or tenant; KeyPurpose is api.
	KeyType          string   `json:"key_type,omitempty"`
	KeyPurpose       string   `json:"key_purpose,omitempty"`
	AllowedIPs       []string `json:"allowed_ips,omitempty"`
	ServiceAccountID string   `json:"service_account_id,omitempty"`
	RateLimitRPM     *int     `json:"rate_limit_rpm,omitempty"`
	// Status is active or revoked.
	Status          string `json:"status,omitempty"`
	WorkspaceID     string `json:"workspace_id,omitempty"`
	ReplacesKeyID   string `json:"replaces_key_id,omitempty"`
	ReplacedByKeyID string `json:"replaced_by_key_id,omitempty"`
	RotatedAt       string `json:"rotated_at,omitempty"`
	RevokedAt       string `json:"revoked_at,omitempty"`
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

	// ExpiresInDays is the field the API reads for expiry. Prefer it to ExpiresAt.
	ExpiresInDays int `json:"expires_in_days,omitempty"`
	// KeyType is user (default), platform or tenant; platform and tenant keys
	// need an organization owner or admin.
	KeyType string `json:"key_type,omitempty"`
	// WorkspaceID binds the key to one workspace.
	WorkspaceID string `json:"workspace_id,omitempty"`
	// AllowedIPs restricts the key to these addresses or CIDR ranges.
	AllowedIPs []string `json:"allowed_ips,omitempty"`
	// RateLimitRPM is requests per minute for this key.
	RateLimitRPM int `json:"rate_limit_rpm,omitempty"`
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

// ApiKeyPreset is a named scope set for minting a key by name.
type ApiKeyPreset struct {
	// Name is read-only, ci, agent or cli.
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
}

// Presets returns the server's named scope sets (GET /api-keys/presets), so a
// client picks a name instead of keeping its own copy of the scope lists. Use
// a preset's Scopes as CreateApiKeyInput.Scopes.
func (s *ApiKeysService) Presets(ctx context.Context) ([]ApiKeyPreset, error) {
	var env apiResponse[[]ApiKeyPreset]
	if err := s.client.getJSON(ctx, "/api-keys/presets", &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// CreateFromPreset mints a key with the scopes of a named preset. The preset
// is looked up with Presets first, so the scope list is always the server's.
func (s *ApiKeysService) CreateFromPreset(ctx context.Context, preset string, input CreateApiKeyInput) (*ApiKeyCreateResult, error) {
	presets, err := s.Presets(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range presets {
		if p.Name == preset {
			input.Scopes = p.Scopes
			return s.Create(ctx, input)
		}
	}
	return nil, &MiosaError{Message: "unknown API key preset " + preset, Code: "UNKNOWN_PRESET"}
}

// RotateApiKeyResult is the replacement key from a rotation. Key is the
// one-time plaintext.
type RotateApiKeyResult struct {
	ApiKeyData
	Key string `json:"key"`
}

// Rotate atomically replaces an active key and revokes the original. The
// replacement inherits type, scopes, workspace, rate limit, allowed IPs and
// expiry; pass a name to rename it (empty keeps the original's). 409
// API_KEY_ALREADY_ROTATED when the key already has a replacement.
func (s *ApiKeysService) Rotate(ctx context.Context, id, name string) (*RotateApiKeyResult, error) {
	body := map[string]string{}
	if name != "" {
		body["name"] = name
	}
	var env apiResponse[RotateApiKeyResult]
	if err := s.client.postJSON(ctx, "/api-keys/"+id+"/rotate", body, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateApiKeyInput changes a key's name and IP allowlist; scopes and expiry
// are fixed at creation. Nil members are left alone.
type UpdateApiKeyInput struct {
	Name       *string  `json:"name,omitempty"`
	AllowedIPs []string `json:"allowed_ips,omitempty"`
}

// Update changes a key's name and allowed IPs (PATCH /api-keys/:id).
func (s *ApiKeysService) Update(ctx context.Context, id string, input UpdateApiKeyInput) (*ApiKeyData, error) {
	var env apiResponse[ApiKeyData]
	if err := s.client.patchJSON(ctx, "/api-keys/"+id, input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
