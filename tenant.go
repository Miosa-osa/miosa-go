package miosa

import "context"

// TenantService provides access to the current tenant's plan and usage.
type TenantService struct {
	client *Client
}

// TenantPlan is the API representation of a tenant's plan, limits, and live
// usage counters.
type TenantPlan struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Plan      string            `json:"plan"`
	Status    string            `json:"status"`
	Limits    map[string]int64  `json:"limits"`
	Usage     map[string]int64  `json:"usage"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

// Current returns the current tenant's plan, limits, and live usage counters.
func (s *TenantService) Current(ctx context.Context) (*TenantPlan, error) {
	var out TenantPlan
	if err := s.client.getJSON(ctx, "/tenant/plan", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
