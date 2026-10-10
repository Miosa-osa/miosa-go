package miosa

import (
	"context"
	"encoding/json"
	"net/url"
)

// Wire contract: miosa-compute docs/api/environments.md.
// Every path is relative to the API base URL (".../api/v1").

// EnvironmentScope is where an environment lives.
type EnvironmentScope string

const (
	// EnvironmentScopeOrganization is an organization-level environment
	// (workspace_id null).
	EnvironmentScopeOrganization EnvironmentScope = "organization"
	// EnvironmentScopeWorkspace is an environment owned by one workspace.
	EnvironmentScopeWorkspace EnvironmentScope = "workspace"
)

// EnvironmentVariable is a variable as listed on an environment. The API never
// returns the plaintext of a secret variable: Preview is masked (first 6 and
// last 4 characters). Use RevealVariable for the value.
type EnvironmentVariable struct {
	Name    string `json:"name"`
	Secret  bool   `json:"secret"`
	Preview string `json:"preview"`
}

// EnvironmentSecretFile is a secret file as listed on an environment (no contents).
type EnvironmentSecretFile struct {
	Path      string `json:"path"`
	SizeBytes int    `json:"size_bytes"`
}

// EnvironmentRepository is a repository attached to an environment.
type EnvironmentRepository struct {
	// Source is "github" (default) or "forge".
	Source        string `json:"source,omitempty"`
	Repo          string `json:"repo"`
	BaseBranch    string `json:"base_branch,omitempty"`
	SetupScript   string `json:"setup_script,omitempty"`
	SetupBlocking bool   `json:"setup_blocking"`
	// Path is the folder under the machine's home directory.
	Path string `json:"path,omitempty"`
}

// EnvironmentToggles are the four pass-through switches.
type EnvironmentToggles struct {
	PassGithub             bool `json:"pass_github"`
	PassSecrets            bool `json:"pass_secrets"`
	PassSandboxCredentials bool `json:"pass_sandbox_credentials"`
	PassAgentsCredentials  bool `json:"pass_agents_credentials"`
}

// EnvironmentEffective is what a machine actually receives: the four toggles,
// all false while SafeForThirdParties is set.
type EnvironmentEffective = EnvironmentToggles

