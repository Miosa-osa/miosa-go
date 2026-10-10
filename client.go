package miosa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/http2"
)

const (
	defaultBaseURL    = "https://api.miosa.ai/api/v1"
	defaultTimeout    = 60 * time.Second
	defaultMaxRetries = 3
	sdkVersion        = "2.1.0"
	// maxRetryHint is the longest server retry hint the client waits out
	// itself. Longer hints are returned to the caller as the error.
	maxRetryHint = 30 * time.Second
)

// ClientOption is a functional option for configuring a Client.
type ClientOption func(*Client)

// WithBaseURL overrides the default API base URL.
func WithBaseURL(u string) ClientOption {
	return func(c *Client) { c.baseURL = u }
}

// WithHTTPClient replaces the default HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// WithMaxRetries sets the maximum number of retry attempts for retryable errors.
// Set to 0 to disable retries.
func WithMaxRetries(n int) ClientOption {
	return func(c *Client) { c.maxRetries = n }
}

// Client is the root MIOSA API client.
// Use NewClient to construct one.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int
	// defaultHeaders are sent on every request (see WithBillTo).
	defaultHeaders map[string]string
	// accessToken, when set, is sent as the bearer token instead of apiKey
	// (an OAuth access token or session JWT).
	accessToken string
	// tenant is sent as X-MIOSA-Tenant.
	tenant string
	// userAgentSuffix is appended to the default user agent.
	userAgentSuffix string
	// userAgentOverride replaces the user agent entirely.
	userAgentOverride string

	// Services - populated by NewClient.
	Computers           *ComputersService
	Sandboxes           *SandboxesService
	Bulk                *BulkService
	SharedLinks         *SharedLinksService
	Domains             *DomainsService
	AuditLog            *AuditLogService
	SandboxSpend        *SandboxSpendService
	Devices             *DevicesService
	Deployments         *DeploymentsService
	DockerDeploy        *DockerDeployService
	Files               *FilesService
	Credits             *CreditsService
	Admin               *AdminService
	Workspaces          *WorkspacesService
	OpenComputers       *OpenComputersService
	Databases           *DatabasesService
	Storage             *StorageService
	Volumes             *VolumesService
	CustomDomains       *FlatCustomDomainsService
	Functions           *FunctionsService
	CronJobs            *CronJobsService
	HealthChecks        *HealthChecksService
	Webhooks            *WebhooksService
	Templates           *TemplatesService
	SandboxTemplates    *SandboxTemplatesService
	ApiKeys             *ApiKeysService
	Tenant              *TenantService
	Regions             *RegionsService
	Settings            *SettingsService
	Dashboard           *DashboardService
	Analytics           *AnalyticsService
	Usage               *UsageService
	Channels            *ChannelsService
	Integrations        *IntegrationsService
	ProjectIntegrations *ProjectIntegrationsService
	ProjectAuth         *ProjectAuthService
	ExternalKeys        *ExternalKeysService
	Mcp                 *McpService
	Runs                *RunsService
	// P3/P4 services — intelligence gateway, admin, and catalog surfaces.
	Models              *ModelsService
	Completions         *CompletionsService
	Embeddings          *EmbeddingsService
	ProviderDefaults    *ProviderDefaultsService
	Benchmarks          *BenchmarksService
	CommandCenter       *CommandCenterService
	Community           *CommunityService
	Email               *EmailService
	BuilderSessions     *BuilderSessionsService
	SnapshotsStandalone *SnapshotsStandaloneService
	// Members & Invites
	WorkspaceMembers *WorkspaceMembersService
	WorkspaceInvites *WorkspaceInvitesService
	OrgInvites       *OrgInvitesService
	// Egress — tenant-wide secret vault, network allowlist/policy, audit log.
	Secrets *EgressSecretsService
	Network *EgressNetworkService
	Audit   *EgressAuditService
	// Phase 1-4 additions.
	Quotas *QuotasService
	Forge  *ForgeService
	// Sprint 2026-10 additions.
	Environments   *EnvironmentsService
	Snapshots      *AccountSnapshotsService
	NamedSnapshots *NamedSnapshotsService
	BillTo         *BillToService
	MemberCaps     *MemberCapsService
	Agents         *AgentsService
	AgentAccounts  *AgentAccountsService
	Connections    *ConnectionsService
	AIGateway      *AIGatewayService
	// Multi-tenant platform building blocks.
	ScopedTokens *ScopedTokensService
	Policies     *PoliciesService
	// ServiceAccounts manage non-human principals and their keys.
	ServiceAccounts *ServiceAccountsService
	// Events manages event subscriptions and reads the event outbox.
	Events *EventSubscriptionsService
}

