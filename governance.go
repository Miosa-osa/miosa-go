package miosa

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ── Types ─────────────────────────────────────────────────────────────────────

// PolicyDoc is a free-form policy map matching the Phase 6 policy schema.
type PolicyDoc map[string]interface{}

// EffectivePolicyField is a single field in an effective-policy response,
// annotated with the tier it was resolved from.
type EffectivePolicyField struct {
	Value  interface{} `json:"value"`
	Source string      `json:"source"` // "user" | "workspace" | "tenant" | "platform"
}

// EffectivePolicySection is a section of the resolved effective policy (e.g. lifecycle, quotas).
type EffectivePolicySection map[string]EffectivePolicyField

// EffectivePolicy is the resolved policy for an external user, with each field
// annotated with its source tier.
type EffectivePolicy struct {
	Lifecycle map[string]EffectivePolicyField `json:"lifecycle"`
	Quotas    map[string]EffectivePolicyField `json:"quotas"`
	Sizing    map[string]EffectivePolicyField `json:"sizing,omitempty"`
	Features  map[string]EffectivePolicyField `json:"features,omitempty"`
	Egress    map[string]EffectivePolicyField `json:"egress,omitempty"`
}

// MemberRecord is a tenant or workspace member.
type MemberRecord struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
	Role  string `json:"role"`
}

// BulkJobResponse is returned by bulk-operation endpoints.
type BulkJobResponse struct {
	Queued int    `json:"queued"`
	JobID  string `json:"job_id"`
}

// BulkJobStatus is returned by GET /bulk/jobs/{id}.
type BulkJobStatus struct {
	ID        string      `json:"id"`
	Status    string      `json:"status"`
	Processed int         `json:"processed,omitempty"`
	Errors    interface{} `json:"errors,omitempty"`
}

// Invoice is a billing invoice.
type Invoice struct {
	ID     string  `json:"id"`
	Amount float64 `json:"amount"`
}

// PaymentMethod is a saved payment method (read-only).
type PaymentMethod struct {
	ID    string `json:"id"`
	Brand string `json:"brand"`
	Last4 string `json:"last4"`
}

// ImpersonateResult is returned by POST /admin/impersonate.
type ImpersonateResult struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// ScopedApiKeyResult is returned by POST /api-keys/scoped.
type ScopedApiKeyResult struct {
	ID    string `json:"id"`
	Token string `json:"token,omitempty"`
	Key   string `json:"key,omitempty"`
}

// ── TenantPolicyService ───────────────────────────────────────────────────────

// TenantPolicyService manages tenant-level policy.
type TenantPolicyService struct {
	client *Client
}

// Get returns the current tenant policy.
func (s *TenantPolicyService) Get(ctx context.Context) (PolicyDoc, error) {
	var out PolicyDoc
	if err := s.client.getJSON(ctx, "/tenant/policy", &out); err != nil {
		return nil, fmt.Errorf("TenantPolicyService.Get: %w", err)
	}
	return unwrapPolicyDoc(out), nil
}

// Set replaces the tenant policy.
func (s *TenantPolicyService) Set(ctx context.Context, policy PolicyDoc) (PolicyDoc, error) {
	var out PolicyDoc
	if err := s.client.putJSON(ctx, "/tenant/policy", policy, &out); err != nil {
		return nil, fmt.Errorf("TenantPolicyService.Set: %w", err)
	}
	return unwrapPolicyDoc(out), nil
}

// Delete reverts the tenant policy to platform defaults.
func (s *TenantPolicyService) Delete(ctx context.Context) error {
	return s.client.deleteJSON(ctx, "/tenant/policy", nil)
}

// ── TenantMembersService ──────────────────────────────────────────────────────

// TenantMembersService manages tenant membership.
type TenantMembersService struct {
	client *Client
}

// List returns all tenant members.
func (s *TenantMembersService) List(ctx context.Context) ([]MemberRecord, error) {
	var raw map[string]interface{}
	if err := s.client.getJSON(ctx, "/tenant/members", &raw); err != nil {
		return nil, fmt.Errorf("TenantMembersService.List: %w", err)
	}
	return extractMembers(raw), nil
}

