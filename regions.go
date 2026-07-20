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

// ArtifactReadinessState describes whether a template/size can fast boot.
type ArtifactReadinessState string

const (
	ArtifactReadinessFastReady    ArtifactReadinessState = "fast_ready"
	ArtifactReadinessColdBootOnly ArtifactReadinessState = "cold_boot_only"
	ArtifactReadinessMissing      ArtifactReadinessState = "missing"
)

// ComputeArtifactReadiness is the readiness of one template at one size.
type ComputeArtifactReadiness struct {
	Size          string                 `json:"size"`
	State         ArtifactReadinessState `json:"state"`
	CheckedNodes  int                    `json:"checked_nodes"`
	ReadyNodes    int                    `json:"ready_nodes"`
	ColdBootNodes int                    `json:"cold_boot_nodes"`
	MissingNodes  int                    `json:"missing_nodes"`
	CheckedHosts  int                    `json:"checked_hosts"`
	ReadyHosts    int                    `json:"ready_hosts"`
	ColdBootHosts int                    `json:"cold_boot_hosts"`
	MissingHosts  int                    `json:"missing_hosts"`
	Notes         []string               `json:"notes,omitempty"`
}

// ComputeCatalogTemplate describes a concrete runnable template/profile.
type ComputeCatalogTemplate struct {
	ID                string                     `json:"id"`
	TemplateID        string                     `json:"template_id,omitempty"`
	Name              string                     `json:"name"`
	Description       string                     `json:"description"`
	DefaultSize       string                     `json:"default_size"`
	SizeIDs           []string                   `json:"size_ids"`
	SupportedSizes    []string                   `json:"supported_sizes,omitempty"`
	ArtifactReadiness []ComputeArtifactReadiness `json:"artifact_readiness"`
}

// ComputeCatalogProduct describes a canonical compute product lane.
type ComputeCatalogProduct struct {
	ID                string                   `json:"id"`
	ProductID         string                   `json:"product_id,omitempty"`
	Name              string                   `json:"name"`
	Description       string                   `json:"description"`
	DefaultTemplate   string                   `json:"default_template"`
	DefaultTemplateID string                   `json:"default_template_id,omitempty"`
	DefaultSize       string                   `json:"default_size"`
	SizeIDs           []string                 `json:"size_ids"`
	Templates         []ComputeCatalogTemplate `json:"templates"`
}

// ComputeCatalog is the canonical catalog of products, templates, sizes, and
// fast-readiness truth for choosing a launch-safe configuration.
type ComputeCatalog struct {
	Products    []ComputeCatalogProduct `json:"products"`
	Sizes       []SizeData              `json:"sizes"`
	GeneratedAt string                  `json:"generated_at"`
}

// ComputeCatalogResponse wraps GET /compute/catalog.
type ComputeCatalogResponse struct {
	Data ComputeCatalog `json:"data"`
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

// Catalog returns canonical products, templates, sizes, and artifact readiness.
func (s *RegionsService) Catalog(ctx context.Context) (*ComputeCatalogResponse, error) {
	var out ComputeCatalogResponse
	if err := s.client.getJSON(ctx, "/compute/catalog", &out); err != nil {
		return nil, err
	}
	return &out, nil
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
