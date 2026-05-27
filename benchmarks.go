package miosa

import "context"

// BenchmarksService triggers and inspects platform benchmark runs.
// Routes live under /api/v1/admin/benchmarks/. Requires admin credentials.
type BenchmarksService struct {
	client *Client
}

// BenchmarkData is the API representation of a single benchmark run.
type BenchmarkData map[string]interface{}

// List returns all benchmark runs, optionally filtered by query params.
func (s *BenchmarksService) List(ctx context.Context, params map[string]string) ([]BenchmarkData, error) {
	var wrapper struct {
		Data  []BenchmarkData `json:"data"`
		Items []BenchmarkData `json:"items"`
	}
	if err := s.client.getJSON(ctx, "/admin/benchmarks"+buildQuery(params), &wrapper); err != nil {
		// fallback: raw list
		var list []BenchmarkData
		if err2 := s.client.getJSON(ctx, "/admin/benchmarks"+buildQuery(params), &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	if len(wrapper.Data) > 0 {
		return wrapper.Data, nil
	}
	return wrapper.Items, nil
}

// Get returns a single benchmark run by ID.
func (s *BenchmarksService) Get(ctx context.Context, benchmarkID string) (BenchmarkData, error) {
	var out BenchmarkData
	if err := s.client.getJSON(ctx, "/admin/benchmarks/"+benchmarkID, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create starts a new benchmark run. Pass kind and any run-specific options.
func (s *BenchmarksService) Create(ctx context.Context, attrs map[string]interface{}) (BenchmarkData, error) {
	var out BenchmarkData
	if err := s.client.postJSON(ctx, "/admin/benchmarks", attrs, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Cancel stops a running benchmark.
func (s *BenchmarksService) Cancel(ctx context.Context, benchmarkID string) (BenchmarkData, error) {
	var out BenchmarkData
	if err := s.client.postJSON(ctx, "/admin/benchmarks/"+benchmarkID+"/cancel", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Samples returns per-iteration timing samples for a benchmark run.
func (s *BenchmarksService) Samples(ctx context.Context, benchmarkID string, params map[string]string) ([]BenchmarkData, error) {
	var wrapper struct {
		Data    []BenchmarkData `json:"data"`
		Samples []BenchmarkData `json:"samples"`
		Items   []BenchmarkData `json:"items"`
	}
	if err := s.client.getJSON(ctx, "/admin/benchmarks/"+benchmarkID+"/samples"+buildQuery(params), &wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.Data) > 0 {
		return wrapper.Data, nil
	}
	if len(wrapper.Samples) > 0 {
		return wrapper.Samples, nil
	}
	return wrapper.Items, nil
}

// Compare compares two benchmark runs.
func (s *BenchmarksService) Compare(ctx context.Context, leftID, rightID string, opts map[string]interface{}) (BenchmarkData, error) {
	body := map[string]interface{}{
		"left_id":  leftID,
		"right_id": rightID,
	}
	for k, v := range opts {
		body[k] = v
	}
	var out BenchmarkData
	if err := s.client.postJSON(ctx, "/admin/benchmarks/compare", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
