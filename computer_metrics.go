package miosa

import (
	"context"
	"fmt"
)

// ComputerMetricsService reads time-series RAM/CPU/credit metrics for a
// computer. Accessed via Computer.Metrics.
type ComputerMetricsService struct {
	client     *Client
	computerID string
}

// Get returns metric series for the given window (e.g. "1h", "24h", "7d").
func (s *ComputerMetricsService) Get(ctx context.Context, window string) (map[string]interface{}, error) {
	if window == "" {
		window = "1h"
	}
	var out map[string]interface{}
	if err := s.client.getJSON(ctx,
		fmt.Sprintf("/computers/%s/metrics", s.computerID)+buildQuery(map[string]string{"window": window}),
		&out,
	); err != nil {
		return nil, err
	}
	return out, nil
}
