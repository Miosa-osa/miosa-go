package miosa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Wire contract: miosa-compute docs/api/snapshots.md.
//
// The per-machine checkpoint calls stay where they were (Computer.Snapshots,
// SandboxesService.CreateSnapshot and friends). These two services are the
// account-wide view: history across machines, a file browser, fork, safe
// delete, and named snapshots.

// SnapshotResource describes the machine a snapshot belongs to. It is nil on
// a group whose machine row no longer exists.
type SnapshotResource struct {
	// Type is "sandbox" or "computer".
	Type        string `json:"type"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// AccountSnapshot is one snapshot in the account-wide history.
type AccountSnapshot struct {
	ID       string            `json:"id"`
	Resource *SnapshotResource `json:"resource,omitempty"`
	// Status is creating, uploading, ready, restoring, failed or quarantined.
	Status      string `json:"status"`
	Comment     string `json:"comment,omitempty"`
	SizeBytes   int64  `json:"size_bytes"`
	DiskBytes   int64  `json:"disk_bytes"`
	MemoryBytes int64  `json:"memory_bytes"`
	Keep        bool   `json:"keep"`
	// NamedSnapshot is the pinning name, or nil. History lists never contain a
	// pinned snapshot, but a direct read can.
	NamedSnapshot interface{} `json:"named_snapshot"`
	// Browsable is true when the snapshot is ready and stored its disk image.
	Browsable        bool   `json:"browsable"`
	ParentSnapshotID string `json:"parent_snapshot_id,omitempty"`
	Error            string `json:"error,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	LastUsedAt       string `json:"last_used_at,omitempty"`
}

// SnapshotDependent is something that still needs a snapshot's bytes. Type is
// named_snapshot, child_snapshot, computer_backup, suspended_computer,
// restore_in_progress or snapshot_in_progress.
type SnapshotDependent struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
}

// SnapshotDetail is one snapshot and its dependents.
type SnapshotDetail struct {
	Snapshot   AccountSnapshot     `json:"data"`
	Dependents []SnapshotDependent `json:"dependents"`
}

// SnapshotGroup is the history of one machine.
type SnapshotGroup struct {
	ResourceID    string            `json:"resource_id"`
	Resource      *SnapshotResource `json:"resource"`
	SnapshotCount int               `json:"snapshot_count"`
	StoredBytes   int64             `json:"stored_bytes"`
	LatestAt      string            `json:"latest_at,omitempty"`
}

// ListSnapshotsOptions narrows the history list and groups.
type ListSnapshotsOptions struct {
	// ResourceID limits to one machine.
	ResourceID string
	// WorkspaceID limits to machines in a workspace.
	WorkspaceID string
	// Query is a substring of machine name, machine id or snapshot id.
	Query string
	// Limit defaults to 50 on the server, at most 200.
	Limit  int
	Offset int
}

func (o ListSnapshotsOptions) query() map[string]string {
	q := map[string]string{
		"resource_id":  o.ResourceID,
		"workspace_id": o.WorkspaceID,
		"q":            o.Query,
	}
	if o.Limit > 0 {
		q["limit"] = strconv.Itoa(o.Limit)
	}
	if o.Offset > 0 {
		q["offset"] = strconv.Itoa(o.Offset)
	}
	return q
}

// SnapshotTreeEntry is one entry of a snapshot folder listing.
type SnapshotTreeEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Type is directory, file or symlink.
	Type string `json:"type"`
	Size int64  `json:"size"`
	Mode string `json:"mode,omitempty"`
}

// SnapshotTree is a listing of one folder of a snapshot's disk image.
type SnapshotTree struct {
	Path    string              `json:"path"`
	Entries []SnapshotTreeEntry `json:"entries"`
}

// BrowseOptions tunes the file browser calls.
type BrowseOptions struct {
	// WaitTimeout bounds the wait while the snapshot's disk image is being
	// unpacked (the API answers 202 "warming" and asks to be polled).
	// Defaults to 2 minutes. Negative means do not wait: a warming snapshot
	// returns *SnapshotWarmingError.
	WaitTimeout time.Duration
}

