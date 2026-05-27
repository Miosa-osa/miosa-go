package miosa

import "context"

// FlatCustomDomainsService provides tenant-scoped custom domain management
// across all computers and deployments.
type FlatCustomDomainsService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// FlatCustomDomainData is the API representation of a tenant-level custom domain.
type FlatCustomDomainData struct {
	ID                 string `json:"id"`
	TenantID           string `json:"tenant_id"`
	FQDN               string `json:"fqdn"`
	Status             string `json:"status"`
	TargetType         string `json:"target_type,omitempty"`
	TargetID           string `json:"target_id,omitempty"`
	VerificationTarget string `json:"verification_target,omitempty"`
	Instructions       string `json:"instructions,omitempty"`
	VerifiedAt         string `json:"verified_at,omitempty"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// FlatCustomDomainListResponse wraps GET /custom-domains.
type FlatCustomDomainListResponse struct {
	Data []FlatCustomDomainData `json:"data"`
}

// CreateFlatCustomDomainInput is the request body for POST /custom-domains.
type CreateFlatCustomDomainInput struct {
	FQDN           string `json:"fqdn"`
	TargetType     string `json:"target_type,omitempty"`
	TargetID       string `json:"target_id,omitempty"`
	RedirectPolicy string `json:"redirect_policy,omitempty"`
	IdempotencyKey string `json:"-"`
}

// ListFlatCustomDomainsInput holds optional query parameters for GET /custom-domains.
type ListFlatCustomDomainsInput struct {
	Status   string
	TargetID string
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// List returns all tenant-level custom domains.
func (s *FlatCustomDomainsService) List(ctx context.Context, input ListFlatCustomDomainsInput) (*FlatCustomDomainListResponse, error) {
	params := map[string]string{}
	if input.Status != "" {
		params["status"] = input.Status
	}
	if input.TargetID != "" {
		params["target_id"] = input.TargetID
	}
	var out FlatCustomDomainListResponse
	if err := s.client.getJSON(ctx, "/custom-domains"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create attaches a new custom domain at the tenant level.
func (s *FlatCustomDomainsService) Create(ctx context.Context, input CreateFlatCustomDomainInput) (*FlatCustomDomainData, error) {
	var env apiResponse[FlatCustomDomainData]
	if err := s.client.postJSONIdempotent(ctx, "/custom-domains", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete removes a custom domain by ID.
func (s *FlatCustomDomainsService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/custom-domains/"+id, nil)
}
