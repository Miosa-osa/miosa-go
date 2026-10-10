package miosa

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
)

// WebhooksService provides CRUD, test delivery, and delivery history for tenant webhooks.
type WebhooksService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// WebhookData is the API representation of a tenant webhook.
type WebhookData struct {
	ID        string   `json:"id"`
	TenantID  string   `json:"tenant_id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Secret    string   `json:"secret,omitempty"`
	Active    bool     `json:"active"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`

	// Fields the API returns beyond the original set. Secret is shown only in
	// the create response; later reads carry SecretPreview.
	WorkspaceID   string            `json:"workspace_id,omitempty"`
	Name          string            `json:"name,omitempty"`
	Description   string            `json:"description,omitempty"`
	SecretPreview string            `json:"secret_preview,omitempty"`
	State         string            `json:"state,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	// RetryCount is how many times a failed delivery is retried (0 to 10).
	RetryCount      int                    `json:"retry_count,omitempty"`
	FailureCount    int                    `json:"failure_count,omitempty"`
	LastTriggeredAt string                 `json:"last_triggered_at,omitempty"`
	LastStatusCode  *int                   `json:"last_status_code,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// WebhookListResponse wraps GET /webhooks.
type WebhookListResponse struct {
	Data []WebhookData `json:"data"`
}

// WebhookDeliveryData is the API representation of a single webhook delivery attempt.
type WebhookDeliveryData struct {
	ID           string `json:"id"`
	WebhookID    string `json:"webhook_id"`
	EventType    string `json:"event_type"`
	StatusCode   int    `json:"status_code,omitempty"`
	Success      bool   `json:"success"`
	AttemptedAt  string `json:"attempted_at"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
	ResponseBody string `json:"response_body,omitempty"`
	Error        string `json:"error,omitempty"`

	// Fields the API returns beyond the original set.
	// State is pending, retrying, delivered, failed or dead_letter.
	State          string `json:"state,omitempty"`
	ResponseStatus *int   `json:"response_status,omitempty"`
	Attempts       int    `json:"attempts,omitempty"`
	NextRetryAt    string `json:"next_retry_at,omitempty"`
	DeliveredAt    string `json:"delivered_at,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	// ReplayOf is the delivery this one replays, when it is a replay.
	ReplayOf string `json:"replay_of,omitempty"`
}

// WebhookDeliveryListResponse wraps GET /webhooks/:id/deliveries.
type WebhookDeliveryListResponse struct {
	Data []WebhookDeliveryData `json:"data"`
}

// CreateWebhookInput is the request body for POST /webhooks.
type CreateWebhookInput struct {
	URL            string   `json:"url"`
	Events         []string `json:"events"`
	Secret         string   `json:"secret,omitempty"`
	Active         *bool    `json:"active,omitempty"`
	IdempotencyKey string   `json:"-"`

	// WorkspaceID scopes the webhook to one workspace; only workspace-scopable
	// event types are accepted (422 EVENTS_NOT_WORKSPACE_SCOPABLE otherwise).
	WorkspaceID string `json:"workspace_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// Headers are sent on every delivery.
	Headers map[string]string `json:"headers,omitempty"`
	// RetryCount is 0 to 10.
	RetryCount *int                   `json:"retry_count,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateWebhookInput is the request body for PATCH /webhooks/:id.
type UpdateWebhookInput struct {
	URL    string   `json:"url,omitempty"`
	Events []string `json:"events,omitempty"`
	Secret string   `json:"secret,omitempty"`
	Active *bool    `json:"active,omitempty"`

	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	RetryCount  *int                   `json:"retry_count,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	// State sets the webhook state directly (for example "active" or "inactive").
	State string `json:"state,omitempty"`
}

