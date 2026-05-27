package miosa

import (
	"context"
	"fmt"
)

// ComputerOsaService dispatches tasks to the in-VM OSA agent.
// Accessed via Computer.Osa.
type ComputerOsaService struct {
	client     *Client
	computerID string
}

// SubmitTask submits a free-form task to the in-VM OSA agent.
func (s *ComputerOsaService) SubmitTask(ctx context.Context, task string, params map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{"task": task}
	for k, v := range params {
		body[k] = v
	}
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, fmt.Sprintf("/computers/%s/osa/task", s.computerID), body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CancelTask cancels the currently running OSA task, if any.
func (s *ComputerOsaService) CancelTask(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.deleteJSON(ctx, fmt.Sprintf("/computers/%s/osa/task", s.computerID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Status returns the OSA agent's current task, configuration, and health.
func (s *ComputerOsaService) Status(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, fmt.Sprintf("/computers/%s/osa/status", s.computerID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Configure updates OSA runtime configuration (model, tools, secrets, etc.).
func (s *ComputerOsaService) Configure(ctx context.Context, config map[string]interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, fmt.Sprintf("/computers/%s/osa/configure", s.computerID), config, &out); err != nil {
		return nil, err
	}
	return out, nil
}
