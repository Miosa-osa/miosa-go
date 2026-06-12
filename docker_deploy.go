package miosa

import (
	"context"
	"fmt"
	"net/url"
)

// DockerDeployService exposes workspace appliance hosts and starter templates.
type DockerDeployService struct {
	client *Client
}

// DockerDeployHostStatus is the lifecycle of a workspace Docker Deploy host.
type DockerDeployHostStatus string

const (
	DockerDeployHostPending       DockerDeployHostStatus = "pending"
	DockerDeployHostProvisioning  DockerDeployHostStatus = "provisioning"
	DockerDeployHostBootstrapping DockerDeployHostStatus = "bootstrapping"
	DockerDeployHostActive        DockerDeployHostStatus = "active"
	DockerDeployHostDegraded      DockerDeployHostStatus = "degraded"
	DockerDeployHostSuspended     DockerDeployHostStatus = "suspended"
	DockerDeployHostRetired       DockerDeployHostStatus = "retired"
	DockerDeployHostError         DockerDeployHostStatus = "error"
)

// DockerDeployApplianceStatus is the health of the host appliance stack.
type DockerDeployApplianceStatus string

const (
	DockerDeployApplianceNotInstalled DockerDeployApplianceStatus = "not_installed"
	DockerDeployApplianceInstalling   DockerDeployApplianceStatus = "installing"
	DockerDeployApplianceStarting     DockerDeployApplianceStatus = "starting"
	DockerDeployApplianceHealthy      DockerDeployApplianceStatus = "healthy"
	DockerDeployApplianceUnhealthy    DockerDeployApplianceStatus = "unhealthy"
	DockerDeployApplianceUnknown      DockerDeployApplianceStatus = "unknown"
)

// DockerDeployHostData is a dedicated workspace host running the Docker Deploy appliance.
type DockerDeployHostData struct {
	ID                  string                      `json:"id"`
	TenantID            string                      `json:"tenant_id"`
	WorkspaceID         string                      `json:"workspace_id"`
	ExternalWorkspaceID string                      `json:"external_workspace_id,omitempty"`
	ComputerID          string                      `json:"computer_id,omitempty"`
	FleetNodeID         string                      `json:"fleet_node_id,omitempty"`
	Status              DockerDeployHostStatus      `json:"status"`
	Size                string                      `json:"size"`
	Region              string                      `json:"region"`
	PortalDomain        string                      `json:"portal_domain,omitempty"`
	RuntimeBaseURL      string                      `json:"runtime_base_url,omitempty"`
	AgentBaseURL        string                      `json:"agent_base_url,omitempty"`
	ApplianceImage      string                      `json:"appliance_image,omitempty"`
	ApplianceVersion    string                      `json:"appliance_version,omitempty"`
	ApplianceStatus     DockerDeployApplianceStatus `json:"appliance_status"`
	AgentLastSeenAt     string                      `json:"agent_last_seen_at,omitempty"`
	Metadata            map[string]interface{}      `json:"metadata,omitempty"`
	CreatedAt           string                      `json:"created_at,omitempty"`
	UpdatedAt           string                      `json:"updated_at,omitempty"`
}

