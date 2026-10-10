package miosa

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
)

// Governance policy: the same four-part document applies at three tiers.
//
// Resolution, highest priority first: external user, workspace, organization
// (tenant), platform defaults. A policy you write is deep-merged into the one
// already stored, so you only send the fields you change.

// PolicyQuotas caps what a tier may use.
type PolicyQuotas struct {
	MaxSandboxes   *int `json:"max_sandboxes,omitempty"`
	MaxConcurrent  *int `json:"max_concurrent,omitempty"`
	MaxStorageGB   *int `json:"max_storage_gb,omitempty"`
	MaxCreditCents *int `json:"max_credit_cents,omitempty"`
}

// PolicyPermissions grants or withholds capabilities.
type PolicyPermissions struct {
	CanCreateSandbox    *bool `json:"can_create_sandbox,omitempty"`
	CanUseGPU           *bool `json:"can_use_gpu,omitempty"`
	CanUseCustomDomains *bool `json:"can_use_custom_domains,omitempty"`
	CanUseForge         *bool `json:"can_use_forge,omitempty"`
}

// PolicyFeatures toggles product features.
type PolicyFeatures struct {
	SnapshotEnabled       *bool `json:"snapshot_enabled,omitempty"`
	PortForwardingEnabled *bool `json:"port_forwarding_enabled,omitempty"`
}

// PolicyBilling controls billing enforcement.
type PolicyBilling struct {
	EnforceCreditLimit *bool `json:"enforce_credit_limit,omitempty"`
}

// Policy is a governance policy document. Every field is optional: nil means
// "not set at this tier".
type Policy struct {
	Quotas      *PolicyQuotas      `json:"quotas,omitempty"`
	Permissions *PolicyPermissions `json:"permissions,omitempty"`
	Features    *PolicyFeatures    `json:"features,omitempty"`
	Billing     *PolicyBilling     `json:"billing,omitempty"`
}

// EffectivePolicyField is one resolved section of an effective policy with the
// tier that supplied it.
type EffectivePolicyField struct {
	Value json.RawMessage `json:"value"`
	// Source is "user", "workspace", "tenant" or "platform".
	Source string `json:"source"`
}

// EffectivePolicy is an external user's policy after merging every tier, keyed
// by section (quotas, permissions, features, billing).
type EffectivePolicy map[string]EffectivePolicyField

// Quotas decodes the quotas section.
func (p EffectivePolicy) Quotas() (PolicyQuotas, string, error) {
	var out PolicyQuotas
	f, ok := p["quotas"]
	if !ok {
		return out, "", nil
	}
	return out, f.Source, json.Unmarshal(f.Value, &out)
}

// Permissions decodes the permissions section.
func (p EffectivePolicy) Permissions() (PolicyPermissions, string, error) {
	var out PolicyPermissions
	f, ok := p["permissions"]
	if !ok {
		return out, "", nil
	}
	return out, f.Source, json.Unmarshal(f.Value, &out)
}

