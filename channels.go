package miosa

import "context"

// ChannelsService provides access to notification channels and preferences.
type ChannelsService struct {
	client *Client
}

// ChannelData is the API representation of a notification channel.
type ChannelData struct {
	ID        string                 `json:"id"`
	TenantID  string                 `json:"tenant_id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Enabled   bool                   `json:"enabled"`
	Config    map[string]interface{} `json:"config,omitempty"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

// ChannelListResponse wraps GET /channels.
type ChannelListResponse struct {
	Data []ChannelData `json:"data"`
}

// CreateChannelInput is the request body for POST /channels.
type CreateChannelInput struct {
	Name   string                 `json:"name"`
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// UpdateChannelInput is the request body for PATCH /channels/:id.
type UpdateChannelInput struct {
	Name    string                 `json:"name,omitempty"`
	Enabled *bool                  `json:"enabled,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// NotificationPreferences holds cross-channel notification preference state.
type NotificationPreferences struct {
	Data map[string]interface{} `json:"data"`
}

// UpdateNotificationsInput is the request body for PUT /channels/notifications.
type UpdateNotificationsInput struct {
	Preferences map[string]interface{} `json:"preferences"`
}

// List returns all channels for the tenant.
func (s *ChannelsService) List(ctx context.Context) (*ChannelListResponse, error) {
	var out ChannelListResponse
	if err := s.client.getJSON(ctx, "/channels", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single channel by ID.
func (s *ChannelsService) Get(ctx context.Context, channelID string) (*ChannelData, error) {
	var env apiResponse[ChannelData]
	if err := s.client.getJSON(ctx, "/channels/"+channelID, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Create creates a new notification channel.
func (s *ChannelsService) Create(ctx context.Context, input CreateChannelInput) (*ChannelData, error) {
	var env apiResponse[ChannelData]
	if err := s.client.postJSON(ctx, "/channels", input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Update updates a notification channel.
func (s *ChannelsService) Update(ctx context.Context, channelID string, input UpdateChannelInput) (*ChannelData, error) {
	var env apiResponse[ChannelData]
	if err := s.client.patchJSON(ctx, "/channels/"+channelID, input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete removes a notification channel.
func (s *ChannelsService) Delete(ctx context.Context, channelID string) error {
	return s.client.deleteJSON(ctx, "/channels/"+channelID, nil)
}

// ListNotifications returns notification preferences across all channels.
func (s *ChannelsService) ListNotifications(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/channels/notifications", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateNotifications updates notification preferences.
func (s *ChannelsService) UpdateNotifications(ctx context.Context, input UpdateNotificationsInput) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.putJSON(ctx, "/channels/notifications", input, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Enable enables a channel.
func (s *ChannelsService) Enable(ctx context.Context, channelID string) (*ChannelData, error) {
	var env apiResponse[ChannelData]
	if err := s.client.postJSON(ctx, "/channels/"+channelID+"/enable", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Disable disables a channel.
func (s *ChannelsService) Disable(ctx context.Context, channelID string) (*ChannelData, error) {
	var env apiResponse[ChannelData]
	if err := s.client.postJSON(ctx, "/channels/"+channelID+"/disable", nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
