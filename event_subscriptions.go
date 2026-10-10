package miosa

import (
	"context"
	"errors"
	"net/url"
	"strconv"
)

// EventSubscription routes platform events of one type to a consumer (a
// workflow, for example), optionally narrowed by a filter and scoped to a
// workspace or project.
type EventSubscription struct {
	ID           string                 `json:"id"`
	ConsumerKind string                 `json:"consumer_kind"`
	ConsumerID   string                 `json:"consumer_id"`
	EventType    string                 `json:"event_type"`
	Filter       map[string]interface{} `json:"filter,omitempty"`
	WorkspaceID  string                 `json:"workspace_id,omitempty"`
	ProjectID    string                 `json:"project_id,omitempty"`
	Active       bool                   `json:"active"`
	InsertedAt   string                 `json:"inserted_at,omitempty"`
	UpdatedAt    string                 `json:"updated_at,omitempty"`
}

// CreateEventSubscriptionInput subscribes a consumer to an event type. An
// unknown event type is 422 UNKNOWN_EVENT_TYPE, a bad filter 422 INVALID_FILTER,
// a missing consumer 404 CONSUMER_NOT_FOUND.
type CreateEventSubscriptionInput struct {
	ConsumerKind string                 `json:"consumer_kind"`
	ConsumerID   string                 `json:"consumer_id"`
	EventType    string                 `json:"event_type"`
	Filter       map[string]interface{} `json:"filter,omitempty"`
	WorkspaceID  string                 `json:"workspace_id,omitempty"`
	ProjectID    string                 `json:"project_id,omitempty"`
	Active       *bool                  `json:"active,omitempty"`
}

// UpdateEventSubscriptionInput changes the only mutable fields. Nil members are left alone.
type UpdateEventSubscriptionInput struct {
	Active *bool                  `json:"active,omitempty"`
	Filter map[string]interface{} `json:"filter,omitempty"`
}

// EventDelivery is one delivery of an event to a subscription.
type EventDelivery struct {
	ID      string `json:"id"`
	EventID string `json:"event_id"`
	// State is pending, delivered, failed or dead_letter.
	State           string `json:"state"`
	Attempts        int    `json:"attempts"`
	NextAttemptAt   string `json:"next_attempt_at,omitempty"`
	ResponseSummary string `json:"response_summary,omitempty"`
}

// ListEventSubscriptionsOptions narrows the list.
type ListEventSubscriptionsOptions struct {
	ConsumerKind string
	ConsumerID   string
}

// PlatformEvent is an envelope from the durable event outbox. Seq increases
// monotonically per organization; page forward with NextCursor.
type PlatformEvent map[string]interface{}

// PlatformEventsOptions filters the outbox read.
type PlatformEventsOptions struct {
	ProjectID   string
	WorkspaceID string
	// Type is an exact event type.
	Type        string
	SubjectType string
	SubjectID   string
	// AfterSeq returns events with a larger seq; use the previous NextCursor.
	AfterSeq string
	// Limit defaults to 50, at most 200.
	Limit int
}

// PlatformEventPage is one page of events, oldest first.
type PlatformEventPage struct {
	Data       []PlatformEvent `json:"data"`
	NextCursor string          `json:"next_cursor"`
}

// EventSubscriptionsService manages event subscriptions and reads the event
// outbox. Accessed via Client.Events. Scopes: events:read, events:write.
type EventSubscriptionsService struct {
	client *Client
}

// List returns subscriptions, optionally for one consumer.
func (s *EventSubscriptionsService) List(ctx context.Context, opts ListEventSubscriptionsOptions) ([]EventSubscription, error) {
	var out struct {
		Data []EventSubscription `json:"data"`
	}
	q := map[string]string{"consumer_kind": opts.ConsumerKind, "consumer_id": opts.ConsumerID}
	if err := s.client.getJSON(ctx, "/event-subscriptions"+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Create subscribes a consumer to an event type.
func (s *EventSubscriptionsService) Create(ctx context.Context, in CreateEventSubscriptionInput) (*EventSubscription, error) {
	if in.ConsumerKind == "" || in.ConsumerID == "" || in.EventType == "" {
		return nil, errors.New("consumer_kind, consumer_id and event_type are required")
	}
	var out apiResponse[EventSubscription]
	if err := s.client.postJSON(ctx, "/event-subscriptions", in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Update pauses or resumes a subscription, or replaces its filter.
func (s *EventSubscriptionsService) Update(ctx context.Context, id string, in UpdateEventSubscriptionInput) (*EventSubscription, error) {
	var out apiResponse[EventSubscription]
	if err := s.client.patchJSON(ctx, "/event-subscriptions/"+url.PathEscape(id), in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Delete removes a subscription.
func (s *EventSubscriptionsService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/event-subscriptions/"+url.PathEscape(id), nil)
}

// Deliveries lists a subscription's deliveries.
func (s *EventSubscriptionsService) Deliveries(ctx context.Context, id string) ([]EventDelivery, error) {
	var out struct {
		Data []EventDelivery `json:"data"`
	}
	if err := s.client.getJSON(ctx, "/event-subscriptions/"+url.PathEscape(id)+"/deliveries", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Redeliver re-queues a failed or dead-lettered delivery (replay). 409
// NOT_REDELIVERABLE when it is not in a state that can be re-sent.
func (s *EventSubscriptionsService) Redeliver(ctx context.Context, deliveryID string) (*EventDelivery, error) {
	var out apiResponse[EventDelivery]
	if err := s.client.postJSON(ctx, "/event-deliveries/"+url.PathEscape(deliveryID)+"/redeliver", map[string]interface{}{}, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Outbox reads the durable event outbox, oldest first.
func (s *EventSubscriptionsService) Outbox(ctx context.Context, opts PlatformEventsOptions) (*PlatformEventPage, error) {
	q := map[string]string{
		"project_id":   opts.ProjectID,
		"workspace_id": opts.WorkspaceID,
		"type":         opts.Type,
		"subject_type": opts.SubjectType,
		"subject_id":   opts.SubjectID,
		"after_seq":    opts.AfterSeq,
	}
	if opts.Limit > 0 {
		q["limit"] = strconv.Itoa(opts.Limit)
	}
	var out PlatformEventPage
	if err := s.client.getJSON(ctx, "/platform-events"+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OutboxHealth is the outbox health view: delivery counts, lag and recent dead
// letters. Organization admins only.
func (s *EventSubscriptionsService) OutboxHealth(ctx context.Context) (map[string]interface{}, error) {
	var out apiResponse[map[string]interface{}]
	if err := s.client.getJSON(ctx, "/platform-events/outbox", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}