// newDefaultTransport builds an *http.Transport tuned for SDK use:
// HTTP/2 enabled, large keep-alive pool, TLS session resumption left to
// the stdlib defaults (which already cache via ClientSessionCache).
//
// Calling this once per Client (not per request) is what kills the
// per-call TLS handshake tax. The transport is goroutine-safe and is
// designed to be shared.
func newDefaultTransport() *http.Transport {
	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		// Force IPv4/IPv6 dual-stack like stdlib default.
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	// Explicit HTTP/2 negotiation. ForceAttemptHTTP2 above already does
	// this for plain transports, but ConfigureTransport is the canonical
	// way to opt in and is a no-op if already configured. Errors here
	// are non-fatal — we fall back to HTTP/1.1 with keep-alive.
	_ = http2.ConfigureTransport(t)
	return t
}

// NewClient creates a new Client authenticated with the given API key.
// Options are applied in order after defaults.
//
// An empty apiKey falls back to the MIOSA_API_KEY environment variable, and
// the base URL falls back to MIOSA_BASE_URL when no WithBaseURL option is
// given. MIOSA_ACCESS_TOKEN and MIOSA_TENANT likewise set the access token
// and tenant header. Explicit arguments and options always win.
//
// The default *http.Client uses an HTTP/2-capable transport with a
// keep-alive pool. The same client (and therefore the same connection
// pool) is reused across every request the SDK makes — never construct
// a new Client per call.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout:   defaultTimeout,
			Transport: newDefaultTransport(),
		},
		maxRetries: defaultMaxRetries,
	}
	if c.apiKey == "" {
		c.apiKey = os.Getenv("MIOSA_API_KEY")
	}
	if v := os.Getenv("MIOSA_BASE_URL"); v != "" {
		c.baseURL = v
	}
	if v := os.Getenv("MIOSA_ACCESS_TOKEN"); v != "" {
		c.accessToken = v
	}
	if v := os.Getenv("MIOSA_TENANT"); v != "" {
		c.tenant = v
	}
	for _, o := range opts {
		o(c)
	}
	c.wire()
	return c
}

// wire points every service at c. It runs once per client, including the
// derived clients returned by With, so a derived client never calls through the
// client it was derived from.
func (c *Client) wire() {
	c.Computers = &ComputersService{client: c}
	c.Sandboxes = &SandboxesService{client: c, Batches: &SandboxBatchesService{client: c}}
	c.Bulk = &BulkService{client: c}
	c.SharedLinks = &SharedLinksService{client: c}
	c.Domains = &DomainsService{client: c}
	c.SandboxSpend = &SandboxSpendService{client: c}
	c.Devices = &DevicesService{client: c}
	c.Deployments = &DeploymentsService{client: c}
	c.Files = &FilesService{client: c}
	c.Credits = &CreditsService{client: c}
	c.Admin = &AdminService{client: c}
	c.Workspaces = &WorkspacesService{client: c}
	c.OpenComputers = newOpenComputersService(c)
	c.Databases = &DatabasesService{client: c}
	c.Storage = &StorageService{client: c}
	c.Volumes = &VolumesService{client: c}
	c.CustomDomains = &FlatCustomDomainsService{client: c}
	c.Functions = &FunctionsService{client: c}
	c.CronJobs = &CronJobsService{client: c}
	c.HealthChecks = &HealthChecksService{client: c}
	c.Webhooks = &WebhooksService{client: c}
	c.Templates = &TemplatesService{client: c}
	c.SandboxTemplates = &SandboxTemplatesService{client: c}
	c.ApiKeys = &ApiKeysService{client: c}
	c.DockerDeploy = &DockerDeployService{client: c}
	c.Tenant = &TenantService{client: c}
	c.Regions = &RegionsService{client: c}
	c.Settings = &SettingsService{client: c}
	c.Dashboard = &DashboardService{client: c}
	c.Analytics = &AnalyticsService{client: c}
	c.AuditLog = &AuditLogService{client: c}
	c.Usage = &UsageService{client: c}
	c.Channels = &ChannelsService{client: c}
	c.Integrations = &IntegrationsService{client: c}
	c.ProjectIntegrations = &ProjectIntegrationsService{client: c}
	c.ProjectAuth = &ProjectAuthService{client: c}
	c.ExternalKeys = &ExternalKeysService{client: c}
	c.Mcp = &McpService{client: c}
	c.Runs = &RunsService{client: c}
	// P3/P4 services.
	c.Models = &ModelsService{client: c}
	c.Completions = &CompletionsService{client: c}
	c.Embeddings = &EmbeddingsService{client: c}
	c.ProviderDefaults = &ProviderDefaultsService{client: c}
	c.Benchmarks = &BenchmarksService{client: c}
	c.CommandCenter = &CommandCenterService{client: c}
	c.Community = &CommunityService{client: c}
	c.Email = newEmailService(c)
	c.BuilderSessions = &BuilderSessionsService{client: c}
	c.SnapshotsStandalone = &SnapshotsStandaloneService{client: c}
	c.WorkspaceMembers = &WorkspaceMembersService{client: c}
	c.WorkspaceInvites = &WorkspaceInvitesService{client: c}
	c.OrgInvites = &OrgInvitesService{client: c}
	c.Secrets = &EgressSecretsService{client: c}
	c.Network = &EgressNetworkService{client: c}
	c.Audit = &EgressAuditService{client: c}
	c.Quotas = &QuotasService{client: c}
	c.Forge = &ForgeService{client: c}
	c.Environments = &EnvironmentsService{client: c}
	c.Snapshots = &AccountSnapshotsService{client: c}
	c.NamedSnapshots = &NamedSnapshotsService{client: c}
	c.BillTo = &BillToService{client: c}
	c.MemberCaps = &MemberCapsService{client: c}
	c.Agents = &AgentsService{client: c}
	c.AgentAccounts = &AgentAccountsService{client: c}
	c.Connections = &ConnectionsService{client: c}
	c.AIGateway = &AIGatewayService{client: c}
	c.ScopedTokens = &ScopedTokensService{client: c}
	c.Policies = &PoliciesService{client: c}
	c.ServiceAccounts = &ServiceAccountsService{client: c}
	c.Events = &EventSubscriptionsService{client: c}
}

