package miosa

import (
	"context"
	"fmt"
	"net/http"
)

// ─── Tenant preview-domain + branding ─────────────────────────────────────────

// PreviewDomainData is the API representation of a tenant custom preview domain.
type PreviewDomainData struct {
	Domain      string `json:"domain"`
	VerifiedAt  string `json:"verified_at,omitempty"`
	CnameTarget string `json:"cname_target,omitempty"`
}

// GetPreviewDomain returns the tenant's custom preview domain.
func (s *TenantService) GetPreviewDomain(ctx context.Context) (*PreviewDomainData, error) {
	var out PreviewDomainData
	if err := s.client.getJSON(ctx, "/tenant/preview-domain", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetPreviewDomain sets the tenant's custom preview domain.
func (s *TenantService) SetPreviewDomain(ctx context.Context, domain string) (*PreviewDomainData, error) {
	var out PreviewDomainData
	if err := s.client.putJSON(ctx, "/tenant/preview-domain", map[string]string{"domain": domain}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VerifyPreviewDomain triggers DNS verification for the custom preview domain.
func (s *TenantService) VerifyPreviewDomain(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/tenant/preview-domain/verify", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeletePreviewDomain removes the tenant's custom preview domain.
func (s *TenantService) DeletePreviewDomain(ctx context.Context) error {
	return s.client.deleteJSON(ctx, "/tenant/preview-domain", nil)
}

// GetBranding returns the tenant's branding settings.
func (s *TenantService) GetBranding(ctx context.Context) (*BrandingData, error) {
	var out BrandingData
	if err := s.client.getJSON(ctx, "/tenant/branding", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetBranding updates the tenant's branding settings.
func (s *TenantService) SetBranding(ctx context.Context, branding BrandingData) (*BrandingData, error) {
	var out BrandingData
	if err := s.client.putJSON(ctx, "/tenant/branding", branding, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteBranding resets tenant branding to platform defaults.
func (s *TenantService) DeleteBranding(ctx context.Context) error {
	return s.client.deleteJSON(ctx, "/tenant/branding", nil)
}

// ─── Sandbox fork ─────────────────────────────────────────────────────────────

// ForkSandboxInput is the request body for SandboxesService.Fork.
type ForkSandboxInput struct {
	SnapshotID     string `json:"snapshot_id,omitempty"`
	Name           string `json:"name,omitempty"`
	ExternalUserID string `json:"external_user_id,omitempty"`
}

// Fork creates a new sandbox by forking an existing one.
func (s *SandboxesService) Fork(ctx context.Context, sandboxID string, input ForkSandboxInput) (*Computer, error) {
	var out Computer
	if err := s.client.postJSON(ctx, fmt.Sprintf("/sandboxes/%s/fork", sandboxID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Sandbox files ────────────────────────────────────────────────────────────

// SandboxFilesService provides file-system operations on a sandbox.
type SandboxFilesService struct {
	client    *Client
	sandboxID string
}

// SandboxFileEntry is a single node in the file tree.
type SandboxFileEntry struct {
	Path       string             `json:"path"`
	Type       string             `json:"type"` // "dir" | "file"
	Name       string             `json:"name"`
	Size       *int64             `json:"size,omitempty"`
	ModifiedAt string             `json:"modified_at,omitempty"`
	Children   []SandboxFileEntry `json:"children,omitempty"`
}

// WriteFileInput is a single file payload for WriteMany.
type WriteFileInput struct {
	Path          string `json:"path"`
	ContentBase64 string `json:"content_base64"`
}

// WriteManyResult is the response from WriteMany.
type WriteManyResult struct {
	Written []map[string]interface{} `json:"written"`
	Failed  []map[string]interface{} `json:"failed"`
}

// Tree returns a recursive directory listing rooted at path.
func (s *SandboxFilesService) Tree(ctx context.Context, path string, depth int) (*SandboxFileEntry, error) {
	if path == "" {
		path = "/workspace"
	}
	if depth <= 0 {
		depth = 3
	}
	qs := fmt.Sprintf("?path=%s&depth=%d", path, depth)
	var out SandboxFileEntry
	if err := s.client.getJSON(ctx, fmt.Sprintf("/sandboxes/%s/files/tree%s", s.sandboxID, qs), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WriteMany writes multiple files to the sandbox in one request.
// Content is expected as raw bytes; the SDK encodes to base64.
func (s *SandboxFilesService) WriteMany(ctx context.Context, files []WriteFileInput) (*WriteManyResult, error) {
	body := map[string]interface{}{"files": files}
	var out WriteManyResult
	if err := s.client.postJSON(ctx, fmt.Sprintf("/sandboxes/%s/files/write-many", s.sandboxID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Watch opens an SSE stream of file-system change events.
func (s *SandboxFilesService) Watch(ctx context.Context) (<-chan SSEEvent, error) {
	return s.client.streamSSE(ctx, http.MethodGet,
		fmt.Sprintf("/sandboxes/%s/files/watch", s.sandboxID),
		nil,
	)
}

// ─── Sandbox share ────────────────────────────────────────────────────────────

// SandboxShareService manages public share URLs for a sandbox.
type SandboxShareService struct {
	client    *Client
	sandboxID string
}

// ShareData is the API representation of a sandbox share.
type ShareData struct {
	ShareID   string `json:"share_id"`
	ShareURL  string `json:"share_url"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Scope     string `json:"scope"`
}

// CreateShareInput is the request body for Create.
type CreateShareInput struct {
	ExpiresIn *int   `json:"expires_in,omitempty"`
	Scope     string `json:"scope"`
}

// Create mints a public share URL for the sandbox.
func (s *SandboxShareService) Create(ctx context.Context, input CreateShareInput) (*ShareData, error) {
	if input.Scope == "" {
		input.Scope = "read"
	}
	var out ShareData
	if err := s.client.postJSON(ctx, fmt.Sprintf("/sandboxes/%s/shares", s.sandboxID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns all share URLs for the sandbox.
func (s *SandboxShareService) List(ctx context.Context) ([]ShareData, error) {
	var wrapper struct {
		Data []ShareData `json:"data"`
	}
	if err := s.client.getJSON(ctx, fmt.Sprintf("/sandboxes/%s/shares", s.sandboxID), &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data, nil
}

// Revoke deletes a share URL by ID.
func (s *SandboxShareService) Revoke(ctx context.Context, shareID string) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/sandboxes/%s/shares/%s", s.sandboxID, shareID), nil)
}

// ─── SandboxEnvService — full CRUD ────────────────────────────────────────────

// EnvVar represents a single sandbox env variable entry.
type EnvVar struct {
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	Encrypted bool   `json:"encrypted,omitempty"`
}

// Set replaces or creates env vars for the sandbox.
func (s *SandboxEnvService) Set(ctx context.Context, vars []EnvVar) (map[string]interface{}, error) {
	body := map[string]interface{}{"vars": vars}
	var out map[string]interface{}
	if err := s.client.putJSON(ctx, fmt.Sprintf("/sandboxes/%s/env", s.sandboxID), body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete removes a single env var by key.
func (s *SandboxEnvService) Delete(ctx context.Context, key string) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/sandboxes/%s/env/%s", s.sandboxID, key), nil)
}

// ─── SandboxProcessesService ─────────────────────────────────────────────────

// SandboxProcessesService manages long-running background processes on a sandbox.
type SandboxProcessesService struct {
	client    *Client
	sandboxID string
}

// ProcessInfo represents a running background process.
type ProcessInfo struct {
	PID       string `json:"pid"`
	Name      string `json:"name,omitempty"`
	Command   string `json:"command"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	ExitCode  *int   `json:"exit_code,omitempty"`
}

// StartProcessInput is the request body for Start.
type StartProcessInput struct {
	Command string            `json:"command"`
	Env     map[string]string `json:"env,omitempty"`
	Name    string            `json:"name,omitempty"`
}

// Start launches a long-running background process.
func (s *SandboxProcessesService) Start(ctx context.Context, input StartProcessInput) (*ProcessInfo, error) {
	var out ProcessInfo
	if err := s.client.postJSON(ctx, fmt.Sprintf("/sandboxes/%s/processes", s.sandboxID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns all running processes.
func (s *SandboxProcessesService) List(ctx context.Context) ([]ProcessInfo, error) {
	var wrapper struct {
		Data []ProcessInfo `json:"data"`
	}
	if err := s.client.getJSON(ctx, fmt.Sprintf("/sandboxes/%s/processes", s.sandboxID), &wrapper); err != nil {
		// Try direct list
		var list []ProcessInfo
		if err2 := s.client.getJSON(ctx, fmt.Sprintf("/sandboxes/%s/processes", s.sandboxID), &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	return wrapper.Data, nil
}

// Get returns a single process by PID.
func (s *SandboxProcessesService) Get(ctx context.Context, pid string) (*ProcessInfo, error) {
	var out ProcessInfo
	if err := s.client.getJSON(ctx, fmt.Sprintf("/sandboxes/%s/processes/%s", s.sandboxID, pid), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stop sends SIGTERM (then SIGKILL after 5 s) to a process.
func (s *SandboxProcessesService) Stop(ctx context.Context, pid string) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/sandboxes/%s/processes/%s", s.sandboxID, pid), nil)
}

// Logs returns the last tail lines of stdout/stderr from a process.
func (s *SandboxProcessesService) Logs(ctx context.Context, pid string, tail int) (string, error) {
	if tail <= 0 {
		tail = 200
	}
	var out string
	if err := s.client.getJSON(ctx, fmt.Sprintf("/sandboxes/%s/processes/%s/logs?tail=%d", s.sandboxID, pid, tail), &out); err != nil {
		// logs may be plain text
		var raw map[string]interface{}
		if err2 := s.client.getJSON(ctx, fmt.Sprintf("/sandboxes/%s/processes/%s/logs?tail=%d", s.sandboxID, pid, tail), &raw); err2 == nil {
			if data, ok := raw["data"].(string); ok {
				return data, nil
			}
		}
		return "", err
	}
	return out, nil
}

// Stream opens an SSE stream of stdout/stderr lines from a process.
func (s *SandboxProcessesService) Stream(ctx context.Context, pid string) (<-chan SSEEvent, error) {
	return s.client.streamSSE(ctx, http.MethodGet,
		fmt.Sprintf("/sandboxes/%s/processes/%s/stream", s.sandboxID, pid),
		nil,
	)
}

// ─── Extend SandboxHandle with new sub-services ──────────────────────────────

// newSandboxHandleFull wires all sub-services including Phase 1-4 additions.
// Call this instead of newSandboxHandle for the full surface.
func newSandboxHandleFull(c *Client, id string) *SandboxHandleFull {
	return &SandboxHandleFull{
		SandboxHandle: *newSandboxHandle(c, id),
		Files:         &SandboxFilesService{client: c, sandboxID: id},
		Share:         &SandboxShareService{client: c, sandboxID: id},
		Processes:     &SandboxProcessesService{client: c, sandboxID: id},
	}
}

// SandboxHandleFull extends SandboxHandle with Phase 1-4 sub-services.
type SandboxHandleFull struct {
	SandboxHandle
	Files     *SandboxFilesService
	Share     *SandboxShareService
	Processes *SandboxProcessesService
}

// GetFullHandle returns a SandboxHandleFull for an existing sandbox.
func (s *SandboxesService) GetFullHandle(id string) *SandboxHandleFull {
	return newSandboxHandleFull(s.client, id)
}

// ─── QuotasService ────────────────────────────────────────────────────────────

// QuotasService manages per-external_user_id quota overrides.
type QuotasService struct {
	client *Client
}

// QuotaData is the API representation of per-user quotas.
type QuotaData struct {
	ExternalUserID string `json:"external_user_id"`
	MaxSandboxes   *int   `json:"max_sandboxes,omitempty"`
	MaxConcurrent  *int   `json:"max_concurrent,omitempty"`
	MaxStorageGB   *int   `json:"max_storage_gb,omitempty"`
	MaxCreditCents *int   `json:"max_credit_cents,omitempty"`
	// Current usage
	CurrentSandboxes int `json:"current_sandboxes,omitempty"`
}

// SetQuotaInput is the request body for Set.
type SetQuotaInput struct {
	MaxSandboxes   *int `json:"max_sandboxes,omitempty"`
	MaxConcurrent  *int `json:"max_concurrent,omitempty"`
	MaxStorageGB   *int `json:"max_storage_gb,omitempty"`
	MaxCreditCents *int `json:"max_credit_cents,omitempty"`
}

// Get returns current quota limits + usage for an external user.
func (s *QuotasService) Get(ctx context.Context, externalUserID string) (*QuotaData, error) {
	var out QuotaData
	if err := s.client.getJSON(ctx, fmt.Sprintf("/quotas/external/%s", externalUserID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Set overrides quota limits for an external user.
func (s *QuotasService) Set(ctx context.Context, externalUserID string, input SetQuotaInput) (*QuotaData, error) {
	var out QuotaData
	if err := s.client.putJSON(ctx, fmt.Sprintf("/quotas/external/%s", externalUserID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete reverts an external user's quota to the tenant default.
func (s *QuotasService) Delete(ctx context.Context, externalUserID string) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/quotas/external/%s", externalUserID), nil)
}

// ─── UsageService.GetRollup ───────────────────────────────────────────────────

// UsageRollupInput is the query for the grouped usage rollup.
type UsageRollupInput struct {
	// GroupBy accepts "external_user_id", "external_project_id", "workspace_id".
	GroupBy string
	// Period accepts "7d", "30d", "month-to-date". Use Start+End for custom range.
	Period string
	Start  string
	End    string
	// ExternalUserID narrows results to a single user.
	ExternalUserID string
}

// UsageRollupResult is the response from the /usage rollup endpoint.
type UsageRollupResult struct {
	PeriodStart string           `json:"period_start"`
	PeriodEnd   string           `json:"period_end"`
	Results     []UsageRollupRow `json:"results"`
}

// UsageRollupRow is one grouped row.
type UsageRollupRow struct {
	ExternalUserID    string  `json:"external_user_id,omitempty"`
	ExternalProjectID string  `json:"external_project_id,omitempty"`
	WorkspaceID       string  `json:"workspace_id,omitempty"`
	SandboxSeconds    int64   `json:"sandbox_seconds"`
	ComputerSeconds   int64   `json:"computer_seconds"`
	StorageGBHours    float64 `json:"storage_gb_hours"`
	CreditCents       int64   `json:"credit_cents"`
}

// GetRollup returns grouped usage totals for the given period.
func (s *UsageService) GetRollup(ctx context.Context, input UsageRollupInput) (*UsageRollupResult, error) {
	params := map[string]string{}
	if input.GroupBy != "" {
		params["group_by"] = input.GroupBy
	}
	if input.Period != "" {
		params["period"] = input.Period
	}
	if input.Start != "" {
		params["start"] = input.Start
	}
	if input.End != "" {
		params["end"] = input.End
	}
	if input.ExternalUserID != "" {
		params["external_user_id"] = input.ExternalUserID
	}
	var out UsageRollupResult
	if err := s.client.getJSON(ctx, "/usage"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Tenant events SSE ────────────────────────────────────────────────────────

// StreamEvents opens a tenant-wide SSE event stream.
//
// types is a comma-separated list of event type globs, e.g.
// "sandbox.*,webhook.delivered". Pass an empty string for all events.
func (s *TenantService) StreamEvents(ctx context.Context, types string) (<-chan SSEEvent, error) {
	path := "/events/stream"
	if types != "" {
		path += "?types=" + types
	}
	return s.client.streamSSE(ctx, http.MethodGet, path, nil)
}

// ─── SandboxesService.PreviewToken ────────────────────────────────────────────

// PreviewTokenInput configures the preview token.
type PreviewTokenInput struct {
	ExpiresIn int    `json:"expires_in,omitempty"` // default 3600
	Scope     string `json:"scope,omitempty"`      // "read" | "interact"
}

// PreviewTokenData is the response from /preview-token.
type PreviewTokenData struct {
	Token     string `json:"token"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
	Scope     string `json:"scope"`
}

// PreviewToken mints a short-lived preview token for a sandbox.
func (s *SandboxesService) PreviewToken(ctx context.Context, sandboxID string, input PreviewTokenInput) (*PreviewTokenData, error) {
	if input.ExpiresIn == 0 {
		input.ExpiresIn = 3600
	}
	if input.Scope == "" {
		input.Scope = "read"
	}
	var out PreviewTokenData
	if err := s.client.postJSON(ctx, fmt.Sprintf("/sandboxes/%s/preview-token", sandboxID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
