package miosa

import (
	"context"
	"fmt"
)

// ComputerAutoStopService reads and updates the idle-timeout configuration for
// a computer. Accessed via Computer.AutoStop.
type ComputerAutoStopService struct {
	client     *Client
	computerID string
}

// Get returns the current auto-stop configuration.
func (s *ComputerAutoStopService) Get(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, fmt.Sprintf("/computers/%s/auto-stop", s.computerID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Update sets the idle timeout in seconds (0 disables auto-stop).
func (s *ComputerAutoStopService) Update(ctx context.Context, seconds int) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.patchJSON(ctx,
		fmt.Sprintf("/computers/%s/auto-stop", s.computerID),
		map[string]int{"seconds": seconds},
		&out,
	); err != nil {
		return nil, err
	}
	return out, nil
}
