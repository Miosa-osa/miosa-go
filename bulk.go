package miosa

import (
	"context"
	"errors"
	"net/url"
)

// BulkFilter selects sandboxes for a bulk action without listing ids.
type BulkFilter struct {
	State       string `json:"state,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// BulkJob is an accepted or running bulk job. Accepted answers carry JobID and
// Queued; status answers carry the rest.
type BulkJob struct {
	JobID      string           `json:"job_id"`
	Queued     int              `json:"queued,omitempty"`
	Action     string           `json:"action,omitempty"`
	State      string           `json:"state,omitempty"`
	Status     string           `json:"status,omitempty"`
	Total      int              `json:"total,omitempty"`
	Completed  int              `json:"completed,omitempty"`
	Failed     int              `json:"failed,omitempty"`
	Errors     []map[string]any `json:"errors,omitempty"`
	StartedAt  string           `json:"started_at,omitempty"`
	FinishedAt string           `json:"finished_at,omitempty"`
}

// BulkService is reached as client.Bulk.
type BulkService struct{ client *Client }

// SandboxAction runs "pause", "resume" or "destroy" on the given ids.
func (s *BulkService) SandboxAction(ctx context.Context, action string, ids []string) (*BulkJob, error) {
	return s.sandboxAction(ctx, action, map[string]any{"ids": ids})
}

// SandboxActionWhere runs the action on every sandbox the filter matches.
func (s *BulkService) SandboxActionWhere(ctx context.Context, action string, f BulkFilter) (*BulkJob, error) {
	return s.sandboxAction(ctx, action, map[string]any{"filter": f})
}

func (s *BulkService) sandboxAction(ctx context.Context, action string, body map[string]any) (*BulkJob, error) {
	if action == "" {
		return nil, errors.New("miosa: bulk action is required")
	}
	var out BulkJob
	if err := s.client.postJSON(ctx, "/bulk/sandboxes/"+url.PathEscape(action), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ApplyPolicy applies a policy document to the given sandboxes.
func (s *BulkService) ApplyPolicy(ctx context.Context, ids []string, policy map[string]any, workspaceID string) (*BulkJob, error) {
	body := map[string]any{"ids": ids, "policy": policy}
	if workspaceID != "" {
		body["workspace_id"] = workspaceID
	}
	var out BulkJob
	if err := s.client.postJSON(ctx, "/bulk/policy/apply", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Job returns the status of a bulk job.
func (s *BulkService) Job(ctx context.Context, jobID string) (*BulkJob, error) {
	var out BulkJob
	if err := s.client.getJSON(ctx, "/bulk/jobs/"+url.PathEscape(jobID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
