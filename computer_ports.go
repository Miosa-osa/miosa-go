package miosa

import (
	"context"
	"fmt"
	"strconv"
)

// ComputerPortsService controls per-port visibility on a computer.
// Accessed via Computer.Ports.
type ComputerPortsService struct {
	client     *Client
	computerID string
}

// PortData is the API representation of a single port record.
type PortData map[string]interface{}

func (s *ComputerPortsService) base() string {
	return fmt.Sprintf("/computers/%s/ports", s.computerID)
}

// List returns all exposed port records.
func (s *ComputerPortsService) List(ctx context.Context) ([]PortData, error) {
	var wrapper struct {
		Data  []PortData `json:"data"`
		Ports []PortData `json:"ports"`
		Items []PortData `json:"items"`
	}
	if err := s.client.getJSON(ctx, s.base(), &wrapper); err != nil {
		var list []PortData
		if err2 := s.client.getJSON(ctx, s.base(), &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	for _, v := range [][]PortData{wrapper.Data, wrapper.Ports, wrapper.Items} {
		if len(v) > 0 {
			return v, nil
		}
	}
	return []PortData{}, nil
}

// Get returns the record for a specific port, or nil if not exposed.
// Filters the list response client-side (the backend has no single-port GET).
func (s *ComputerPortsService) Get(ctx context.Context, port int) (PortData, error) {
	records, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		switch v := r["port"].(type) {
		case float64:
			if int(v) == port {
				return r, nil
			}
		case int:
			if v == port {
				return r, nil
			}
		case string:
			if p, _ := strconv.Atoi(v); p == port {
				return r, nil
			}
		}
	}
	return nil, nil
}

// Create exposes port with the given visibility options.
func (s *ComputerPortsService) Create(ctx context.Context, port int, opts map[string]interface{}) (PortData, error) {
	body := map[string]interface{}{"port": port}
	for k, v := range opts {
		body[k] = v
	}
	var out PortData
	if err := s.client.postJSON(ctx, s.base(), body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Update patches visibility or auth options for port.
func (s *ComputerPortsService) Update(ctx context.Context, port int, opts map[string]interface{}) (PortData, error) {
	var out PortData
	if err := s.client.patchJSON(ctx,
		fmt.Sprintf("%s/%d", s.base(), port),
		opts,
		&out,
	); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete stops exposing port.
func (s *ComputerPortsService) Delete(ctx context.Context, port int) error {
	return s.client.deleteJSON(ctx, fmt.Sprintf("%s/%d", s.base(), port), nil)
}
