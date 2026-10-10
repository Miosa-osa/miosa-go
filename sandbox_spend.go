package miosa

import (
	"context"
	"net/url"
)

// SandboxSpend is a tenant's sandbox spend cap and usage for the period.
type SandboxSpend struct {
	PeriodStart     string `json:"period_start"`
	PeriodEnd       string `json:"period_end"`
	CapCents        *int64 `json:"cap_cents"`
	Mode            string `json:"mode"`
	AccountedCents  int64  `json:"accounted_cents"`
	RemainingCents  *int64 `json:"remaining_cents"`
	Status          string `json:"status"`
	AlertThresholds []int  `json:"alert_thresholds"`
}

// SandboxSpendUpdate changes the cap. Mode is "limited" or "unlimited".
type SandboxSpendUpdate struct {
	Mode            string `json:"mode,omitempty"`
	CapCents        *int64 `json:"cap_cents,omitempty"`
	AlertThresholds []int  `json:"alert_thresholds,omitempty"`
}

// SandboxSpendService is reached as client.SandboxSpend. Updates need an
// owner or admin; both routes need a user credential.
type SandboxSpendService struct{ client *Client }

// Get returns the cap and usage for tenantID.
func (s *SandboxSpendService) Get(ctx context.Context, tenantID string) (*SandboxSpend, error) {
	var out apiResponse[SandboxSpend]
	if err := s.client.getJSON(ctx, "/tenants/"+url.PathEscape(tenantID)+"/sandbox-spend", &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Update changes the cap and returns the new state.
func (s *SandboxSpendService) Update(ctx context.Context, tenantID string, in SandboxSpendUpdate) (*SandboxSpend, error) {
	var out apiResponse[SandboxSpend]
	if err := s.client.patchJSON(ctx, "/tenants/"+url.PathEscape(tenantID)+"/sandbox-spend", in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}
