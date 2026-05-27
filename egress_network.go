package miosa

import (
	"context"
)

// ─── Egress network — policies, allowlist, suggestions ───────────────────────
//
// Backed by:
//
//	GET    /api/v1/egress/policies
//	POST   /api/v1/egress/policies
//	PATCH  /api/v1/egress/policies
//	PATCH  /api/v1/egress/policies/:id
//
//	GET    /api/v1/egress/allowlist
//	POST   /api/v1/egress/allowlist
//	DELETE /api/v1/egress/allowlist/:id
//
//	GET    /api/v1/egress/audit/suggestions?resource_id=...&since=...
//
// The egress firewall layers on top of the per-resource NetworkPolicy
// (which is the legacy nftables resource). The egress namespace adds:
//
//   - multi-rule **policies** that can be attached to resources,
//   - an **allowlist** (host + method + path-glob) the proxy enforces,
//   - a **mode** flag (audit_only vs enforce) so callers can run in observe
//     mode first and graduate to lockdown.
//   - AI-generated **suggestions** based on recent denied traffic.

// ─── Types ───────────────────────────────────────────────────────────────────

// EgressPolicy is the API representation of a multi-rule egress policy.
type EgressPolicy struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name,omitempty"`
	Mode          string                 `json:"mode,omitempty"`           // "audit_only" | "enforce"
	DefaultEffect string                 `json:"default_effect,omitempty"` // "allow" | "deny"
	ResourceID    string                 `json:"resource_id,omitempty"`
	ResourceType  string                 `json:"resource_type,omitempty"`
	Description   string                 `json:"description,omitempty"`
	CreatedAt     string                 `json:"created_at,omitempty"`
	UpdatedAt     string                 `json:"updated_at,omitempty"`
	Extra         map[string]interface{} `json:"-"`
}

// EgressAllowlistRule is one entry in the allowlist.
type EgressAllowlistRule struct {
	ID           string                 `json:"id"`
	Host         string                 `json:"host"`
	Effect       string                 `json:"effect"` // "allow" | "deny"
	Methods      []string               `json:"methods,omitempty"`
	PathGlob     string                 `json:"path_glob,omitempty"`
	PolicyID     string                 `json:"policy_id,omitempty"`
	ResourceID   string                 `json:"resource_id,omitempty"`
	ResourceType string                 `json:"resource_type,omitempty"`
	Note         string                 `json:"note,omitempty"`
	CreatedAt    string                 `json:"created_at,omitempty"`
	Extra        map[string]interface{} `json:"-"`
}

// EgressSuggestion is one AI-generated allowlist suggestion.
type EgressSuggestion struct {
	Host         string                 `json:"host,omitempty"`
	Methods      []string               `json:"methods,omitempty"`
	PathGlob     string                 `json:"path_glob,omitempty"`
	ResourceID   string                 `json:"resource_id,omitempty"`
	ResourceType string                 `json:"resource_type,omitempty"`
	Reason       string                 `json:"reason,omitempty"`
	DeniedCount  int                    `json:"denied_count,omitempty"`
	Extra        map[string]interface{} `json:"-"`
}

// AllowInput is the request body for Allow/Deny.
type AllowInput struct {
	Host         string   `json:"host"`
	Methods      []string `json:"methods,omitempty"`
	PathGlob     string   `json:"path_glob,omitempty"`
	PolicyID     string   `json:"policy_id,omitempty"`
	ResourceID   string   `json:"resource_id,omitempty"`
	ResourceType string   `json:"resource_type,omitempty"`
	Note         string   `json:"note,omitempty"`
	Effect       string   `json:"effect,omitempty"`
}

// PolicyCreateInput is the request body for CreatePolicy.
type PolicyCreateInput struct {
	Name          string `json:"name"`
	Mode          string `json:"mode,omitempty"`
	DefaultEffect string `json:"default_effect,omitempty"`
	ResourceID    string `json:"resource_id,omitempty"`
	ResourceType  string `json:"resource_type,omitempty"`
	Description   string `json:"description,omitempty"`
}

