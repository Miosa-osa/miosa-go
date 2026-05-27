package miosa

import "context"

// ProjectIntegrationsService provides access to per-project third-party API
// key connections (Stripe, Resend, Twilio, etc.) that are injected as env
// vars into sandbox and deployment VMs at boot.
type ProjectIntegrationsService struct {
	client *Client
}

// ProjectIntegrationData is the API representation of a project integration.
type ProjectIntegrationData struct {
	ID        string                 `json:"id"`
	TenantID  string                 `json:"tenant_id"`
	Provider  string                 `json:"provider"`
	Name      string                 `json:"name"`
	EnvKey    string                 `json:"env_key"`
	Masked    string                 `json:"masked,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

// ProjectIntegrationListResponse wraps GET /project-integrations.
type ProjectIntegrationListResponse struct {
	Data []ProjectIntegrationData `json:"data"`
}

// ProjectIntegrationCatalogEntry describes a supported provider and its schema.
type ProjectIntegrationCatalogEntry struct {
	Provider    string                 `json:"provider"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	LogoURL     string                 `json:"logo_url,omitempty"`
}

// ProjectIntegrationCatalogResponse wraps GET /project-integrations/catalog.
type ProjectIntegrationCatalogResponse struct {
	Data []ProjectIntegrationCatalogEntry `json:"data"`
}

// CreateProjectIntegrationInput is the request body for POST /project-integrations.
type CreateProjectIntegrationInput struct {
	Provider string                 `json:"provider"`
	Name     string                 `json:"name,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// UpdateProjectIntegrationInput is the request body for PATCH /project-integrations/:id.
type UpdateProjectIntegrationInput struct {
	Name   string                 `json:"name,omitempty"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// ListProjectIntegrationsInput holds optional query filters.
type ListProjectIntegrationsInput struct {
	Provider string
}

// List returns project integrations with optional filters.
func (s *ProjectIntegrationsService) List(ctx context.Context, input ListProjectIntegrationsInput) (*ProjectIntegrationListResponse, error) {
	params := map[string]string{}
	if input.Provider != "" {
		params["provider"] = input.Provider
	}
	var out ProjectIntegrationListResponse
	if err := s.client.getJSON(ctx, "/project-integrations"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Catalog lists supported providers and their schemas.
func (s *ProjectIntegrationsService) Catalog(ctx context.Context) (*ProjectIntegrationCatalogResponse, error) {
	var out ProjectIntegrationCatalogResponse
	if err := s.client.getJSON(ctx, "/project-integrations/catalog", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a project integration by ID.
func (s *ProjectIntegrationsService) Get(ctx context.Context, integrationID string) (*ProjectIntegrationData, error) {
	var env apiResponse[ProjectIntegrationData]
	if err := s.client.getJSON(ctx, "/project-integrations/"+integrationID, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Create creates a new project integration.
func (s *ProjectIntegrationsService) Create(ctx context.Context, input CreateProjectIntegrationInput) (*ProjectIntegrationData, error) {
	var env apiResponse[ProjectIntegrationData]
	if err := s.client.postJSON(ctx, "/project-integrations", input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Update updates a project integration.
func (s *ProjectIntegrationsService) Update(ctx context.Context, integrationID string, input UpdateProjectIntegrationInput) (*ProjectIntegrationData, error) {
	var env apiResponse[ProjectIntegrationData]
	if err := s.client.patchJSON(ctx, "/project-integrations/"+integrationID, input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete removes a project integration.
func (s *ProjectIntegrationsService) Delete(ctx context.Context, integrationID string) error {
	return s.client.deleteJSON(ctx, "/project-integrations/"+integrationID, nil)
}