// SnapshotWarmingError is returned by the file browser when the snapshot's
// disk image is still being unpacked and waiting was disabled or timed out.
type SnapshotWarmingError struct {
	// RetryAfter is the server's suggested delay before asking again.
	RetryAfter time.Duration
}

func (e *SnapshotWarmingError) Error() string {
	return fmt.Sprintf("miosa: snapshot is still unpacking, retry in %s", e.RetryAfter)
}

// SnapshotForkResult is the response of a snapshot fork or named-snapshot deploy.
type SnapshotForkResult struct {
	// MachineType is "sandbox" or "computer".
	MachineType string `json:"machine_type"`
	ID          string `json:"id"`
	// State is provisioning while the restore runs in the background.
	State string `json:"state"`
	// Machine is the full machine JSON when the API sent more than the above.
	Machine json.RawMessage `json:"-"`
	// Snapshot is the source snapshot, when the API returned it.
	Snapshot *AccountSnapshot `json:"-"`
	// NamedSnapshot is the name that was deployed, for a deploy.
	NamedSnapshot string `json:"-"`
}

// ForkSnapshotInput is the optional body of a snapshot fork or named-snapshot
// deploy. A fork's secrets come only from its own environment.
type ForkSnapshotInput struct {
	MachineEnvironmentOptions
	Name string `json:"name,omitempty"`
	// IdempotencyKey is sent as the Idempotency-Key header.
	IdempotencyKey string `json:"-"`
}

// DeleteSnapshotsInput selects snapshots for DeleteMany: either IDs, or all of
// one machine's history (ResourceID with All and Confirm equal to ResourceID).
type DeleteSnapshotsInput struct {
	IDs        []string `json:"ids,omitempty"`
	ResourceID string   `json:"resource_id,omitempty"`
	All        bool     `json:"all,omitempty"`
	// Confirm must equal ResourceID, so deleting a whole history is always a
	// fully spelled request.
	Confirm string `json:"confirm,omitempty"`
}

// BlockedSnapshot is a snapshot a bulk delete refused, with why.
type BlockedSnapshot struct {
	SnapshotID string              `json:"snapshot_id"`
	Dependents []SnapshotDependent `json:"dependents"`
}

// DeleteSnapshotsResult reports each snapshot of a bulk delete on its own.
type DeleteSnapshotsResult struct {
	Deleted  []string          `json:"deleted"`
	Blocked  []BlockedSnapshot `json:"blocked"`
	NotFound []string          `json:"not_found"`
}

// AccountSnapshotsService is the account-wide snapshot history and browser.
// Accessed via Client.Snapshots. Reads need sandboxes:read, writes
// sandboxes:write.
type AccountSnapshotsService struct {
	client *Client
}

