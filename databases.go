package miosa

import (
	"context"
	"strconv"
)

// DatabasesService provides CRUD and lifecycle operations for managed Postgres databases.
type DatabasesService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// DatabaseData is the API representation of a managed database.
type DatabaseData struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TenantID  string `json:"tenant_id"`
	Engine    string `json:"engine"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// DatabaseListResponse wraps GET /databases.
type DatabaseListResponse struct {
	Data []DatabaseData `json:"data"`
}

// CreateDatabaseInput is the request body for POST /databases.
type CreateDatabaseInput struct {
	Name           string `json:"name"`
	Engine         string `json:"engine,omitempty"`
	Version        string `json:"version,omitempty"`
	SizeGB         int    `json:"size_gb,omitempty"`
	IdempotencyKey string `json:"-"`
}

// ListDatabasesInput holds optional query parameters for GET /databases.
type ListDatabasesInput struct {
	Status string
	Limit  int
	Cursor string
}

// DatabaseCredentials holds the connection details for a database.
type DatabaseCredentials struct {
	URL      string `json:"url"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// DatabaseLogsInput holds optional query parameters for GET /databases/:id/logs.
type DatabaseLogsInput struct {
	Lines int
	Since string
}

// DatabaseLogsResponse wraps the database logs response.
type DatabaseLogsResponse struct {
	Lines []string `json:"lines"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// List returns managed databases for the authenticated tenant.
func (s *DatabasesService) List(ctx context.Context, input ListDatabasesInput) (*DatabaseListResponse, error) {
	params := map[string]string{}
	if input.Status != "" {
		params["status"] = input.Status
	}
	if input.Limit > 0 {
		params["limit"] = strconv.Itoa(input.Limit)
	}
	if input.Cursor != "" {
		params["cursor"] = input.Cursor
	}
	var out DatabaseListResponse
	if err := s.client.getJSON(ctx, "/databases"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single database by ID.
func (s *DatabasesService) Get(ctx context.Context, id string) (*DatabaseData, error) {
	var env apiResponse[DatabaseData]
	if err := s.client.getJSON(ctx, "/databases/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Create provisions a new managed database.
func (s *DatabasesService) Create(ctx context.Context, input CreateDatabaseInput) (*DatabaseData, error) {
	var env apiResponse[DatabaseData]
	if err := s.client.postJSONIdempotent(ctx, "/databases", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete removes a database.
func (s *DatabasesService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/databases/"+id, nil)
}

// Start brings a stopped database online.
func (s *DatabasesService) Start(ctx context.Context, id string) (*DatabaseData, error) {
	var env apiResponse[DatabaseData]
	if err := s.client.postJSON(ctx, "/databases/"+id+"/start", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Stop shuts down a running database.
func (s *DatabasesService) Stop(ctx context.Context, id string) (*DatabaseData, error) {
	var env apiResponse[DatabaseData]
	if err := s.client.postJSON(ctx, "/databases/"+id+"/stop", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Restart bounces a database.
func (s *DatabasesService) Restart(ctx context.Context, id string) (*DatabaseData, error) {
	var env apiResponse[DatabaseData]
	if err := s.client.postJSON(ctx, "/databases/"+id+"/restart", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Credentials returns the connection URL and credentials for a database.
func (s *DatabasesService) Credentials(ctx context.Context, id string) (*DatabaseCredentials, error) {
	var env apiResponse[DatabaseCredentials]
	if err := s.client.getJSON(ctx, "/databases/"+id+"/credentials", &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Logs returns recent log lines for a database.
func (s *DatabasesService) Logs(ctx context.Context, id string, input DatabaseLogsInput) (*DatabaseLogsResponse, error) {
	params := map[string]string{}
	if input.Lines > 0 {
		params["lines"] = strconv.Itoa(input.Lines)
	}
	if input.Since != "" {
		params["since"] = input.Since
	}
	var out DatabaseLogsResponse
	if err := s.client.getJSON(ctx, "/databases/"+id+"/logs"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
