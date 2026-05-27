package miosa

import "context"

// ProjectAuthService provides access to built-in auth configuration for
// sandboxes and deployments.
type ProjectAuthService struct {
	client *Client
}

// ProjectAuthStatus is the current project-auth status and configuration.
type ProjectAuthStatus struct {
	Enabled   bool                   `json:"enabled"`
	Provider  string                 `json:"provider,omitempty"`
	Config    map[string]interface{} `json:"config,omitempty"`
	UpdatedAt string                 `json:"updated_at"`
}

// EnableProjectAuthInput is the request body for POST /project-auth/enable.
type EnableProjectAuthInput struct {
	Provider string                 `json:"provider,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// UpdateProjectAuthInput is the request body for PATCH /project-auth/config.
type UpdateProjectAuthInput struct {
	Provider string                 `json:"provider,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// Status returns the current project-auth status and configuration.
func (s *ProjectAuthService) Status(ctx context.Context) (*ProjectAuthStatus, error) {
	var out ProjectAuthStatus
	if err := s.client.getJSON(ctx, "/project-auth/status", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Enable enables project auth with optional configuration.
func (s *ProjectAuthService) Enable(ctx context.Context, input EnableProjectAuthInput) (*ProjectAuthStatus, error) {
	var env apiResponse[ProjectAuthStatus]
	if err := s.client.postJSON(ctx, "/project-auth/enable", input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Disable disables project auth.
func (s *ProjectAuthService) Disable(ctx context.Context) (*ProjectAuthStatus, error) {
	var env apiResponse[ProjectAuthStatus]
	if err := s.client.postJSON(ctx, "/project-auth/disable", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Update updates project-auth configuration.
func (s *ProjectAuthService) Update(ctx context.Context, input UpdateProjectAuthInput) (*ProjectAuthStatus, error) {
	var env apiResponse[ProjectAuthStatus]
	if err := s.client.patchJSON(ctx, "/project-auth/config", input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
