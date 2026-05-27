package miosa

import "context"

// ModelsService lists LLM models available through the MIOSA intelligence
// gateway. Routes live under /api/v1/intelligence/.
type ModelsService struct {
	client *Client
}

// ModelData is the API representation of a single LLM model.
type ModelData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsListResponse wraps the intelligence models list response.
type ModelsListResponse struct {
	Object string      `json:"object"`
	Data   []ModelData `json:"data"`
}

// List returns all models available to the calling tenant.
// Accepts optional query params as a map (e.g. {"provider": "anthropic"}).
func (s *ModelsService) List(ctx context.Context, params map[string]string) ([]ModelData, error) {
	var out ModelsListResponse
	if err := s.client.getJSON(ctx, "/intelligence/models"+buildQuery(params), &out); err != nil {
		// Try raw list fallback.
		var list []ModelData
		if err2 := s.client.getJSON(ctx, "/intelligence/models"+buildQuery(params), &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	return out.Data, nil
}

// Get returns a single model by ID. The intelligence gateway does not expose a
// per-model GET; this filters the List payload client-side.
func (s *ModelsService) Get(ctx context.Context, modelID string) (*ModelData, error) {
	models, err := s.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	for i := range models {
		if models[i].ID == modelID {
			return &models[i], nil
		}
	}
	return nil, &NotFoundError{MiosaError: MiosaError{Message: "model not found: " + modelID}}
}