// PolicyUpdateInput is the request body for UpdatePolicy.
type PolicyUpdateInput struct {
	Mode          string `json:"mode,omitempty"`
	DefaultEffect string `json:"default_effect,omitempty"`
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
}

// RulesListInput is the filter for Rules().
type RulesListInput struct {
	PolicyID     string
	ResourceID   string
	ResourceType string
}

// SuggestionsInput is the filter for Suggestions().
type SuggestionsInput struct {
	ResourceID   string
	ResourceType string
	// Since is a duration string like "7d", "24h", or an ISO-8601 timestamp.
	// Defaults to "7d" when empty.
	Since string
}

// ─── Wire envelopes ──────────────────────────────────────────────────────────

type policyEnvelope struct {
	Data     *EgressPolicy  `json:"data,omitempty"`
	Policy   *EgressPolicy  `json:"policy,omitempty"`
	Policies []EgressPolicy `json:"policies,omitempty"`
	Items    []EgressPolicy `json:"items,omitempty"`
}

type ruleEnvelope struct {
	Data      *EgressAllowlistRule  `json:"data,omitempty"`
	Rule      *EgressAllowlistRule  `json:"rule,omitempty"`
	Rules     []EgressAllowlistRule `json:"rules,omitempty"`
	Allowlist []EgressAllowlistRule `json:"allowlist,omitempty"`
	Items     []EgressAllowlistRule `json:"items,omitempty"`
}

type suggestionsEnvelope struct {
	Data        []EgressSuggestion `json:"data,omitempty"`
	Suggestions []EgressSuggestion `json:"suggestions,omitempty"`
	Items       []EgressSuggestion `json:"items,omitempty"`
}

// ─── Service ─────────────────────────────────────────────────────────────────

// EgressNetworkService is the tenant-wide egress allowlist + policy surface.
// Accessed via Client.Network.
type EgressNetworkService struct {
	client *Client
}

// Allow adds an allow rule for host to the allowlist.
func (s *EgressNetworkService) Allow(ctx context.Context, host string, input AllowInput) (*EgressAllowlistRule, error) {
	input.Host = host
	input.Effect = "allow"
	var env ruleEnvelope
	if err := s.client.postJSON(ctx, "/egress/allowlist", input, &env); err != nil {
		return nil, err
	}
	return ruleFrom(env), nil
}

// Deny adds a deny rule for host to the allowlist.
func (s *EgressNetworkService) Deny(ctx context.Context, host string, input AllowInput) (*EgressAllowlistRule, error) {
	input.Host = host
	input.Effect = "deny"
	var env ruleEnvelope
	if err := s.client.postJSON(ctx, "/egress/allowlist", input, &env); err != nil {
		return nil, err
	}
	return ruleFrom(env), nil
}

// Rules lists allowlist rules with optional filters.
func (s *EgressNetworkService) Rules(ctx context.Context, input RulesListInput) ([]EgressAllowlistRule, error) {
	params := map[string]string{}
	if input.PolicyID != "" {
		params["policy_id"] = input.PolicyID
	}
	if input.ResourceID != "" {
		params["resource_id"] = input.ResourceID
	}
	if input.ResourceType != "" {
		params["resource_type"] = input.ResourceType
	}
	var env ruleEnvelope
	if err := s.client.getJSON(ctx, "/egress/allowlist"+buildQuery(params), &env); err != nil {
		return nil, err
	}
	return ruleListFrom(env), nil
}

// RemoveRule deletes an allowlist rule by ID.
func (s *EgressNetworkService) RemoveRule(ctx context.Context, ruleID string) error {
	return s.client.deleteJSON(ctx, "/egress/allowlist/"+ruleID, nil)
}

// Policies lists egress policies.
func (s *EgressNetworkService) Policies(ctx context.Context) ([]EgressPolicy, error) {
	var env policyEnvelope
	if err := s.client.getJSON(ctx, "/egress/policies", &env); err != nil {
		return nil, err
	}
	return policyListFrom(env), nil
}

