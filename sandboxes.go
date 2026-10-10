package miosa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

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
	// Batches creates and tracks many sandboxes at once.
	Batches *SandboxBatchesService
}

// CreateSandboxInput is the request body for Create. Template defaults to
// "miosa-sandbox" when empty.
type CreateSandboxInput struct {
	Name       string       `json:"name,omitempty"`
	Size       ComputerSize `json:"size,omitempty"` // SizeXS | SizeSmall | SizeMedium | SizeLarge | SizeXL
	TemplateID string       `json:"template_id,omitempty"`
	// Template is a compatibility alias for TemplateID.
	Template       string            `json:"-"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	TimeoutSec     int               `json:"timeout_sec,omitempty"`
	IdleTimeoutSec int               `json:"idle_timeout_sec,omitempty"`
	CPUCount       int               `json:"cpu_count,omitempty"`
	MemoryMB       int               `json:"memory_mb,omitempty"`
	DiskSizeMB     int               `json:"disk_size_mb,omitempty"`
	Persistent     *bool             `json:"persistent,omitempty"`
	AlwaysOn       bool              `json:"always_on,omitempty"`
	Region         string            `json:"region,omitempty"`
	// AllowProvision opts in to the in-sandbox L3 token carrying the
	// "provision" scope. Defaults to false on the server when nil.
	AllowProvision *bool `json:"allow_provision,omitempty"`
	// Canonical MIOSA ownership. Slugs and names are resolved by the control plane.
	WorkspaceID   string `json:"workspace_id,omitempty"`
	WorkspaceSlug string `json:"workspace_slug,omitempty"`
	WorkspaceName string `json:"workspace_name,omitempty"`
	ProjectID     string `json:"project_id,omitempty"`
	ProjectSlug   string `json:"project_slug,omitempty"`
	ProjectName   string `json:"project_name,omitempty"`
	// White-label attribution. Stored as text on the sandbox row by Phase 2A.
	ExternalWorkspaceID string `json:"external_workspace_id,omitempty"`
	ExternalUserID      string `json:"external_user_id,omitempty"`
	ExternalProjectID   string `json:"external_project_id,omitempty"`
	// IdempotencyKey prevents duplicate sandboxes when a create request is retried.
	IdempotencyKey string `json:"-"`

	// Environment names the environment the sandbox starts from; EnvironmentID
	// picks one by id. Neither means the workspace's effective default.
	Environment   string `json:"-"`
	EnvironmentID string `json:"-"`
	// NoEnv gives the sandbox nothing of the owner's, permanently.
	NoEnv bool `json:"-"`
	// SetupFile is a script run once in the background after the sandbox is
	// ready (UTF-8, at most 64 KiB, no NUL byte). See SetupStatus on the result.
	SetupFile string `json:"-"`
	// BillTo bills this create to an organization (id, slug or name) instead of
	// the account setting. Sent as the bill_to body parameter.
	BillTo string `json:"-"`
	// WaitSeconds makes the create request itself wait up to that many seconds
	// (2 to 120) for the sandbox to be ready. The create answers 201 either
	// way; check Ready, or CreateSandboxResult.WaitOutcome.
	WaitSeconds int `json:"-"`
	// WaitForReady waits for readiness with the API's default budget (30 s) and
	// fails with 504 SANDBOX_READY_TIMEOUT if the sandbox is not ready.
	WaitForReady bool `json:"-"`
}

// SandboxResourceContract is the exact, versioned shape assigned to a sandbox.
// Instance responses use id/version; catalog responses use contract_id/contract_version.
type SandboxResourceContract struct {
	ID         string       `json:"id"`
	Version    string       `json:"version"`
	Product    string       `json:"product"`
	Size       ComputerSize `json:"size"`
	VCPUs      int          `json:"vcpus"`
	MemoryMB   int          `json:"memory_mb"`
	DiskSizeMB int          `json:"disk_size_mb"`
}

// SandboxSnapshot is a durable point-in-time sandbox checkpoint.
type SandboxSnapshot struct {
	ID               string `json:"id"`
	SandboxID        string `json:"sandbox_id"`
	Status           string `json:"status"`
	Comment          string `json:"comment"`
	Keep             bool   `json:"keep"`
	RetentionSeconds int    `json:"retention_seconds"`
	LastUsedAt       string `json:"last_used_at"`
	ExpiresAt        string `json:"expires_at"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// CreateSandboxSnapshotInput configures checkpoint retention.
type CreateSandboxSnapshotInput struct {
	Comment           string `json:"comment,omitempty"`
	Keep              bool   `json:"keep,omitempty"`
	RetentionSeconds  int    `json:"retention_seconds,omitempty"`
	ExpirationSeconds int    `json:"expiration_seconds,omitempty"`
	ExpirationDays    int    `json:"expiration_days,omitempty"`
}

// Sandbox is the native representation returned by /sandboxes.
type Sandbox struct {
	ID                  string                   `json:"id"`
	Name                string                   `json:"name"`
	Slug                string                   `json:"slug"`
	State               string                   `json:"state"`
	TemplateID          string                   `json:"template_id"`
	Size                ComputerSize             `json:"size"`
	ResourceContract    *SandboxResourceContract `json:"resource_contract"`
	CPUCount            int                      `json:"cpu_count"`
	MemoryMB            int                      `json:"memory_mb"`
	DiskSizeMB          int                      `json:"disk_size_mb"`
	TenantID            string                   `json:"tenant_id"`
	WorkspaceID         string                   `json:"workspace_id"`
	ProjectID           string                   `json:"project_id"`
	ExternalWorkspaceID string                   `json:"external_workspace_id"`
	ExternalUserID      string                   `json:"external_user_id"`
	ExternalProjectID   string                   `json:"external_project_id"`
	Ready               bool                     `json:"ready"`
	Persistent          bool                     `json:"persistent"`
	AlwaysOn            bool                     `json:"always_on"`
	TimeoutSec          int                      `json:"timeout_sec"`
	TimeoutRemainingMS  *int64                   `json:"timeout_remaining_ms"`
	IdleTimeoutSec      int                      `json:"idle_timeout_sec"`
	BootPath            string                   `json:"boot_path"`
	BootMS              int                      `json:"boot_ms"`
	PreviewURL          string                   `json:"preview_url"`
	PreviewDomain       string                   `json:"preview_domain"`
	Metadata            map[string]interface{}   `json:"metadata"`
	CreatedAt           string                   `json:"created_at"`
	StartedAt           string                   `json:"started_at"`
	ReadyAt             string                   `json:"ready_at"`
	DestroyedAt         string                   `json:"destroyed_at"`
	TotalRuntimeSec     int                      `json:"total_runtime_sec"`

	// MachineEnvironment and MachineSetup are the environment and setup
	// blocks of the sandbox JSON.
	MachineEnvironment
	MachineSetup
}

type createSandboxRequest struct {
	Name                string            `json:"name,omitempty"`
	Size                ComputerSize      `json:"size,omitempty"`
	TemplateID          string            `json:"template_id"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	Env                 map[string]string `json:"env,omitempty"`
	TimeoutSec          int               `json:"timeout_sec,omitempty"`
	IdleTimeoutSec      int               `json:"idle_timeout_sec,omitempty"`
	CPUCount            int               `json:"cpu_count,omitempty"`
	MemoryMB            int               `json:"memory_mb,omitempty"`
	DiskSizeMB          int               `json:"disk_size_mb,omitempty"`
	Persistent          *bool             `json:"persistent,omitempty"`
	AlwaysOn            bool              `json:"always_on,omitempty"`
	Region              string            `json:"region,omitempty"`
	WorkspaceID         string            `json:"workspace_id,omitempty"`
	WorkspaceSlug       string            `json:"workspace_slug,omitempty"`
	WorkspaceName       string            `json:"workspace_name,omitempty"`
	ProjectID           string            `json:"project_id,omitempty"`
	ProjectSlug         string            `json:"project_slug,omitempty"`
	ProjectName         string            `json:"project_name,omitempty"`
	ExternalWorkspaceID string            `json:"external_workspace_id,omitempty"`
	ExternalUserID      string            `json:"external_user_id,omitempty"`
	ExternalProjectID   string            `json:"external_project_id,omitempty"`
	AllowProvision      *bool             `json:"allow_provision,omitempty"`
	Environment         string            `json:"environment,omitempty"`
	EnvironmentID       string            `json:"environment_id,omitempty"`
	NoEnv               bool              `json:"no_env,omitempty"`
	SetupFile           string            `json:"setup_file,omitempty"`
	BillTo              string            `json:"bill_to,omitempty"`
}

// ListSandboxesInput contains canonical tenant-scoped sandbox filters.
type ListSandboxesInput struct {
	ExternalWorkspaceID string
	ExternalUserID      string
	ExternalProjectID   string

	// WorkspaceID and ProjectID narrow to a workspace or project. A
	// workspace-bound key or scoped token is always limited to its own.
	WorkspaceID string
	ProjectID   string
	// Tags keeps only sandboxes whose tags contain every key and value
	// (tags[key]=value).
	Tags map[string]string
	// State is one lifecycle state; naming "error" lists errored sandboxes only.
	State      string
	TemplateID string
	// Search is a substring of name, id or slug.
	Search string
	// Sort defaults to "created-desc".
	Sort string
	// IncludeErrored shows errored sandboxes beside the rest. Nil leaves the
	// server default, which is on when an external attribution filter is given.
	IncludeErrored *bool
	// Page starts at 1; Limit defaults to 50.
	Page  int
	Limit int
}

// SandboxPageMeta is the paging block of a sandbox list.
type SandboxPageMeta struct {
	Page        int  `json:"page"`
	Limit       int  `json:"limit"`
	Total       int  `json:"total"`
	TotalPages  int  `json:"total_pages"`
	HasNextPage bool `json:"has_next_page"`
	// StateCounts counts every sandbox in the account by state, so errored
	// rows are visible even when the page hides them.
	StateCounts      map[string]int `json:"state_counts"`
	ReservedMemoryMB int64          `json:"reserved_memory_mb"`
}

// SandboxPage is one page of sandboxes.
type SandboxPage struct {
	Data []Sandbox       `json:"data"`
	Meta SandboxPageMeta `json:"meta"`
}

func (in ListSandboxesInput) query() map[string]string {
	q := map[string]string{
		"external_workspace_id": in.ExternalWorkspaceID,
		"external_user_id":      in.ExternalUserID,
		"external_project_id":   in.ExternalProjectID,
		"workspace_id":          in.WorkspaceID,
		"project_id":            in.ProjectID,
		"state":                 in.State,
		"template_id":           in.TemplateID,
		"search":                in.Search,
		"sort":                  in.Sort,
	}
	for k, v := range in.Tags {
		q["tags["+k+"]"] = v
	}
	if in.IncludeErrored != nil {
		q["include_errored"] = fmt.Sprint(*in.IncludeErrored)
	}
	if in.Page > 0 {
		q["page"] = fmt.Sprint(in.Page)
	}
	if in.Limit > 0 {
		q["limit"] = fmt.Sprint(in.Limit)
	}
	return q
}

// SandboxUsage contains measured and provisioned usage for one sandbox.
type SandboxUsage struct {
	SandboxID             string            `json:"sandbox_id"`
	State                 string            `json:"state"`
	RuntimeSec            int               `json:"runtime_sec"`
	ProvisionedVCPUMS     int64             `json:"provisioned_vcpu_ms"`
	ProvisionedMemoryMBMS *int64            `json:"provisioned_memory_mb_ms"`
	CreationCount         int               `json:"creation_count"`
	ActiveCPUMS           *int64            `json:"active_cpu_ms"`
	NetworkIngressBytes   *int64            `json:"network_ingress_bytes"`
	NetworkEgressBytes    *int64            `json:"network_egress_bytes"`
	MeasurementStatus     map[string]string `json:"measurement_status"`
	EstimatedCostCents    int               `json:"estimated_cost_cents"`
	TimeoutSec            int               `json:"timeout_sec"`
	TimeoutRemainingMS    *int64            `json:"timeout_remaining_ms"`
}

type sandboxResponse struct {
	Data Sandbox
}

func (r *sandboxResponse) UnmarshalJSON(data []byte) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		return json.Unmarshal(envelope.Data, &r.Data)
	}
	return json.Unmarshal(data, &r.Data)
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

// CreateSandboxResult is a created sandbox plus what the create response
// headers say.
type CreateSandboxResult struct {
	Sandbox *Sandbox
	// WaitOutcome is "ready" or "timeout" when WaitSeconds was used, else "".
	WaitOutcome string
	// BilledOrganizationID is the organization the create billed
	// (X-Miosa-Bill-To), and BillToSource why ("request", "setting",
	// "credential").
	BilledOrganizationID string
	BillToSource         BillToSource
}

// Create provisions a sandbox (a computer with the miosa-sandbox template).
func (s *SandboxesService) Create(ctx context.Context, input CreateSandboxInput) (*Sandbox, error) {
	res, err := s.CreateDetailed(ctx, input)
	if err != nil {
		return nil, err
	}
	return res.Sandbox, nil
}

// CreateDetailed is Create that also returns the response headers' data: how a
// ?wait=N create ended, and which organization was billed.
func (s *SandboxesService) CreateDetailed(ctx context.Context, input CreateSandboxInput) (*CreateSandboxResult, error) {
	template := input.TemplateID
	if template == "" {
		template = input.Template
	}
	if template == "" {
		template = SandboxTemplate
	}
	size, err := resolveSandboxSize(input)
	if err != nil {
		return nil, err
	}
	if input.SetupFile != "" {
		if err := ValidateSetupFile(input.SetupFile); err != nil {
			return nil, err
		}
	}
	if input.WaitSeconds != 0 && (input.WaitSeconds < 2 || input.WaitSeconds > 120) {
		return nil, fmt.Errorf("WaitSeconds must be between 2 and 120, got %d", input.WaitSeconds)
	}

	request := createSandboxRequest{
		Name:                input.Name,
		Size:                size,
		TemplateID:          template,
		Metadata:            input.Metadata,
		Env:                 input.Env,
		TimeoutSec:          input.TimeoutSec,
		IdleTimeoutSec:      input.IdleTimeoutSec,
		CPUCount:            input.CPUCount,
		MemoryMB:            input.MemoryMB,
		DiskSizeMB:          input.DiskSizeMB,
		Persistent:          input.Persistent,
		AlwaysOn:            input.AlwaysOn,
		Region:              input.Region,
		WorkspaceID:         input.WorkspaceID,
		WorkspaceSlug:       input.WorkspaceSlug,
		WorkspaceName:       input.WorkspaceName,
		ProjectID:           input.ProjectID,
		ProjectSlug:         input.ProjectSlug,
		ProjectName:         input.ProjectName,
		ExternalWorkspaceID: input.ExternalWorkspaceID,
		ExternalUserID:      input.ExternalUserID,
		ExternalProjectID:   input.ExternalProjectID,
		AllowProvision:      input.AllowProvision,
		Environment:         input.Environment,
		EnvironmentID:       input.EnvironmentID,
		NoEnv:               input.NoEnv,
		SetupFile:           input.SetupFile,
		BillTo:              input.BillTo,
	}
	path := "/sandboxes"
	client := s.client
	switch {
	case input.WaitSeconds > 0:
		path += "?wait=" + strconv.Itoa(input.WaitSeconds)
		// The wait happens inside the request, so the HTTP timeout must outlast it.
		client = client.withMinTimeout(time.Duration(input.WaitSeconds+15) * time.Second)
	case input.WaitForReady:
		path += "?wait=true"
		client = client.withMinTimeout(45 * time.Second)
	}
	headers := map[string]string{}
	if key := input.IdempotencyKey; key != "" {
		headers["Idempotency-Key"] = key
	}
	var response sandboxResponse
	hdr, _, err := client.sendJSONResponse(ctx, http.MethodPost, path, request, &response, headers)
	if err != nil {
		return nil, err
	}
	return &CreateSandboxResult{
		Sandbox:              &response.Data,
		WaitOutcome:          hdr.Get(HeaderWaitOutcome),
		BilledOrganizationID: hdr.Get(HeaderBillTo),
		BillToSource:         BillToSource(hdr.Get(HeaderBillToSource)),
	}, nil
}

// Extend replaces the sandbox activity timeout and resets its deadline.
func (s *SandboxesService) Extend(ctx context.Context, id string, timeoutSec int) (*Sandbox, error) {
	if timeoutSec < 0 {
		return nil, fmt.Errorf("timeoutSec must not be negative")
	}
	if timeoutSec == 0 {
		return s.lifecycle(ctx, id, "extend", map[string]interface{}{})
	}
	return s.lifecycle(ctx, id, "extend", map[string]int{"timeout_sec": timeoutSec})
}

// Stop stops the sandbox while preserving state according to its persistence policy.
func (s *SandboxesService) Stop(ctx context.Context, id string) (*Sandbox, error) {
	return s.lifecycle(ctx, id, "stop", map[string]interface{}{})
}

// Pause snapshots and suspends a sandbox for later resume.
func (s *SandboxesService) Pause(ctx context.Context, id string) (*Sandbox, error) {
	return s.lifecycle(ctx, id, "pause", map[string]interface{}{})
}

// Resume restores a paused sandbox.
func (s *SandboxesService) Resume(ctx context.Context, id string) (*Sandbox, error) {
	return s.lifecycle(ctx, id, "resume", map[string]interface{}{})
}

// ResumeWithIdempotency restores a paused sandbox with retry protection.
func (s *SandboxesService) ResumeWithIdempotency(ctx context.Context, id, idempotencyKey string) (*Sandbox, error) {
	var response sandboxResponse
	if err := s.client.postJSONIdempotent(ctx, "/sandboxes/"+id+"/resume", map[string]interface{}{}, &response, idempotencyKey); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (s *SandboxesService) lifecycle(ctx context.Context, id, action string, body interface{}) (*Sandbox, error) {
	var response sandboxResponse
	if err := s.client.postJSON(ctx, "/sandboxes/"+id+"/"+action, body, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// CreateSnapshot creates a retained checkpoint of a running sandbox.
func (s *SandboxesService) CreateSnapshot(ctx context.Context, id string, input CreateSandboxSnapshotInput) (*SandboxSnapshot, error) {
	var response apiResponse[SandboxSnapshot]
	if err := s.client.postJSON(ctx, "/sandboxes/"+id+"/snapshots", input, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// ListSnapshots lists non-deleted checkpoints for a sandbox.
func (s *SandboxesService) ListSnapshots(ctx context.Context, id string) ([]SandboxSnapshot, error) {
	var response apiResponse[[]SandboxSnapshot]
	if err := s.client.getJSON(ctx, "/sandboxes/"+id+"/snapshots", &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// RestoreSnapshot creates a sandbox from a ready, unexpired checkpoint.
func (s *SandboxesService) RestoreSnapshot(ctx context.Context, id, snapshotID string) (*Sandbox, error) {
	var response sandboxResponse
	if err := s.client.postJSON(ctx, "/sandboxes/"+id+"/restore/"+snapshotID, map[string]interface{}{}, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// DeleteSnapshot retires a checkpoint and schedules object cleanup.
func (s *SandboxesService) DeleteSnapshot(ctx context.Context, id, snapshotID string) error {
	return s.client.deleteJSON(ctx, "/sandboxes/"+id+"/snapshots/"+snapshotID, nil)
}

// Get fetches a native sandbox by ID.
func (s *SandboxesService) Get(ctx context.Context, id string) (*Sandbox, error) {
	var response sandboxResponse
	if err := s.client.getJSON(ctx, "/sandboxes/"+id, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// List returns sandboxes owned by the authenticated tenant: the first page
// (50 by default). Use ListPage for paging metadata and ListAll to walk every page.
func (s *SandboxesService) List(ctx context.Context, input ListSandboxesInput) ([]Sandbox, error) {
	page, err := s.ListPage(ctx, input)
	if err != nil {
		return nil, err
	}
	return page.Data, nil
}

// ListPage returns one page of sandboxes with its paging metadata.
func (s *SandboxesService) ListPage(ctx context.Context, input ListSandboxesInput) (*SandboxPage, error) {
	var page SandboxPage
	if err := s.client.getJSON(ctx, "/sandboxes"+buildQuery(input.query()), &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// ListAll walks every page of the filtered list and returns all sandboxes. Any
// Page set on input is ignored.
func (s *SandboxesService) ListAll(ctx context.Context, input ListSandboxesInput) ([]Sandbox, error) {
	var all []Sandbox
	input.Page = 1
	for {
		page, err := s.ListPage(ctx, input)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Data...)
		if !page.Meta.HasNextPage || len(page.Data) == 0 {
			return all, nil
		}
		input.Page++
	}
}

// Usage returns measured runtime, provisioned vCPU time, and timeout visibility.
func (s *SandboxesService) Usage(ctx context.Context, id string) (*SandboxUsage, error) {
	var response apiResponse[SandboxUsage]
	if err := s.client.getJSON(ctx, "/sandboxes/"+id+"/usage", &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func resolveSandboxSize(input CreateSandboxInput) (ComputerSize, error) {
	type shape struct{ cpu, memory, disk int }
	contracts := map[ComputerSize]shape{
		SizeXS:     {1, 2_048, 10_240},
		SizeSmall:  {2, 4_096, 10_240},
		SizeMedium: {4, 8_192, 20_480},
		SizeLarge:  {8, 16_384, 40_960},
		SizeXL:     {16, 32_768, 81_920},
	}
	supplied := 0
	for _, value := range []int{input.CPUCount, input.MemoryMB, input.DiskSizeMB} {
		if value != 0 {
			supplied++
		}
	}
	requested := input.Size
	if supplied == 0 {
		if requested == "" {
			return "", nil
		}
		if _, ok := contracts[requested]; !ok {
			return "", fmt.Errorf("unknown sandbox size %q", requested)
		}
		return requested, nil
	}
	if supplied != 3 {
		return "", fmt.Errorf("raw sandbox resources require CPUCount, MemoryMB, and DiskSizeMB together")
	}
	wanted := shape{input.CPUCount, input.MemoryMB, input.DiskSizeMB}
	for name, contract := range contracts {
		if contract == wanted {
			if requested != "" && requested != name {
				return "", fmt.Errorf("raw sandbox resources match %s, not %s", name, requested)
			}
			return name, nil
		}
	}
	return "", fmt.Errorf("raw sandbox resources must match a named size contract")
}

// Delete permanently destroys a native sandbox.
func (s *SandboxesService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/sandboxes/"+id, nil)
}

// Deploy publishes a sandbox using the default MIOSA Deploy path unless DeploymentType is set.
func (s *SandboxesService) Deploy(ctx context.Context, id string, input SandboxDeployInput) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSONIdempotent(ctx, "/sandboxes/"+id+"/deploy", input, &out, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return out, nil
}

// DeployDocker publishes a sandbox through the workspace App Engine appliance.
func (s *SandboxesService) DeployDocker(ctx context.Context, id string, input SandboxDeployInput) (map[string]interface{}, error) {
	input.DeploymentType = "docker_deploy"
	return s.Deploy(ctx, id, input)
}
