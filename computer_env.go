package miosa

import (
	"context"
	"fmt"
)

// ComputerEnvService manages encrypted environment variables scoped to a
// single computer. Accessed via Computer.Env.
type ComputerEnvService struct {
	client     *Client
	computerID string
}

func (s *ComputerEnvService) base() string {
	return fmt.Sprintf("/computers/%s/env", s.computerID)
}

// EnvVarData is the API representation of a single env var record.
type EnvVarData map[string]interface{}

// List returns all env vars for the computer. Values may be masked by the
// server depending on tenant policy.
func (s *ComputerEnvService) List(ctx context.Context) ([]EnvVarData, error) {
	var wrapper struct {
		Data  []EnvVarData `json:"data"`
		Env   []EnvVarData `json:"env"`
		Items []EnvVarData `json:"items"`
	}
	if err := s.client.getJSON(ctx, s.base(), &wrapper); err != nil {
		var list []EnvVarData
		if err2 := s.client.getJSON(ctx, s.base(), &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	for _, v := range [][]EnvVarData{wrapper.Data, wrapper.Env, wrapper.Items} {
		if len(v) > 0 {
			return v, nil
		}
	}
	return []EnvVarData{}, nil
}

// Set creates a new env var. Use Update to change an existing one.
func (s *ComputerEnvService) Set(ctx context.Context, name, value string) (EnvVarData, error) {
	var out EnvVarData
	if err := s.client.postJSON(ctx, s.base(),
		map[string]string{"name": name, "value": value},
		&out,
	); err != nil {
		return nil, err
	}
	return out, nil
}

// Update patches the value of an existing env var by name.
func (s *ComputerEnvService) Update(ctx context.Context, name, value string) (EnvVarData, error) {
	var out EnvVarData
	if err := s.client.patchJSON(ctx, s.base()+"/"+name,
		map[string]string{"value": value},
		&out,
	); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete removes an env var by name.
func (s *ComputerEnvService) Delete(ctx context.Context, name string) error {
	return s.client.deleteJSON(ctx, s.base()+"/"+name, nil)
}

// BulkSet creates one env var per entry in the provided map. Calls Set for
// each key (the backend has no bulk endpoint).
func (s *ComputerEnvService) BulkSet(ctx context.Context, env map[string]string) ([]EnvVarData, error) {
	results := make([]EnvVarData, 0, len(env))
	for name, value := range env {
		data, err := s.Set(ctx, name, value)
		if err != nil {
			return results, err
		}
		results = append(results, data)
	}
	return results, nil
}
