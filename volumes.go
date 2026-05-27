package miosa

import (
	"context"
	"strconv"
)

// VolumesService provides CRUD for persistent block storage volumes.
type VolumesService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// VolumeData is the API representation of a persistent volume.
type VolumeData struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TenantID  string `json:"tenant_id"`
	SizeGB    int    `json:"size_gb"`
	Status    string `json:"status"`
	Region    string `json:"region,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// VolumeListResponse wraps GET /volumes.
type VolumeListResponse struct {
	Data []VolumeData `json:"data"`
}

// CreateVolumeInput is the request body for POST /volumes.
type CreateVolumeInput struct {
	Name           string `json:"name"`
	SizeGB         int    `json:"size_gb"`
	Region         string `json:"region,omitempty"`
	IdempotencyKey string `json:"-"`
}

// ListVolumesInput holds optional query parameters for GET /volumes.
type ListVolumesInput struct {
	Status string
	Limit  int
	Cursor string
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// List returns volumes for the authenticated tenant.
func (s *VolumesService) List(ctx context.Context, input ListVolumesInput) (*VolumeListResponse, error) {
	params := map[string]string{}
	if input.Status != "" {
		params["status"] = input.Status
	}
	if input.Limit > 0 {
		params["limit"] = strconv.Itoa(input.Limit)
	}
	if input.Cursor != "" {
		params["cursor"] = input.Cursor
	}
	var out VolumeListResponse
	if err := s.client.getJSON(ctx, "/volumes"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single volume by ID.
func (s *VolumesService) Get(ctx context.Context, id string) (*VolumeData, error) {
	var env apiResponse[VolumeData]
	if err := s.client.getJSON(ctx, "/volumes/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Create provisions a new persistent volume.
func (s *VolumesService) Create(ctx context.Context, input CreateVolumeInput) (*VolumeData, error) {
	var env apiResponse[VolumeData]
	if err := s.client.postJSONIdempotent(ctx, "/volumes", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete removes a volume.
func (s *VolumesService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/volumes/"+id, nil)
}
