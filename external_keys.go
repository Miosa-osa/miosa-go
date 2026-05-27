package miosa

import "context"

// ExternalKeysService provides access to BYOK encrypted per-user provider
// keys (Anthropic, OpenAI, Google, Groq, etc.) used by dashboard features.
type ExternalKeysService struct {
	client *Client
}

// ExternalKeyData is the API representation of a stored external provider key.
type ExternalKeyData struct {
	Provider  string `json:"provider"`
	Masked    string `json:"masked"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ExternalKeyListResponse wraps GET /external-keys.
type ExternalKeyListResponse struct {
	Data []ExternalKeyData `json:"data"`
}

// CreateExternalKeyInput is the request body for POST /external-keys.
type CreateExternalKeyInput struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
	Label    string `json:"label,omitempty"`
}

// List returns all configured external provider keys.
func (s *ExternalKeysService) List(ctx context.Context) (*ExternalKeyListResponse, error) {
	var out ExternalKeyListResponse
	if err := s.client.getJSON(ctx, "/external-keys", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create registers a new external provider key.
func (s *ExternalKeysService) Create(ctx context.Context, input CreateExternalKeyInput) (*ExternalKeyData, error) {
	var env apiResponse[ExternalKeyData]
	if err := s.client.postJSON(ctx, "/external-keys", input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Resolve previews the stored key for a provider.
func (s *ExternalKeysService) Resolve(ctx context.Context, provider string) (*ExternalKeyData, error) {
	var env apiResponse[ExternalKeyData]
	if err := s.client.getJSON(ctx, "/external-keys/"+provider+"/resolve", &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete removes the stored key for a provider. The backend keys external
// keys by provider slug, not by a UUID.
func (s *ExternalKeysService) Delete(ctx context.Context, provider string) error {
	return s.client.deleteJSON(ctx, "/external-keys/"+provider, nil)
}