// ListWebhooksInput holds optional query parameters for GET /webhooks.
type ListWebhooksInput struct {
	Active *bool
	Limit  int
	Cursor string
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// List returns webhooks for the authenticated tenant.
func (s *WebhooksService) List(ctx context.Context, input ListWebhooksInput) (*WebhookListResponse, error) {
	params := map[string]string{}
	if input.Active != nil {
		if *input.Active {
			params["active"] = "true"
		} else {
			params["active"] = "false"
		}
	}
	if input.Cursor != "" {
		params["cursor"] = input.Cursor
	}
	var out WebhookListResponse
	if err := s.client.getJSON(ctx, "/webhooks"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single webhook by ID.
func (s *WebhooksService) Get(ctx context.Context, id string) (*WebhookData, error) {
	var env apiResponse[WebhookData]
	if err := s.client.getJSON(ctx, "/webhooks/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Create registers a new webhook endpoint.
func (s *WebhooksService) Create(ctx context.Context, input CreateWebhookInput) (*WebhookData, error) {
	var env apiResponse[WebhookData]
	if err := s.client.postJSONIdempotent(ctx, "/webhooks", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Update patches an existing webhook.
func (s *WebhooksService) Update(ctx context.Context, id string, input UpdateWebhookInput) (*WebhookData, error) {
	var env apiResponse[WebhookData]
	if err := s.client.patchJSON(ctx, "/webhooks/"+id, input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete removes a webhook.
func (s *WebhooksService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/webhooks/"+id, nil)
}

// Test sends a synthetic test event to the webhook endpoint and returns the delivery result.
func (s *WebhooksService) Test(ctx context.Context, id string) (*WebhookDeliveryData, error) {
	var env apiResponse[WebhookDeliveryData]
	if err := s.client.postJSON(ctx, "/webhooks/"+id+"/test", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Deliveries returns recent delivery attempts for a webhook.
func (s *WebhooksService) Deliveries(ctx context.Context, id string) (*WebhookDeliveryListResponse, error) {
	var out WebhookDeliveryListResponse
	if err := s.client.getJSON(ctx, "/webhooks/"+id+"/deliveries", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WebhookEventCatalog describes the event types a webhook can subscribe to and
// how deliveries are signed.
type WebhookEventCatalog struct {
	// Data groups event types by product surface.
	Data map[string]interface{} `json:"data"`
	// Events is the flat list of event type names.
	Events []string `json:"events"`
	// WorkspaceScopedCategories are the categories a workspace webhook may use.
	WorkspaceScopedCategories []string `json:"workspace_scoped_categories"`
	// Signing says how to verify a delivery; VerifyWebhook implements it.
	Signing struct {
		Header    string `json:"header"`
		Algorithm string `json:"algorithm"`
		Format    string `json:"format"`
	} `json:"signing"`
	DeliveryHeaders []string `json:"delivery_headers"`
}

// Events returns the supported event types and the signing scheme
// (GET /webhooks/events).
func (s *WebhooksService) Events(ctx context.Context) (*WebhookEventCatalog, error) {
	var out WebhookEventCatalog
	if err := s.client.getJSON(ctx, "/webhooks/events", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDeliveriesOptions pages a webhook's delivery history.
type ListDeliveriesOptions struct {
	// Limit defaults to 50 on the server.
	Limit int
	// Before is an ISO 8601 timestamp: only deliveries created before it.
	Before string
}

// DeliveriesWith is Deliveries with paging options.
func (s *WebhooksService) DeliveriesWith(ctx context.Context, id string, opts ListDeliveriesOptions) (*WebhookDeliveryListResponse, error) {
	params := map[string]string{"before": opts.Before}
	if opts.Limit > 0 {
		params["limit"] = fmt.Sprint(opts.Limit)
	}
	var out WebhookDeliveryListResponse
	if err := s.client.getJSON(ctx, "/webhooks/"+url.PathEscape(id)+"/deliveries"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Replay re-sends a past delivery (POST /webhooks/:id/deliveries/:delivery_id/replay)
// and returns the new delivery, queued. replayID makes the call idempotent: a
// replay id the API has already seen is 409 "Replay ID already used". Pass ""
// to have the SDK generate one. The webhook must be active (409 otherwise).
func (s *WebhooksService) Replay(ctx context.Context, webhookID, deliveryID, replayID string) (*WebhookDeliveryData, error) {
	if replayID == "" {
		replayID = newUUID()
	}
	var env apiResponse[WebhookDeliveryData]
	path := "/webhooks/" + url.PathEscape(webhookID) + "/deliveries/" + url.PathEscape(deliveryID) + "/replay"
	if err := s.client.postJSON(ctx, path, map[string]string{"replay_id": replayID}, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// newUUID returns a random version 4 UUID.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// IsActive reports whether the webhook delivers: the API reports a state, older
// responses an active flag.
func (w WebhookData) IsActive() bool {
	if w.State != "" {
		return w.State == "active"
	}
	return w.Active
}
