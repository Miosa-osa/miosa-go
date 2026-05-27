package miosa

import "context"

// SettingsService provides access to tenant settings, branding, and BYOK
// provider keys.
type SettingsService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// TenantSettings is the API representation of tenant-level configuration.
type TenantSettings struct {
	ID        string                 `json:"id"`
	TenantID  string                 `json:"tenant_id"`
	Settings  map[string]interface{} `json:"settings"`
	UpdatedAt string                 `json:"updated_at"`
}

// UpdateSettingsInput is the request body for PUT /settings.
type UpdateSettingsInput struct {
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// BrandingData is the API representation of tenant branding configuration.
type BrandingData struct {
	LogoURL      string `json:"logo_url,omitempty"`
	FaviconURL   string `json:"favicon_url,omitempty"`
	PrimaryColor string `json:"primary_color,omitempty"`
	AppName      string `json:"app_name,omitempty"`
	CustomCSS    string `json:"custom_css,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

// ProviderKeyData is the API representation of a BYOK provider key entry.
type ProviderKeyData struct {
	Provider  string `json:"provider"`
	Masked    string `json:"masked"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ProviderKeyListResponse wraps GET /settings/provider-keys.
type ProviderKeyListResponse struct {
	Data []ProviderKeyData `json:"data"`
}

// UpsertProviderKeyInput is the request body for PUT /settings/provider-keys/:provider.
type UpsertProviderKeyInput struct {
	Key string `json:"key"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// Get returns the current tenant settings.
func (s *SettingsService) Get(ctx context.Context) (*TenantSettings, error) {
	var out TenantSettings
	if err := s.client.getJSON(ctx, "/settings", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update updates tenant settings.
func (s *SettingsService) Update(ctx context.Context, input UpdateSettingsInput) (*TenantSettings, error) {
	var out TenantSettings
	if err := s.client.putJSON(ctx, "/settings", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBranding returns tenant branding configuration.
func (s *SettingsService) GetBranding(ctx context.Context) (*BrandingData, error) {
	var out BrandingData
	if err := s.client.getJSON(ctx, "/settings/branding", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateBranding updates tenant branding.
func (s *SettingsService) UpdateBranding(ctx context.Context, input BrandingData) (*BrandingData, error) {
	var out BrandingData
	if err := s.client.putJSON(ctx, "/settings/branding", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ComputePricing returns tenant-scoped compute pricing.
func (s *SettingsService) ComputePricing(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/settings/compute-pricing", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GpuPricing returns tenant-scoped GPU pricing.
func (s *SettingsService) GpuPricing(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/settings/gpu-pricing", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AvailableModels lists AI models available to this tenant.
func (s *SettingsService) AvailableModels(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/settings/available-models", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Regions lists datacenter regions enabled for this tenant.
func (s *SettingsService) Regions(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/settings/regions", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListProviderKeys lists tenant-level BYOK provider keys.
func (s *SettingsService) ListProviderKeys(ctx context.Context) (*ProviderKeyListResponse, error) {
	var out ProviderKeyListResponse
	if err := s.client.getJSON(ctx, "/settings/provider-keys", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpsertProviderKey creates or updates a BYOK provider key.
func (s *SettingsService) UpsertProviderKey(ctx context.Context, provider string, input UpsertProviderKeyInput) (*ProviderKeyData, error) {
	var env apiResponse[ProviderKeyData]
	if err := s.client.putJSON(ctx, "/settings/provider-keys/"+provider, input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// DeleteProviderKey deletes a BYOK provider key.
func (s *SettingsService) DeleteProviderKey(ctx context.Context, provider string) error {
	return s.client.deleteJSON(ctx, "/settings/provider-keys/"+provider, nil)
}
