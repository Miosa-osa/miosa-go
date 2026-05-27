package miosa

import "context"

// RegionsService provides access to datacenter availability, sizes, pricing,
// and community templates (read-only catalog).
type RegionsService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// RegionData is the API representation of a datacenter region.
type RegionData struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Country   string `json:"country"`
	City      string `json:"city"`
	Available bool   `json:"available"`
}

// RegionListResponse wraps GET /compute/regions.
type RegionListResponse struct {
	Data []RegionData `json:"data"`
}

// SizeData is the API representation of a compute size.
type SizeData struct {
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	VCPUs     int     `json:"vcpus"`
	MemoryMB  int     `json:"memory_mb"`
	DiskGB    int     `json:"disk_gb"`
	PriceHour float64 `json:"price_hour"`
}

// SizeListResponse wraps GET /compute/sizes.
type SizeListResponse struct {
	Data []SizeData `json:"data"`
}

// ComputeTemplate is the API representation of a community computer template.
type ComputeTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// TemplateListResponse wraps GET /compute/templates.
type TemplateListResponse struct {
	Data []ComputeTemplate `json:"data"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// ListRegions lists available datacenter regions.
func (s *RegionsService) ListRegions(ctx context.Context) (*RegionListResponse, error) {
	var out RegionListResponse
	if err := s.client.getJSON(ctx, "/compute/regions", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSizes lists available compute sizes.
func (s *RegionsService) ListSizes(ctx context.Context) (*SizeListResponse, error) {
	var out SizeListResponse
	if err := s.client.getJSON(ctx, "/compute/sizes", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Pricing returns static compute pricing data.
func (s *RegionsService) Pricing(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/compute/pricing", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTemplates lists community computer templates.
func (s *RegionsService) ListTemplates(ctx context.Context) (*TemplateListResponse, error) {
	var out TemplateListResponse
	if err := s.client.getJSON(ctx, "/compute/templates", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTemplate fetches a single community template by ID.
func (s *RegionsService) GetTemplate(ctx context.Context, templateID string) (*ComputeTemplate, error) {
	var env apiResponse[ComputeTemplate]
	if err := s.client.getJSON(ctx, "/compute/templates/"+templateID, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
