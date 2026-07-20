package miosa

import (
	"context"
	"encoding/json"
	"fmt"
)

// TemplatesService discovers canonical product templates across sandbox,
// computer, and App Engine appliance products.
//
// This is read-only product/template/size readiness metadata.
// Use SandboxTemplates for tenant-owned custom sandbox template CRUD/builds.
type TemplatesService struct {
	client *Client
}

// ShapeContract describes one exact, versioned product resource shape.
type ShapeContract struct {
	ContractID      string `json:"contract_id,omitempty"`
	ContractVersion string `json:"contract_version,omitempty"`
	Product         string `json:"product,omitempty"`
	Size            string `json:"size,omitempty"`
	VCPUs           int    `json:"vcpus,omitempty"`
	MemoryMB        int    `json:"memory_mb,omitempty"`
	DiskSizeMB      int    `json:"disk_size_mb,omitempty"`
}

// ProductTemplateCatalogResponse wraps GET /templates.
type ProductTemplateCatalogResponse struct {
	Data            *ProductTemplateCatalog    `json:"data,omitempty"`
	Templates       []ProductTemplate          `json:"templates,omitempty"`
	Products        []ProductCatalogEntry      `json:"products,omitempty"`
	Sizes           []map[string]interface{}   `json:"sizes,omitempty"`
	ShapeContracts  map[string][]ShapeContract `json:"shape_contracts,omitempty"`
	ReadinessStates []string                   `json:"readiness_states,omitempty"`
	Rules           map[string]interface{}     `json:"rules,omitempty"`
}

// ProductTemplateCatalog is the canonical product/template/size catalog.
type ProductTemplateCatalog struct {
	Templates       []ProductTemplate          `json:"templates"`
	Products        []ProductCatalogEntry      `json:"products,omitempty"`
	Sizes           []map[string]interface{}   `json:"sizes,omitempty"`
	ShapeContracts  map[string][]ShapeContract `json:"shape_contracts,omitempty"`
	ReadinessStates []string                   `json:"readiness_states,omitempty"`
	Rules           map[string]interface{}     `json:"rules,omitempty"`
}

// ProductCatalogEntry describes a MIOSA product, such as sandbox or computer.
type ProductCatalogEntry struct {
	ID              string            `json:"id"`
	Name            string            `json:"name,omitempty"`
	Primitive       string            `json:"primitive,omitempty"`
	Description     string            `json:"description,omitempty"`
	DefaultTemplate string            `json:"default_template,omitempty"`
	DefaultSize     string            `json:"default_size,omitempty"`
	SizeIDs         []string          `json:"size_ids,omitempty"`
	Templates       []ProductTemplate `json:"templates,omitempty"`
}

// ProductTemplate describes a canonical template/profile for a product.
type ProductTemplate struct {
	ID                string                  `json:"id"`
	Name              string                  `json:"name,omitempty"`
	Product           string                  `json:"product,omitempty"`
	Primitive         string                  `json:"primitive,omitempty"`
	Description       string                  `json:"description,omitempty"`
	DefaultSize       string                  `json:"default_size,omitempty"`
	ImageID           string                  `json:"image_id,omitempty"`
	SDKName           string                  `json:"sdk_name,omitempty"`
	CLIName           string                  `json:"cli_name,omitempty"`
	InstalledTools    []string                `json:"installed_tools,omitempty"`
	InstallCommand    string                  `json:"install_command,omitempty"`
	StartCommand      string                  `json:"start_command,omitempty"`
	ReadinessContract map[string]interface{}  `json:"readiness_contract,omitempty"`
	BenchmarkLane     map[string]interface{}  `json:"benchmark_lane,omitempty"`
	Aliases           []string                `json:"aliases,omitempty"`
	Sizes             []TemplateSizeReadiness `json:"sizes,omitempty"`
	Readiness         string                  `json:"readiness,omitempty"`
}

// TemplateSizeReadiness describes whether a product/template/size is fast-ready.
type TemplateSizeReadiness struct {
	Size             string                 `json:"size"`
	State            string                 `json:"state"`
	FastReady        bool                   `json:"fast_ready,omitempty"`
	ColdBootOnly     bool                   `json:"cold_boot_only,omitempty"`
	ReadinessScope   string                 `json:"readiness_scope,omitempty"`
	CheckedNodes     int                    `json:"checked_nodes,omitempty"`
	ReadyNodes       int                    `json:"ready_nodes,omitempty"`
	ColdBootNodes    int                    `json:"cold_boot_nodes,omitempty"`
	MissingNodes     int                    `json:"missing_nodes,omitempty"`
	UnavailableNodes int                    `json:"unavailable_nodes,omitempty"`
	Details          map[string]interface{} `json:"details,omitempty"`
}

// ListProductTemplatesInput holds optional query parameters for product template discovery.
type ListProductTemplatesInput struct {
	Product string
}

// Catalog returns the complete canonical product/template/size catalog.
func (s *TemplatesService) Catalog(ctx context.Context) (*ProductTemplateCatalog, error) {
	var raw json.RawMessage
	if err := s.client.getJSON(ctx, "/templates", &raw); err != nil {
		return nil, err
	}
	var out ProductTemplateCatalogResponse
	if err := json.Unmarshal(raw, &out); err == nil {
		if out.Data != nil {
			return out.Data, nil
		}
		return &ProductTemplateCatalog{
			Templates: out.Templates, Products: out.Products, Sizes: out.Sizes,
			ShapeContracts: out.ShapeContracts, ReadinessStates: out.ReadinessStates,
			Rules: out.Rules,
		}, nil
	}
	// The public V1 fixture includes a compatibility `data` array alongside
	// the canonical top-level catalog fields, so decode those fields separately.
	var topLevel ProductTemplateCatalog
	if err := json.Unmarshal(raw, &topLevel); err != nil {
		return nil, err
	}
	return &topLevel, nil
}

// List returns canonical product templates, optionally filtered by product.
func (s *TemplatesService) List(ctx context.Context, input ListProductTemplatesInput) ([]ProductTemplate, error) {
	catalog, err := s.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	if input.Product == "" {
		return catalog.Templates, nil
	}
	filtered := make([]ProductTemplate, 0, len(catalog.Templates))
	for _, template := range catalog.Templates {
		if template.Product == input.Product {
			filtered = append(filtered, template)
		}
	}
	return filtered, nil
}

// Get returns one canonical product template by ID.
func (s *TemplatesService) Get(ctx context.Context, templateID string) (*ProductTemplate, error) {
	templates, err := s.List(ctx, ListProductTemplatesInput{})
	if err != nil {
		return nil, err
	}
	for i := range templates {
		if templates[i].ID == templateID {
			return &templates[i], nil
		}
	}
	return nil, fmt.Errorf("template not found: %s", templateID)
}

// Readiness returns size readiness rows for one canonical product template.
func (s *TemplatesService) Readiness(ctx context.Context, templateID string) ([]TemplateSizeReadiness, error) {
	template, err := s.Get(ctx, templateID)
	if err != nil {
		return nil, err
	}
	return template.Sizes, nil
}
