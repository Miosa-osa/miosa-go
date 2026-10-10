package miosa

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
)

// Wire contract: miosa-compute docs/api/agents.md (the Agents tab),
// docs/api/connections.md and docs/api/harness-sessions.md.
//
// Credentials and settings are held at the organization level and a workspace
// may override them. Pass a workspace id to read a workspace's effective view
// or to write its override; leave it empty for the organization level.
// Every response is {"data": ...}; secrets are never returned, only previews.

// AgentCredentialField describes one form field of a credential.
type AgentCredentialField struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Input       string   `json:"input"` // password, text or select
	Secret      bool     `json:"secret"`
	Required    bool     `json:"required"`
	Help        string   `json:"help,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []string `json:"options,omitempty"`
}

// AgentCredentialMode is one way to connect a credential (Bedrock has two).
type AgentCredentialMode struct {
	ID       string                 `json:"id"`
	Label    string                 `json:"label"`
	Fields   []AgentCredentialField `json:"fields"`
	Required []string               `json:"required"`
}

// AgentCredential is a model-provider credential and its connection state.
// ID is claude_subscription, chatgpt_subscription, kimi_subscription,
// mistral_subscription, anthropic, openai, openrouter, deepseek, llmgateway,
// moonshot, mistral or bedrock.
type AgentCredential struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Vendor    string `json:"vendor"`
	Kind      string `json:"kind"` // subscription, api_key or cloud
	Connected bool   `json:"connected"`
	// Status is connected, needs_reauth, expired or error; empty when not
	// connected. Only "connected" counts as usable.
	Status string `json:"status,omitempty"`
	// Preview is a redacted form of the secret.
	Preview      string                 `json:"preview,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	AccountEmail string                 `json:"account_email,omitempty"`
	AccountPlan  string                 `json:"account_plan,omitempty"`
	// Source is agent_credential, agent_account or organization_key.
	Source string `json:"source,omitempty"`
	// Scope is "workspace" or "organization"; Inherited is true when the
	// workspace view shows the organization's credential.
	Scope     string   `json:"scope,omitempty"`
	Inherited bool     `json:"inherited"`
	UsableBy  []string `json:"usable_by,omitempty"`
	Shared    bool     `json:"shared"`
	UpdatedAt string   `json:"updated_at,omitempty"`

	Fields []AgentCredentialField `json:"fields,omitempty"`
	Modes  []AgentCredentialMode  `json:"modes,omitempty"`
	// Signin is the provider to send to AgentAccounts.StartSignin for a
	// subscription that has a sign-in (claude_code, codex, kimi_code,
	// mistral_vibe), else "".
	Signin string `json:"signin,omitempty"`
	// Paste is true when the subscription can also be connected by pasting
	// its token or key.
	Paste     bool     `json:"paste,omitempty"`
	Harnesses []string `json:"harnesses,omitempty"`
}

// PutAgentCredentialInput connects or replaces a credential. Fields holds the
// credential's form fields (for example {"api_key": "sk-..."}; Bedrock takes
// region plus api_key, or access_key_id and secret_access_key). The Claude and
// ChatGPT subscriptions are sign-in only (422 USE_SIGNIN).
type PutAgentCredentialInput struct {
	Fields map[string]string
	// WorkspaceID writes the workspace's own override.
	WorkspaceID string
	// UsableBy is a list of "agents" and "gateway". A key of a gateway provider
	// saved at organization level is also stored as the AI Gateway key unless
	// this is ["agents"]; a subscription is never usable by the gateway.
	UsableBy []string
}

// MarshalJSON implements json.Marshaler.
func (in PutAgentCredentialInput) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{}
	for k, v := range in.Fields {
		m[k] = v
	}
	if in.WorkspaceID != "" {
		m["workspace_id"] = in.WorkspaceID
	}
	if in.UsableBy != nil {
		m["usable_by"] = in.UsableBy
	}
	return json.Marshal(m)
}

// AgentHarnessStatus is a harness's readiness.
type AgentHarnessStatus struct {
	// State is "ready", "no_credential" or "not_applicable".
	State   string `json:"state"`
	Text    string `json:"text,omitempty"`
	Enabled int    `json:"enabled"`
	Total   int    `json:"total"`
	Hint    string `json:"hint,omitempty"`
}

// AgentAuthMethod is one way a single-method harness can authenticate.
type AgentAuthMethod struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	Connected bool   `json:"connected"`
	Active    bool   `json:"active"`
	Signin    bool   `json:"signin"`
	Scope     string `json:"scope,omitempty"`
}

