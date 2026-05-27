package miosa

import (
	"context"
	"strconv"
)

// AuditLogService provides access to the admin-scoped audit event stream.
type AuditLogService struct {
	client *Client
}

// AuditLogEntry is a single event in the audit log.
type AuditLogEntry struct {
	ID         string                 `json:"id"`
	TenantID   string                 `json:"tenant_id"`
	ActorID    string                 `json:"actor_id"`
	ActorType  string                 `json:"actor_type"`
	Action     string                 `json:"action"`
	Resource   string                 `json:"resource"`
	ResourceID string                 `json:"resource_id"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	CreatedAt  string                 `json:"created_at"`
}

// AuditLogListResponse wraps GET /audit-log.
type AuditLogListResponse struct {
	Data []AuditLogEntry `json:"data"`
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
}

// ListAuditLogInput holds optional filters for GET /audit-log.
type ListAuditLogInput struct {
	ActorID  string
	Action   string
	Resource string
	Since    string
	Until    string
	Page     int
	PerPage  int
}

// List returns audit-log events with optional filters.
func (s *AuditLogService) List(ctx context.Context, input ListAuditLogInput) (*AuditLogListResponse, error) {
	params := map[string]string{}
	if input.ActorID != "" {
		params["actor_id"] = input.ActorID
	}
	if input.Action != "" {
		params["action"] = input.Action
	}
	if input.Resource != "" {
		params["resource"] = input.Resource
	}
	if input.Since != "" {
		params["since"] = input.Since
	}
	if input.Until != "" {
		params["until"] = input.Until
	}
	if input.Page > 0 {
		params["page"] = strconv.Itoa(input.Page)
	}
	if input.PerPage > 0 {
		params["per_page"] = strconv.Itoa(input.PerPage)
	}
	var out AuditLogListResponse
	if err := s.client.getJSON(ctx, "/audit-log"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