// DockerDeployTemplate is a starter template for Docker Deploy apps.
type DockerDeployTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Runtime     string                 `json:"runtime,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Extra       map[string]interface{} `json:"-"`
}

// ListDockerDeployHostsInput filters Docker Deploy hosts.
type ListDockerDeployHostsInput struct {
	WorkspaceID string
}

// EnsureDockerDeployHostInput ensures a workspace has a Docker Deploy host.
type EnsureDockerDeployHostInput struct {
	WorkspaceID         string `json:"workspace_id,omitempty"`
	ExternalWorkspaceID string `json:"external_workspace_id,omitempty"`
}

// EnsureDockerDeployHostResult is returned by EnsureHost.
type EnsureDockerDeployHostResult struct {
	Host   DockerDeployHostData `json:"host"`
	Queued bool                 `json:"queued"`
}

type dockerDeployHostListResponse struct {
	Data  []DockerDeployHostData `json:"data"`
	Hosts []DockerDeployHostData `json:"hosts"`
}

type dockerDeployHostResponse struct {
	Data   *DockerDeployHostData `json:"data"`
	Host   *DockerDeployHostData `json:"host"`
	Queued bool                  `json:"queued"`
}

type dockerDeployTemplateListResponse struct {
	Data      []DockerDeployTemplate `json:"data"`
	Templates []DockerDeployTemplate `json:"templates"`
}

type dockerDeployTemplateResponse struct {
	Data     *DockerDeployTemplate `json:"data"`
	Template *DockerDeployTemplate `json:"template"`
}

// ListHosts lists Docker Deploy appliance hosts for the current tenant.
func (s *DockerDeployService) ListHosts(ctx context.Context, input ListDockerDeployHostsInput) ([]DockerDeployHostData, error) {
	params := map[string]string{}
	if input.WorkspaceID != "" {
		params["workspace_id"] = input.WorkspaceID
	}
	var out dockerDeployHostListResponse
	if err := s.client.getJSON(ctx, "/docker-deploy/hosts"+buildQuery(params), &out); err != nil {
		return nil, fmt.Errorf("DockerDeployService.ListHosts: %w", err)
	}
	if out.Data != nil {
		return out.Data, nil
	}
	return out.Hosts, nil
}

// EnsureHost creates or returns the workspace's dedicated Docker Deploy host.
func (s *DockerDeployService) EnsureHost(ctx context.Context, input EnsureDockerDeployHostInput) (*EnsureDockerDeployHostResult, error) {
	var out dockerDeployHostResponse
	if err := s.client.postJSON(ctx, "/docker-deploy/hosts/ensure", input, &out); err != nil {
		return nil, fmt.Errorf("DockerDeployService.EnsureHost: %w", err)
	}
	host := unwrapDockerDeployHost(out)
	if host == nil {
		return nil, fmt.Errorf("DockerDeployService.EnsureHost: empty host response")
	}
	return &EnsureDockerDeployHostResult{Host: *host, Queued: out.Queued}, nil
}

// GetHost fetches one Docker Deploy host by ID.
func (s *DockerDeployService) GetHost(ctx context.Context, hostID string) (*DockerDeployHostData, error) {
	var out dockerDeployHostResponse
	if err := s.client.getJSON(ctx, "/docker-deploy/hosts/"+hostID, &out); err != nil {
		return nil, fmt.Errorf("DockerDeployService.GetHost: %w", err)
	}
	host := unwrapDockerDeployHost(out)
	if host == nil {
		return nil, fmt.Errorf("DockerDeployService.GetHost: empty host response")
	}
	return host, nil
}

// ListTemplates lists Docker Deploy starter templates.
func (s *DockerDeployService) ListTemplates(ctx context.Context) ([]DockerDeployTemplate, error) {
	var out dockerDeployTemplateListResponse
	if err := s.client.getJSON(ctx, "/docker-deploy/templates", &out); err != nil {
		return nil, fmt.Errorf("DockerDeployService.ListTemplates: %w", err)
	}
	if out.Data != nil {
		return out.Data, nil
	}
	return out.Templates, nil
}

// GetTemplate fetches one Docker Deploy starter template by ID.
func (s *DockerDeployService) GetTemplate(ctx context.Context, templateID string) (*DockerDeployTemplate, error) {
	var out dockerDeployTemplateResponse
	if err := s.client.getJSON(ctx, "/docker-deploy/templates/"+url.PathEscape(templateID), &out); err != nil {
		return nil, fmt.Errorf("DockerDeployService.GetTemplate: %w", err)
	}
	if out.Data != nil {
		return out.Data, nil
	}
	if out.Template != nil {
		return out.Template, nil
	}
	return nil, fmt.Errorf("DockerDeployService.GetTemplate: empty template response")
}

func unwrapDockerDeployHost(out dockerDeployHostResponse) *DockerDeployHostData {
	if out.Data != nil {
		return out.Data
	}
	return out.Host
}
