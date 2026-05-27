package miosa

import "context"

// DashboardService provides access to aggregated platform overview endpoints.
type DashboardService struct {
	client *Client
}

// DashboardSummary is the aggregated user dashboard payload.
type DashboardSummary struct {
	Computers  int                    `json:"computers"`
	Sandboxes  int                    `json:"sandboxes"`
	Credits    int                    `json:"credits"`
	ActiveJobs int                    `json:"active_jobs"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

// PlatformOverview is the public status/health overview payload.
type PlatformOverview struct {
	Status     string                 `json:"status"`
	Regions    int                    `json:"regions"`
	Incidents  int                    `json:"incidents"`
	UpdatedAt  string                 `json:"updated_at"`
	Components map[string]interface{} `json:"components,omitempty"`
}

// Summary returns the aggregated user dashboard payload.
func (s *DashboardService) Summary(ctx context.Context) (*DashboardSummary, error) {
	var out DashboardSummary
	if err := s.client.getJSON(ctx, "/dashboard", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Overview returns the public status/health overview.
func (s *DashboardService) Overview(ctx context.Context) (*PlatformOverview, error) {
	var out PlatformOverview
	if err := s.client.getJSON(ctx, "/overview", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
