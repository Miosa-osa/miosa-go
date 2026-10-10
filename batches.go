package miosa

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"
)

// Wire contract: POST /sandboxes/batch, GET /sandboxes/batches/:id,
// GET /sandboxes/batches/:id/items, POST /sandboxes/batches/:id/cancel.

// BatchCreateInput creates many sandboxes in one request. Count is required;
// give TemplateID or Image.
type BatchCreateInput struct {
	Count          int               `json:"count"`
	Concurrency    int               `json:"concurrency,omitempty"`
	TemplateID     string            `json:"template_id,omitempty"`
	Image          string            `json:"image,omitempty"`
	Size           string            `json:"size,omitempty"`
	SetupFile      string            `json:"setup_file,omitempty"`
	Environment    string            `json:"environment,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
	Region         string            `json:"region,omitempty"`
	AlwaysOn       *bool             `json:"always_on,omitempty"`
	TimeoutSec     int               `json:"timeout_sec,omitempty"`
	IdleTimeoutSec int               `json:"idle_timeout_sec,omitempty"`
	ProjectID      string            `json:"project_id,omitempty"`
	WorkspaceID    string            `json:"workspace_id,omitempty"`
	NamePrefix     string            `json:"name_prefix,omitempty"`
	ReadinessProbe map[string]any    `json:"readiness_probe,omitempty"`
}

// BatchAccepted is the 202 answer to a batch create.
type BatchAccepted struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	RequestedCount   int    `json:"requested_count"`
	ConcurrencyLimit int    `json:"concurrency_limit"`
	TemplateID       string `json:"template_id,omitempty"`
	InsertedAt       string `json:"inserted_at,omitempty"`
}

// BatchStatus is a point-in-time snapshot of a batch.
type BatchStatus struct {
	ID                  string           `json:"id"`
	Status              string           `json:"status"`
	Requested           int              `json:"requested"`
	Dispatched          int              `json:"dispatched"`
	Pending             int              `json:"pending"`
	Succeeded           int              `json:"succeeded"`
	DispatchFailed      int              `json:"dispatch_failed"`
	Cancelled           int              `json:"cancelled"`
	LiveCount           int              `json:"live_count"`
	BootErrorCount      int              `json:"boot_error_count"`
	DestroyedCount      int              `json:"destroyed_count"`
	FailedTotal         int              `json:"failed_total"`
	CreatesPerSecondAvg float64          `json:"creates_per_second_avg"`
	Settled             bool             `json:"settled?"`
	FailureSamples      []map[string]any `json:"failure_samples,omitempty"`
}

// BatchItem is one sandbox of a batch.
type BatchItem struct {
	ID             string `json:"id"`
	BatchItemIndex int    `json:"batch_item_index"`
	State          string `json:"state"`
	Ready          bool   `json:"ready"`
	Name           string `json:"name,omitempty"`
}

// SandboxBatchesService is reached as client.Sandboxes.Batches.
type SandboxBatchesService struct{ client *Client }

// Create queues a batch. The call returns at once; poll Get or use Wait.
func (s *SandboxBatchesService) Create(ctx context.Context, in BatchCreateInput) (*BatchAccepted, error) {
	if in.Count <= 0 {
		return nil, errors.New("miosa: batch count must be positive")
	}
	var out apiResponse[BatchAccepted]
	if err := s.client.postJSON(ctx, "/sandboxes/batch", in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Get returns the batch snapshot.
func (s *SandboxBatchesService) Get(ctx context.Context, id string) (*BatchStatus, error) {
	var out apiResponse[BatchStatus]
	if err := s.client.getJSON(ctx, "/sandboxes/batches/"+url.PathEscape(id), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Items lists the batch's sandboxes; limit 0 uses the server default (100).
func (s *SandboxBatchesService) Items(ctx context.Context, id string, limit int) ([]BatchItem, error) {
	q := map[string]string{}
	if limit > 0 {
		q["limit"] = strconv.Itoa(limit)
	}
	var out apiResponse[[]BatchItem]
	if err := s.client.getJSON(ctx, "/sandboxes/batches/"+url.PathEscape(id)+"/items"+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Cancel stops dispatching. A finished batch answers 409 BATCH_ALREADY_TERMINAL.
func (s *SandboxBatchesService) Cancel(ctx context.Context, id string) (*BatchStatus, error) {
	var out apiResponse[BatchStatus]
	if err := s.client.postJSON(ctx, "/sandboxes/batches/"+url.PathEscape(id)+"/cancel", nil, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Wait polls until the batch settles or ctx ends. interval 0 means 2 s.
func (s *SandboxBatchesService) Wait(ctx context.Context, id string, interval time.Duration) (*BatchStatus, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	for {
		st, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if st.Settled {
			return st, nil
		}
		select {
		case <-ctx.Done():
			return st, ctx.Err()
		case <-time.After(interval):
		}
	}
}
