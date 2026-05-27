package miosa

import (
	"context"
	"strconv"
)

// BuilderSessionsService manages durable, cross-device Builder UI session
// state. Routes live under /api/v1/builder/sessions/.
type BuilderSessionsService struct {
	client *Client
}

// BuilderSessionData is the API representation of a builder session.
type BuilderSessionData map[string]interface{}

// List returns builder sessions. limit defaults to 50.
func (s *BuilderSessionsService) List(ctx context.Context, limit int, params map[string]string) ([]BuilderSessionData, error) {
	if params == nil {
		params = map[string]string{}
	}
	if limit <= 0 {
		limit = 50
	}
	params["limit"] = strconv.Itoa(limit)
	var wrapper struct {
		Data     []BuilderSessionData `json:"data"`
		Sessions []BuilderSessionData `json:"sessions"`
		Items    []BuilderSessionData `json:"items"`
	}
	if err := s.client.getJSON(ctx, "/builder/sessions"+buildQuery(params), &wrapper); err != nil {
		var list []BuilderSessionData
		if err2 := s.client.getJSON(ctx, "/builder/sessions"+buildQuery(params), &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	for _, v := range [][]BuilderSessionData{wrapper.Data, wrapper.Sessions, wrapper.Items} {
		if len(v) > 0 {
			return v, nil
		}
	}
	return []BuilderSessionData{}, nil
}

// Get fetches a session by ID. The platform only exposes an index route, so
// this filters the list client-side.
func (s *BuilderSessionsService) Get(ctx context.Context, sessionID string) (BuilderSessionData, error) {
	sessions, err := s.List(ctx, 0, nil)
	if err != nil {
		return nil, err
	}
	for _, sess := range sessions {
		if id, _ := sess["id"].(string); id == sessionID {
			return sess, nil
		}
	}
	return nil, &NotFoundError{MiosaError: MiosaError{Message: "builder session not found: " + sessionID}}
}

// UpdateTitle updates the title of a builder session (PATCH).
func (s *BuilderSessionsService) UpdateTitle(ctx context.Context, sessionID, title string) (BuilderSessionData, error) {
	var out BuilderSessionData
	if err := s.client.patchJSON(ctx, "/builder/sessions/"+sessionID+"/title", map[string]string{"title": title}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete removes a builder session.
func (s *BuilderSessionsService) Delete(ctx context.Context, sessionID string) error {
	return s.client.deleteJSON(ctx, "/builder/sessions/"+sessionID, nil)
}