// Invite sends an invitation to email with the given role.
func (s *TenantMembersService) Invite(ctx context.Context, email, role string) (MemberRecord, error) {
	var out MemberRecord
	body := map[string]string{"email": email, "role": role}
	if err := s.client.postJSON(ctx, "/tenant/members", body, &out); err != nil {
		return MemberRecord{}, fmt.Errorf("TenantMembersService.Invite: %w", err)
	}
	return out, nil
}

// UpdateRole changes a member's role.
func (s *TenantMembersService) UpdateRole(ctx context.Context, memberID, role string) (MemberRecord, error) {
	var out MemberRecord
	body := map[string]string{"role": role}
	if err := s.client.patchJSON(ctx, fmt.Sprintf("/tenant/members/%s/role", memberID), body, &out); err != nil {
		return MemberRecord{}, fmt.Errorf("TenantMembersService.UpdateRole: %w", err)
	}
	return out, nil
}

// Remove removes a member from the tenant.
func (s *TenantMembersService) Remove(ctx context.Context, memberID string) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/tenant/members/%s", memberID), nil)
}

// TransferOwnership transfers the owner role to a new user.
func (s *TenantMembersService) TransferOwnership(ctx context.Context, newOwnerUserID string) (map[string]interface{}, error) {
	var out map[string]interface{}
	body := map[string]string{"new_owner_user_id": newOwnerUserID}
	if err := s.client.postJSON(ctx, "/tenant/transfer-ownership", body, &out); err != nil {
		return nil, fmt.Errorf("TenantMembersService.TransferOwnership: %w", err)
	}
	return out, nil
}

// ── TenantEventStreamService ──────────────────────────────────────────────────

// TenantStreamEvent is a single event from the admin SSE stream.
type TenantStreamEvent map[string]interface{}

// TenantEventStreamService streams tenant admin events via SSE.
type TenantEventStreamService struct {
	client *Client
}

// Stream returns a channel that receives admin events. Close ctx to stop.
func (s *TenantEventStreamService) Stream(ctx context.Context, types []string, scope string) (<-chan TenantStreamEvent, error) {
	params := url.Values{}
	if len(types) > 0 {
		params.Set("types", joinStrings(types, ","))
	}
	if scope != "" {
		params.Set("scope", scope)
	}
	path := "/tenant/events/stream"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.client.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("TenantEventStreamService.Stream: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.client.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "miosa-go/"+sdkVersion)

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TenantEventStreamService.Stream: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("TenantEventStreamService.Stream: HTTP %d", resp.StatusCode)
	}

	ch := make(chan TenantStreamEvent, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		scanner := bufio.NewScanner(resp.Body)
		var dataBuf strings.Builder
		flush := func() {
			raw := strings.TrimSpace(dataBuf.String())
			dataBuf.Reset()
			if raw == "" {
				return
			}
			var ev TenantStreamEvent
			if err := json.Unmarshal([]byte(raw), &ev); err != nil {
				ev = TenantStreamEvent{"raw": raw}
			}
			select {
			case ch <- ev:
			case <-ctx.Done():
			}
		}
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			line := scanner.Text()
			if line == "" {
				flush()
			} else if strings.HasPrefix(line, "data:") {
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if dataBuf.Len() > 0 {
					dataBuf.WriteByte('\n')
				}
				dataBuf.WriteString(payload)
			}
		}
		flush()
	}()
	return ch, nil
}

// ── GovernanceTenantService ───────────────────────────────────────────────────

// GovernanceTenantService combines policy + members + events for the tenant.
type GovernanceTenantService struct {
	client  *Client
	Policy  *TenantPolicyService
	Members *TenantMembersService
	Events  *TenantEventStreamService
}

// ── WorkspacePolicyService ────────────────────────────────────────────────────

// WorkspacePolicyService manages workspace-level policy.
type WorkspacePolicyService struct {
	client      *Client
	workspaceID string
}

func (s *WorkspacePolicyService) Get(ctx context.Context) (PolicyDoc, error) {
	var out PolicyDoc
	if err := s.client.getJSON(ctx, fmt.Sprintf("/workspaces/%s/policy", s.workspaceID), &out); err != nil {
		return nil, fmt.Errorf("WorkspacePolicyService.Get: %w", err)
	}
	return unwrapPolicyDoc(out), nil
}

