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
	"strconv"
	"time"

	"golang.org/x/net/http2"
)

const (
	defaultBaseURL    = "https://api.miosa.ai/api/v1"
	defaultTimeout    = 60 * time.Second
	defaultMaxRetries = 3
	sdkVersion        = "0.3.0"
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

	// Services — populated by NewClient.
	Computers           *ComputersService
	Sandboxes           *SandboxesService
	Deployments         *DeploymentsService
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
	SandboxTemplates    *SandboxTemplatesService
	ApiKeys             *ApiKeysService
	Tenant              *TenantService
	Regions             *RegionsService
	Settings            *SettingsService
	Dashboard           *DashboardService
	Analytics           *AnalyticsService
	AuditLog            *AuditLogService
	Usage               *UsageService
	Channels            *ChannelsService
	Integrations        *IntegrationsService
	ProjectIntegrations *ProjectIntegrationsService
	ProjectAuth         *ProjectAuthService
	ExternalKeys        *ExternalKeysService
	Mcp                 *McpService
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
	for _, o := range opts {
		o(c)
	}
	c.Computers = &ComputersService{client: c}
	c.Sandboxes = &SandboxesService{client: c}
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
	c.SandboxTemplates = &SandboxTemplatesService{client: c}
	c.ApiKeys = &ApiKeysService{client: c}
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
	return c
}

// ─── Core HTTP helpers ────────────────────────────────────────────────────────

// do executes an HTTP request with retry logic for retryable errors.
// The response body is the caller's responsibility to close.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		// Buffer the body so it can be re-read on retry.
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
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("User-Agent", "miosa-go/"+sdkVersion)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = &ConnectionError{Cause: err}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := errorFromResponse(resp)
			if isRetryable(apiErr) && attempt < c.maxRetries {
				lastErr = apiErr
				continue
			}
			return nil, apiErr
		}
		return resp, nil
	}
	return nil, lastErr
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
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("User-Agent", "miosa-go/"+sdkVersion)
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
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := errorFromResponse(resp)
			if isRetryable(apiErr) && attempt < c.maxRetries {
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
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "miosa-go/"+sdkVersion)
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
