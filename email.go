package miosa

import "context"

// EmailService is the admin email surface with Campaigns, Templates, and
// Inbox sub-services. Routes live under /api/v1/admin/email-{campaigns,
// templates,inbox}/. Requires admin credentials.
type EmailService struct {
	client    *Client
	Campaigns *EmailCampaignsService
	Templates *EmailTemplatesService
	Inbox     *EmailInboxService
}

func newEmailService(c *Client) *EmailService {
	return &EmailService{
		client:    c,
		Campaigns: &EmailCampaignsService{client: c},
		Templates: &EmailTemplatesService{client: c},
		Inbox:     &EmailInboxService{client: c},
	}
}

// ── Campaigns ─────────────────────────────────────────────────────────────────

// EmailCampaignsService manages bulk email send-out lifecycle.
type EmailCampaignsService struct {
	client *Client
}

// List returns email campaigns, optionally filtered by query params.
func (s *EmailCampaignsService) List(ctx context.Context, params map[string]string) ([]map[string]interface{}, error) {
	return emailGetList(s.client, ctx, "/admin/email-campaigns"+buildQuery(params))
}

// Create creates a new email campaign.
func (s *EmailCampaignsService) Create(ctx context.Context, attrs map[string]interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/admin/email-campaigns", attrs, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipientCount returns the expected recipient count for the given filters.
func (s *EmailCampaignsService) RecipientCount(ctx context.Context, params map[string]string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/admin/email-campaigns/recipient-count"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Send triggers a campaign send.
func (s *EmailCampaignsService) Send(ctx context.Context, campaignID string, opts map[string]interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/admin/email-campaigns/"+campaignID+"/send", opts, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Cancel cancels an in-progress campaign.
func (s *EmailCampaignsService) Cancel(ctx context.Context, campaignID string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/admin/email-campaigns/"+campaignID+"/cancel", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Deliveries returns delivery records for a campaign.
func (s *EmailCampaignsService) Deliveries(ctx context.Context, campaignID string, params map[string]string) ([]map[string]interface{}, error) {
	return emailGetList(s.client, ctx, "/admin/email-campaigns/"+campaignID+"/deliveries"+buildQuery(params))
}

// ── Templates ─────────────────────────────────────────────────────────────────

// EmailTemplatesService manages reusable email templates keyed by name.
type EmailTemplatesService struct {
	client *Client
}

// List returns all email templates.
func (s *EmailTemplatesService) List(ctx context.Context, params map[string]string) ([]map[string]interface{}, error) {
	return emailGetList(s.client, ctx, "/admin/email-templates"+buildQuery(params))
}

// Create creates a new email template. key is required.
func (s *EmailTemplatesService) Create(ctx context.Context, key string, attrs map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{"key": key}
	for k, v := range attrs {
		body[k] = v
	}
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/admin/email-templates", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Update updates an existing email template by key (PUT).
func (s *EmailTemplatesService) Update(ctx context.Context, key string, attrs map[string]interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.putJSON(ctx, "/admin/email-templates/"+key, attrs, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Reset resets an email template to its platform default.
func (s *EmailTemplatesService) Reset(ctx context.Context, key string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/admin/email-templates/"+key+"/reset", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ── Inbox ─────────────────────────────────────────────────────────────────────

// EmailInboxService manages inbound and outbound direct messages.
type EmailInboxService struct {
	client *Client
}

// List returns inbox messages.
func (s *EmailInboxService) List(ctx context.Context, params map[string]string) ([]map[string]interface{}, error) {
	return emailGetList(s.client, ctx, "/admin/email-inbox"+buildQuery(params))
}

// Send sends a direct message.
func (s *EmailInboxService) Send(ctx context.Context, attrs map[string]interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/admin/email-inbox/send", attrs, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MarkRead marks a message as read.
func (s *EmailInboxService) MarkRead(ctx context.Context, messageID string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/admin/email-inbox/"+messageID+"/read", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Archive archives a message.
func (s *EmailInboxService) Archive(ctx context.Context, messageID string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/admin/email-inbox/"+messageID+"/archive", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ── shared helper ──────────────────────────────────────────────────────────────

func emailGetList(c *Client, ctx context.Context, path string) ([]map[string]interface{}, error) {
	var wrapper struct {
		Data       []map[string]interface{} `json:"data"`
		Campaigns  []map[string]interface{} `json:"campaigns"`
		Templates  []map[string]interface{} `json:"templates"`
		Inbox      []map[string]interface{} `json:"inbox"`
		Deliveries []map[string]interface{} `json:"deliveries"`
		Items      []map[string]interface{} `json:"items"`
	}
	if err := c.getJSON(ctx, path, &wrapper); err != nil {
		var list []map[string]interface{}
		if err2 := c.getJSON(ctx, path, &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	for _, v := range [][]map[string]interface{}{
		wrapper.Data, wrapper.Campaigns, wrapper.Templates,
		wrapper.Inbox, wrapper.Deliveries, wrapper.Items,
	} {
		if len(v) > 0 {
			return v, nil
		}
	}
	return []map[string]interface{}{}, nil
}