// CreatePolicy creates a new policy.
func (s *EgressNetworkService) CreatePolicy(ctx context.Context, input PolicyCreateInput) (*EgressPolicy, error) {
	if input.Mode == "" {
		input.Mode = "enforce"
	}
	if input.DefaultEffect == "" {
		input.DefaultEffect = "deny"
	}
	var env policyEnvelope
	if err := s.client.postJSON(ctx, "/egress/policies", input, &env); err != nil {
		return nil, err
	}
	return policyFrom(env), nil
}

// UpdatePolicy updates an existing policy by id.
func (s *EgressNetworkService) UpdatePolicy(ctx context.Context, policyID string, input PolicyUpdateInput) (*EgressPolicy, error) {
	var env policyEnvelope
	if err := s.client.patchJSON(ctx, "/egress/policies/"+policyID, input, &env); err != nil {
		return nil, err
	}
	return policyFrom(env), nil
}

// LockdownInput selects which policy to flip into enforce mode.
type LockdownInput struct {
	PolicyID     string
	ResourceID   string
	ResourceType string
}

// Lockdown sets the policy to mode=enforce — denied requests are blocked.
//
// With no PolicyID and no ResourceID/ResourceType the tenant-default policy
// is updated via PATCH /egress/policies.
func (s *EgressNetworkService) Lockdown(ctx context.Context, input LockdownInput) (*EgressPolicy, error) {
	return s.setMode(ctx, "enforce", input)
}

// Observe sets the policy to mode=audit_only — denials are logged but not
// blocked.
func (s *EgressNetworkService) Observe(ctx context.Context, input LockdownInput) (*EgressPolicy, error) {
	return s.setMode(ctx, "audit_only", input)
}

func (s *EgressNetworkService) setMode(ctx context.Context, mode string, input LockdownInput) (*EgressPolicy, error) {
	if input.PolicyID == "" && (input.ResourceID == "" || input.ResourceType == "") {
		// Tenant-default policy
		body := map[string]interface{}{"mode": mode}
		var env policyEnvelope
		if err := s.client.patchJSON(ctx, "/egress/policies", body, &env); err != nil {
			return nil, err
		}
		return policyFrom(env), nil
	}
	if input.PolicyID != "" {
		return s.UpdatePolicy(ctx, input.PolicyID, PolicyUpdateInput{Mode: mode})
	}
	body := map[string]interface{}{
		"mode":          mode,
		"resource_id":   input.ResourceID,
		"resource_type": input.ResourceType,
	}
	var env policyEnvelope
	if err := s.client.patchJSON(ctx, "/egress/policies", body, &env); err != nil {
		return nil, err
	}
	return policyFrom(env), nil
}

// Suggestions returns AI-generated allowlist suggestions from recent
// denied egress.
func (s *EgressNetworkService) Suggestions(ctx context.Context, resourceID, since string) ([]EgressSuggestion, error) {
	return s.SuggestionsFiltered(ctx, SuggestionsInput{ResourceID: resourceID, Since: since})
}

// SuggestionsFiltered is the form-style variant of Suggestions.
func (s *EgressNetworkService) SuggestionsFiltered(ctx context.Context, input SuggestionsInput) ([]EgressSuggestion, error) {
	params := map[string]string{}
	if input.ResourceID != "" {
		params["resource_id"] = input.ResourceID
	}
	if input.ResourceType != "" {
		params["resource_type"] = input.ResourceType
	}
	since := input.Since
	if since == "" {
		since = "7d"
	}
	params["since"] = since
	var env suggestionsEnvelope
	if err := s.client.getJSON(ctx, "/egress/audit/suggestions"+buildQuery(params), &env); err != nil {
		return nil, err
	}
	switch {
	case len(env.Suggestions) > 0:
		return env.Suggestions, nil
	case len(env.Items) > 0:
		return env.Items, nil
	case len(env.Data) > 0:
		return env.Data, nil
	}
	return []EgressSuggestion{}, nil
}

// ─── Resource-scoped wrapper ─────────────────────────────────────────────────

// SandboxNetworkBinding pre-scopes resource_id + resource_type on every call.
type SandboxNetworkBinding struct {
	client       *Client
	resourceID   string
	resourceType string
	delegate     *EgressNetworkService
}