func (s *WorkspacePolicyService) Set(ctx context.Context, policy PolicyDoc) (PolicyDoc, error) {
	var out PolicyDoc
	if err := s.client.putJSON(ctx, fmt.Sprintf("/workspaces/%s/policy", s.workspaceID), policy, &out); err != nil {
		return nil, fmt.Errorf("WorkspacePolicyService.Set: %w", err)
	}
	return unwrapPolicyDoc(out), nil
}

func (s *WorkspacePolicyService) Delete(ctx context.Context) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/workspaces/%s/policy", s.workspaceID), nil)
}

// ── WorkspaceMembersGovernanceService ─────────────────────────────────────────

// WorkspaceMembersGovernanceService manages workspace membership (Phase 6).
type WorkspaceMembersGovernanceService struct {
	client      *Client
	workspaceID string
}

func (s *WorkspaceMembersGovernanceService) List(ctx context.Context) ([]MemberRecord, error) {
	var raw map[string]interface{}
	if err := s.client.getJSON(ctx, fmt.Sprintf("/workspaces/%s/members", s.workspaceID), &raw); err != nil {
		return nil, fmt.Errorf("WorkspaceMembersGovernanceService.List: %w", err)
	}
	return extractMembers(raw), nil
}

func (s *WorkspaceMembersGovernanceService) Invite(ctx context.Context, email, role string) (MemberRecord, error) {
	var out MemberRecord
	body := map[string]string{"email": email, "role": role}
	if err := s.client.postJSON(ctx, fmt.Sprintf("/workspaces/%s/members", s.workspaceID), body, &out); err != nil {
		return MemberRecord{}, fmt.Errorf("WorkspaceMembersGovernanceService.Invite: %w", err)
	}
	return out, nil
}

func (s *WorkspaceMembersGovernanceService) UpdateRole(ctx context.Context, memberID, role string) (MemberRecord, error) {
	var out MemberRecord
	body := map[string]string{"role": role}
	if err := s.client.patchJSON(ctx, fmt.Sprintf("/workspaces/%s/members/%s/role", s.workspaceID, memberID), body, &out); err != nil {
		return MemberRecord{}, fmt.Errorf("WorkspaceMembersGovernanceService.UpdateRole: %w", err)
	}
	return out, nil
}

func (s *WorkspaceMembersGovernanceService) Remove(ctx context.Context, memberID string) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/workspaces/%s/members/%s", s.workspaceID, memberID), nil)
}

// ── WorkspaceGovernanceProxy ──────────────────────────────────────────────────

// WorkspaceGovernanceProxy provides per-workspace policy, members, and transfer.
type WorkspaceGovernanceProxy struct {
	client      *Client
	workspaceID string
	Policy      *WorkspacePolicyService
	Members     *WorkspaceMembersGovernanceService
}

// Transfer moves resources from this workspace to another.
func (p *WorkspaceGovernanceProxy) Transfer(ctx context.Context, resourceIDs []string, targetWorkspaceID string) (map[string]interface{}, error) {
	var out map[string]interface{}
	body := map[string]interface{}{
		"resource_ids":        resourceIDs,
		"target_workspace_id": targetWorkspaceID,
	}
	if err := p.client.postJSON(ctx, fmt.Sprintf("/workspaces/%s/transfer", p.workspaceID), body, &out); err != nil {
		return nil, fmt.Errorf("WorkspaceGovernanceProxy.Transfer: %w", err)
	}
	return out, nil
}

// ── GovernanceWorkspacesService ───────────────────────────────────────────────

// GovernanceWorkspacesService provides workspace CRUD + per-ID proxy.
type GovernanceWorkspacesService struct {
	client *Client
}

// Workspace returns a per-workspace proxy for policy/members/transfer.
func (s *GovernanceWorkspacesService) Workspace(workspaceID string) *WorkspaceGovernanceProxy {
	return &WorkspaceGovernanceProxy{
		client:      s.client,
		workspaceID: workspaceID,
		Policy:      &WorkspacePolicyService{client: s.client, workspaceID: workspaceID},
		Members:     &WorkspaceMembersGovernanceService{client: s.client, workspaceID: workspaceID},
	}
}

