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

// SandboxDeployInput is the request body for POST /sandboxes/:id/deploy.
type SandboxDeployInput struct {
	Name               string                 `json:"name,omitempty"`
	DeploymentID       string                 `json:"deployment_id,omitempty"`
	OutputPath         string                 `json:"output_path,omitempty"`
	SourceSnapshotPath string                 `json:"source_snapshot_path,omitempty"`
	Entrypoint         string                 `json:"entrypoint,omitempty"`
	BuildCommand       string                 `json:"build_command,omitempty"`
	RunCommand         string                 `json:"run_command,omitempty"`
	StartCommand       string                 `json:"start_command,omitempty"`
	Port               int                    `json:"port,omitempty"`
	HealthCheckPath    string                 `json:"health_check_path,omitempty"`
	DeploymentType     string                 `json:"deployment_type,omitempty"`
	Type               string                 `json:"type,omitempty"`
	Mode               string                 `json:"mode,omitempty"`
	Database           interface{}            `json:"database,omitempty"`
	Resources          map[string]interface{} `json:"resources,omitempty"`
	Domain             string                 `json:"domain,omitempty"`
	CustomDomain       string                 `json:"custom_domain,omitempty"`
	IdempotencyKey     string                 `json:"-"`
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

// Deploy publishes a sandbox using the default MIOSA Deploy path unless DeploymentType is set.
func (s *SandboxesService) Deploy(ctx context.Context, id string, input SandboxDeployInput) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSONIdempotent(ctx, "/sandboxes/"+id+"/deploy", input, &out, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return out, nil
}

// DeployDocker publishes a sandbox through the workspace Docker Deploy appliance.
func (s *SandboxesService) DeployDocker(ctx context.Context, id string, input SandboxDeployInput) (map[string]interface{}, error) {
	input.DeploymentType = "docker_deploy"
	return s.Deploy(ctx, id, input)
}