// With returns a client that shares this one's HTTP transport and connection
// pool but applies extra options. Use it to act as a different end user,
// organization or bill-to without building a second pool:
//
//	userClient := admin.With(miosa.WithAccessToken(scopedToken))
func (c *Client) With(opts ...ClientOption) *Client {
	cp := *c
	if c.defaultHeaders != nil {
		cp.defaultHeaders = make(map[string]string, len(c.defaultHeaders))
		for k, v := range c.defaultHeaders {
			cp.defaultHeaders[k] = v
		}
	}
	for _, o := range opts {
		o(&cp)
	}
	cp.wire()
	return &cp
}

// AsUser returns a client that authenticates with an end user's scoped token
// (see ScopedTokensService.Mint) instead of this client's API key. It shares
// the transport. Every call it makes is limited to that token's workspace and
// scopes.
func (c *Client) AsUser(token string) *Client {
	return c.With(WithAccessToken(token))
}

// ForTenant returns a client that acts in the given organization
// (X-MIOSA-Tenant). The credential must belong to it.
func (c *Client) ForTenant(tenantID string) *Client {
	return c.With(WithTenant(tenantID))
}

// withMinTimeout returns a shallow copy of the client whose HTTP timeout is at
// least d. It shares the connection pool. Used by calls that legitimately hold
// the request open (a create that waits for readiness).
func (c *Client) withMinTimeout(d time.Duration) *Client {
	if c.httpClient.Timeout == 0 || c.httpClient.Timeout >= d {
		return c
	}
	hc := *c.httpClient
	hc.Timeout = d
	cp := *c
	cp.httpClient = &hc
	return &cp
}

// ─── Core HTTP helpers ────────────────────────────────────────────────────────

// do executes an HTTP request with retry logic for retryable errors.
// The response body is the caller's responsibility to close.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	return c.doWithHeaders(ctx, method, path, body, nil)
}

// getJSON issues a GET request and JSON-decodes the response into out.
func (c *Client) getJSON(ctx context.Context, path string, out interface{}) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

// postJSON issues a POST request with a JSON body and decodes the response into out.
// out may be nil when the caller does not need the response body.
func (c *Client) postJSON(ctx context.Context, path string, in, out interface{}) error {
	return c.sendJSON(ctx, http.MethodPost, path, in, out)
}

// deleteJSON issues a DELETE request.
func (c *Client) deleteJSON(ctx context.Context, path string, out interface{}) error {
	return c.sendJSON(ctx, http.MethodDelete, path, nil, out)
}

// patchJSON issues a PATCH request with a JSON body and decodes the response into out.
func (c *Client) patchJSON(ctx context.Context, path string, in, out interface{}) error {
	return c.sendJSON(ctx, http.MethodPatch, path, in, out)
}

// putJSON issues a PUT request with a JSON body and decodes the response into out.
func (c *Client) putJSON(ctx context.Context, path string, in, out interface{}) error {
	return c.sendJSON(ctx, http.MethodPut, path, in, out)
}

