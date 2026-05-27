package miosa

import "context"

// ProviderDefaultsService reads and writes fleet-wide LLM provider routing
// configuration. Routes live under /api/v1/admin/provider-defaults and
// /api/v1/admin/tenants/:id/provider-config. Requires admin credentials.
type ProviderDefaultsService struct {
	client *Client
}

// ProviderDefaultsData is the raw map returned by the provider-defaults endpoint.
type ProviderDefaultsData map[string]interface{}

// List returns the current fleet-wide provider defaults.
func (s *ProviderDefaultsService) List(ctx context.Context) (ProviderDefaultsData, error) {
	var out ProviderDefaultsData
	if err := s.client.getJSON(ctx, "/admin/provider-defaults", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns the defaults entry for a single provider. Returns an empty map
// when the provider is not present.
func (s *ProviderDefaultsService) Get(ctx context.Context, provider string) (ProviderDefaultsData, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	if providers, ok := all["providers"]; ok {
		if pm, ok := providers.(map[string]interface{}); ok {
			if v, ok := pm[provider]; ok {
				if m, ok := v.(map[string]interface{}); ok {
					return ProviderDefaultsData(m), nil
				}
			}
		}
	}
	if v, ok := all[provider]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return ProviderDefaultsData(m), nil
		}
	}
	return ProviderDefaultsData{}, nil
}

// Update replaces the fleet-wide provider defaults (PUT).
func (s *ProviderDefaultsService) Update(ctx context.Context, fields map[string]interface{}) (ProviderDefaultsData, error) {
	var out ProviderDefaultsData
	if err := s.client.putJSON(ctx, "/admin/provider-defaults", fields, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTenant returns provider config overrides for a specific tenant.
func (s *ProviderDefaultsService) GetTenant(ctx context.Context, tenantID string) (ProviderDefaultsData, error) {
	var out ProviderDefaultsData
	if err := s.client.getJSON(ctx, "/admin/tenants/"+tenantID+"/provider-config", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetTenant replaces provider config overrides for a specific tenant (PUT).
func (s *ProviderDefaultsService) SetTenant(ctx context.Context, tenantID string, fields map[string]interface{}) (ProviderDefaultsData, error) {
	var out ProviderDefaultsData
	if err := s.client.putJSON(ctx, "/admin/tenants/"+tenantID+"/provider-config", fields, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ResetTenant removes per-tenant provider config overrides (DELETE).
func (s *ProviderDefaultsService) ResetTenant(ctx context.Context, tenantID string) error {
	return s.client.deleteJSON(ctx, "/admin/tenants/"+tenantID+"/provider-config", nil)
}
