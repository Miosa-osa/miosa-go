package miosa

import "context"

// SandboxTemplate is the template slug used for the lightweight code-exec
// sandbox rootfs. It resolves to /srv/miosa/templates/miosa-sandbox.ext4 on
// the compute host.
const SandboxTemplate = "miosa-sandbox"

// SandboxesService is a thin wrapper around ComputersService that defaults
// Template to "miosa-sandbox". It mirrors the product model used by E2B and
// Daytona: there is one resource type — a computer — and the template slug
// selects its flavour. A sandbox is just a computer with the lightweight
// template; every computer method works identically.
type SandboxesService struct {
	client *Client
}

// CreateSandboxInput is the request body for Create. Template defaults to
// "miosa-sandbox" when empty.
type CreateSandboxInput struct {
	Name     string            `json:"name"`
	Size     ComputerSize      `json:"size,omitempty"` // SizeSmall | SizeMedium | SizeLarge | SizeXLarge
	Template string            `json:"template_type,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	// White-label attribution. Stored as text on the sandbox row by Phase 2A.
	ExternalWorkspaceID string `json:"external_workspace_id,omitempty"`
	ExternalUserID      string `json:"external_user_id,omitempty"`
	ExternalProjectID   string `json:"external_project_id,omitempty"`
}

// Create provisions a sandbox (a computer with the miosa-sandbox template).
func (s *SandboxesService) Create(ctx context.Context, input CreateSandboxInput) (*Computer, error) {
	template := input.Template
	if template == "" {
		template = SandboxTemplate
	}
	return s.client.Computers.Create(ctx, CreateComputerInput{
		Name:                input.Name,
		Size:                input.Size,
		TemplateType:        template,
		Metadata:            input.Metadata,
		ExternalWorkspaceID: input.ExternalWorkspaceID,
		ExternalUserID:      input.ExternalUserID,
		ExternalProjectID:   input.ExternalProjectID,
	})
}

// Get fetches a sandbox by ID. Alias for Computers.Get.
func (s *SandboxesService) Get(ctx context.Context, id string) (*Computer, error) {
	return s.client.Computers.Get(ctx, id)
}

// Delete tears down a sandbox. Alias for Computers.Delete.
func (s *SandboxesService) Delete(ctx context.Context, id string) error {
	return s.client.Computers.Delete(ctx, id)
}