// AgentProvider is one provider of a multi-provider harness.
type AgentProvider struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	Connected bool   `json:"connected"`
	Enabled   bool   `json:"enabled"`
	Signin    bool   `json:"signin"`
	Scope     string `json:"scope,omitempty"`
}

// AgentModelDefault is the dropdown's "Default (X)" entry.
type AgentModelDefault struct {
	Label  string `json:"label"`
	Model  string `json:"model"`
	Effort string `json:"effort,omitempty"`
}

// AgentHarnessEffective is what a run would use right now.
type AgentHarnessEffective struct {
	Model         string             `json:"model"`
	ModelName     string             `json:"model_name"`
	Effort        string             `json:"effort,omitempty"`
	EffortSource  string             `json:"effort_source,omitempty"`
	Label         string             `json:"label"`
	DefaultOption *AgentModelDefault `json:"default_option,omitempty"`
}

// AgentHarness is one harness in the Agents tab settings. ID is claude-code,
// codex, pi, opencode, prime, kimi, mistral, osa or custom; it is the run's
// runner.
type AgentHarness struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Provider  string `json:"provider"`
	Mode      string `json:"mode"` // single, multi, managed or gateway
	IsDefault bool   `json:"is_default"`
	// Prompt is false for a harness that cannot answer a prompt (custom).
	Prompt *bool              `json:"prompt,omitempty"`
	Status AgentHarnessStatus `json:"status"`

	// Single-method harnesses.
	AuthMethod         string            `json:"auth_method,omitempty"`
	ActiveMethod       string            `json:"active_method,omitempty"`
	ActiveMethodSource string            `json:"active_method_source,omitempty"`
	AuthMethods        []AgentAuthMethod `json:"auth_methods,omitempty"`

	// Multi-provider harnesses. EnabledProviders nil means every connected
	// provider is enabled.
	EnabledProviders []string        `json:"enabled_providers"`
	Providers        []AgentProvider `json:"providers,omitempty"`
	EnabledCount     int             `json:"enabled_count"`
	ProviderCount    int             `json:"provider_count"`

	DefaultModel  string                 `json:"default_model,omitempty"`
	DefaultEffort string                 `json:"default_effort,omitempty"`
	Inherited     []string               `json:"inherited,omitempty"`
	Effective     *AgentHarnessEffective `json:"effective,omitempty"`
}

// AgentSettingsScope says which level a settings view is for.
type AgentSettingsScope struct {
	Level       string `json:"level"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// AgentSettings is the Agents tab: credentials plus per-harness settings.
type AgentSettings struct {
	Scope AgentSettingsScope `json:"scope"`
	// DefaultHarness answers a prompt (POST /runs with an instruction and no runner).
	DefaultHarness      string            `json:"default_harness"`
	DefaultHarnessScope string            `json:"default_harness_scope"` // platform, organization or workspace
	Credentials         []AgentCredential `json:"credentials"`
	Harnesses           []AgentHarness    `json:"harnesses"`
}

// UpdateAgentHarnessInput changes one harness's settings. Nil members are left
// alone; the Clear flags send null, which clears the value at this scope so it
// is inherited again.
type UpdateAgentHarnessInput struct {
	// AuthMethod (single-method harnesses) must be connected; it replaces the
	// active one.
	AuthMethod *string
	// EnabledProviders (multi-provider harnesses) narrows the enabled list.
	EnabledProviders []string
	// EnableAllProviders sends null: every connected provider is enabled.
	EnableAllProviders bool
	DefaultModel       *string
	ClearDefaultModel  bool
	// DefaultEffort is one of max xhigh high medium low minimal none that the
	// model takes.
	DefaultEffort      *string
	ClearDefaultEffort bool
	WorkspaceID        string
}

// MarshalJSON implements json.Marshaler.
func (in UpdateAgentHarnessInput) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{}
	if in.AuthMethod != nil {
		m["auth_method"] = *in.AuthMethod
	}
	switch {
	case in.EnableAllProviders:
		m["enabled_providers"] = nil
	case in.EnabledProviders != nil:
		m["enabled_providers"] = in.EnabledProviders
	}
	switch {
	case in.ClearDefaultModel:
		m["default_model"] = nil
	case in.DefaultModel != nil:
		m["default_model"] = *in.DefaultModel
	}
	switch {
	case in.ClearDefaultEffort:
		m["default_effort"] = nil
	case in.DefaultEffort != nil:
		m["default_effort"] = *in.DefaultEffort
	}
	if in.WorkspaceID != "" {
		m["workspace_id"] = in.WorkspaceID
	}
	return json.Marshal(m)
}

// AgentModelPrice is USD per million tokens.
type AgentModelPrice struct {
	Input      float64  `json:"input"`
	Output     float64  `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	CacheWrite *float64 `json:"cache_write"`
}

