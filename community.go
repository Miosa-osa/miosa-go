package miosa

import "context"

// CommunityService provides the public template and agent catalog with install
// and rate operations. Routes live under /api/v1/community/.
type CommunityService struct {
	client *Client
}

// ListAgents returns community agents, optionally filtered by query params.
func (s *CommunityService) ListAgents(ctx context.Context, params map[string]string) ([]map[string]interface{}, error) {
	return s.getList(ctx, "/community/agents"+buildQuery(params))
}

// GetAgent returns a single community agent by ID.
func (s *CommunityService) GetAgent(ctx context.Context, agentID string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/community/agents/"+agentID, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTemplates returns community templates, optionally filtered.
func (s *CommunityService) ListTemplates(ctx context.Context, params map[string]string) ([]map[string]interface{}, error) {
	return s.getList(ctx, "/community/templates"+buildQuery(params))
}

// GetTemplate returns a single community template by ID.
func (s *CommunityService) GetTemplate(ctx context.Context, templateID string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/community/templates/"+templateID, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// InstallTemplate installs a community template into the caller's tenant.
func (s *CommunityService) InstallTemplate(ctx context.Context, templateID string, opts map[string]interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/community/templates/"+templateID+"/install", opts, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RateTemplate rates a community template (1–5).
func (s *CommunityService) RateTemplate(ctx context.Context, templateID string, rating int, opts map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{"rating": rating}
	for k, v := range opts {
		body[k] = v
	}
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/community/templates/"+templateID+"/rate", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *CommunityService) getList(ctx context.Context, path string) ([]map[string]interface{}, error) {
	var wrapper struct {
		Data      []map[string]interface{} `json:"data"`
		Templates []map[string]interface{} `json:"templates"`
		Agents    []map[string]interface{} `json:"agents"`
		Items     []map[string]interface{} `json:"items"`
	}
	if err := s.client.getJSON(ctx, path, &wrapper); err != nil {
		var list []map[string]interface{}
		if err2 := s.client.getJSON(ctx, path, &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	for _, v := range [][]map[string]interface{}{
		wrapper.Data, wrapper.Templates, wrapper.Agents, wrapper.Items,
	} {
		if len(v) > 0 {
			return v, nil
		}
	}
	return []map[string]interface{}{}, nil
}