func (s *GovernanceWorkspacesService) List(ctx context.Context) (*WorkspaceListResponse, error) {
	var out WorkspaceListResponse
	if err := s.client.getJSON(ctx, "/workspaces", &out); err != nil {
		return nil, fmt.Errorf("GovernanceWorkspacesService.List: %w", err)
	}
	return &out, nil
}

func (s *GovernanceWorkspacesService) Create(ctx context.Context, input CreateWorkspaceInput) (*Workspace, error) {
	var out Workspace
	if err := s.client.postJSON(ctx, "/workspaces", input, &out); err != nil {
		return nil, fmt.Errorf("GovernanceWorkspacesService.Create: %w", err)
	}
	return &out, nil
}

func (s *GovernanceWorkspacesService) Get(ctx context.Context, id string) (*Workspace, error) {
	var out Workspace
	if err := s.client.getJSON(ctx, "/workspaces/"+id, &out); err != nil {
		return nil, fmt.Errorf("GovernanceWorkspacesService.Get: %w", err)
	}
	return &out, nil
}

func (s *GovernanceWorkspacesService) Update(ctx context.Context, id string, input UpdateWorkspaceInput) (*Workspace, error) {
	var out Workspace
	if err := s.client.patchJSON(ctx, "/workspaces/"+id, input, &out); err != nil {
		return nil, fmt.Errorf("GovernanceWorkspacesService.Update: %w", err)
	}
	return &out, nil
}

func (s *GovernanceWorkspacesService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/workspaces/"+id, nil)
}

// ── ExternalUserPolicyService ─────────────────────────────────────────────────

// ExternalUserPolicyService manages policy for a single external user.
type ExternalUserPolicyService struct {
	client         *Client
	externalUserID string
}

func (s *ExternalUserPolicyService) Get(ctx context.Context) (PolicyDoc, error) {
	var out PolicyDoc
	if err := s.client.getJSON(ctx, fmt.Sprintf("/external-users/%s/policy", s.externalUserID), &out); err != nil {
		return nil, fmt.Errorf("ExternalUserPolicyService.Get: %w", err)
	}
	return unwrapPolicyDoc(out), nil
}

func (s *ExternalUserPolicyService) Set(ctx context.Context, policy PolicyDoc) (PolicyDoc, error) {
	var out PolicyDoc
	if err := s.client.putJSON(ctx, fmt.Sprintf("/external-users/%s/policy", s.externalUserID), policy, &out); err != nil {
		return nil, fmt.Errorf("ExternalUserPolicyService.Set: %w", err)
	}
	return unwrapPolicyDoc(out), nil
}

func (s *ExternalUserPolicyService) Delete(ctx context.Context) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/external-users/%s/policy", s.externalUserID), nil)
}

// Effective returns the fully-resolved policy for the external user.
func (s *ExternalUserPolicyService) Effective(ctx context.Context) (*EffectivePolicy, error) {
	var out EffectivePolicy
	if err := s.client.getJSON(ctx, fmt.Sprintf("/external-users/%s/effective-policy", s.externalUserID), &out); err != nil {
		return nil, fmt.Errorf("ExternalUserPolicyService.Effective: %w", err)
	}
	return &out, nil
}

// ── ExternalUsersGovernanceService ───────────────────────────────────────────

// ExternalUsersGovernanceService is the client.ExternalUsers gateway.
type ExternalUsersGovernanceService struct {
	client *Client
}

// User returns the policy service for a single external user.
func (s *ExternalUsersGovernanceService) User(externalUserID string) *ExternalUserPolicyService {
	return &ExternalUserPolicyService{client: s.client, externalUserID: externalUserID}
}

// ── BulkSandboxesService ──────────────────────────────────────────────────────

// BulkSandboxInput specifies either IDs or a filter for bulk sandbox ops.
type BulkSandboxInput struct {
	IDs    []string               `json:"ids,omitempty"`
	Filter map[string]interface{} `json:"filter,omitempty"`
}

// BulkSandboxesService handles bulk sandbox operations.
type BulkSandboxesService struct {
	client *Client
}

func (s *BulkSandboxesService) Pause(ctx context.Context, input BulkSandboxInput) (BulkJobResponse, error) {
	var out BulkJobResponse
	if err := s.client.postJSON(ctx, "/bulk/sandboxes/pause", input, &out); err != nil {
		return BulkJobResponse{}, fmt.Errorf("BulkSandboxesService.Pause: %w", err)
	}
	return out, nil
}

