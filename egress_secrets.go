package miosa

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// ─── Egress secrets — encrypted API-key + OAuth credential vault ─────────────
//
// Backed by:
//
//	POST   /api/v1/egress/secrets
//	GET    /api/v1/egress/secrets
//	GET    /api/v1/egress/secrets/:id
//	PATCH  /api/v1/egress/secrets/:id     (rotate)
//	DELETE /api/v1/egress/secrets/:id
//
//	POST   /api/v1/egress/bindings
//	GET    /api/v1/egress/bindings
//	DELETE /api/v1/egress/bindings/:id
//
//	GET    /api/v1/egress/oauth/providers
//	POST   /api/v1/egress/oauth/start
//	GET    /api/v1/egress/oauth/status?state=...
//
// The three access surfaces mirror the Python/TypeScript spec:
//
//   - EgressSecretsService — tenant-wide CRUD + OAuth connect.
//   - SandboxSecretsBinding — sandbox-scoped (pre-scopes resource_id +
//     resource_type="sandbox" on every call).
//   - ComputerSecretsBinding — same shape, resource_type="computer".

// ─── Types ───────────────────────────────────────────────────────────────────

// EgressSecretData is the API representation of a stored secret. Values are
// never returned in plaintext after creation; only MaskedValue is visible.
type EgressSecretData struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name,omitempty"`
	Type                string                 `json:"type,omitempty"`
	Scope               string                 `json:"scope,omitempty"`
	WorkspaceID         string                 `json:"workspace_id,omitempty"`
	OwnerUserID         string                 `json:"owner_user_id,omitempty"`
	ExternalUserID      string                 `json:"external_user_id,omitempty"`
	ExternalWorkspaceID string                 `json:"external_workspace_id,omitempty"`
	ResourceID          string                 `json:"resource_id,omitempty"`
	ResourceType        string                 `json:"resource_type,omitempty"`
	MaskedValue         string                 `json:"masked_value,omitempty"`
	ExpiresAt           string                 `json:"expires_at,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt           string                 `json:"created_at,omitempty"`
	UpdatedAt           string                 `json:"updated_at,omitempty"`
}

// EgressBindingData is a single secret-to-resource binding (env-var injection).
type EgressBindingData struct {
	ID           string `json:"id"`
	SecretID     string `json:"secret_id"`
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	ExposeAsEnv  string `json:"expose_as_env"`
	CreatedAt    string `json:"created_at,omitempty"`
}

// OauthProvider is one entry in the discovery list.
type OauthProvider struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name,omitempty"`
	Scopes      []string               `json:"scopes,omitempty"`
	Extra       map[string]interface{} `json:"-"`
}