// Environment is a named template that new sandboxes and computers inherit.
type Environment struct {
	ID                     string                  `json:"id"`
	Name                   string                  `json:"name"`
	WorkspaceID            string                  `json:"workspace_id,omitempty"`
	Scope                  EnvironmentScope        `json:"scope,omitempty"`
	IsDefault              bool                    `json:"is_default"`
	EffectiveDefault       bool                    `json:"effective_default"`
	SafeForThirdParties    bool                    `json:"safe_for_third_parties"`
	PassGithub             bool                    `json:"pass_github"`
	PassSecrets            bool                    `json:"pass_secrets"`
	PassSandboxCredentials bool                    `json:"pass_sandbox_credentials"`
	PassAgentsCredentials  bool                    `json:"pass_agents_credentials"`
	Effective              EnvironmentEffective    `json:"effective"`
	LatestVersion          int                     `json:"latest_version"`
	VersionCount           int                     `json:"version_count"`
	PinnedMachineCount     int                     `json:"pinned_machine_count"`
	PinnedSandboxCount     int                     `json:"pinned_sandbox_count"`
	PinnedComputerCount    int                     `json:"pinned_computer_count"`
	OutdatedMachineCount   int                     `json:"outdated_machine_count"`
	ExtendsDefault         bool                    `json:"extends_default"`
	Variables              []EnvironmentVariable   `json:"variables"`
	SecretFiles            []EnvironmentSecretFile `json:"secret_files"`
	Repositories           []EnvironmentRepository `json:"repositories"`
	// CLITools maps a tool to "latest" or a version spec.
	CLITools map[string]string `json:"cli_tools,omitempty"`
	// Connections are references such as "mcp_server:<id>" or
	// "connected_account:<id>"; never a credential value.
	Connections   []string               `json:"connections,omitempty"`
	SetupScript   string                 `json:"setup_script,omitempty"`
	SetupBlocking bool                   `json:"setup_blocking"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     string                 `json:"created_at,omitempty"`
	UpdatedAt     string                 `json:"updated_at,omitempty"`
	DeletedAt     string                 `json:"deleted_at,omitempty"`
}

// EnvironmentVersion is one immutable version of an environment. It never
// carries values, only names and paths.
type EnvironmentVersion struct {
	Version                int                     `json:"version"`
	CreatedAt              string                  `json:"created_at,omitempty"`
	CreatedBy              string                  `json:"created_by,omitempty"`
	PinnedMachineCount     int                     `json:"pinned_machine_count"`
	PinnedSandboxCount     int                     `json:"pinned_sandbox_count"`
	PinnedComputerCount    int                     `json:"pinned_computer_count"`
	SafeForThirdParties    bool                    `json:"safe_for_third_parties"`
	PassGithub             bool                    `json:"pass_github"`
	PassSecrets            bool                    `json:"pass_secrets"`
	PassSandboxCredentials bool                    `json:"pass_sandbox_credentials"`
	PassAgentsCredentials  bool                    `json:"pass_agents_credentials"`
	VariableNames          []string                `json:"variable_names"`
	SecretFilePaths        []string                `json:"secret_file_paths"`
	Repositories           []EnvironmentRepository `json:"repositories"`
}

// BillingAccount is the account new machines bill ("New sandboxes bill X").
// Wallet is nil until wallets ship; Kind is "organization" today.
type BillingAccount struct {
	Kind   string                 `json:"kind"`
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Wallet map[string]interface{} `json:"wallet"`
}

// EnvironmentList is the response of GET /environments.
type EnvironmentList struct {
	// Environments lists the effective default first, then workspace before
	// organization, then by name.
	Environments         []Environment   `json:"environments"`
	DefaultEnvironmentID string          `json:"default_environment_id"`
	Billing              *BillingAccount `json:"billing,omitempty"`
}

// EnvironmentSecretFileInput is a secret file to write.
type EnvironmentSecretFileInput struct {
	// Path is relative to the machine's home directory.
	Path     string `json:"path"`
	Contents string `json:"contents"`
}

// EnvironmentRepositoryInput is a repository to attach.
type EnvironmentRepositoryInput struct {
	Repo string `json:"repo"`
	// Source is "github" (default) or "forge".
	Source        string `json:"source,omitempty"`
	BaseBranch    string `json:"base_branch,omitempty"`
	SetupScript   string `json:"setup_script,omitempty"`
	SetupBlocking *bool  `json:"setup_blocking,omitempty"`
}

// EnvironmentRepositoryRef names a repository to detach. It marshals as the
// bare "owner/name" string when Source is empty, else as {"repo","source"}.
type EnvironmentRepositoryRef struct {
	Repo   string
	Source string
}

// MarshalJSON implements json.Marshaler.
func (r EnvironmentRepositoryRef) MarshalJSON() ([]byte, error) {
	if r.Source == "" {
		return json.Marshal(r.Repo)
	}
	return json.Marshal(map[string]string{"repo": r.Repo, "source": r.Source})
}

// CreateEnvironmentInput is the body of POST /environments.
type CreateEnvironmentInput struct {
	Name                   string                       `json:"name"`
	WorkspaceID            string                       `json:"workspace_id,omitempty"`
	IsDefault              *bool                        `json:"is_default,omitempty"`
	SafeForThirdParties    *bool                        `json:"safe_for_third_parties,omitempty"`
	PassGithub             *bool                        `json:"pass_github,omitempty"`
	PassSecrets            *bool                        `json:"pass_secrets,omitempty"`
	PassSandboxCredentials *bool                        `json:"pass_sandbox_credentials,omitempty"`
	PassAgentsCredentials  *bool                        `json:"pass_agents_credentials,omitempty"`
	ExtendsDefault         *bool                        `json:"extends_default,omitempty"`
	Variables              map[string]string            `json:"variables,omitempty"`
	PlainVariables         []string                     `json:"plain_variables,omitempty"`
	SecretFiles            []EnvironmentSecretFileInput `json:"secret_files,omitempty"`
	Repositories           []EnvironmentRepositoryInput `json:"repositories,omitempty"`
	CLITools               map[string]string            `json:"cli_tools,omitempty"`
	Connections            []string                     `json:"connections,omitempty"`
	SetupScript            string                       `json:"setup_script,omitempty"`
	SetupBlocking          *bool                        `json:"setup_blocking,omitempty"`
}

// UpdateEnvironmentInput is the body of PATCH /environments/:id: one atomic
// save. Everything set here applies together and produces exactly one new
// version (or none when nothing a machine sees changed). Unset fields are
// omitted from the request.
//
// Prefer the *Set/*Unset members: they cannot drop data you did not mention.
// Variables, SecretFiles, Repositories, CLITools and Connections are full
// replacements.
type UpdateEnvironmentInput struct {
	// Not versioned.
	Name      *string
	IsDefault *bool

	SafeForThirdParties    *bool
	PassGithub             *bool
	PassSecrets            *bool
	PassSandboxCredentials *bool
	PassAgentsCredentials  *bool
	ExtendsDefault         *bool

	VariablesSet      map[string]string
	VariablesUnset    []string
	PlainVariables    []string
	SecretFilesSet    []EnvironmentSecretFileInput
	SecretFilesUnset  []string
	RepositoriesSet   []EnvironmentRepositoryInput
	RepositoriesUnset []EnvironmentRepositoryRef
	CLIToolsSet       map[string]string
	CLIToolsUnset     []string
	ConnectionsSet    []string
	ConnectionsUnset  []string
	SetupScript       *string
	ClearSetupScript  bool
	SetupBlocking     *bool
	Variables         map[string]string
	SecretFiles       []EnvironmentSecretFileInput
	Repositories      []EnvironmentRepositoryInput
	CLITools          map[string]string
	Connections       []string
	// WorkspaceID is filled from the service scope when empty.
	WorkspaceID string
}

// MarshalJSON implements json.Marshaler, emitting only what is set.
func (in UpdateEnvironmentInput) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{}
	put := func(k string, v interface{}) { m[k] = v }
	if in.Name != nil {
		put("name", *in.Name)
	}
	if in.IsDefault != nil {
		put("is_default", *in.IsDefault)
	}
	for k, v := range map[string]*bool{
		"safe_for_third_parties":   in.SafeForThirdParties,
		"pass_github":              in.PassGithub,
		"pass_secrets":             in.PassSecrets,
		"pass_sandbox_credentials": in.PassSandboxCredentials,
		"pass_agents_credentials":  in.PassAgentsCredentials,
		"extends_default":          in.ExtendsDefault,
		"setup_blocking":           in.SetupBlocking,
	} {
		if v != nil {
			put(k, *v)
		}
	}
	if len(in.VariablesSet) > 0 {
		put("variables_set", in.VariablesSet)
	}
	if len(in.VariablesUnset) > 0 {
		put("variables_unset", in.VariablesUnset)
	}
	if len(in.PlainVariables) > 0 {
		put("plain_variables", in.PlainVariables)
	}
	if len(in.SecretFilesSet) > 0 {
		put("secret_files_set", in.SecretFilesSet)
	}
	if len(in.SecretFilesUnset) > 0 {
		put("secret_files_unset", in.SecretFilesUnset)
	}
	if len(in.RepositoriesSet) > 0 {
		put("repositories_set", in.RepositoriesSet)
	}
	if len(in.RepositoriesUnset) > 0 {
		put("repositories_unset", in.RepositoriesUnset)
	}
	if len(in.CLIToolsSet) > 0 {
		put("cli_tools_set", in.CLIToolsSet)
	}
	if len(in.CLIToolsUnset) > 0 {
		put("cli_tools_unset", in.CLIToolsUnset)
	}
	if len(in.ConnectionsSet) > 0 {
		put("connections_set", in.ConnectionsSet)
	}
	if len(in.ConnectionsUnset) > 0 {
		put("connections_unset", in.ConnectionsUnset)
	}
	switch {
	case in.ClearSetupScript:
		put("setup_script", nil)
	case in.SetupScript != nil:
		put("setup_script", *in.SetupScript)
	}
	if in.Variables != nil {
		put("variables", in.Variables)
	}
	if in.SecretFiles != nil {
		put("secret_files", in.SecretFiles)
	}
	if in.Repositories != nil {
		put("repositories", in.Repositories)
	}
	if in.CLITools != nil {
		put("cli_tools", in.CLITools)
	}
	if in.Connections != nil {
		put("connections", in.Connections)
	}
	if in.WorkspaceID != "" {
		put("workspace_id", in.WorkspaceID)
	}
	return json.Marshal(m)
}

// EnvironmentUpgradedMachine is one machine moved by an upgrade.
type EnvironmentUpgradedMachine struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	FromVersion int    `json:"from_version"`
	ToVersion   int    `json:"to_version"`
	// Via is "environment", or "default" when the machine's environment
	// extends this one as its default.
	Via string `json:"via,omitempty"`
	// Status is "applying" for a running machine, "pending_resume" for a
	// stopped or paused one.
	Status string `json:"status"`
}

// EnvironmentUpgradeResult is the response of POST /environments/:id/upgrade.
type EnvironmentUpgradeResult struct {
	EnvironmentID string                       `json:"environment_id"`
	LatestVersion int                          `json:"latest_version"`
	Machines      []EnvironmentUpgradedMachine `json:"machines"`
}

// EnvironmentEffectiveVariable names one variable and where it came from.
// Values are never returned.
type EnvironmentEffectiveVariable struct {
	Name   string `json:"name"`
	Source string `json:"source,omitempty"`
	Target string `json:"target,omitempty"`
}

// EnvironmentLayer is one layer of an effective view, lowest first.
type EnvironmentLayer struct {
	// Source is organization_defaults, workspace_defaults,
	// default_environment, environment, agent or credentials.
	Source    string                         `json:"source"`
	Variables []EnvironmentEffectiveVariable `json:"variables"`
}

// EnvironmentEffectiveView is what a machine (or an agent run) in an
// environment would receive, as layers, with names only.
type EnvironmentEffectiveView struct {
	EnvironmentID string                         `json:"environment_id"`
	Version       int                            `json:"version"`
	Layers        []EnvironmentLayer             `json:"layers"`
	Variables     []EnvironmentEffectiveVariable `json:"variables"`
	SecretFiles   []string                       `json:"secret_files"`
	Repositories  []EnvironmentRepository        `json:"repositories"`
	CLITools      map[string]string              `json:"cli_tools"`
	Connections   []string                       `json:"connections"`
	SetupScript   *string                        `json:"setup_script"`
	SetupBlocking bool                           `json:"setup_blocking"`
	Effective     EnvironmentEffective           `json:"effective"`
}

// EffectiveOptions narrows GET /environments/:id/effective.
type EffectiveOptions struct {
	// AgentID adds the agent layer (runtime env rows with target "agent").
	AgentID string
}

// EnvironmentsService manages environments. Accessed via Client.Environments.
//
// Environments belong to the organization or to one workspace. ForWorkspace
// returns a view pinned to one workspace; the unscoped service lets the API
// resolve the scope (a workspace-bound key's workspace, else the organization).
type EnvironmentsService struct {
	client      *Client
	workspaceID string
}

// ForWorkspace returns the service scoped to a workspace id. An empty id
// returns the organization-level service.
func (s *EnvironmentsService) ForWorkspace(workspaceID string) *EnvironmentsService {
	return &EnvironmentsService{client: s.client, workspaceID: workspaceID}
}

// WorkspaceID is the workspace this service is scoped to, or "".
func (s *EnvironmentsService) WorkspaceID() string { return s.workspaceID }

// at builds a request path with the workspace scope and any extra query. The
// API reads the scope from a query parameter on GET and DELETE.
func (s *EnvironmentsService) at(path string, query map[string]string) string {
	q := map[string]string{}
	for k, v := range query {
		q[k] = v
	}
	if s.workspaceID != "" {
		q["workspace_id"] = s.workspaceID
	}
	return path + buildQuery(q)
}

// body adds the scope as a body field, as the API reads it on POST, PUT, PATCH.
func (s *EnvironmentsService) body(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		m = map[string]interface{}{}
	}
	if s.workspaceID != "" {
		m["workspace_id"] = s.workspaceID
	}
	return m
}

func envPath(idOrName string, parts ...string) string {
	p := "/environments/" + url.PathEscape(idOrName)
	for _, part := range parts {
		p += "/" + part
	}
	return p
}

type environmentEnvelope struct {
	Environment Environment `json:"environment"`
}

// ListEnvironmentsOptions narrows List.
type ListEnvironmentsOptions struct {
	IncludeDeleted bool
}

// List returns every environment visible in the scope, effective default first.
func (s *EnvironmentsService) List(ctx context.Context) (*EnvironmentList, error) {
	return s.ListWith(ctx, ListEnvironmentsOptions{})
}

// ListWith is List with options.
func (s *EnvironmentsService) ListWith(ctx context.Context, opts ListEnvironmentsOptions) (*EnvironmentList, error) {
	q := map[string]string{}
	if opts.IncludeDeleted {
		q["include_deleted"] = "true"
	}
	var out EnvironmentList
	if err := s.client.getJSON(ctx, s.at("/environments", q), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Billing returns the account new machines bill.
func (s *EnvironmentsService) Billing(ctx context.Context) (*BillingAccount, error) {
	var out struct {
		Billing BillingAccount `json:"billing"`
	}
	if err := s.client.getJSON(ctx, s.at("/environments/billing", nil), &out); err != nil {
		return nil, err
	}
	return &out.Billing, nil
}

// Get fetches one environment by id or name.
func (s *EnvironmentsService) Get(ctx context.Context, idOrName string) (*Environment, error) {
	var out environmentEnvelope
	if err := s.client.getJSON(ctx, s.at(envPath(idOrName), nil), &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// Default returns the effective default environment of the scope.
func (s *EnvironmentsService) Default(ctx context.Context) (*Environment, error) {
	list, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list.Environments {
		e := &list.Environments[i]
		if e.ID == list.DefaultEnvironmentID || e.EffectiveDefault {
			return e, nil
		}
	}
	return nil, &MiosaError{Message: "no default environment returned by the API"}
}

// Create makes a new environment at version 1.
func (s *EnvironmentsService) Create(ctx context.Context, in CreateEnvironmentInput) (*Environment, error) {
	if in.WorkspaceID == "" {
		in.WorkspaceID = s.workspaceID
	}
	var out environmentEnvelope
	if err := s.client.postJSON(ctx, "/environments", in, &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// Update applies one atomic save (PATCH). See UpdateEnvironmentInput.
func (s *EnvironmentsService) Update(ctx context.Context, idOrName string, in UpdateEnvironmentInput) (*Environment, error) {
	if in.WorkspaceID == "" {
		in.WorkspaceID = s.workspaceID
	}
	var out environmentEnvelope
	if err := s.client.patchJSON(ctx, envPath(idOrName), in, &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// Rename renames an environment; machines stay pinned. Not versioned.
func (s *EnvironmentsService) Rename(ctx context.Context, idOrName, newName string) (*Environment, error) {
	var out environmentEnvelope
	body := s.body(map[string]interface{}{"name": newName})
	if err := s.client.postJSON(ctx, envPath(idOrName, "rename"), body, &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// MakeDefault makes the environment the default of its own scope. A workspace
// environment made default overrides the organization default for that workspace.
func (s *EnvironmentsService) MakeDefault(ctx context.Context, idOrName string) (*Environment, error) {
	var out environmentEnvelope
	if err := s.client.postJSON(ctx, envPath(idOrName, "default"), s.body(nil), &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// SetDefault is MakeDefault.
func (s *EnvironmentsService) SetDefault(ctx context.Context, idOrName string) (*Environment, error) {
	return s.MakeDefault(ctx, idOrName)
}

// InheritDefault stops a workspace overriding the organization default
// (DELETE /environments/default). It needs a workspace scope: at the
// organization level the API answers 422 WORKSPACE_REQUIRED. It returns the id
// of the default now in effect.
func (s *EnvironmentsService) InheritDefault(ctx context.Context) (string, error) {
	var out struct {
		DefaultEnvironmentID string `json:"default_environment_id"`
	}
	if err := s.client.deleteJSON(ctx, s.at("/environments/default", nil), &out); err != nil {
		return "", err
	}
	return out.DefaultEnvironmentID, nil
}

// Delete soft-deletes an environment. Pinned machines keep running. The
// organization default cannot be deleted (409 ENVIRONMENT_IS_DEFAULT).
func (s *EnvironmentsService) Delete(ctx context.Context, idOrName string) error {
	return s.client.deleteJSON(ctx, s.at(envPath(idOrName), nil), nil)
}

// SetVariable sets or replaces one variable (a new version).
func (s *EnvironmentsService) SetVariable(ctx context.Context, idOrName, name, value string) (*Environment, error) {
	var out environmentEnvelope
	body := s.body(map[string]interface{}{"value": value})
	if err := s.client.putJSON(ctx, envPath(idOrName, "variables", url.PathEscape(name)), body, &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// DeleteVariable removes one variable (a new version).
func (s *EnvironmentsService) DeleteVariable(ctx context.Context, idOrName, name string) (*Environment, error) {
	var out environmentEnvelope
	if err := s.client.deleteJSON(ctx, s.at(envPath(idOrName, "variables", url.PathEscape(name)), nil), &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// RevealVariable returns the plaintext value of one variable (audit-logged,
// needs env:write).
func (s *EnvironmentsService) RevealVariable(ctx context.Context, idOrName, name string) (string, error) {
	var out struct {
		Value string `json:"value"`
	}
	if err := s.client.getJSON(ctx, s.at(envPath(idOrName, "variables", url.PathEscape(name), "reveal"), nil), &out); err != nil {
		return "", err
	}
	return out.Value, nil
}

// SetSecretFile writes one secret file by path, relative to the machine home
// (a new version).
func (s *EnvironmentsService) SetSecretFile(ctx context.Context, idOrName, path, contents string) (*Environment, error) {
	var out environmentEnvelope
	body := s.body(map[string]interface{}{"path": path, "contents": contents})
	if err := s.client.putJSON(ctx, envPath(idOrName, "files"), body, &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// DeleteSecretFile removes one secret file (a new version).
func (s *EnvironmentsService) DeleteSecretFile(ctx context.Context, idOrName, path string) (*Environment, error) {
	var out environmentEnvelope
	if err := s.client.deleteJSON(ctx, s.at(envPath(idOrName, "files"), map[string]string{"path": path}), &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// RevealSecretFile returns the contents of one secret file (audit-logged,
// needs env:write).
func (s *EnvironmentsService) RevealSecretFile(ctx context.Context, idOrName, path string) (string, error) {
	var out struct {
		Contents string `json:"contents"`
	}
	if err := s.client.getJSON(ctx, s.at(envPath(idOrName, "files", "reveal"), map[string]string{"path": path}), &out); err != nil {
		return "", err
	}
	return out.Contents, nil
}

// AddRepository attaches a repository, or replaces the settings of one that is
// already attached (a new version).
func (s *EnvironmentsService) AddRepository(ctx context.Context, idOrName string, in EnvironmentRepositoryInput) (*Environment, error) {
	body := map[string]interface{}{"repo": in.Repo}
	if in.Source != "" {
		body["source"] = in.Source
	}
	if in.BaseBranch != "" {
		body["base_branch"] = in.BaseBranch
	}
	if in.SetupScript != "" {
		body["setup_script"] = in.SetupScript
	}
	if in.SetupBlocking != nil {
		body["setup_blocking"] = *in.SetupBlocking
	}
	var out environmentEnvelope
	if err := s.client.postJSON(ctx, envPath(idOrName, "repositories"), s.body(body), &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// RemoveRepository detaches a repository, given as "owner/name" (a new version).
func (s *EnvironmentsService) RemoveRepository(ctx context.Context, idOrName, repo string) (*Environment, error) {
	var out environmentEnvelope
	if err := s.client.deleteJSON(ctx, s.at(envPath(idOrName, "repositories"), map[string]string{"repo": repo}), &out); err != nil {
		return nil, err
	}
	return &out.Environment, nil
}

// Versions lists every version, newest first.
func (s *EnvironmentsService) Versions(ctx context.Context, idOrName string) ([]EnvironmentVersion, error) {
	var out struct {
		Versions []EnvironmentVersion `json:"versions"`
	}
	if err := s.client.getJSON(ctx, s.at(envPath(idOrName, "versions"), nil), &out); err != nil {
		return nil, err
	}
	return out.Versions, nil
}

// Effective returns what a machine in this environment would receive, as
// layers with the source of every variable name.
func (s *EnvironmentsService) Effective(ctx context.Context, idOrName string, opts EffectiveOptions) (*EnvironmentEffectiveView, error) {
	var out struct {
		Effective EnvironmentEffectiveView `json:"effective"`
	}
	q := map[string]string{"agent_id": opts.AgentID}
	if err := s.client.getJSON(ctx, s.at(envPath(idOrName, "effective"), q), &out); err != nil {
		return nil, err
	}
	return &out.Effective, nil
}

// Upgrade moves machines onto the environment's latest version. With no
// machine ids it moves every live machine that is behind. Secrets the new
// version withholds are deleted from the machines, which cannot be undone.
func (s *EnvironmentsService) Upgrade(ctx context.Context, idOrName string, machineIDs ...string) (*EnvironmentUpgradeResult, error) {
	body := map[string]interface{}{}
	if len(machineIDs) > 0 {
		body["machine_ids"] = machineIDs
	}
	var out EnvironmentUpgradeResult
	if err := s.client.postJSON(ctx, envPath(idOrName, "upgrade"), s.body(body), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeploymentEnvironment is one entry of the deployment-environment inventory
// (production, staging, preview): what "environment" means on deployments.
type DeploymentEnvironment map[string]interface{}

// DeploymentInventory returns the deployment-environment inventory from
// GET /deployment-environments. Before environments existed this answered at
// GET /environments (still repeated in its deprecated data member for one
// release); it takes the same workspace scope.
func (s *EnvironmentsService) DeploymentInventory(ctx context.Context) ([]DeploymentEnvironment, error) {
	var out struct {
		Data []DeploymentEnvironment `json:"data"`
	}
	if err := s.client.getJSON(ctx, s.at("/deployment-environments", nil), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}