// AgentModel is one entry of a harness's model catalog.
type AgentModel struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Vendor   string   `json:"vendor,omitempty"`
	Variant  string   `json:"variant,omitempty"`
	ServedBy []string `json:"served_by,omitempty"`
	// Efforts are the reasoning efforts the model takes, strongest first. An
	// empty list means the model takes no effort setting.
	Efforts       []string `json:"efforts"`
	DefaultEffort string   `json:"default_effort,omitempty"`
	Default       bool     `json:"default"`
	// Available is false for a model the account cannot use today;
	// UnavailableReason says why.
	Available         bool             `json:"available"`
	UnavailableReason string           `json:"unavailable_reason,omitempty"`
	Via               string           `json:"via,omitempty"`
	ContextWindow     *int64           `json:"context_window"`
	Capabilities      map[string]bool  `json:"capabilities"`
	Tier              string           `json:"tier,omitempty"`
	PricePerMTok      *AgentModelPrice `json:"price_per_mtok"`
}

// AgentModels is the model catalog of one harness with the account's
// availability applied.
type AgentModels struct {
	Harness           string             `json:"harness"`
	AuthMethod        string             `json:"auth_method,omitempty"`
	UsableCredentials []string           `json:"usable_credentials"`
	DefaultModel      string             `json:"default_model,omitempty"`
	DefaultOption     *AgentModelDefault `json:"default_option,omitempty"`
	Efforts           []string           `json:"efforts"`
	Models            []AgentModel       `json:"models"`
}

// AgentHarnessCatalog is GET /agents/harnesses. Entries carry more keys than
// the typed ones; Raw has them all.
type AgentHarnessCatalog struct {
	Data                []map[string]interface{} `json:"data"`
	RuntimeCapabilities map[string]interface{}   `json:"runtime_capabilities,omitempty"`
}

// AgentsService reads and changes the Agents tab: which credential powers
// which harness, default models and efforts, and the default harness. Accessed
// via Client.Agents.
//
// Scopes: agents:read and agents:write for settings and models;
// integrations:read and integrations:write for credentials.
type AgentsService struct {
	client *Client
}

func withWorkspace(path, workspaceID string) string {
	return path + buildQuery(map[string]string{"workspace_id": workspaceID})
}