// sendJSON is the common implementation for postJSON/deleteJSON.
func (c *Client) sendJSON(ctx context.Context, method, path string, in, out interface{}) error {
	return c.sendJSONWithHeaders(ctx, method, path, in, out, nil)
}

// sendJSONWithHeaders is sendJSON with additional request headers.
// Used by idempotent mutations to forward Idempotency-Key.
func (c *Client) sendJSONWithHeaders(ctx context.Context, method, path string, in, out interface{}, headers map[string]string) error {
	var bodyReader io.ReadSeeker
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}
	resp, err := c.doWithHeaders(ctx, method, path, bodyReader, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil || resp.ContentLength == 0 {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// postJSONIdempotent posts JSON with an Idempotency-Key header. Empty key
// is treated as absent.
func (c *Client) postJSONIdempotent(ctx context.Context, path string, in, out interface{}, key string) error {
	headers := map[string]string{}
	if key != "" {
		headers["Idempotency-Key"] = key
	}
	return c.sendJSONWithHeaders(ctx, http.MethodPost, path, in, out, headers)
}

// doWithHeaders is do() with caller-supplied request headers. Reuses retry
// logic.
func (c *Client) doWithHeaders(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoff(attempt)
			if hint, ok := RetryAfter(lastErr); ok {
				// The server said when to come back. A hint longer than the
				// retry ceiling means retrying now would only fail again.
				if hint > maxRetryHint {
					return nil, lastErr
				}
				delay = hint
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		var bodyReader io.Reader
		if body != nil {
			if seeker, ok := body.(io.ReadSeeker); ok {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return nil, fmt.Errorf("failed to rewind request body: %w", err)
				}
				bodyReader = seeker
			} else {
				bodyReader = body
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("failed to build request: %w", err)
		}
		c.setAuthHeaders(ctx, req.Header)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = &ConnectionError{Cause: err}
			if !retrySafe(method, headers) {
				// The request may have reached the server; repeating a
				// non-idempotent call could do it twice.
				return nil, lastErr
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := errorFromResponse(resp)
			if shouldRetry(method, headers, apiErr) && attempt < c.maxRetries {
				lastErr = apiErr
				continue
			}
			return nil, apiErr
		}
		return resp, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("request failed after %d attempts", c.maxRetries+1)
}

// getRaw issues a GET request and returns the raw response body bytes.
func (c *Client) getRaw(ctx context.Context, path string) ([]byte, string, error) {
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response body: %w", err)
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// postMultipart issues a POST with a prebuilt multipart body.
func (c *Client) postMultipart(ctx context.Context, path string, body io.ReadSeeker, contentType string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	c.setAuthHeaders(ctx, req.Header)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &ConnectionError{Cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errorFromResponse(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ─── Query string helpers ─────────────────────────────────────────────────────

// buildQuery converts a map to a URL-encoded query string including "?".
// Returns "" if the map is empty.
func buildQuery(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	q := url.Values{}
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// ─── Retry helpers ────────────────────────────────────────────────────────────

// backoff returns the wait duration before attempt n (1-indexed).
// Strategy: capped exponential backoff with full jitter.
func backoff(attempt int) time.Duration {
	cap := 30 * time.Second
	base := 500 * time.Millisecond
	exp := time.Duration(math.Pow(2, float64(attempt-1))) * base
	if exp > cap {
		exp = cap
	}
	// Full jitter: [0, exp)
	jitter := time.Duration(rand.Int63n(int64(exp) + 1))
	return jitter
}

// ─── Credits service ──────────────────────────────────────────────────────────

// CreditsService provides access to credit-related API endpoints.
type CreditsService struct {
	client *Client
}

// Balance returns the current credit balance for the authenticated tenant.
func (s *CreditsService) Balance(ctx context.Context) (*CreditBalance, error) {
	var out CreditBalance
	if err := s.client.getJSON(ctx, "/credits/balance", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Usage returns credit consumption for the current billing period.
func (s *CreditsService) Usage(ctx context.Context) (*CreditUsage, error) {
	var out CreditUsage
	if err := s.client.getJSON(ctx, "/credits/usage", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Transactions returns a paginated list of credit transactions.
func (s *CreditsService) Transactions(ctx context.Context, page, perPage int) (*CreditTransactionListResponse, error) {
	params := map[string]string{}
	if page > 0 {
		params["page"] = strconv.Itoa(page)
	}
	if perPage > 0 {
		params["per_page"] = strconv.Itoa(perPage)
	}
	var out CreditTransactionListResponse
	if err := s.client.getJSON(ctx, "/credits/transactions"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