// List returns the flat history, newest first. Named snapshots are never part
// of history.
func (s *AccountSnapshotsService) List(ctx context.Context, opts ListSnapshotsOptions) ([]AccountSnapshot, error) {
	var out apiResponse[[]AccountSnapshot]
	if err := s.client.getJSON(ctx, "/snapshots"+buildQuery(opts.query()), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Groups returns history grouped by machine, newest activity first.
func (s *AccountSnapshotsService) Groups(ctx context.Context, opts ListSnapshotsOptions) ([]SnapshotGroup, error) {
	q := opts.query()
	delete(q, "resource_id")
	var out apiResponse[[]SnapshotGroup]
	if err := s.client.getJSON(ctx, "/snapshots/groups"+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Get returns one snapshot and what depends on it.
func (s *AccountSnapshotsService) Get(ctx context.Context, id string) (*SnapshotDetail, error) {
	var out SnapshotDetail
	if err := s.client.getJSON(ctx, "/snapshots/"+url.PathEscape(id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete deletes one snapshot if nothing depends on it. When something does,
// the error is a *SnapshotInUseError listing the dependents and nothing changed.
func (s *AccountSnapshotsService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/snapshots/"+url.PathEscape(id), nil)
}

// DeleteMany deletes several snapshots, or all of one machine's history. Each
// snapshot is judged on its own and a blocked one never stops the others. When
// every snapshot asked for was blocked the API answers 409; the per-snapshot
// result is still returned (with a nil error) so callers read Blocked.
func (s *AccountSnapshotsService) DeleteMany(ctx context.Context, in DeleteSnapshotsInput) (*DeleteSnapshotsResult, error) {
	var out apiResponse[DeleteSnapshotsResult]
	err := s.client.postJSON(ctx, "/snapshots/delete", in, &out)
	if err != nil {
		if m := AsMiosaError(err); m != nil && m.StatusCode == http.StatusConflict {
			var env apiResponse[DeleteSnapshotsResult]
			if json.Unmarshal(m.Body, &env) == nil && (len(env.Data.Blocked) > 0 || len(env.Data.Deleted) > 0) {
				return &env.Data, nil
			}
		}
		return nil, err
	}
	return &out.Data, nil
}

// Fork creates a new, independent machine of the same kind from the snapshot.
// The new machine starts in provisioning and the restore runs in the background.
func (s *AccountSnapshotsService) Fork(ctx context.Context, id string, in ForkSnapshotInput) (*SnapshotForkResult, error) {
	return forkSnapshot(ctx, s.client, "/snapshots/"+url.PathEscape(id)+"/fork", in)
}

func forkSnapshot(ctx context.Context, c *Client, path string, in ForkSnapshotInput) (*SnapshotForkResult, error) {
	var raw struct {
		Data          json.RawMessage  `json:"data"`
		Snapshot      *AccountSnapshot `json:"snapshot"`
		NamedSnapshot string           `json:"named_snapshot"`
	}
	headers := map[string]string{}
	if in.IdempotencyKey != "" {
		headers["Idempotency-Key"] = in.IdempotencyKey
	}
	if _, _, err := c.sendJSONResponse(ctx, http.MethodPost, path, in, &raw, headers); err != nil {
		return nil, err
	}
	var out SnapshotForkResult
	if len(raw.Data) > 0 {
		if err := json.Unmarshal(raw.Data, &out); err != nil {
			return nil, err
		}
		out.Machine = raw.Data
	}
	out.Snapshot = raw.Snapshot
	out.NamedSnapshot = raw.NamedSnapshot
	return &out, nil
}

// Tree lists one folder of a snapshot's disk image (path defaults to "/"). It
// works while the machine is stopped or destroyed. The first request for a
// snapshot unpacks its image in the background; Tree waits for that, see
// BrowseOptions.
func (s *AccountSnapshotsService) Tree(ctx context.Context, id, path string, opts BrowseOptions) (*SnapshotTree, error) {
	p := "/snapshots/" + url.PathEscape(id) + "/tree" + buildQuery(map[string]string{"path": path})
	deadline := browseDeadline(opts)
	for {
		var out struct {
			Data         *SnapshotTree `json:"data"`
			Status       string        `json:"status"`
			RetryAfterMS int64         `json:"retry_after_ms"`
		}
		_, status, err := s.client.sendJSONResponse(ctx, http.MethodGet, p, nil, &out, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusAccepted && out.Data != nil {
			return out.Data, nil
		}
		if err := waitWarming(ctx, out.RetryAfterMS, deadline); err != nil {
			return nil, err
		}
	}
}

// Download streams one file of a snapshot, or a folder as a .tar archive, to w
// and returns the byte count. A file can be up to 2 GiB; a folder is refused
// with 413 TOO_LARGE past 1 GiB, 50,000 entries or 5,000 directories.
func (s *AccountSnapshotsService) Download(ctx context.Context, id, path string, w io.Writer, opts BrowseOptions) (int64, error) {
	p := "/snapshots/" + url.PathEscape(id) + "/files" + buildQuery(map[string]string{"path": path})
	deadline := browseDeadline(opts)
	for {
		resp, err := s.client.do(ctx, http.MethodGet, p, nil)
		if err != nil {
			return 0, err
		}
		if resp.StatusCode != http.StatusAccepted {
			defer resp.Body.Close()
			return io.Copy(w, resp.Body)
		}
		var warm struct {
			RetryAfterMS int64 `json:"retry_after_ms"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&warm)
		resp.Body.Close()
		if err := waitWarming(ctx, warm.RetryAfterMS, deadline); err != nil {
			return 0, err
		}
	}
}

// DownloadBytes is Download into memory.
func (s *AccountSnapshotsService) DownloadBytes(ctx context.Context, id, path string, opts BrowseOptions) ([]byte, error) {
	var buf byteSink
	if _, err := s.Download(ctx, id, path, &buf, opts); err != nil {
		return nil, err
	}
	return buf.b, nil
}

type byteSink struct{ b []byte }

func (s *byteSink) Write(p []byte) (int, error) { s.b = append(s.b, p...); return len(p), nil }

func browseDeadline(opts BrowseOptions) time.Time {
	switch {
	case opts.WaitTimeout < 0:
		return time.Time{}
	case opts.WaitTimeout == 0:
		return time.Now().Add(2 * time.Minute)
	default:
		return time.Now().Add(opts.WaitTimeout)
	}
}

// waitWarming sleeps the server's hint, or returns *SnapshotWarmingError when
// waiting is off or would pass the deadline.
func waitWarming(ctx context.Context, retryAfterMS int64, deadline time.Time) error {
	wait := time.Duration(retryAfterMS) * time.Millisecond
	if wait <= 0 {
		wait = 2 * time.Second
	}
	if deadline.IsZero() || time.Now().Add(wait).After(deadline) {
		return &SnapshotWarmingError{RetryAfter: wait}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

// ─── Named snapshots ──────────────────────────────────────────────────────────

// NamedSnapshotSource is where a named snapshot came from.
type NamedSnapshotSource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NamedSnapshot is a name that pins one snapshot until removed.
type NamedSnapshot struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Status is saving, ready or failed.
	Status      string               `json:"status"`
	SnapshotID  string               `json:"snapshot_id"`
	WorkspaceID string               `json:"workspace_id,omitempty"`
	Source      *NamedSnapshotSource `json:"source,omitempty"`
	// Promoted is true when an existing history snapshot was pinned, false when
	// the name owns a snapshot taken for it.
	Promoted  bool   `json:"promoted"`
	SizeBytes int64  `json:"size_bytes"`
	DiskBytes int64  `json:"disk_bytes"`
	Browsable bool   `json:"browsable"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// NamedSnapshotAllowance is the organization's named-snapshot quota: 10 free,
// each name above that priced monthly.
type NamedSnapshotAllowance struct {
	Included          int `json:"included"`
	Used              int `json:"used"`
	RemainingFree     int `json:"remaining_free"`
	Billable          int `json:"billable"`
	MonthlyPriceCents int `json:"monthly_price_cents"`
	MonthlyCostCents  int `json:"monthly_cost_cents"`
}

// NamedSnapshotList is the response of listing names.
type NamedSnapshotList struct {
	Data      []NamedSnapshot        `json:"data"`
	Allowance NamedSnapshotAllowance `json:"allowance"`
}

// NamedSnapshotDetail is one name and what depends on its snapshot.
type NamedSnapshotDetail struct {
	Snapshot   NamedSnapshot       `json:"data"`
	Dependents []SnapshotDependent `json:"dependents"`
}

// SaveNamedSnapshotInput names a snapshot. Give exactly one source.
type SaveNamedSnapshotInput struct {
	// Name is 1 to 63 letters, digits, dots, dashes or underscores, starting
	// with a letter or digit, unique per organization ignoring case.
	Name string `json:"name"`
	// SnapshotID pins that ready history snapshot.
	SnapshotID string `json:"snapshot_id,omitempty"`
	// SandboxID or ComputerID: a running machine takes a new snapshot for the
	// name; a stopped one pins its newest ready history snapshot.
	SandboxID  string `json:"sandbox_id,omitempty"`
	ComputerID string `json:"computer_id,omitempty"`
}

// SavedNamedSnapshot is the response of saving a name.
type SavedNamedSnapshot struct {
	NamedSnapshot
	// OperationID is set when a new snapshot is being captured for the name;
	// the name is "saving" until it finishes.
	OperationID string `json:"-"`
}

// NamedSnapshotOutcome is what happened to the snapshot when a name was removed.
type NamedSnapshotOutcome string

const (
	// NamedSnapshotReleased: the name pinned a history snapshot, now ordinary history again.
	NamedSnapshotReleased NamedSnapshotOutcome = "released"
	// NamedSnapshotDeleted: the name owned its snapshot, which is deleted.
	NamedSnapshotDeleted NamedSnapshotOutcome = "deleted"
	// NamedSnapshotKept: something still depends on the snapshot; it stays as history.
	NamedSnapshotKept NamedSnapshotOutcome = "kept"
)

// RemovedNamedSnapshot is the response of removing a name.
type RemovedNamedSnapshot struct {
	Name       string               `json:"name"`
	Removed    bool                 `json:"removed"`
	SnapshotID string               `json:"snapshot_id"`
	Outcome    NamedSnapshotOutcome `json:"outcome"`
}

// NamedSnapshotsService manages named snapshots: explicit, pinned restore
// points kept until removed. Accessed via Client.NamedSnapshots.
type NamedSnapshotsService struct {
	client *Client
}

// List returns every name of the organization, newest first, with the
// allowance. workspaceID narrows to one workspace when non-empty.
func (s *NamedSnapshotsService) List(ctx context.Context, workspaceID string) (*NamedSnapshotList, error) {
	var out NamedSnapshotList
	if err := s.client.getJSON(ctx, "/named-snapshots"+buildQuery(map[string]string{"workspace_id": workspaceID}), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one name and what depends on its snapshot.
func (s *NamedSnapshotsService) Get(ctx context.Context, name string) (*NamedSnapshotDetail, error) {
	var out NamedSnapshotDetail
	if err := s.client.getJSON(ctx, "/named-snapshots/"+url.PathEscape(name), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Save names a snapshot. Saving a name that exists does not replace it (409
// NAME_TAKEN): remove it first. Going past the free allowance needs credit
// (402 CREDIT_REQUIRED).
func (s *NamedSnapshotsService) Save(ctx context.Context, in SaveNamedSnapshotInput) (*SavedNamedSnapshot, error) {
	if in.Name == "" {
		return nil, errors.New("name is required")
	}
	var out struct {
		Data        NamedSnapshot `json:"data"`
		OperationID string        `json:"operation_id"`
	}
	if err := s.client.postJSON(ctx, "/named-snapshots", in, &out); err != nil {
		return nil, err
	}
	return &SavedNamedSnapshot{NamedSnapshot: out.Data, OperationID: out.OperationID}, nil
}

// Remove removes the name. Outcome says what happened to its snapshot.
func (s *NamedSnapshotsService) Remove(ctx context.Context, name string) (*RemovedNamedSnapshot, error) {
	var out apiResponse[RemovedNamedSnapshot]
	if err := s.client.deleteJSON(ctx, "/named-snapshots/"+url.PathEscape(name), &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Deploy forks the name into a new sandbox or computer of the same kind as its
// source. It keeps working after the source machine is gone. The source's
// environment variables, services and tags are not carried over. While the
// name is still saving the API answers 409 SNAPSHOT_NOT_READY (retryable).
func (s *NamedSnapshotsService) Deploy(ctx context.Context, name string, in ForkSnapshotInput) (*SnapshotForkResult, error) {
	return forkSnapshot(ctx, s.client, "/named-snapshots/"+url.PathEscape(name)+"/deploy", in)
}
