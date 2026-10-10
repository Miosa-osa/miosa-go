package miosa

import (
	"context"
	"fmt"
	"net/url"
)

// ResumeWith resumes a paused sandbox, choosing its environment. Without an
// environment the sandbox keeps its current pin. Secrets the chosen
// environment withholds are not delivered.
func (s *SandboxesService) ResumeWith(ctx context.Context, id string, opts MachineEnvironmentOptions) (*Sandbox, error) {
	var response sandboxResponse
	if err := s.client.postJSON(ctx, "/sandboxes/"+url.PathEscape(id)+"/resume", opts, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// RestoreSnapshotWith is RestoreSnapshot choosing the restored sandbox's
// environment (with none named it inherits its source's).
func (s *SandboxesService) RestoreSnapshotWith(ctx context.Context, id, snapshotID string, opts MachineEnvironmentOptions) (*Sandbox, error) {
	var response sandboxResponse
	path := "/sandboxes/" + url.PathEscape(id) + "/restore/" + url.PathEscape(snapshotID)
	if err := s.client.postJSON(ctx, path, opts, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// StartWith powers on a stopped computer, choosing its environment. Without an
// environment the computer keeps its current pin (or the default when it has none).
func (c *Computer) StartWith(ctx context.Context, opts MachineEnvironmentOptions) error {
	return c.client.postJSON(ctx, fmt.Sprintf("/computers/%s/start", url.PathEscape(c.ID)), opts, nil)
}
