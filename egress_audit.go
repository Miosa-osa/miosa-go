package miosa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// ─── Egress audit log — paginated query + live tail ──────────────────────────
//
// Backed by:
//
//	GET    /api/v1/egress/audit
//	GET    /api/v1/egress/audit/:id
//
// Tenant-wide tail (Client.Audit.Tail) long-polls the REST endpoint. The
// resource-scoped tail (sandbox/computer) opens a WebSocket to
//
//	/api/v1/egress/audit/resource/:resource_id
//
// with subprotocol "miosa-egress-audit-v1" and a ?token=<jwt> query param.

// EgressAuditEvent is one audit event.
type EgressAuditEvent struct {
	ID                  string                 `json:"id"`
	TenantID            string                 `json:"tenant_id,omitempty"`
	ResourceID          string                 `json:"resource_id,omitempty"`
	ResourceType        string                 `json:"resource_type,omitempty"`
	Action              string                 `json:"action,omitempty"`
	Host                string                 `json:"host,omitempty"`
	Method              string                 `json:"method,omitempty"`
	Path                string                 `json:"path,omitempty"`
	StatusCode          int                    `json:"status_code,omitempty"`
	Effect              string                 `json:"effect,omitempty"`
	PolicyID            string                 `json:"policy_id,omitempty"`
	RuleID              string                 `json:"rule_id,omitempty"`
	ExternalUserID      string                 `json:"external_user_id,omitempty"`
	ExternalWorkspaceID string                 `json:"external_workspace_id,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	InsertedAt          string                 `json:"inserted_at,omitempty"`
	Timestamp           string                 `json:"timestamp,omitempty"`
	Raw                 json.RawMessage        `json:"-"`
}

// EgressAuditListInput is the filter for List/Tail.
type EgressAuditListInput struct {
	ResourceID          string
	ResourceType        string
	Host                string
	Action              string
	Since               string
	Until               string
	Limit               int
	Cursor              string
	ExternalUserID      string
	ExternalWorkspaceID string
}

type auditEnvelope struct {
	Data   *EgressAuditEvent  `json:"data,omitempty"`
	Event  *EgressAuditEvent  `json:"event,omitempty"`
	Events []EgressAuditEvent `json:"events,omitempty"`
	Audit  []EgressAuditEvent `json:"audit,omitempty"`
	Items  []EgressAuditEvent `json:"items,omitempty"`
}

// EgressAuditService is the tenant-wide audit log. Accessed via Client.Audit.
type EgressAuditService struct {
	client *Client
}

// List returns audit events with optional filters.
func (s *EgressAuditService) List(ctx context.Context, input EgressAuditListInput) ([]EgressAuditEvent, error) {
	q := buildAuditQuery(input)
	var env auditEnvelope
	if err := s.client.getJSON(ctx, "/egress/audit"+q, &env); err != nil {
		return nil, err
	}
	return auditListFrom(env), nil
}

// Get fetches a single audit event by id.
func (s *EgressAuditService) Get(ctx context.Context, id string) (*EgressAuditEvent, error) {
	var env auditEnvelope
	if err := s.client.getJSON(ctx, "/egress/audit/"+id, &env); err != nil {
		return nil, err
	}
	return auditFrom(env), nil
}

// TailOptions tunes Tail.
type TailOptions struct {
	// PollInterval is the delay between REST polls when no live transport is
	// available. Defaults to 2 seconds.
	PollInterval time.Duration
	// BufferSize sets the channel buffer (default 32).
	BufferSize int
}

func (o TailOptions) pollInterval() time.Duration {
	if o.PollInterval <= 0 {
		return 2 * time.Second
	}
	return o.PollInterval
}

func (o TailOptions) bufferSize() int {
	if o.BufferSize <= 0 {
		return 32
	}
	return o.BufferSize
}

// Tail long-polls the audit endpoint and emits new events to the returned
// channel. The channel is closed when ctx is cancelled.
//
// Note: tenant-wide Tail uses REST polling because there is no tenant-wide
// WebSocket. Use SandboxAuditBinding.Tail or ComputerAuditBinding.Tail for
// the sub-second live stream.
func (s *EgressAuditService) Tail(ctx context.Context, input EgressAuditListInput, opts TailOptions) (<-chan EgressAuditEvent, error) {
	ch := make(chan EgressAuditEvent, opts.bufferSize())
	go func() {
		defer close(ch)
		seen := map[string]struct{}{}
		since := input.Since
		ticker := time.NewTicker(opts.pollInterval())
		defer ticker.Stop()
		for {
			poll := input
			poll.Since = since
			events, err := s.List(ctx, poll)
			if err == nil {
				for _, ev := range events {
					if ev.ID != "" {
						if _, ok := seen[ev.ID]; ok {
							continue
						}
						seen[ev.ID] = struct{}{}
					}
					select {
					case ch <- ev:
					case <-ctx.Done():
						return
					}
					ts := ev.InsertedAt
					if ts == "" {
						ts = ev.Timestamp
					}
					if ts != "" {
						since = ts
					}
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return ch, nil
}

// ─── Resource-scoped binding ─────────────────────────────────────────────────

// SandboxAuditBinding pre-scopes resource_id + resource_type and upgrades
// Tail() to a WebSocket for sub-second latency.
type SandboxAuditBinding struct {
	client       *Client
	resourceID   string
	resourceType string
	delegate     *EgressAuditService
}

func newSandboxAuditBinding(c *Client, resourceID, resourceType string) *SandboxAuditBinding {
	return &SandboxAuditBinding{
		client:       c,
		resourceID:   resourceID,
		resourceType: resourceType,
		delegate:     &EgressAuditService{client: c},
	}
}

// List returns events pre-filtered to this resource.
func (b *SandboxAuditBinding) List(ctx context.Context, input EgressAuditListInput) ([]EgressAuditEvent, error) {
	if input.ResourceID == "" {
		input.ResourceID = b.resourceID
	}
	if input.ResourceType == "" {
		input.ResourceType = b.resourceType
	}
	return b.delegate.List(ctx, input)
}

// Get fetches a single audit event by id.
func (b *SandboxAuditBinding) Get(ctx context.Context, id string) (*EgressAuditEvent, error) {
	return b.delegate.Get(ctx, id)
}

// AuditTail is the live audit stream returned by SandboxAuditBinding.Tail.
//
// Events arrive on Events(). Call Close() to terminate the stream — Events()
// will be closed shortly after. If the WebSocket disconnects, Err() reports
// the cause once Events() closes.
type AuditTail struct {
	events chan EgressAuditEvent
	cancel context.CancelFunc
	errMu  chan struct{}
	err    error
	closed chan struct{}
}

// Events returns the channel of incoming audit events.
func (t *AuditTail) Events() <-chan EgressAuditEvent { return t.events }

// Close terminates the underlying WebSocket connection. Idempotent.
func (t *AuditTail) Close() {
	t.cancel()
	<-t.closed
}

// Err returns the connection-terminating error, if any. Safe to call after
// Events() is closed; nil while the stream is healthy.
func (t *AuditTail) Err() error {
	select {
	case <-t.errMu:
		return t.err
	default:
		return nil
	}
}

// Tail opens a WebSocket to /egress/audit/resource/:resource_id with
// subprotocol "miosa-egress-audit-v1" and yields events on the returned
// AuditTail. Falls back to REST long-polling if the WebSocket handshake
// fails.
//
// The caller MUST invoke AuditTail.Close() to release the underlying
// connection.
func (b *SandboxAuditBinding) Tail(ctx context.Context, opts TailOptions) (*AuditTail, error) {
	tailCtx, cancel := context.WithCancel(ctx)
	bufferSize := opts.bufferSize()
	events := make(chan EgressAuditEvent, bufferSize)
	closed := make(chan struct{})
	errMu := make(chan struct{}, 1)
	tail := &AuditTail{
		events: events,
		cancel: cancel,
		closed: closed,
		errMu:  errMu,
	}

	wsURL := buildAuditWsURL(b.client.baseURL, b.resourceID)
	if wsURL == "" {
		// Couldn't synthesize ws URL → fall back to REST polling.
		go func() {
			defer close(closed)
			defer close(events)
			restCh, err := b.delegate.Tail(tailCtx, EgressAuditListInput{
				ResourceID:   b.resourceID,
				ResourceType: b.resourceType,
			}, opts)
			if err != nil {
				tail.setErr(err)
				return
			}
			for ev := range restCh {
				select {
				case events <- ev:
				case <-tailCtx.Done():
					return
				}
			}
		}()
		return tail, nil
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+b.client.apiKey)
	headers.Set("User-Agent", "miosa-go/"+sdkVersion)

	dialer := &websocket.Dialer{
		Subprotocols:     []string{auditWsSubprotocol},
		HandshakeTimeout: 30 * time.Second,
	}
	conn, _, err := dialer.DialContext(tailCtx, wsURL, headers)
	if err != nil {
		// WS handshake failed (404, 503, network) → fall back to REST polling.
		go func() {
			defer close(closed)
			defer close(events)
			restCh, restErr := b.delegate.Tail(tailCtx, EgressAuditListInput{
				ResourceID:   b.resourceID,
				ResourceType: b.resourceType,
			}, opts)
			if restErr != nil {
				tail.setErr(fmt.Errorf("ws dial failed and rest fallback failed: ws=%w rest=%v", err, restErr))
				return
			}
			for ev := range restCh {
				select {
				case events <- ev:
				case <-tailCtx.Done():
					return
				}
			}
		}()
		return tail, nil
	}

	go func() {
		defer close(closed)
		defer close(events)
		defer conn.Close()

		// Close the conn when context is cancelled.
		go func() {
			<-tailCtx.Done()
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client closed"),
				time.Now().Add(5*time.Second))
			_ = conn.Close()
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if tailCtx.Err() == nil {
					tail.setErr(err)
				}
				return
			}
			var ev EgressAuditEvent
			if err := json.Unmarshal(message, &ev); err != nil {
				// Backend may envelope as {"event": {...}} or {"data": {...}}.
				var env auditEnvelope
				if err2 := json.Unmarshal(message, &env); err2 == nil {
					if env.Event != nil {
						ev = *env.Event
					} else if env.Data != nil {
						ev = *env.Data
					} else {
						continue
					}
				} else {
					continue
				}
			}
			ev.Raw = append(ev.Raw[:0], message...)
			select {
			case events <- ev:
			case <-tailCtx.Done():
				return
			}
		}
	}()
	return tail, nil
}

func (t *AuditTail) setErr(err error) {
	t.err = err
	select {
	case t.errMu <- struct{}{}:
	default:
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

const auditWsSubprotocol = "miosa-egress-audit-v1"

func buildAuditQuery(in EgressAuditListInput) string {
	params := map[string]string{}
	if in.ResourceID != "" {
		params["resource_id"] = in.ResourceID
	}
	if in.ResourceType != "" {
		params["resource_type"] = in.ResourceType
	}
	if in.Host != "" {
		params["host"] = in.Host
	}
	if in.Action != "" {
		params["action"] = in.Action
	}
	if in.Since != "" {
		params["since"] = in.Since
	}
	if in.Until != "" {
		params["until"] = in.Until
	}
	if in.Limit > 0 {
		params["limit"] = strconv.Itoa(in.Limit)
	}
	if in.Cursor != "" {
		params["cursor"] = in.Cursor
	}
	if in.ExternalUserID != "" {
		params["external_user_id"] = in.ExternalUserID
	}
	if in.ExternalWorkspaceID != "" {
		params["external_workspace_id"] = in.ExternalWorkspaceID
	}
	return buildQuery(params)
}

func auditFrom(env auditEnvelope) *EgressAuditEvent {
	switch {
	case env.Data != nil:
		return env.Data
	case env.Event != nil:
		return env.Event
	}
	return &EgressAuditEvent{}
}

func auditListFrom(env auditEnvelope) []EgressAuditEvent {
	switch {
	case len(env.Events) > 0:
		return env.Events
	case len(env.Audit) > 0:
		return env.Audit
	case len(env.Items) > 0:
		return env.Items
	}
	return []EgressAuditEvent{}
}

// buildAuditWsURL converts the SDK base URL (http(s)://host/api/v1) into the
// WebSocket URL for the audit-tail endpoint with the given resource id. The
// API key is passed as a bearer token in the Authorization header; the
// ?token= query param is supported by the backend for browser clients and
// is not added here because the server-side normaliser accepts the header
// form too. Returns "" if the base URL is malformed.
func buildAuditWsURL(baseURL, resourceID string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return ""
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	u.Path += "egress/audit/resource/" + url.PathEscape(resourceID)
	return u.String()
}