// Settings returns the Agents tab for the organization, or for a workspace's
// effective view when workspaceID is set.
func (s *AgentsService) Settings(ctx context.Context, workspaceID string) (*AgentSettings, error) {
	var out apiResponse[AgentSettings]
	if err := s.client.getJSON(ctx, withWorkspace("/agents/settings", workspaceID), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// SetDefaultHarness stars a harness as the default at this scope. An unknown
// harness is 404 UNKNOWN_HARNESS; "custom" cannot be the default.
func (s *AgentsService) SetDefaultHarness(ctx context.Context, harness, workspaceID string) (*AgentSettings, error) {
	if harness == "" {
		return nil, errors.New("harness is required")
	}
	body := map[string]string{"default_harness": harness}
	if workspaceID != "" {
		body["workspace_id"] = workspaceID
	}
	var out apiResponse[AgentSettings]
	if err := s.client.patchJSON(ctx, "/agents/settings", body, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// UpdateHarness changes one harness's authentication, enabled providers,
// default model or default effort. The osa and custom harnesses have no
// settings here (422 VALIDATION_FAILED).
func (s *AgentsService) UpdateHarness(ctx context.Context, harness string, in UpdateAgentHarnessInput) (*AgentHarness, error) {
	if harness == "" {
		return nil, errors.New("harness is required")
	}
	var out apiResponse[AgentHarness]
	if err := s.client.patchJSON(ctx, "/agents/settings/harnesses/"+url.PathEscape(harness), in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Models returns the model catalog of a harness for the account, with
// availability and efforts.
func (s *AgentsService) Models(ctx context.Context, harness, workspaceID string) (*AgentModels, error) {
	if harness == "" {
		return nil, errors.New("harness is required")
	}
	q := map[string]string{"harness": harness, "workspace_id": workspaceID}
	var out apiResponse[AgentModels]
	if err := s.client.getJSON(ctx, "/agents/models"+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Credentials lists every credential with its connection state.
func (s *AgentsService) Credentials(ctx context.Context, workspaceID string) ([]AgentCredential, error) {
	var out apiResponse[[]AgentCredential]
	if err := s.client.getJSON(ctx, withWorkspace("/agents/credentials", workspaceID), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// PutCredential connects or replaces a credential and returns it.
func (s *AgentsService) PutCredential(ctx context.Context, credential string, in PutAgentCredentialInput) (*AgentCredential, error) {
	if credential == "" {
		return nil, errors.New("credential is required")
	}
	var out apiResponse[AgentCredential]
	if err := s.client.putJSON(ctx, "/agents/credentials/"+url.PathEscape(credential), in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// DeleteCredential disconnects a credential at exactly the given scope (a
// workspace delete leaves the organization's, which the workspace then
// inherits) and returns the credential afterwards. A linked AI Gateway key is
// removed with it.
func (s *AgentsService) DeleteCredential(ctx context.Context, credential, workspaceID string) (*AgentCredential, error) {
	var out apiResponse[AgentCredential]
	if err := s.client.deleteJSON(ctx, withWorkspace("/agents/credentials/"+url.PathEscape(credential), workspaceID), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Harnesses returns the harness catalog with models and runtime capability flags.
func (s *AgentsService) Harnesses(ctx context.Context) (*AgentHarnessCatalog, error) {
	var out AgentHarnessCatalog
	if err := s.client.getJSON(ctx, "/agents/harnesses", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Chats: context meter and compaction ─────────────────────────────────────

// ChatCompaction is one compaction recorded in a chat, oldest first.
type ChatCompaction struct {
	RunID        string `json:"run_id"`
	BeforeTokens *int64 `json:"before_tokens"`
	AfterTokens  *int64 `json:"after_tokens"`
	At           string `json:"at,omitempty"`
}

// ChatContextUse is how full the harness context was at the newest measured run.
type ChatContextUse struct {
	UsedTokens      *int64   `json:"used_tokens"`
	WindowTokens    *int64   `json:"window_tokens"`
	RemainingTokens *int64   `json:"remaining_tokens"`
	UsedRatio       *float64 `json:"used_ratio"`
	Compactions     int      `json:"compactions"`
	Compacted       bool     `json:"compacted"`
	// WindowMode is "1m", "standard" or "".
	WindowMode      string `json:"window_mode,omitempty"`
	MeasuredByRunID string `json:"measured_by_run_id,omitempty"`
	MeasuredAt      string `json:"measured_at,omitempty"`
}

// ChatContext is the context a chat has used and whether it can be compacted.
type ChatContext struct {
	ChatID           string `json:"chat_id"`
	Harness          string `json:"harness"`
	HarnessSessionID string `json:"harness_session_id,omitempty"`
	Runs             int    `json:"runs"`
	LatestRun        struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at,omitempty"`
	} `json:"latest_run"`
	Machine struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"machine"`
	// Context is nil until a run reported token counts.
	Context     *ChatContextUse  `json:"context"`
	Compactions []ChatCompaction `json:"compactions"`
	// Compactable says whether a compaction can be requested now; when false,
	// CompactableReason is harness_unsupported, run_in_progress or no_session.
	Compactable       bool   `json:"compactable"`
	CompactableReason string `json:"compactable_reason,omitempty"`
}

// ChatContext returns the context use of a chat (the runs sharing a chat_id).
// 404 CHAT_NOT_FOUND when the chat has no runs. Needs agents:read.
func (s *AgentsService) ChatContext(ctx context.Context, chatID string) (*ChatContext, error) {
	var out apiResponse[ChatContext]
	if err := s.client.getJSON(ctx, "/agents/chats/"+url.PathEscape(chatID)+"/context", &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// CompactChatResult is the acknowledgement of a compaction request.
type CompactChatResult struct {
	// Run is the run started in the same chat to send the harness its compact
	// command; the compaction shows up as a run.compacted event and in
	// ChatContext once that run finishes.
	Run         Run    `json:"data"`
	ChatID      string `json:"chat_id"`
	Instruction string `json:"instruction"`
}

// CompactChat asks the harness to compact the chat's session by starting a run
// in the same chat (202). focus tells the harness what to keep and may be
// empty. 409 when the chat is busy, 422 COMPACTION_UNSUPPORTED for a harness
// that cannot compact. Needs agents:run.
func (s *AgentsService) CompactChat(ctx context.Context, chatID, focus string) (*CompactChatResult, error) {
	body := map[string]interface{}{}
	if focus != "" {
		body["focus"] = focus
	}
	var out CompactChatResult
	if err := s.client.postJSON(ctx, "/agents/chats/"+url.PathEscape(chatID)+"/compact", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Connected accounts and sign-ins ─────────────────────────────────────────

// SigninProvider is a subscription provider that has a sign-in flow.
type SigninProvider string

const (
	SigninClaudeCode  SigninProvider = "claude_code"
	SigninCodex       SigninProvider = "codex"
	SigninKimiCode    SigninProvider = "kimi_code"
	SigninMistralVibe SigninProvider = "mistral_vibe"
)

// SigninSession is a subscription sign-in in progress.
type SigninSession struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	// Status is awaiting_code, pending, succeeded, failed, canceled or expired.
	Status string `json:"status"`
	// Mode is "paste_code" (Claude: approve, then SubmitSigninCode) or
	// "device_code" (Codex, Kimi Code, Mistral Vibe: open VerificationURL and
	// poll GetSignin; UserCode is set where the user must type one).
	Mode            string `json:"mode"`
	VerificationURL string `json:"verification_url,omitempty"`
	UserCode        string `json:"user_code,omitempty"`
	Error           string `json:"error,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	WorkspaceID     string `json:"workspace_id,omitempty"`
}

// StartSigninInput starts a sign-in.
type StartSigninInput struct {
	Provider    SigninProvider `json:"provider"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	SandboxID   string         `json:"sandbox_id,omitempty"`
	ComputerID  string         `json:"computer_id,omitempty"`
}

// ConnectedAccount is a connected CLI account. Only previews are returned.
type ConnectedAccount map[string]interface{}

// AgentAccountsService manages connected Claude/Codex/Kimi/Mistral accounts
// and the sign-in sessions that create them. Accessed via Client.AgentAccounts.
// Scopes: connections:read and connections:write.
type AgentAccountsService struct {
	client *Client
}

// ListAccountsOptions narrows List.
type ListAccountsOptions struct {
	WorkspaceID string
	Provider    string
	Kind        string
	Status      string
}

// List returns the connected accounts.
func (s *AgentAccountsService) List(ctx context.Context, opts ListAccountsOptions) ([]ConnectedAccount, error) {
	q := map[string]string{"workspace_id": opts.WorkspaceID, "provider": opts.Provider, "kind": opts.Kind, "status": opts.Status}
	var out struct {
		Data []ConnectedAccount `json:"data"`
	}
	if err := s.client.getJSON(ctx, "/agent-accounts"+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// CreateAPIKey stores a fallback API key for a provider.
func (s *AgentAccountsService) CreateAPIKey(ctx context.Context, provider, apiKey, workspaceID, label string) (ConnectedAccount, error) {
	body := map[string]string{"provider": provider, "api_key": apiKey}
	if workspaceID != "" {
		body["workspace_id"] = workspaceID
	}
	if label != "" {
		body["label"] = label
	}
	var out struct {
		Data ConnectedAccount `json:"data"`
	}
	if err := s.client.postJSON(ctx, "/agent-accounts/api-keys", body, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Delete revokes a connection.
func (s *AgentAccountsService) Delete(ctx context.Context, id string) (ConnectedAccount, error) {
	var out struct {
		Data ConnectedAccount `json:"data"`
	}
	if err := s.client.deleteJSON(ctx, "/agent-accounts/"+url.PathEscape(id), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// StartSignin starts a subscription sign-in for claude_code, codex, kimi_code
// or mistral_vibe. The credential of kimi_code and mistral_vibe lands in the
// shared store (kimi_subscription, mistral_subscription). Sessions live 15
// minutes.
func (s *AgentAccountsService) StartSignin(ctx context.Context, in StartSigninInput) (*SigninSession, error) {
	if in.Provider == "" {
		return nil, errors.New("provider is required")
	}
	var out apiResponse[SigninSession]
	if err := s.client.postJSON(ctx, "/agent-signins", in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// GetSignin polls a sign-in session. For a device-code sign-in it asks the
// provider whether the user approved, and connects the account once they did.
func (s *AgentAccountsService) GetSignin(ctx context.Context, id string) (*SigninSession, error) {
	var out apiResponse[SigninSession]
	if err := s.client.getJSON(ctx, "/agent-signins/"+url.PathEscape(id), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// SubmitSigninCode hands over the code the user pasted (Claude's code#state).
func (s *AgentAccountsService) SubmitSigninCode(ctx context.Context, id, code string) (*SigninSession, error) {
	if code == "" {
		return nil, errors.New("code is required")
	}
	var out apiResponse[SigninSession]
	if err := s.client.postJSON(ctx, "/agent-signins/"+url.PathEscape(id)+"/code", map[string]string{"code": code}, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// CancelSignin cancels a sign-in session.
func (s *AgentAccountsService) CancelSignin(ctx context.Context, id string) (*SigninSession, error) {
	var out apiResponse[SigninSession]
	if err := s.client.deleteJSON(ctx, "/agent-signins/"+url.PathEscape(id), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// ─── Connections ─────────────────────────────────────────────────────────────

// Connection is one entry of the unified read model of model providers, apps
// and tool servers. No token, key or secret is ever returned.
type Connection struct {
	ID string `json:"id"`
	// Ref is "<type>:<id>", unique across families.
	Ref string `json:"ref"`
	// Type is agent_credential, provider_key, integration,
	// project_integration, connect_connector or mcp_server.
	Type string `json:"type"`
	// Family is models, apps or tools.
	Family string `json:"family"`
	Kind   string `json:"kind"`
	// Provider is github, openrouter, bedrock, a server name, and so on.
	Provider string `json:"provider"`
	Label    string `json:"label"`
	Account  struct {
		ID          *string `json:"id"`
		DisplayName *string `json:"display_name"`
	} `json:"account"`
	Scope struct {
		Level       string `json:"level"`
		WorkspaceID string `json:"workspace_id,omitempty"`
		ProjectID   string `json:"project_id,omitempty"`
	} `json:"scope"`
	Inherited     bool     `json:"inherited"`
	ScopesGranted []string `json:"scopes_granted"`
	Status        string   `json:"status"`
	ExpiresAt     string   `json:"expires_at,omitempty"`
	Preview       string   `json:"preview,omitempty"`
	// UsableBy is any of agents, gateway, workflows.
	UsableBy []string `json:"usable_by"`
	Shared   bool     `json:"shared"`
	// UsedBy is the harness ids for a model credential, or {"agent_versions":n}
	// for a tool server, or nil for an app.
	UsedBy      json.RawMessage        `json:"used_by"`
	UsedByCount int                    `json:"used_by_count"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	ProjectID   string                 `json:"project_id,omitempty"`
	Owner       *ConnectionOwner       `json:"owner"`
	InsertedAt  string                 `json:"inserted_at,omitempty"`
	Connect     map[string]interface{} `json:"connect,omitempty"`
}

// ConnectionOwner is who connected an app.
type ConnectionOwner struct {
	UserID string `json:"user_id"`
	Mine   bool   `json:"mine"`
}

// ConnectionList is the response of listing connections.
type ConnectionList struct {
	Data   []Connection   `json:"data"`
	Total  int            `json:"total"`
	Counts map[string]int `json:"counts"`
}

// ListConnectionsOptions narrows ListConnections.
type ListConnectionsOptions struct {
	// WorkspaceID is the workspace's effective view (its own plus the
	// organization's); empty is the organization level.
	WorkspaceID string
	// Family is "models", "apps" or "tools".
	Family   string
	Provider string
	// Status is connected, needs_reauth, expired, revoked, error, unverified,
	// reachable, unreachable or unauthorized.
	Status string
	// UsableBy is "agents", "gateway" or "workflows".
	UsableBy string
}

// ConnectionsService reads every connection family in one list. Accessed via
// Client.Connections. Scopes: integrations:read and integrations:write.
type ConnectionsService struct {
	client *Client
}

// List returns connections across model providers, apps and tool servers.
func (s *ConnectionsService) List(ctx context.Context, opts ListConnectionsOptions) (*ConnectionList, error) {
	q := map[string]string{
		"workspace_id": opts.WorkspaceID,
		"family":       opts.Family,
		"provider":     opts.Provider,
		"status":       opts.Status,
		"usable_by":    opts.UsableBy,
	}
	var out ConnectionList
	if err := s.client.getJSON(ctx, "/connections"+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ShareApp sets whether other people in the organization may use an OAuth app
// you connected. 404 NOT_FOUND for someone else's app.
func (s *ConnectionsService) ShareApp(ctx context.Context, id string, shared bool) (*Connection, error) {
	var out apiResponse[Connection]
	if err := s.client.patchJSON(ctx, "/connections/apps/"+url.PathEscape(id), map[string]bool{"shared": shared}, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}
