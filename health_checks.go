package miosa

import "context"

// HealthChecksService provides CRUD for uptime monitors.
type HealthChecksService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// HealthCheckData is the API representation of a health check.
type HealthCheckData struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	TenantID       string `json:"tenant_id"`
	URL            string `json:"url"`
	Method         string `json:"method,omitempty"`
	IntervalSec    int    `json:"interval_sec,omitempty"`
	TimeoutSec     int    `json:"timeout_sec,omitempty"`
	ExpectedStatus int    `json:"expected_status,omitempty"`
	Status         string `json:"status"`
	LastCheckedAt  string `json:"last_checked_at,omitempty"`
	LastStatusCode int    `json:"last_status_code,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// HealthCheckListResponse wraps GET /health-checks.
type HealthCheckListResponse struct {
	Data []HealthCheckData `json:"data"`
}

// CreateHealthCheckInput is the request body for POST /health-checks.
type CreateHealthCheckInput struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Method         string `json:"method,omitempty"`
	IntervalSec    int    `json:"interval_sec,omitempty"`
	TimeoutSec     int    `json:"timeout_sec,omitempty"`
	ExpectedStatus int    `json:"expected_status,omitempty"`
	IdempotencyKey string `json:"-"`
}

// UpdateHealthCheckInput is the request body for PATCH /health-checks/:id.
type UpdateHealthCheckInput struct {
	Name           string `json:"name,omitempty"`
	URL            string `json:"url,omitempty"`
	Method         string `json:"method,omitempty"`
	IntervalSec    int    `json:"interval_sec,omitempty"`
	TimeoutSec     int    `json:"timeout_sec,omitempty"`
	ExpectedStatus int    `json:"expected_status,omitempty"`
}

// ListHealthChecksInput holds optional query parameters for GET /health-checks.
type ListHealthChecksInput struct {
	Status string
	Limit  int
	Cursor string
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// List returns health checks for the authenticated tenant.
func (s *HealthChecksService) List(ctx context.Context, input ListHealthChecksInput) (*HealthCheckListResponse, error) {
	params := map[string]string{}
	if input.Status != "" {
		params["status"] = input.Status
	}
	if input.Cursor != "" {
		params["cursor"] = input.Cursor
	}
	var out HealthCheckListResponse
	if err := s.client.getJSON(ctx, "/health-checks"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single health check by ID.
func (s *HealthChecksService) Get(ctx context.Context, id string) (*HealthCheckData, error) {
	var env apiResponse[HealthCheckData]
	if err := s.client.getJSON(ctx, "/health-checks/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Create provisions a new health check.
func (s *HealthChecksService) Create(ctx context.Context, input CreateHealthCheckInput) (*HealthCheckData, error) {
	var env apiResponse[HealthCheckData]
	if err := s.client.postJSONIdempotent(ctx, "/health-checks", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Update patches an existing health check.
func (s *HealthChecksService) Update(ctx context.Context, id string, input UpdateHealthCheckInput) (*HealthCheckData, error) {
	var env apiResponse[HealthCheckData]
	if err := s.client.patchJSON(ctx, "/health-checks/"+id, input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete removes a health check.
func (s *HealthChecksService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/health-checks/"+id, nil)
}
