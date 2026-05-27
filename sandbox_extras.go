package miosa

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ─── Sandbox sub-services (Batch 3) ─────────────────────────────────────────
// These types attach to the *Computer handle when the sandbox template is in
// use, or may be accessed directly via SandboxesService.GetHandle if a thin
// sandbox-specific handle is preferred. They mirror the Python
// sandbox_namespaces.py surface.

// SandboxTerminalService manages PTY sessions on a sandbox.
type SandboxTerminalService struct {
	client    *Client
	sandboxID string
}

// SandboxCreateTerminalInput configures a new sandbox PTY session.
type SandboxCreateTerminalInput struct {
	Cols  int               `json:"cols,omitempty"`
	Rows  int               `json:"rows,omitempty"`
	Shell string            `json:"shell,omitempty"`
	Cwd   string            `json:"cwd,omitempty"`
	Env   map[string]string `json:"env,omitempty"`
}

// Create opens a new PTY session.
func (s *SandboxTerminalService) Create(ctx context.Context, input SandboxCreateTerminalInput) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, fmt.Sprintf("/sandboxes/%s/terminal", s.sandboxID), input, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete closes a PTY session by session ID.
func (s *SandboxTerminalService) Delete(ctx context.Context, sessionID string) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("/sandboxes/%s/terminal/%s", s.sandboxID, sessionID), nil)
}

// ─── SandboxEventsService ────────────────────────────────────────────────────

// SandboxEventsService streams live SSE events from a sandbox.
type SandboxEventsService struct {
	client    *Client
	sandboxID string
}

// Stream opens an SSE connection and returns a channel of events. The channel
// is closed when the stream ends or ctx is cancelled.
func (s *SandboxEventsService) Stream(ctx context.Context) (<-chan SSEEvent, error) {
	return s.client.streamSSE(ctx, http.MethodGet,
		fmt.Sprintf("/sandboxes/%s/events", s.sandboxID),
		nil,
	)
}

// ─── SandboxPreviewsService ──────────────────────────────────────────────────

// SandboxPreviewsService provides full CRUD on sandbox preview URLs plus share
// token management.
type SandboxPreviewsService struct {
	client    *Client
	sandboxID string
}

func (s *SandboxPreviewsService) base() string {
	return fmt.Sprintf("/sandboxes/%s/previews", s.sandboxID)
}