func (s *BulkSandboxesService) Resume(ctx context.Context, input BulkSandboxInput) (BulkJobResponse, error) {
	var out BulkJobResponse
	if err := s.client.postJSON(ctx, "/bulk/sandboxes/resume", input, &out); err != nil {
		return BulkJobResponse{}, fmt.Errorf("BulkSandboxesService.Resume: %w", err)
	}
	return out, nil
}

func (s *BulkSandboxesService) Destroy(ctx context.Context, input BulkSandboxInput) (BulkJobResponse, error) {
	var out BulkJobResponse
	if err := s.client.postJSON(ctx, "/bulk/sandboxes/destroy", input, &out); err != nil {
		return BulkJobResponse{}, fmt.Errorf("BulkSandboxesService.Destroy: %w", err)
	}
	return out, nil
}

// ── BulkPolicyService ─────────────────────────────────────────────────────────

// BulkPolicyApplyInput is the request body for POST /bulk/policy/apply.
type BulkPolicyApplyInput struct {
	Tier   string                 `json:"tier"`
	IDs    []string               `json:"ids,omitempty"`
	Filter map[string]interface{} `json:"filter,omitempty"`
	Policy PolicyDoc              `json:"policy"`
}

// BulkPolicyService applies policy in bulk.
type BulkPolicyService struct {
	client *Client
}

func (s *BulkPolicyService) Apply(ctx context.Context, input BulkPolicyApplyInput) (BulkJobResponse, error) {
	var out BulkJobResponse
	if err := s.client.postJSON(ctx, "/bulk/policy/apply", input, &out); err != nil {
		return BulkJobResponse{}, fmt.Errorf("BulkPolicyService.Apply: %w", err)
	}
	return out, nil
}

// ── BulkJobsService ───────────────────────────────────────────────────────────

// BulkJobsService retrieves async job status.
type BulkJobsService struct {
	client *Client
}

func (s *BulkJobsService) Get(ctx context.Context, jobID string) (BulkJobStatus, error) {
	var raw map[string]interface{}
	if err := s.client.getJSON(ctx, "/bulk/jobs/"+jobID, &raw); err != nil {
		return BulkJobStatus{}, fmt.Errorf("BulkJobsService.Get: %w", err)
	}
	// Unwrap data envelope if present
	if data, ok := raw["data"]; ok {
		if m, ok := data.(map[string]interface{}); ok {
			return mapToBulkJobStatus(m), nil
		}
	}
	return mapToBulkJobStatus(raw), nil
}

// ── BulkService ───────────────────────────────────────────────────────────────

// BulkService is the client.Bulk gateway.
type BulkService struct {
	Sandboxes *BulkSandboxesService
	Policy    *BulkPolicyService
	Jobs      *BulkJobsService
}

// ── BillingInvoicesService ────────────────────────────────────────────────────

// BillingInvoicesService provides invoice list and detail.
type BillingInvoicesService struct {
	client *Client
}

func (s *BillingInvoicesService) List(ctx context.Context, limit int, cursor string) ([]Invoice, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	path := "/billing/invoices"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var raw map[string]interface{}
	if err := s.client.getJSON(ctx, path, &raw); err != nil {
		return nil, fmt.Errorf("BillingInvoicesService.List: %w", err)
	}
	return extractInvoices(raw), nil
}

func (s *BillingInvoicesService) Get(ctx context.Context, invoiceID string) (Invoice, error) {
	var raw map[string]interface{}
	if err := s.client.getJSON(ctx, "/billing/invoices/"+invoiceID, &raw); err != nil {
		return Invoice{}, fmt.Errorf("BillingInvoicesService.Get: %w", err)
	}
	return mapToInvoice(unwrapData(raw)), nil
}

// ── BillingService ────────────────────────────────────────────────────────────

// BillingService is the client.Billing gateway.
type BillingService struct {
	client   *Client
	Invoices *BillingInvoicesService
}

