package miosa

import (
	"context"
	"fmt"
)

// ComputerInboxService reads and updates the per-computer inbox configuration
// (inbound-email inbox for Optimal). Accessed via Computer.Inbox.
type ComputerInboxService struct {
	client     *Client
	computerID string
}

// Get fetches the current inbox configuration.
func (s *ComputerInboxService) Get(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, fmt.Sprintf("/computers/%s/inbox", s.computerID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Update patches one or more inbox fields (e.g. alias, enabled).
func (s *ComputerInboxService) Update(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.patchJSON(ctx, fmt.Sprintf("/computers/%s/inbox", s.computerID), fields, &out); err != nil {
		return nil, err
	}
	return out, nil
}
