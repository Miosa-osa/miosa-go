package miosa

import (
	"context"
	"fmt"
)

// ComputerTerminalService manages PTY sessions on a specific computer.
// Accessed via Computer.Terminal.
type ComputerTerminalService struct {
	client     *Client
	computerID string
}

// CreateTerminalInput configures a new PTY session.
type CreateTerminalInput struct {
	Cols  int               `json:"cols,omitempty"`
	Rows  int               `json:"rows,omitempty"`
	Shell string            `json:"shell,omitempty"`
	Cwd   string            `json:"cwd,omitempty"`
	Env   map[string]string `json:"env,omitempty"`
}

// Create opens a new PTY session. Returns the server payload including the
// session ID.
func (s *ComputerTerminalService) Create(ctx context.Context, input CreateTerminalInput) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, fmt.Sprintf("/computers/%s/terminal", s.computerID), input, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Resize resizes an existing PTY session.
func (s *ComputerTerminalService) Resize(ctx context.Context, sessionID string, cols, rows int) (map[string]interface{}, error) {
	body := map[string]int{"cols": cols, "rows": rows}
	var out map[string]interface{}
	if err := s.client.postJSON(ctx,
		fmt.Sprintf("/computers/%s/pty/%s/resize", s.computerID, sessionID),
		body, &out,
	); err != nil {
		return nil, err
	}
	return out, nil
}
