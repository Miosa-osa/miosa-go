package miosa

import "context"

// IntegrationsService provides access to OAuth account-level integrations
// (GitHub, Slack, Linear, Discord).
type IntegrationsService struct {
	client *Client
}

// IntegrationData is the API representation of an active OAuth integration.
type IntegrationData struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	Provider    string                 `json:"provider"`
	Status      string                 `json:"status"`
	AccountName string                 `json:"account_name,omitempty"`
	Scopes      []string               `json:"scopes,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ConnectedAt string                 `json:"connected_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// IntegrationListResponse wraps GET /integrations.
type IntegrationListResponse struct {
	Data []IntegrationData `json:"data"`
}

// IntegrationCatalogEntry describes a provider available for OAuth connection.
type IntegrationCatalogEntry struct {
	Provider    string   `json:"provider"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
	LogoURL     string   `json:"logo_url,omitempty"`
}

// IntegrationCatalogResponse wraps GET /integrations/catalog.
type IntegrationCatalogResponse struct {
	Data []IntegrationCatalogEntry `json:"data"`
}

// IntegrationStartResult is the response from GET /integrations/:provider/start.
type IntegrationStartResult struct {
	AuthorizeURL string `json:"authorize_url"`
	State        string `json:"state"`
}

// GitHubRepo describes a GitHub repository accessible via the integration.
type GitHubRepo struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
}

// GitHubRepoListResponse wraps GET /integrations/github/repos.
type GitHubRepoListResponse struct {
	Data []GitHubRepo `json:"data"`
}

// GitHubSSHKey describes a configured GitHub deploy key.
type GitHubSSHKey struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Key       string `json:"key"`
	ReadOnly  bool   `json:"read_only"`
	CreatedAt string `json:"created_at"`
}

// GitHubSSHKeyListResponse wraps GET /integrations/github/ssh-keys.
type GitHubSSHKeyListResponse struct {
	Data []GitHubSSHKey `json:"data"`
}

// LinearCreateIssueInput is the request body for POST /integrations/linear/create-issue.
type LinearCreateIssueInput struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	TeamID      string `json:"team_id,omitempty"`
	Priority    int    `json:"priority,omitempty"`
}

// LinearIssueResult is the response from POST /integrations/linear/create-issue.
type LinearIssueResult struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// SlackSendTestInput is the request body for POST /integrations/slack/send-test.
type SlackSendTestInput struct {
	Message string `json:"message,omitempty"`
	Channel string `json:"channel,omitempty"`
}

// DiscordSendTestInput is the request body for POST /integrations/discord/send-test.
type DiscordSendTestInput struct {
	Message string `json:"message,omitempty"`
}

// List returns active OAuth integrations for the tenant.
func (s *IntegrationsService) List(ctx context.Context) (*IntegrationListResponse, error) {
	var out IntegrationListResponse
	if err := s.client.getJSON(ctx, "/integrations", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Catalog lists available providers in the integration catalog.
func (s *IntegrationsService) Catalog(ctx context.Context) (*IntegrationCatalogResponse, error) {
	var out IntegrationCatalogResponse
	if err := s.client.getJSON(ctx, "/integrations/catalog", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Start begins the OAuth flow for a provider and returns an authorize URL.
func (s *IntegrationsService) Start(ctx context.Context, provider string) (*IntegrationStartResult, error) {
	var env apiResponse[IntegrationStartResult]
	if err := s.client.getJSON(ctx, "/integrations/"+provider+"/start", &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Refresh force-refreshes the access token for a provider.
func (s *IntegrationsService) Refresh(ctx context.Context, provider string) (*IntegrationData, error) {
	var env apiResponse[IntegrationData]
	if err := s.client.postJSON(ctx, "/integrations/"+provider+"/refresh", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Disconnect revokes and removes an integration.
func (s *IntegrationsService) Disconnect(ctx context.Context, provider string) error {
	return s.client.deleteJSON(ctx, "/integrations/"+provider, nil)
}

// GitHubRepos lists GitHub repositories accessible via the integration.
func (s *IntegrationsService) GitHubRepos(ctx context.Context) (*GitHubRepoListResponse, error) {
	var out GitHubRepoListResponse
	if err := s.client.getJSON(ctx, "/integrations/github/repos", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GitHubSSHKeys lists configured GitHub deploy keys.
func (s *IntegrationsService) GitHubSSHKeys(ctx context.Context) (*GitHubSSHKeyListResponse, error) {
	var out GitHubSSHKeyListResponse
	if err := s.client.getJSON(ctx, "/integrations/github/ssh-keys", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SlackSendTest sends a test message to the connected Slack channel.
func (s *IntegrationsService) SlackSendTest(ctx context.Context, input SlackSendTestInput) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/integrations/slack/send-test", input, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DiscordSendTest sends a test message to the connected Discord channel.
func (s *IntegrationsService) DiscordSendTest(ctx context.Context, input DiscordSendTestInput) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/integrations/discord/send-test", input, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// LinearStart begins the Linear OAuth flow.
func (s *IntegrationsService) LinearStart(ctx context.Context) (*IntegrationStartResult, error) {
	var env apiResponse[IntegrationStartResult]
	if err := s.client.getJSON(ctx, "/integrations/linear/start", &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// LinearCreateIssue creates a Linear issue via the connected workspace.
func (s *IntegrationsService) LinearCreateIssue(ctx context.Context, input LinearCreateIssueInput) (*LinearIssueResult, error) {
	var env apiResponse[LinearIssueResult]
	if err := s.client.postJSON(ctx, "/integrations/linear/create-issue", input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