func newSandboxNetworkBinding(c *Client, resourceID, resourceType string) *SandboxNetworkBinding {
	return &SandboxNetworkBinding{
		client:       c,
		resourceID:   resourceID,
		resourceType: resourceType,
		delegate:     &EgressNetworkService{client: c},
	}
}

// Allow adds an allow rule pre-scoped to this resource.
func (b *SandboxNetworkBinding) Allow(ctx context.Context, host string, input AllowInput) (*EgressAllowlistRule, error) {
	if input.ResourceID == "" {
		input.ResourceID = b.resourceID
	}
	if input.ResourceType == "" {
		input.ResourceType = b.resourceType
	}
	return b.delegate.Allow(ctx, host, input)
}

// Deny adds a deny rule pre-scoped to this resource.
func (b *SandboxNetworkBinding) Deny(ctx context.Context, host string, input AllowInput) (*EgressAllowlistRule, error) {
	if input.ResourceID == "" {
		input.ResourceID = b.resourceID
	}
	if input.ResourceType == "" {
		input.ResourceType = b.resourceType
	}
	return b.delegate.Deny(ctx, host, input)
}

// Rules returns rules pre-filtered to this resource.
func (b *SandboxNetworkBinding) Rules(ctx context.Context, input RulesListInput) ([]EgressAllowlistRule, error) {
	if input.ResourceID == "" {
		input.ResourceID = b.resourceID
	}
	if input.ResourceType == "" {
		input.ResourceType = b.resourceType
	}
	return b.delegate.Rules(ctx, input)
}

// RemoveRule deletes an allowlist rule by ID.
func (b *SandboxNetworkBinding) RemoveRule(ctx context.Context, ruleID string) error {
	return b.delegate.RemoveRule(ctx, ruleID)
}

// Lockdown sets this resource's policy to mode=enforce.
func (b *SandboxNetworkBinding) Lockdown(ctx context.Context, policyID string) (*EgressPolicy, error) {
	return b.delegate.Lockdown(ctx, LockdownInput{
		PolicyID:     policyID,
		ResourceID:   b.resourceID,
		ResourceType: b.resourceType,
	})
}

// Observe sets this resource's policy to mode=audit_only.
func (b *SandboxNetworkBinding) Observe(ctx context.Context, policyID string) (*EgressPolicy, error) {
	return b.delegate.Observe(ctx, LockdownInput{
		PolicyID:     policyID,
		ResourceID:   b.resourceID,
		ResourceType: b.resourceType,
	})
}

// Suggestions returns AI-generated allowlist suggestions for this resource.
func (b *SandboxNetworkBinding) Suggestions(ctx context.Context, since string) ([]EgressSuggestion, error) {
	if since == "" {
		since = "7d"
	}
	return b.delegate.SuggestionsFiltered(ctx, SuggestionsInput{
		ResourceID:   b.resourceID,
		ResourceType: b.resourceType,
		Since:        since,
	})
}

// Policies lists egress policies (filtered to this resource if the backend
// supports it; otherwise returns the tenant-wide list).
func (b *SandboxNetworkBinding) Policies(ctx context.Context) ([]EgressPolicy, error) {
	return b.delegate.Policies(ctx)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func policyFrom(env policyEnvelope) *EgressPolicy {
	switch {
	case env.Data != nil:
		return env.Data
	case env.Policy != nil:
		return env.Policy
	}
	return &EgressPolicy{}
}

func policyListFrom(env policyEnvelope) []EgressPolicy {
	switch {
	case len(env.Policies) > 0:
		return env.Policies
	case len(env.Items) > 0:
		return env.Items
	}
	return []EgressPolicy{}
}

func ruleFrom(env ruleEnvelope) *EgressAllowlistRule {
	switch {
	case env.Data != nil:
		return env.Data
	case env.Rule != nil:
		return env.Rule
	}
	return &EgressAllowlistRule{}
}

func ruleListFrom(env ruleEnvelope) []EgressAllowlistRule {
	switch {
	case len(env.Rules) > 0:
		return env.Rules
	case len(env.Allowlist) > 0:
		return env.Allowlist
	case len(env.Items) > 0:
		return env.Items
	}
	return []EgressAllowlistRule{}
}