// PaymentMethods lists saved payment methods.
func (s *BillingService) PaymentMethods(ctx context.Context) ([]PaymentMethod, error) {
	var raw map[string]interface{}
	if err := s.client.getJSON(ctx, "/billing/payment-methods", &raw); err != nil {
		return nil, fmt.Errorf("BillingService.PaymentMethods: %w", err)
	}
	return extractPaymentMethods(raw), nil
}

// Upcoming returns the next invoice preview.
func (s *BillingService) Upcoming(ctx context.Context) (map[string]interface{}, error) {
	var raw map[string]interface{}
	if err := s.client.getJSON(ctx, "/billing/upcoming", &raw); err != nil {
		return nil, fmt.Errorf("BillingService.Upcoming: %w", err)
	}
	return unwrapData(raw), nil
}

// ── ScopedApiKeyInput ─────────────────────────────────────────────────────────

// CreateScopedApiKeyInput is the request body for POST /api-keys/scoped.
type CreateScopedApiKeyInput struct {
	ExternalUserID string   `json:"external_user_id"`
	Scopes         []string `json:"scopes"`
	ExpiresAt      string   `json:"expires_at,omitempty"`
}

// ── ImpersonateInput ──────────────────────────────────────────────────────────

// ImpersonateInput is the request body for POST /admin/impersonate.
type ImpersonateInput struct {
	ExternalUserID string `json:"external_user_id"`
	TTLSec         int    `json:"ttl_sec"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func unwrapPolicyDoc(raw PolicyDoc) PolicyDoc {
	if data, ok := raw["data"]; ok {
		if m, ok := data.(map[string]interface{}); ok {
			return PolicyDoc(m)
		}
	}
	if policy, ok := raw["policy"]; ok {
		if m, ok := policy.(map[string]interface{}); ok {
			return PolicyDoc(m)
		}
	}
	return raw
}

func unwrapData(raw map[string]interface{}) map[string]interface{} {
	if data, ok := raw["data"]; ok {
		if m, ok := data.(map[string]interface{}); ok {
			return m
		}
	}
	return raw
}

func extractMembers(raw map[string]interface{}) []MemberRecord {
	var items []interface{}
	if data, ok := raw["data"]; ok {
		if arr, ok := data.([]interface{}); ok {
			items = arr
		}
	} else if members, ok := raw["members"]; ok {
		if arr, ok := members.([]interface{}); ok {
			items = arr
		}
	}
	out := make([]MemberRecord, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			rec := MemberRecord{}
			if id, ok := m["id"].(string); ok {
				rec.ID = id
			}
			if email, ok := m["email"].(string); ok {
				rec.Email = email
			}
			if role, ok := m["role"].(string); ok {
				rec.Role = role
			}
			out = append(out, rec)
		}
	}
	return out
}

func extractInvoices(raw map[string]interface{}) []Invoice {
	var items []interface{}
	if data, ok := raw["data"]; ok {
		if arr, ok := data.([]interface{}); ok {
			items = arr
		}
	}
	out := make([]Invoice, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, mapToInvoice(m))
		}
	}
	return out
}

func mapToInvoice(m map[string]interface{}) Invoice {
	inv := Invoice{}
	if id, ok := m["id"].(string); ok {
		inv.ID = id
	}
	if amt, ok := m["amount"].(float64); ok {
		inv.Amount = amt
	}
	return inv
}

func extractPaymentMethods(raw map[string]interface{}) []PaymentMethod {
	var items []interface{}
	if data, ok := raw["data"]; ok {
		if arr, ok := data.([]interface{}); ok {
			items = arr
		}
	}
	out := make([]PaymentMethod, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			pm := PaymentMethod{}
			if id, ok := m["id"].(string); ok {
				pm.ID = id
			}
			if brand, ok := m["brand"].(string); ok {
				pm.Brand = brand
			}
			if last4, ok := m["last4"].(string); ok {
				pm.Last4 = last4
			}
			out = append(out, pm)
		}
	}
	return out
}

func mapToBulkJobStatus(m map[string]interface{}) BulkJobStatus {
	s := BulkJobStatus{}
	if id, ok := m["id"].(string); ok {
		s.ID = id
	}
	if status, ok := m["status"].(string); ok {
		s.Status = status
	}
	if proc, ok := m["processed"].(float64); ok {
		s.Processed = int(proc)
	}
	return s
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}
