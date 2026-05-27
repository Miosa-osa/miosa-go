package miosa

import "context"

// EmbeddingsService provides OpenAI-compatible embedding vector creation
// under /api/v1/intelligence/embeddings.
type EmbeddingsService struct {
	client *Client
}

// EmbeddingRequest is the request body for POST /intelligence/embeddings.
type EmbeddingRequest struct {
	Input interface{} `json:"input"` // string or []string
	Model string      `json:"model"`
	// Optional extra fields (encoding_format, dimensions, user…).
	Extra map[string]interface{} `json:"-"`
}

// EmbeddingObject is a single embedding result within the response.
type EmbeddingObject struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingResponse is the OpenAI-compatible envelope returned by the endpoint.
type EmbeddingResponse struct {
	Object string            `json:"object"`
	Data   []EmbeddingObject `json:"data"`
	Model  string            `json:"model"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// Create requests one or more embedding vectors for the given input.
func (s *EmbeddingsService) Create(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	body := map[string]interface{}{
		"input": req.Input,
		"model": req.Model,
	}
	for k, v := range req.Extra {
		body[k] = v
	}
	var out EmbeddingResponse
	if err := s.client.postJSON(ctx, "/intelligence/embeddings", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