// List returns all preview records for the sandbox.
func (s *SandboxPreviewsService) List(ctx context.Context) ([]map[string]interface{}, error) {
	var wrapper struct {
		Data     []map[string]interface{} `json:"data"`
		Previews []map[string]interface{} `json:"previews"`
		Items    []map[string]interface{} `json:"items"`
	}
	if err := s.client.getJSON(ctx, s.base(), &wrapper); err != nil {
		var list []map[string]interface{}
		if err2 := s.client.getJSON(ctx, s.base(), &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	for _, v := range [][]map[string]interface{}{wrapper.Data, wrapper.Previews, wrapper.Items} {
		if len(v) > 0 {
			return v, nil
		}
	}
	return []map[string]interface{}{}, nil
}

// Create creates a preview for the given port with optional extra options.
func (s *SandboxPreviewsService) Create(ctx context.Context, port int, opts map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{"port": port}
	for k, v := range opts {
		body[k] = v
	}
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, s.base(), body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single preview by ID.
func (s *SandboxPreviewsService) Get(ctx context.Context, previewID string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, s.base()+"/"+previewID, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete removes a preview by ID.
func (s *SandboxPreviewsService) Delete(ctx context.Context, previewID string) error {
	return s.client.deleteJSON(ctx, s.base()+"/"+previewID, nil)
}

// Share mints a share token for previewID, valid for expiresInSec seconds.
func (s *SandboxPreviewsService) Share(ctx context.Context, previewID string, expiresInSec int) (map[string]interface{}, error) {
	if expiresInSec <= 0 {
		expiresInSec = 3600
	}
	var out map[string]interface{}
	if err := s.client.postJSON(ctx,
		s.base()+"/"+previewID+"/share",
		map[string]int{"expires_in_sec": expiresInSec},
		&out,
	); err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeShare invalidates all share tokens for previewID.
func (s *SandboxPreviewsService) RevokeShare(ctx context.Context, previewID string) error {
	return s.client.deleteJSON(ctx, s.base()+"/"+previewID+"/share", nil)
}

// ─── SandboxEnvService ───────────────────────────────────────────────────────

// SandboxEnvService exposes the read-only sandbox env listing.
// (The backend has no per-name CRUD for sandbox env — use CreateSandboxInput.Env
// or template build-spec env to set values at create time.)
type SandboxEnvService struct {
	client    *Client
	sandboxID string
}

// List returns the current env var map for the sandbox.
func (s *SandboxEnvService) List(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, fmt.Sprintf("/sandboxes/%s/env", s.sandboxID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── SandboxTagsService ──────────────────────────────────────────────────────

// SandboxTagsService replaces the full sandbox tag list.
type SandboxTagsService struct {
	client    *Client
	sandboxID string
}

// Set replaces the full tag list (PATCH /sandboxes/:id/tags).
func (s *SandboxTagsService) Set(ctx context.Context, tags []string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.patchJSON(ctx,
		fmt.Sprintf("/sandboxes/%s/tags", s.sandboxID),
		map[string][]string{"tags": tags},
		&out,
	); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── SandboxHandle ───────────────────────────────────────────────────────────

// SandboxHandle is a thin bound handle to a specific sandbox that exposes
// Batch-3 sub-services. It is obtained from SandboxesService.GetHandle and
// is intentionally separate from the existing Computer-backed sandbox path.
type SandboxHandle struct {
	ID       string
	client   *Client
	Terminal *SandboxTerminalService
	Events   *SandboxEventsService
	Previews *SandboxPreviewsService
	Env      *SandboxEnvService
	Tags     *SandboxTagsService
	// Egress — secrets, network allowlist/policy, and audit log pre-scoped
	// to this sandbox (resource_type="sandbox").
	Secrets *SandboxSecretsBinding
	Network *SandboxNetworkBinding
	Audit   *SandboxAuditBinding
}

// newSandboxHandle wires sub-services for a sandbox ID.
func newSandboxHandle(c *Client, id string) *SandboxHandle {
	return &SandboxHandle{
		ID:       id,
		client:   c,
		Terminal: &SandboxTerminalService{client: c, sandboxID: id},
		Events:   &SandboxEventsService{client: c, sandboxID: id},
		Previews: &SandboxPreviewsService{client: c, sandboxID: id},
		Env:      &SandboxEnvService{client: c, sandboxID: id},
		Tags:     &SandboxTagsService{client: c, sandboxID: id},
		Secrets:  newSandboxSecretsBinding(c, id, "sandbox"),
		Network:  newSandboxNetworkBinding(c, id, "sandbox"),
		Audit:    newSandboxAuditBinding(c, id, "sandbox"),
	}
}

// Readiness returns the sandbox readiness-probe state.
func (h *SandboxHandle) Readiness(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := h.client.getJSON(ctx, fmt.Sprintf("/sandboxes/%s/readiness", h.ID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// WaitUntilReadyOptions tunes WaitUntilReady.
type WaitUntilReadyOptions struct {
	// Timeout bounds the total wait. Defaults to 30 s.
	Timeout time.Duration
	// Stream selects the SSE fast-path. When nil it defaults to true so
	// callers automatically benefit from the server-push endpoint. Set
	// to a *false pointer to force the polling fallback.
	Stream *bool
}

func (o WaitUntilReadyOptions) streamEnabled() bool {
	if o.Stream == nil {
		return true
	}
	return *o.Stream
}

func (o WaitUntilReadyOptions) timeout() time.Duration {
	if o.Timeout <= 0 {
		return 30 * time.Second
	}
	return o.Timeout
}

// WaitUntilReady blocks until the sandbox reports ready, or opts.Timeout
// elapses. Returns (true, nil) once the sandbox is ready, (false, nil) on
// timeout — including the server-side "event: timeout" SSE frame — and a
// non-nil error only for transport failures the SDK cannot recover from.
//
// When opts.Stream is true (the default) this opens an SSE connection to
// GET /sandboxes/:id/readiness/stream. The server emits "event: ready"
// immediately if already ready, otherwise as soon as the readiness PubSub
// message fires. If the SSE endpoint returns 404 (server pre-dates the
// streaming endpoint) the call transparently falls back to polling
// Readiness every 10 ms.
func (s *SandboxesService) WaitUntilReady(
	ctx context.Context,
	id string,
	opts WaitUntilReadyOptions,
) (bool, error) {
	timeout := opts.timeout()
	streamCtx, cancel := context.WithTimeout(ctx, timeout+5*time.Second)
	defer cancel()

	if opts.streamEnabled() {
		ready, used := tryReadinessStream(streamCtx, s.client, id)
		if used {
			return ready, nil
		}
	}

	// Polling fallback — fixed 10 ms tick, no exponential backoff.
	pollCtx, pollCancel := context.WithTimeout(ctx, timeout)
	defer pollCancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	handle := s.GetHandle(id)
	for {
		data, err := handle.Readiness(pollCtx)
		if err == nil {
			if isReadyMap(data) {
				return true, nil
			}
		}
		select {
		case <-pollCtx.Done():
			return false, nil
		case <-ticker.C:
		}
	}
}

// isReadyMap returns true if the readiness payload (either the raw API
// envelope or its unwrapped “data“ member) signals readiness.
func isReadyMap(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	candidates := []map[string]interface{}{payload}
	if inner, ok := payload["data"].(map[string]interface{}); ok {
		candidates = append(candidates, inner)
	}
	for _, m := range candidates {
		if ready, ok := m["ready"].(bool); ok && ready {
			return true
		}
		if status, ok := m["status"].(string); ok && status == "ready" {
			return true
		}
	}
	return false
}

// tryReadinessStream opens the SSE endpoint and returns:
//   - (true, true)  → server emitted "event: ready"
//   - (false, true) → server emitted "event: timeout"
//   - (_,    false) → endpoint unavailable (404) or transport error; caller
//     should fall through to polling.
func tryReadinessStream(ctx context.Context, c *Client, id string) (bool, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+fmt.Sprintf("/sandboxes/%s/readiness/stream", id), nil)
	if err != nil {
		return false, false
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "miosa-go/"+sdkVersion)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, false
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, false
	}

	scanner := bufio.NewScanner(resp.Body)
	// readiness frames are tiny; bump the buffer to 64 KiB just in case.
	scanner.Buffer(make([]byte, 0, 4096), 64*1024)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: ready"), strings.HasPrefix(line, "event:ready"):
			return true, true
		case strings.HasPrefix(line, "event: timeout"), strings.HasPrefix(line, "event:timeout"):
			return false, true
		}
		if err := ctx.Err(); err != nil {
			return false, true
		}
	}
	// stream closed without a terminal event — defer to caller.
	return false, false
}

// GetHandle returns a SandboxHandle for an existing sandbox by ID, providing
// direct access to Batch-3 sub-services without re-fetching the full sandbox.
func (s *SandboxesService) GetHandle(id string) *SandboxHandle {
	return newSandboxHandle(s.client, id)
}
