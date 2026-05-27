package miosa

import (
	"context"
	"net/http"
)

// CommandCenterService exposes read-only views of the Optimal AI agent fleet.
// Routes live under /api/v1/command-center/.
type CommandCenterService struct {
	client *Client
}

// Overview returns a top-level snapshot of the command center.
func (s *CommandCenterService) Overview(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/command-center", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Agents lists all agents registered in the command center.
func (s *CommandCenterService) Agents(ctx context.Context) ([]map[string]interface{}, error) {
	return s.getList(ctx, "/command-center/agents")
}

// RunningAgents lists only the currently running agents.
func (s *CommandCenterService) RunningAgents(ctx context.Context) ([]map[string]interface{}, error) {
	return s.getList(ctx, "/command-center/agents/running")
}

// Metrics returns aggregate fleet metrics.
func (s *CommandCenterService) Metrics(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/command-center/metrics", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Presets returns available agent configuration presets.
func (s *CommandCenterService) Presets(ctx context.Context) ([]map[string]interface{}, error) {
	return s.getList(ctx, "/command-center/presets")
}

// Tiers returns the tier configuration for the command center.
func (s *CommandCenterService) Tiers(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/command-center/tiers", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Events opens an SSE stream for live command-center events. The returned
// channel is closed when the stream ends or ctx is cancelled.
func (s *CommandCenterService) Events(ctx context.Context) (<-chan SSEEvent, error) {
	return s.client.streamSSE(ctx, http.MethodGet, "/command-center/events", nil)
}

func (s *CommandCenterService) getList(ctx context.Context, path string) ([]map[string]interface{}, error) {
	var wrapper struct {
		Data    []map[string]interface{} `json:"data"`
		Agents  []map[string]interface{} `json:"agents"`
		Running []map[string]interface{} `json:"running"`
		Presets []map[string]interface{} `json:"presets"`
		Items   []map[string]interface{} `json:"items"`
	}
	if err := s.client.getJSON(ctx, path, &wrapper); err != nil {
		// fallback: raw list
		var list []map[string]interface{}
		if err2 := s.client.getJSON(ctx, path, &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	for _, v := range [][]map[string]interface{}{
		wrapper.Data, wrapper.Agents, wrapper.Running, wrapper.Presets, wrapper.Items,
	} {
		if len(v) > 0 {
			return v, nil
		}
	}
	return []map[string]interface{}{}, nil
}