// SecretSetInput is the request body for Set.
type SecretSetInput struct {
	Name string `json:"name"`
	// Value is the plaintext secret. It is never returned by the API.
	Value               string                 `json:"value"`
	Type                string                 `json:"type,omitempty"`
	Scope               string                 `json:"scope,omitempty"`
	ExposeAsEnv         string                 `json:"expose_as_env,omitempty"`
	WorkspaceID         string                 `json:"workspace_id,omitempty"`
	OwnerUserID         string                 `json:"owner_user_id,omitempty"`
	ExternalUserID      string                 `json:"external_user_id,omitempty"`
	ExternalWorkspaceID string                 `json:"external_workspace_id,omitempty"`
	ResourceID          string                 `json:"resource_id,omitempty"`
	ResourceType        string                 `json:"resource_type,omitempty"`
	RefreshToken        string                 `json:"refresh_token,omitempty"`
	ExpiresAt           string                 `json:"expires_at,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// SecretListInput is the optional filter for List.
type SecretListInput struct {
	Scope               string
	Type                string
	WorkspaceID         string
	OwnerUserID         string
	ExternalUserID      string
	ExternalWorkspaceID string
	ResourceID          string
	ResourceType        string
}

// SecretRotateInput is the request body for Rotate.
type SecretRotateInput struct {
	NewValue     string `json:"value"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// BindingCreateInput is the request body for CreateBinding.
type BindingCreateInput struct {
	SecretID     string `json:"secret_id"`
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	ExposeAsEnv  string `json:"expose_as_env"`
}

// BindingListInput is the optional filter for ListBindings.
type BindingListInput struct {
	ResourceID   string
	ResourceType string
	SecretID     string
}

// OauthConnectInput is the request body for Connect.
type OauthConnectInput struct {
	Provider            string `json:"provider"`
	ExposeAsEnv         string `json:"expose_as_env,omitempty"`
	Scope               string `json:"scope,omitempty"`
	OwnerUserID         string `json:"owner_user_id,omitempty"`
	ExternalUserID      string `json:"external_user_id,omitempty"`
	ExternalWorkspaceID string `json:"external_workspace_id,omitempty"`
	ResourceID          string `json:"resource_id,omitempty"`
	ResourceType        string `json:"resource_type,omitempty"`
	RedirectURI         string `json:"redirect_uri,omitempty"`
}

// OauthStartResult is the payload returned by POST /egress/oauth/start.
type OauthStartResult struct {
	AuthorizeURL string                 `json:"authorize_url"`
	State        string                 `json:"state"`
	Provider     string                 `json:"provider,omitempty"`
	ExpiresAt    string                 `json:"expires_at,omitempty"`
	Raw          map[string]interface{} `json:"-"`
}

// OauthStatusResult is the payload returned by GET /egress/oauth/status.
type OauthStatusResult struct {
	Status   string                 `json:"status"`
	State    string                 `json:"state,omitempty"`
	SecretID string                 `json:"secret_id,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Message  string                 `json:"message,omitempty"`
	Raw      map[string]interface{} `json:"-"`
}

// ─── Wire envelopes ──────────────────────────────────────────────────────────

type secretEnvelope struct {
	Data    *EgressSecretData  `json:"data,omitempty"`
	Secret  *EgressSecretData  `json:"secret,omitempty"`
	Secrets []EgressSecretData `json:"secrets,omitempty"`
	Items   []EgressSecretData `json:"items,omitempty"`
	List    []EgressSecretData `json:"-"`
}

type bindingEnvelope struct {
	Data     *EgressBindingData  `json:"data,omitempty"`
	Binding  *EgressBindingData  `json:"binding,omitempty"`
	Bindings []EgressBindingData `json:"bindings,omitempty"`
	Items    []EgressBindingData `json:"items,omitempty"`
}

type providerEnvelope struct {
	Data      []OauthProvider `json:"data,omitempty"`
	Providers []OauthProvider `json:"providers,omitempty"`
	Items     []OauthProvider `json:"items,omitempty"`
}

type oauthStartEnvelope struct {
	Data         *OauthStartResult `json:"data,omitempty"`
	AuthorizeURL string            `json:"authorize_url,omitempty"`
	State        string            `json:"state,omitempty"`
	Provider     string            `json:"provider,omitempty"`
	ExpiresAt    string            `json:"expires_at,omitempty"`
}

type oauthStatusEnvelope struct {
	Data     *OauthStatusResult `json:"data,omitempty"`
	Status   string             `json:"status,omitempty"`
	State    string             `json:"state,omitempty"`
	SecretID string             `json:"secret_id,omitempty"`
	Error    string             `json:"error,omitempty"`
	Message  string             `json:"message,omitempty"`
}

// ─── Service ─────────────────────────────────────────────────────────────────

// EgressSecretsService is the tenant-wide secret + OAuth credential surface.
// Accessed via Client.Secrets.
type EgressSecretsService struct {
	client *Client
}

// Set creates a secret. When ExposeAsEnv and ResourceID are both provided the
// backend additionally creates a binding so the value is injected as an
// environment variable on that resource.
func (s *EgressSecretsService) Set(ctx context.Context, input SecretSetInput) (*EgressSecretData, error) {
	if input.Type == "" {
		input.Type = "api_key"
	}
	if input.Scope == "" {
		input.Scope = "user"
	}
	var env secretEnvelope
	if err := s.client.postJSON(ctx, "/egress/secrets", input, &env); err != nil {
		return nil, err
	}
	return secretFrom(env), nil
}

// Get fetches a single secret by ID.
func (s *EgressSecretsService) Get(ctx context.Context, id string) (*EgressSecretData, error) {
	var env secretEnvelope
	if err := s.client.getJSON(ctx, "/egress/secrets/"+id, &env); err != nil {
		return nil, err
	}
	return secretFrom(env), nil
}

// List returns secrets matching the optional filter.
func (s *EgressSecretsService) List(ctx context.Context, input SecretListInput) ([]EgressSecretData, error) {
	q := buildSecretListQuery(input)
	var env secretEnvelope
	if err := s.client.getJSON(ctx, "/egress/secrets"+q, &env); err != nil {
		return nil, err
	}
	return secretListFrom(env), nil
}

// Rotate updates the secret's value (PATCH /egress/secrets/:id).
func (s *EgressSecretsService) Rotate(ctx context.Context, id string, input SecretRotateInput) (*EgressSecretData, error) {
	var env secretEnvelope
	if err := s.client.patchJSON(ctx, "/egress/secrets/"+id, input, &env); err != nil {
		return nil, err
	}
	return secretFrom(env), nil
}

// Delete permanently removes a secret.
func (s *EgressSecretsService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/egress/secrets/"+id, nil)
}

// CreateBinding binds an existing secret to a resource as an env var.
func (s *EgressSecretsService) CreateBinding(ctx context.Context, input BindingCreateInput) (*EgressBindingData, error) {
	var env bindingEnvelope
	if err := s.client.postJSON(ctx, "/egress/bindings", input, &env); err != nil {
		return nil, err
	}
	return bindingFrom(env), nil
}

// ListBindings returns secret bindings matching the optional filter.
func (s *EgressSecretsService) ListBindings(ctx context.Context, input BindingListInput) ([]EgressBindingData, error) {
	params := map[string]string{}
	if input.ResourceID != "" {
		params["resource_id"] = input.ResourceID
	}
	if input.ResourceType != "" {
		params["resource_type"] = input.ResourceType
	}
	if input.SecretID != "" {
		params["secret_id"] = input.SecretID
	}
	var env bindingEnvelope
	if err := s.client.getJSON(ctx, "/egress/bindings"+buildQuery(params), &env); err != nil {
		return nil, err
	}
	return bindingListFrom(env), nil
}

// DeleteBinding removes a binding by ID.
func (s *EgressSecretsService) DeleteBinding(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/egress/bindings/"+id, nil)
}

// Providers lists OAuth providers visible to the current tenant.
func (s *EgressSecretsService) Providers(ctx context.Context) ([]OauthProvider, error) {
	var env providerEnvelope
	if err := s.client.getJSON(ctx, "/egress/oauth/providers", &env); err != nil {
		return nil, err
	}
	for _, lst := range [][]OauthProvider{env.Data, env.Providers, env.Items} {
		if len(lst) > 0 {
			return lst, nil
		}
	}
	return []OauthProvider{}, nil
}

// Connect starts an OAuth Connect flow.
//
// The SDK does NOT open a browser — the caller is responsible for surfacing
// flow.AuthorizeURL to the end user. Once the user has granted consent,
// call flow.WaitForCompletion() (or poll Status() directly) to receive the
// resulting secret id.
func (s *EgressSecretsService) Connect(ctx context.Context, input OauthConnectInput) (*OAuthFlow, error) {
	var env oauthStartEnvelope
	if err := s.client.postJSON(ctx, "/egress/oauth/start", input, &env); err != nil {
		return nil, err
	}
	result := oauthStartFrom(env)
	if result.Provider == "" {
		result.Provider = input.Provider
	}
	return &OAuthFlow{
		AuthorizeURL: result.AuthorizeURL,
		State:        result.State,
		Provider:     result.Provider,
		Data:         result,
		client:       s.client,
	}, nil
}

// ─── OAuth flow handle ───────────────────────────────────────────────────────

// OAuthFlow is a pending OAuth Connect flow.
type OAuthFlow struct {
	AuthorizeURL string
	State        string
	Provider     string
	Data         *OauthStartResult
	client       *Client
}

// WaitForCompletionOptions tunes WaitForCompletion polling.
type WaitForCompletionOptions struct {
	// Timeout bounds the total wait. Defaults to 5 minutes.
	Timeout time.Duration
	// PollInterval is the delay between status polls. Defaults to 2 seconds.
	PollInterval time.Duration
}

func (o WaitForCompletionOptions) timeout() time.Duration {
	if o.Timeout <= 0 {
		return 5 * time.Minute
	}
	return o.Timeout
}

func (o WaitForCompletionOptions) pollInterval() time.Duration {
	if o.PollInterval <= 0 {
		return 2 * time.Second
	}
	return o.PollInterval
}

// Status polls GET /egress/oauth/status?state=... once and returns the
// current state. Useful for callers that want to drive their own polling
// loop. WaitForCompletion is the convenience wrapper.
func (f *OAuthFlow) Status(ctx context.Context) (*OauthStatusResult, error) {
	var env oauthStatusEnvelope
	if err := f.client.getJSON(ctx, "/egress/oauth/status?state="+url.QueryEscape(f.State), &env); err != nil {
		return nil, err
	}
	return oauthStatusFrom(env), nil
}

// WaitForCompletion polls /egress/oauth/status until the flow completes,
// times out, or fails.
//
// Returns the final status payload on completion. Returns an error if the
// flow ends in failed/error/denied or the wait exceeds opts.Timeout.
func (f *OAuthFlow) WaitForCompletion(ctx context.Context, opts WaitForCompletionOptions) (*OauthStatusResult, error) {
	deadline := time.Now().Add(opts.timeout())
	ticker := time.NewTicker(opts.pollInterval())
	defer ticker.Stop()

	for {
		status, err := f.Status(ctx)
		if err != nil {
			return nil, err
		}
		switch status.Status {
		case "completed", "ready", "succeeded":
			return status, nil
		case "failed", "error", "denied":
			detail := status.Error
			if detail == "" {
				detail = status.Message
			}
			if detail == "" {
				detail = "no detail"
			}
			return status, fmt.Errorf("oauth flow %s ended in status=%s: %s", f.State, status.Status, detail)
		}
		if time.Now().After(deadline) {
			return status, fmt.Errorf("oauth flow %s did not complete within %s", f.State, opts.timeout())
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// ─── Resource-scoped wrappers ────────────────────────────────────────────────

// SandboxSecretsBinding pre-scopes resource_id + resource_type="sandbox" on
// every call. Accessed via SandboxHandle.Secrets and Sandbox / Computer
// sub-services.
type SandboxSecretsBinding struct {
	client       *Client
	resourceID   string
	resourceType string
	delegate     *EgressSecretsService
}

func newSandboxSecretsBinding(c *Client, resourceID, resourceType string) *SandboxSecretsBinding {
	return &SandboxSecretsBinding{
		client:       c,
		resourceID:   resourceID,
		resourceType: resourceType,
		delegate:     &EgressSecretsService{client: c},
	}
}

// Set creates a sandbox/computer-scoped secret.
func (b *SandboxSecretsBinding) Set(ctx context.Context, input SecretSetInput) (*EgressSecretData, error) {
	if input.ResourceID == "" {
		input.ResourceID = b.resourceID
	}
	if input.ResourceType == "" {
		input.ResourceType = b.resourceType
	}
	return b.delegate.Set(ctx, input)
}

// Get fetches a single secret by ID.
func (b *SandboxSecretsBinding) Get(ctx context.Context, id string) (*EgressSecretData, error) {
	return b.delegate.Get(ctx, id)
}

// List returns secrets pre-filtered to this resource.
func (b *SandboxSecretsBinding) List(ctx context.Context, input SecretListInput) ([]EgressSecretData, error) {
	if input.ResourceID == "" {
		input.ResourceID = b.resourceID
	}
	if input.ResourceType == "" {
		input.ResourceType = b.resourceType
	}
	return b.delegate.List(ctx, input)
}

// Rotate rotates a secret's value.
func (b *SandboxSecretsBinding) Rotate(ctx context.Context, id string, input SecretRotateInput) (*EgressSecretData, error) {
	return b.delegate.Rotate(ctx, id, input)
}

// Delete permanently removes a secret.
func (b *SandboxSecretsBinding) Delete(ctx context.Context, id string) error {
	return b.delegate.Delete(ctx, id)
}

// Connect starts an OAuth Connect flow pre-scoped to this resource.
func (b *SandboxSecretsBinding) Connect(ctx context.Context, input OauthConnectInput) (*OAuthFlow, error) {
	if input.ResourceID == "" {
		input.ResourceID = b.resourceID
	}
	if input.ResourceType == "" {
		input.ResourceType = b.resourceType
	}
	return b.delegate.Connect(ctx, input)
}

// ListBindings lists bindings pre-filtered to this resource.
func (b *SandboxSecretsBinding) ListBindings(ctx context.Context, input BindingListInput) ([]EgressBindingData, error) {
	if input.ResourceID == "" {
		input.ResourceID = b.resourceID
	}
	if input.ResourceType == "" {
		input.ResourceType = b.resourceType
	}
	return b.delegate.ListBindings(ctx, input)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func secretFrom(env secretEnvelope) *EgressSecretData {
	switch {
	case env.Data != nil:
		return env.Data
	case env.Secret != nil:
		return env.Secret
	}
	return &EgressSecretData{}
}

func secretListFrom(env secretEnvelope) []EgressSecretData {
	switch {
	case len(env.Secrets) > 0:
		return env.Secrets
	case len(env.Items) > 0:
		return env.Items
	case env.Data != nil:
		return []EgressSecretData{*env.Data}
	}
	return []EgressSecretData{}
}

func bindingFrom(env bindingEnvelope) *EgressBindingData {
	switch {
	case env.Data != nil:
		return env.Data
	case env.Binding != nil:
		return env.Binding
	}
	return &EgressBindingData{}
}

func bindingListFrom(env bindingEnvelope) []EgressBindingData {
	switch {
	case len(env.Bindings) > 0:
		return env.Bindings
	case len(env.Items) > 0:
		return env.Items
	}
	return []EgressBindingData{}
}

func oauthStartFrom(env oauthStartEnvelope) *OauthStartResult {
	if env.Data != nil {
		return env.Data
	}
	return &OauthStartResult{
		AuthorizeURL: env.AuthorizeURL,
		State:        env.State,
		Provider:     env.Provider,
		ExpiresAt:    env.ExpiresAt,
	}
}

func oauthStatusFrom(env oauthStatusEnvelope) *OauthStatusResult {
	if env.Data != nil {
		return env.Data
	}
	return &OauthStatusResult{
		Status:   env.Status,
		State:    env.State,
		SecretID: env.SecretID,
		Error:    env.Error,
		Message:  env.Message,
	}
}

func buildSecretListQuery(in SecretListInput) string {
	params := map[string]string{}
	if in.Scope != "" {
		params["scope"] = in.Scope
	}
	if in.Type != "" {
		params["type"] = in.Type
	}
	if in.WorkspaceID != "" {
		params["workspace_id"] = in.WorkspaceID
	}
	if in.OwnerUserID != "" {
		params["owner_user_id"] = in.OwnerUserID
	}
	if in.ExternalUserID != "" {
		params["external_user_id"] = in.ExternalUserID
	}
	if in.ExternalWorkspaceID != "" {
		params["external_workspace_id"] = in.ExternalWorkspaceID
	}
	if in.ResourceID != "" {
		params["resource_id"] = in.ResourceID
	}
	if in.ResourceType != "" {
		params["resource_type"] = in.ResourceType
	}
	return buildQuery(params)
}
