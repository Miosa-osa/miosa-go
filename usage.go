package miosa

import "context"

// UsageService provides access to per-session metering, period summaries, and
// usage reports.
type UsageService struct {
	client *Client
}

// UsageSummary is the current period usage summary.
type UsageSummary struct {
	PeriodStart  string  `json:"period_start"`
	PeriodEnd    string  `json:"period_end"`
	ComputeHours float64 `json:"compute_hours"`
	StorageGB    float64 `json:"storage_gb"`
	BandwidthGB  float64 `json:"bandwidth_gb"`
	AITokens     int64   `json:"ai_tokens"`
	TotalCredits int64   `json:"total_credits"`
}

// UsageSession is a single metering session entry.
type UsageSession struct {
	ID          string  `json:"id"`
	ComputerID  string  `json:"computer_id"`
	StartedAt   string  `json:"started_at"`
	EndedAt     string  `json:"ended_at"`
	DurationSec int64   `json:"duration_sec"`
	CreditCost  float64 `json:"credit_cost"`
}

// UsageSessionListResponse wraps GET /usage/sessions.
type UsageSessionListResponse struct {
	Data []UsageSession `json:"data"`
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
}

// UsageReportInput holds optional query parameters for the usage report.
type UsageReportInput struct {
	PeriodStart string
	PeriodEnd   string
}

// UsageSessionsInput holds optional query parameters for GET /usage/sessions.
type UsageSessionsInput struct {
	ComputerID string
	Since      string
	Until      string
}

// Current returns the current period usage summary.
func (s *UsageService) Current(ctx context.Context) (*UsageSummary, error) {
	var out UsageSummary
	if err := s.client.getJSON(ctx, "/usage/summary", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Sessions lists per-session metering events with optional filters.
func (s *UsageService) Sessions(ctx context.Context, input UsageSessionsInput) (*UsageSessionListResponse, error) {
	params := map[string]string{}
	if input.ComputerID != "" {
		params["computer_id"] = input.ComputerID
	}
	if input.Since != "" {
		params["since"] = input.Since
	}
	if input.Until != "" {
		params["until"] = input.Until
	}
	var out UsageSessionListResponse
	if err := s.client.getJSON(ctx, "/usage/sessions"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Report returns a usage report for a period.
func (s *UsageService) Report(ctx context.Context, input UsageReportInput) (*UsageSummary, error) {
	params := map[string]string{}
	if input.PeriodStart != "" {
		params["period_start"] = input.PeriodStart
	}
	if input.PeriodEnd != "" {
		params["period_end"] = input.PeriodEnd
	}
	var out UsageSummary
	if err := s.client.getJSON(ctx, "/usage/summary"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
