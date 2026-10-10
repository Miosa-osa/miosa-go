package miosa

import (
	"context"
	"errors"
	"net/url"
)

// ServiceAccount is a non-human principal that owns scope-limited API keys.
type ServiceAccount struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	CreatedByUserID string `json:"created_by_user_id,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

// CreateServiceAccountInput is the body of creating a service account.
type CreateServiceAccountInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpdateServiceAccountInput changes a service account. Nil members are left alone.
type UpdateServiceAccountInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ServiceAccountKeyInput mints a key owned by a service account.
type ServiceAccountKeyInput struct {
	Name string `json:"name,omitempty"`
	// Scopes is required: a non-empty list of valid scopes.
	Scopes []string `json:"scopes"`
	// AllowedIPs restricts the key to these addresses or CIDR ranges.
	AllowedIPs []string `json:"allowed_ips,omitempty"`
	// WorkspaceID binds the key to one workspace of the organization.
	WorkspaceID string `json:"workspace_id,omitempty"`
	// ExpiresInDays sets an expiry; zero means the key does not expire.
	ExpiresInDays int `json:"expires_in_days,omitempty"`
}

// ServiceAccountKey is a key owned by a service account. Key is the one-time
// plaintext, set only on the create response.
type ServiceAccountKey struct {
	ApiKeyData
	Key string `json:"key,omitempty"`
}

// DeletedServiceAccount is the result of deleting a service account.
type DeletedServiceAccount struct {
	ID          string `json:"id"`
	Deleted     bool   `json:"deleted"`
	RevokedKeys int    `json:"revoked_keys"`
}

// ServiceAccountsService manages service accounts. Owners and admins only.
// Accessed via Client.ServiceAccounts. Keys minted here are rotated,
// restricted and revoked through ApiKeysService.
type ServiceAccountsService struct {
	client *Client
}

// List returns the organization's service accounts.
func (s *ServiceAccountsService) List(ctx context.Context) ([]ServiceAccount, error) {
	var out apiResponse[[]ServiceAccount]
	if err := s.client.getJSON(ctx, "/service-accounts", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Create makes a service account. 429 SERVICE_ACCOUNT_LIMIT at the cap.
func (s *ServiceAccountsService) Create(ctx context.Context, in CreateServiceAccountInput) (*ServiceAccount, error) {
	if in.Name == "" {
		return nil, errors.New("name is required")
	}
	var out apiResponse[ServiceAccount]
	if err := s.client.postJSON(ctx, "/service-accounts", in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Update changes a service account's name or description.
func (s *ServiceAccountsService) Update(ctx context.Context, id string, in UpdateServiceAccountInput) (*ServiceAccount, error) {
	var out apiResponse[ServiceAccount]
	if err := s.client.patchJSON(ctx, "/service-accounts/"+url.PathEscape(id), in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Delete deletes a service account and revokes every key it owns.
func (s *ServiceAccountsService) Delete(ctx context.Context, id string) (*DeletedServiceAccount, error) {
	var out apiResponse[DeletedServiceAccount]
	if err := s.client.deleteJSON(ctx, "/service-accounts/"+url.PathEscape(id), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Keys lists the keys a service account owns.
func (s *ServiceAccountsService) Keys(ctx context.Context, id string) ([]ApiKeyData, error) {
	var out apiResponse[[]ApiKeyData]
	if err := s.client.getJSON(ctx, "/service-accounts/"+url.PathEscape(id)+"/keys", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// CreateKey mints a key owned by the service account. The plaintext Key is
// returned once.
func (s *ServiceAccountsService) CreateKey(ctx context.Context, id string, in ServiceAccountKeyInput) (*ServiceAccountKey, error) {
	if len(in.Scopes) == 0 {
		return nil, errors.New("scopes must be a non-empty list")
	}
	var out apiResponse[ServiceAccountKey]
	if err := s.client.postJSON(ctx, "/service-accounts/"+url.PathEscape(id)+"/keys", in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}
