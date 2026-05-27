package miosa

import "context"

// SandboxTemplatesService provides CRUD, build-spec utilities, and build management
// for sandbox templates.
type SandboxTemplatesService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// SandboxTemplateData is the API representation of a sandbox template.
type SandboxTemplateData struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	BuildSpec   map[string]interface{} `json:"build_spec,omitempty"`
	Status      string                 `json:"status"`
	ImageRef    string                 `json:"image_ref,omitempty"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// SandboxTemplateListResponse wraps GET /sandbox-templates.
type SandboxTemplateListResponse struct {
	Data []SandboxTemplateData `json:"data"`
}

// SandboxTemplateBuildData is the API representation of a template build.
type SandboxTemplateBuildData struct {
	ID         string `json:"id"`
	TemplateID string `json:"template_id"`
	Status     string `json:"status"`
	LogURL     string `json:"log_url,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	Error      string `json:"error,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// SandboxTemplateBuildListResponse wraps GET /sandbox-templates/:id/builds.
type SandboxTemplateBuildListResponse struct {
	Data []SandboxTemplateBuildData `json:"data"`
}

// CreateSandboxTemplateInput is the request body for POST /sandbox-templates.
type CreateSandboxTemplateInput struct {
	Name           string                 `json:"name"`
	BuildSpec      map[string]interface{} `json:"build_spec"`
	Description    string                 `json:"description,omitempty"`
	IdempotencyKey string                 `json:"-"`
}

// CreateSandboxTemplateBuildInput is the request body for POST /sandbox-templates/:id/builds.
type CreateSandboxTemplateBuildInput struct {
	BuildSpec      map[string]interface{} `json:"build_spec,omitempty"`
	IdempotencyKey string                 `json:"-"`
}

// ValidateBuildSpecInput is the request body for POST /sandbox-templates/validate.
type ValidateBuildSpecInput struct {
	BuildSpec map[string]interface{} `json:"build_spec"`
}

// ValidateBuildSpecResult is the response from POST /sandbox-templates/validate.
type ValidateBuildSpecResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// ListSandboxTemplatesInput holds optional query parameters for GET /sandbox-templates.
type ListSandboxTemplatesInput struct {
	IncludeAliases bool
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// List returns sandbox templates for the authenticated tenant.
func (s *SandboxTemplatesService) List(ctx context.Context, input ListSandboxTemplatesInput) (*SandboxTemplateListResponse, error) {
	params := map[string]string{}
	if input.IncludeAliases {
		params["include_aliases"] = "true"
	}
	var out SandboxTemplateListResponse
	if err := s.client.getJSON(ctx, "/sandbox-templates"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single sandbox template by ID.
func (s *SandboxTemplatesService) Get(ctx context.Context, id string) (*SandboxTemplateData, error) {
	var env apiResponse[SandboxTemplateData]
	if err := s.client.getJSON(ctx, "/sandbox-templates/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Create provisions a new sandbox template.
func (s *SandboxTemplatesService) Create(ctx context.Context, input CreateSandboxTemplateInput) (*SandboxTemplateData, error) {
	var env apiResponse[SandboxTemplateData]
	if err := s.client.postJSONIdempotent(ctx, "/sandbox-templates", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// BuildSpecSchema returns the JSON schema for build specs.
func (s *SandboxTemplatesService) BuildSpecSchema(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/sandbox-templates/build-spec", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Validate checks a build spec without creating a template.
func (s *SandboxTemplatesService) Validate(ctx context.Context, buildSpec map[string]interface{}) (*ValidateBuildSpecResult, error) {
	var out ValidateBuildSpecResult
	if err := s.client.postJSON(ctx, "/sandbox-templates/validate", ValidateBuildSpecInput{BuildSpec: buildSpec}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListBuilds returns the build history for a template.
func (s *SandboxTemplatesService) ListBuilds(ctx context.Context, templateID string) (*SandboxTemplateBuildListResponse, error) {
	var out SandboxTemplateBuildListResponse
	if err := s.client.getJSON(ctx, "/sandbox-templates/"+templateID+"/builds", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateBuild triggers a new build for a sandbox template.
func (s *SandboxTemplatesService) CreateBuild(ctx context.Context, templateID string, input CreateSandboxTemplateBuildInput) (*SandboxTemplateBuildData, error) {
	var env apiResponse[SandboxTemplateBuildData]
	if err := s.client.postJSONIdempotent(ctx, "/sandbox-templates/"+templateID+"/builds", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