// decodePolicy reads a policy that may or may not be wrapped in "data".
func decodePolicy(raw json.RawMessage) (*Policy, error) {
	var probe struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &probe) == nil && len(probe.Data) > 0 {
		raw = probe.Data
	}
	var p Policy
	if len(raw) == 0 || string(raw) == "null" {
		return &p, nil
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// policyScope reads and writes one tier's policy.
type policyScope struct {
	client *Client
	path   string
}

func (s policyScope) get(ctx context.Context) (*Policy, error) {
	var raw json.RawMessage
	if err := s.client.getJSON(ctx, s.path, &raw); err != nil {
		return nil, err
	}
	return decodePolicy(raw)
}

func (s policyScope) put(ctx context.Context, p Policy) (*Policy, error) {
	var raw json.RawMessage
	if err := s.client.putJSON(ctx, s.path, p, &raw); err != nil {
		return nil, err
	}
	return decodePolicy(raw)
}

func (s policyScope) del(ctx context.Context) error {
	return s.client.deleteJSON(ctx, s.path, nil)
}

// PoliciesService manages governance policy at the organization, workspace and
// end-user tiers. Accessed via Client.Policies. Writing needs a role with the
// manage_policy permission (403 otherwise).
type PoliciesService struct {
	client *Client
}

// Tenant returns the organization-wide policy (GET /tenant/policy). It is empty
// when none is set.
func (s *PoliciesService) Tenant(ctx context.Context) (*Policy, error) {
	return policyScope{s.client, "/tenant/policy"}.get(ctx)
}

// SetTenant merges p into the organization policy (PUT /tenant/policy).
func (s *PoliciesService) SetTenant(ctx context.Context, p Policy) (*Policy, error) {
	return policyScope{s.client, "/tenant/policy"}.put(ctx, p)
}

// DeleteTenant removes the organization policy; the platform defaults apply.
func (s *PoliciesService) DeleteTenant(ctx context.Context) error {
	return policyScope{s.client, "/tenant/policy"}.del(ctx)
}

func workspacePolicyPath(id string) string { return "/workspaces/" + url.PathEscape(id) + "/policy" }

// Workspace returns a workspace's policy (GET /workspaces/:id/policy).
func (s *PoliciesService) Workspace(ctx context.Context, workspaceID string) (*Policy, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	return policyScope{s.client, workspacePolicyPath(workspaceID)}.get(ctx)
}

// SetWorkspace merges p into a workspace's policy.
func (s *PoliciesService) SetWorkspace(ctx context.Context, workspaceID string, p Policy) (*Policy, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	return policyScope{s.client, workspacePolicyPath(workspaceID)}.put(ctx, p)
}

// DeleteWorkspace removes a workspace's policy; its users fall back to the
// organization policy.
func (s *PoliciesService) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	if workspaceID == "" {
		return errors.New("workspaceID is required")
	}
	return policyScope{s.client, workspacePolicyPath(workspaceID)}.del(ctx)
}

func externalUserPolicyPath(id string, parts ...string) string {
	p := "/external-users/" + url.PathEscape(id)
	for _, part := range parts {
		p += "/" + part
	}
	return p
}

// ExternalUser returns one end user's own policy (GET /external-users/:id/policy).
func (s *PoliciesService) ExternalUser(ctx context.Context, externalUserID string) (*Policy, error) {
	if externalUserID == "" {
		return nil, errors.New("externalUserID is required")
	}
	return policyScope{s.client, externalUserPolicyPath(externalUserID, "policy")}.get(ctx)
}

// SetExternalUser merges p into one end user's policy.
func (s *PoliciesService) SetExternalUser(ctx context.Context, externalUserID string, p Policy) (*Policy, error) {
	if externalUserID == "" {
		return nil, errors.New("externalUserID is required")
	}
	return policyScope{s.client, externalUserPolicyPath(externalUserID, "policy")}.put(ctx, p)
}

// DeleteExternalUser removes one end user's policy.
func (s *PoliciesService) DeleteExternalUser(ctx context.Context, externalUserID string) error {
	if externalUserID == "" {
		return errors.New("externalUserID is required")
	}
	return policyScope{s.client, externalUserPolicyPath(externalUserID, "policy")}.del(ctx)
}

// EffectiveForExternalUser returns what applies to an end user after merging
// user, workspace, organization and platform tiers, each section tagged with
// its source. workspaceID is optional and brings the workspace tier into the merge.
func (s *PoliciesService) EffectiveForExternalUser(ctx context.Context, externalUserID, workspaceID string) (EffectivePolicy, error) {
	if externalUserID == "" {
		return nil, errors.New("externalUserID is required")
	}
	var out struct {
		Data EffectivePolicy `json:"data"`
	}
	path := externalUserPolicyPath(externalUserID, "effective-policy") + buildQuery(map[string]string{"workspace_id": workspaceID})
	if err := s.client.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}
