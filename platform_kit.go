package miosa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
)

// ─── Tenant branding (white label) ───────────────────────────────────────────

// TenantBranding is GET /settings/branding. It supersedes the narrower
// BrandingData, whose field names do not match the API.
type TenantBranding struct {
	CustomLogoURL       string `json:"custom_logo_url,omitempty"`
	CustomLogoLightURL  string `json:"custom_logo_light_url,omitempty"`
	CustomAppName       string `json:"custom_app_name,omitempty"`
	CustomFaviconURL    string `json:"custom_favicon_url,omitempty"`
	BrandColorPrimary   string `json:"brand_color_primary,omitempty"`
	BrandColorSecondary string `json:"brand_color_secondary,omitempty"`
	BrandColorAccent    string `json:"brand_color_accent,omitempty"`
	EmailSenderName     string `json:"email_sender_name,omitempty"`
	EmailSenderAddress  string `json:"email_sender_address,omitempty"`
	SupportEmail        string `json:"support_email,omitempty"`
	SupportURL          string `json:"support_url,omitempty"`
	PoweredByVisible    bool   `json:"powered_by_visible"`
	DesktopWallpaperURL string `json:"desktop_wallpaper_url,omitempty"`
	EmailHeroURL        string `json:"email_hero_url,omitempty"`
	BrandingSource      string `json:"branding_source,omitempty"`
	PreviewDomain       string `json:"preview_domain,omitempty"`
}

// BrandingUpdate changes branding; only the fields set are sent.
type BrandingUpdate struct {
	CustomLogoURL       *string `json:"custom_logo_url,omitempty"`
	CustomLogoLightURL  *string `json:"custom_logo_light_url,omitempty"`
	CustomAppName       *string `json:"custom_app_name,omitempty"`
	CustomFaviconURL    *string `json:"custom_favicon_url,omitempty"`
	BrandColorPrimary   *string `json:"brand_color_primary,omitempty"`
	BrandColorSecondary *string `json:"brand_color_secondary,omitempty"`
	BrandColorAccent    *string `json:"brand_color_accent,omitempty"`
	EmailSenderName     *string `json:"email_sender_name,omitempty"`
	EmailSenderAddress  *string `json:"email_sender_address,omitempty"`
	SupportEmail        *string `json:"support_email,omitempty"`
	SupportURL          *string `json:"support_url,omitempty"`
	PoweredByVisible    *bool   `json:"powered_by_visible,omitempty"`
}

