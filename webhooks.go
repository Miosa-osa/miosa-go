package miosa

import "context"

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
}

// UpdateWebhookInput is the request body for PATCH /webhooks/:id.
type UpdateWebhookInput struct {
	URL    string   `json:"url,omitempty"`
	Events []string `json:"events,omitempty"`
	Secret string   `json:"secret,omitempty"`
	Active *bool    `json:"active,omitempty"`
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
