package miosa

import "context"

// CronJobsService provides CRUD, lifecycle control, and execution history for cron jobs.
type CronJobsService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// CronJobData is the API representation of a cron job.
type CronJobData struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TenantID  string `json:"tenant_id"`
	Schedule  string `json:"schedule"`
	Command   string `json:"command,omitempty"`
	Status    string `json:"status"`
	Paused    bool   `json:"paused"`
	LastRunAt string `json:"last_run_at,omitempty"`
	NextRunAt string `json:"next_run_at,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CronJobListResponse wraps GET /cron-jobs.
type CronJobListResponse struct {
	Data []CronJobData `json:"data"`
}

// CronJobExecutionData is the API representation of a single cron job run.
type CronJobExecutionData struct {
	ID         string `json:"id"`
	CronJobID  string `json:"cron_job_id"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

// CronJobExecutionListResponse wraps GET /cron-jobs/:id/executions.
type CronJobExecutionListResponse struct {
	Data []CronJobExecutionData `json:"data"`
}

// CreateCronJobInput is the request body for POST /cron-jobs.
type CreateCronJobInput struct {
	Name           string            `json:"name"`
	Schedule       string            `json:"schedule"`
	Command        string            `json:"command,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	IdempotencyKey string            `json:"-"`
}

// UpdateCronJobInput is the request body for PATCH /cron-jobs/:id.
type UpdateCronJobInput struct {
	Name     string            `json:"name,omitempty"`
	Schedule string            `json:"schedule,omitempty"`
	Command  string            `json:"command,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

// ListCronJobsInput holds optional query parameters for GET /cron-jobs.
type ListCronJobsInput struct {
	Status string
	Limit  int
	Cursor string
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// List returns cron jobs for the authenticated tenant.
func (s *CronJobsService) List(ctx context.Context, input ListCronJobsInput) (*CronJobListResponse, error) {
	params := map[string]string{}
	if input.Status != "" {
		params["status"] = input.Status
	}
	if input.Cursor != "" {
		params["cursor"] = input.Cursor
	}
	var out CronJobListResponse
	if err := s.client.getJSON(ctx, "/cron-jobs"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single cron job by ID.
func (s *CronJobsService) Get(ctx context.Context, id string) (*CronJobData, error) {
	var env apiResponse[CronJobData]
	if err := s.client.getJSON(ctx, "/cron-jobs/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Create provisions a new cron job. Schedule is a standard cron expression (e.g. "0 4 * * *").
func (s *CronJobsService) Create(ctx context.Context, input CreateCronJobInput) (*CronJobData, error) {
	var env apiResponse[CronJobData]
	if err := s.client.postJSONIdempotent(ctx, "/cron-jobs", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Update patches a cron job.
func (s *CronJobsService) Update(ctx context.Context, id string, input UpdateCronJobInput) (*CronJobData, error) {
	var env apiResponse[CronJobData]
	if err := s.client.patchJSON(ctx, "/cron-jobs/"+id, input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete removes a cron job.
func (s *CronJobsService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/cron-jobs/"+id, nil)
}

// Pause suspends scheduling of a cron job without deleting it.
func (s *CronJobsService) Pause(ctx context.Context, id string) (*CronJobData, error) {
	var env apiResponse[CronJobData]
	if err := s.client.postJSON(ctx, "/cron-jobs/"+id+"/pause", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Resume re-enables a paused cron job.
func (s *CronJobsService) Resume(ctx context.Context, id string) (*CronJobData, error) {
	var env apiResponse[CronJobData]
	if err := s.client.postJSON(ctx, "/cron-jobs/"+id+"/resume", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// RunNow triggers an immediate out-of-schedule execution.
func (s *CronJobsService) RunNow(ctx context.Context, id string) (*CronJobExecutionData, error) {
	var env apiResponse[CronJobExecutionData]
	if err := s.client.postJSON(ctx, "/cron-jobs/"+id+"/run-now", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// ListExecutions returns the execution history for a cron job.
func (s *CronJobsService) ListExecutions(ctx context.Context, id string) (*CronJobExecutionListResponse, error) {
	var out CronJobExecutionListResponse
	if err := s.client.getJSON(ctx, "/cron-jobs/"+id+"/executions", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetExecution fetches a single execution record.
func (s *CronJobsService) GetExecution(ctx context.Context, jobID, executionID string) (*CronJobExecutionData, error) {
	var env apiResponse[CronJobExecutionData]
	if err := s.client.getJSON(ctx, "/cron-jobs/"+jobID+"/executions/"+executionID, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