// Branding returns the tenant's white-label branding.
func (s *SettingsService) Branding(ctx context.Context) (*TenantBranding, error) {
	var out TenantBranding
	if err := s.client.getJSON(ctx, "/settings/branding", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetBranding updates branding and returns the result.
func (s *SettingsService) SetBranding(ctx context.Context, in BrandingUpdate) (*TenantBranding, error) {
	var out TenantBranding
	if err := s.client.putJSON(ctx, "/settings/branding", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadBrandingAsset uploads an image for kind (for example "logo") as a
// multipart file and returns its public URL.
func (s *SettingsService) UploadBrandingAsset(ctx context.Context, kind, filename string, data []byte) (string, error) {
	if kind == "" || len(data) == 0 {
		return "", errors.New("miosa: branding asset kind and data are required")
	}
	if filename == "" {
		filename = kind
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	resp, err := s.client.doWithHeaders(ctx, http.MethodPost, "/settings/branding/assets/"+url.PathEscape(kind),
		bytes.NewReader(buf.Bytes()), map[string]string{"Content-Type": mw.FormDataContentType()})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.URL, nil
}

// BrandingDiscovery is the answer to a discover request.
type BrandingDiscovery struct {
	Status  string `json:"status"`
	Website string `json:"website"`
}

// DiscoverBranding queues a lookup of logo and colors from a website.
func (s *SettingsService) DiscoverBranding(ctx context.Context, website string) (*BrandingDiscovery, error) {
	var out BrandingDiscovery
	if err := s.client.postJSON(ctx, "/settings/branding/discover", map[string]string{"website": website}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Computer share links ────────────────────────────────────────────────────

// CreateComputerShareInput creates a share. Kind is "link" or "user"; a user share
// needs Email. Role is for example "viewer".
type CreateComputerShareInput struct {
	Kind           string `json:"kind"`
	Role           string `json:"role"`
	Email          string `json:"email,omitempty"`
	ExpiresInHours int    `json:"expires_in_hours,omitempty"`
}

// ComputerShare is one share of a computer.
type ComputerShare struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Role      string `json:"role"`
	Email     string `json:"email,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// ComputerShareCreated is the answer to CreateShare. ShareURL is shown once, for links.
type ComputerShareCreated struct {
	Data     ComputerShare `json:"data"`
	ShareURL string        `json:"share_url,omitempty"`
}

// CreateShare shares a computer by link or with a user.
func (s *ComputersService) CreateShare(ctx context.Context, id string, in CreateComputerShareInput) (*ComputerShareCreated, error) {
	var out ComputerShareCreated
	if err := s.client.postJSON(ctx, "/computers/"+url.PathEscape(id)+"/shares", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SharedComputer is what a share token resolves to.
type SharedComputer struct {
	Computer struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		TemplateType string `json:"template_type"`
		Size         string `json:"size"`
		Status       string `json:"status"`
	} `json:"computer"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expires_at"`
}

// SharedLinksService is reached as client.SharedLinks.
type SharedLinksService struct{ client *Client }

// Resolve looks up a share token (public route; 404 when unknown or expired).
func (s *SharedLinksService) Resolve(ctx context.Context, token string) (*SharedComputer, error) {
	var out SharedComputer
	if err := s.client.getJSON(ctx, "/shared/"+url.PathEscape(token), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Custom domains by hostname ──────────────────────────────────────────────

// Domain is a custom domain.
type Domain struct {
	ID                 string      `json:"id"`
	Hostname           string      `json:"hostname,omitempty"`
	FQDN               string      `json:"fqdn,omitempty"`
	Status             string      `json:"status"`
	Verified           bool        `json:"verified"`
	DNSRecords         []DNSRecord `json:"dns_records,omitempty"`
	DeploymentID       string      `json:"deployment_id,omitempty"`
	SandboxID          string      `json:"sandbox_id,omitempty"`
	ComputerID         string      `json:"computer_id,omitempty"`
	VerificationTarget string      `json:"verification_target,omitempty"`
	VerificationToken  string      `json:"verification_token,omitempty"`
	RedirectPolicy     string      `json:"redirect_policy,omitempty"`
}

// DNSRecord is a record the customer must create.
type DNSRecord struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CreateDomainInput registers a hostname, optionally pointing at a deployment.
type CreateDomainInput struct {
	Hostname     string `json:"hostname"`
	DeploymentID string `json:"deployment_id,omitempty"`
}

// AssignDomainInput points a hostname at a deployment.
type AssignDomainInput struct {
	DeploymentID   string `json:"deployment_id"`
	RedirectPolicy string `json:"redirect_policy,omitempty"`
}

// DomainsService is reached as client.Domains.
type DomainsService struct{ client *Client }

func (s *DomainsService) one(ctx context.Context, method, path string, in interface{}) (*Domain, error) {
	var out apiResponse[Domain]
	if err := s.client.sendJSON(ctx, method, path, in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Create registers a hostname.
func (s *DomainsService) Create(ctx context.Context, in CreateDomainInput) (*Domain, error) {
	if in.Hostname == "" {
		return nil, errors.New("miosa: hostname is required")
	}
	return s.one(ctx, http.MethodPost, "/domains", in)
}

// Get returns one domain by hostname.
func (s *DomainsService) Get(ctx context.Context, hostname string) (*Domain, error) {
	return s.one(ctx, http.MethodGet, "/domains/"+url.PathEscape(hostname), nil)
}

// Verify checks DNS and updates the status.
func (s *DomainsService) Verify(ctx context.Context, hostname string) (*Domain, error) {
	return s.one(ctx, http.MethodPost, "/domains/"+url.PathEscape(hostname)+"/verify", nil)
}

// Assign points the hostname at a deployment.
func (s *DomainsService) Assign(ctx context.Context, hostname string, in AssignDomainInput) (*Domain, error) {
	return s.one(ctx, http.MethodPost, "/domains/"+url.PathEscape(hostname)+"/assign", in)
}

// Delete removes the domain.
func (s *DomainsService) Delete(ctx context.Context, hostname string) error {
	return s.client.deleteJSON(ctx, "/domains/"+url.PathEscape(hostname), nil)
}

// ─── Audit log ───────────────────────────────────────────────────────────────

// AuditEntry is one audit-log record.
type AuditEntry struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	Actor       map[string]interface{} `json:"actor,omitempty"`
	Resource    map[string]interface{} `json:"resource,omitempty"`
	Timestamp   string                 `json:"ts"`
	Status      string                 `json:"status,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AuditLogPage is one page; pass NextCursor as After to continue.
type AuditLogPage struct {
	Data       []AuditEntry `json:"data"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

// AuditLogOptions filters the log. Category "security" needs an org admin.
type AuditLogOptions struct {
	After, Type, ActorID, ResourceType, ResourceID, Category string
	Limit                                                    int
}

// ListPage returns one page of the audit log in the cursor form the API
// sends (type, actor, resource, ts). List keeps the older page-number shape.
func (s *AuditLogService) ListPage(ctx context.Context, o AuditLogOptions) (*AuditLogPage, error) {
	q := map[string]string{
		"after": o.After, "type": o.Type, "actor_id": o.ActorID,
		"resource_type": o.ResourceType, "resource_id": o.ResourceID, "category": o.Category,
	}
	if o.Limit > 0 {
		q["limit"] = strconv.Itoa(o.Limit)
	}
	var out AuditLogPage
	if err := s.client.getJSON(ctx, "/audit-log"+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Run a command in a fresh sandbox ────────────────────────────────────────

// RunSandboxInput creates a sandbox, runs Command and returns the output.
type RunSandboxInput struct {
	Command    string `json:"command"`
	TemplateID string `json:"template_id,omitempty"`
	Size       string `json:"size,omitempty"`
	Image      string `json:"image,omitempty"`
}

// RunSandboxResult is the answer to Run.
type RunSandboxResult struct {
	Sandbox Sandbox           `json:"data"`
	Exec    SandboxExecResult `json:"exec"`
	Timings map[string]int64  `json:"timings,omitempty"`
}

// Run creates a sandbox, waits for it, runs the command and returns the result.
func (s *SandboxesService) Run(ctx context.Context, in RunSandboxInput) (*RunSandboxResult, error) {
	if in.Command == "" {
		return nil, errors.New("miosa: command is required")
	}
	var out RunSandboxResult
	if err := s.client.postJSON(ctx, "/sandboxes/run", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
