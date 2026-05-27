package miosa

import (
	"context"
	"fmt"
	"net/http"
)

// ComputerLogsService reads and streams VM logs for a specific computer.
// Accessed via Computer.Logs.
type ComputerLogsService struct {
	client     *Client
	computerID string
}

// GetLogsInput configures the log snapshot fetch.
type GetLogsInput struct {
	Lines int    // Maximum number of lines to return.
	Since string // ISO-8601 timestamp lower bound.
}

// Get fetches the most recent log snapshot.
func (s *ComputerLogsService) Get(ctx context.Context, input GetLogsInput) (map[string]interface{}, error) {
	params := map[string]string{}
	if input.Lines > 0 {
		params["lines"] = fmt.Sprintf("%d", input.Lines)
	}
	if input.Since != "" {
		params["since"] = input.Since
	}
	var out map[string]interface{}
	if err := s.client.getJSON(ctx,
		fmt.Sprintf("/computers/%s/logs", s.computerID)+buildQuery(params),
		&out,
	); err != nil {
		return nil, err
	}
	return out, nil
}

// Stream opens an SSE connection to /computers/:id/logs/stream and returns a
// channel of events. The channel is closed when the stream ends or ctx is
// cancelled.
func (s *ComputerLogsService) Stream(ctx context.Context) (<-chan SSEEvent, error) {
	return s.client.streamSSE(ctx, http.MethodGet,
		fmt.Sprintf("/computers/%s/logs/stream", s.computerID),
		nil,
	)
}
