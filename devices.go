package miosa

import (
	"context"
	"strings"
)

// DeviceKind identifies the product-level execution surface.
type DeviceKind string

const (
	DeviceKindSandboxWorker    DeviceKind = "sandbox_worker"
	DeviceKindComputer         DeviceKind = "computer"
	DeviceKindLocalDevice      DeviceKind = "local_device"
	DeviceKindDockerDeployHost DeviceKind = "docker_deploy_host"
)

// DeviceSource identifies the backing API collection for hosted devices.
type DeviceSource string

const (
	DeviceSourceSandboxes DeviceSource = "sandboxes"
	DeviceSourceComputers DeviceSource = "computers"
)

// DeviceCatalogEntry describes when to use each MIOSA device type.
type DeviceCatalogEntry struct {
	Kind            DeviceKind `json:"kind"`
	Label           string     `json:"label"`
	Purpose         string     `json:"purpose"`
	Lifecycle       string     `json:"lifecycle"`
	Persistence     string     `json:"persistence"`
	PrimaryCommands []string   `json:"primary_commands"`
	UseWhen         []string   `json:"use_when"`
	AvoidWhen       []string   `json:"avoid_when"`
}

// DeviceRecord is a normalized hosted device row.
type DeviceRecord struct {
	ID                 string       `json:"id"`
	Kind               DeviceKind   `json:"kind"`
	Source             DeviceSource `json:"source"`
	Name               string       `json:"name,omitempty"`
	State              string       `json:"state,omitempty"`
	Ready              *bool        `json:"ready,omitempty"`
	Persistent         *bool        `json:"persistent,omitempty"`
	AlwaysOn           *bool        `json:"always_on,omitempty"`
	Region             string       `json:"region,omitempty"`
	Template           string       `json:"template,omitempty"`
	PreviewURL         string       `json:"preview_url,omitempty"`
	TimeoutRemainingMS *int         `json:"timeout_remaining_ms,omitempty"`
}

// DeviceListInput filters the hosted device inventory.
type DeviceListInput struct {
	Kind DeviceKind `json:"-"`
}

// DeviceListError describes a partial inventory failure.
type DeviceListError struct {
	Source    DeviceSource `json:"source"`
	Message   string       `json:"message"`
	Retryable bool         `json:"retryable"`
}

// DeviceListResponse is returned by Devices.List.
type DeviceListResponse struct {
	Devices []DeviceRecord    `json:"devices"`
	Errors  []DeviceListError `json:"errors"`
}

// DevicesService provides a product-level facade over Sandboxes, Computers,
// local devices, and Docker Deploy hosts.
type DevicesService struct {
	client *Client
}

// Catalog returns static routing guidance for agent orchestration apps.
func (s *DevicesService) Catalog() []DeviceCatalogEntry {
	out := make([]DeviceCatalogEntry, len(deviceCatalog))
	copy(out, deviceCatalog)
	return out
}

// List normalizes hosted sandboxes and computers into one device inventory.
// Partial endpoint failures are returned in Errors so callers can still render
// the usable half of the inventory.
func (s *DevicesService) List(ctx context.Context, input DeviceListInput) (*DeviceListResponse, error) {
	kind := normalizeDeviceKind(input.Kind)
	out := &DeviceListResponse{}

	if kind == "" || kind == DeviceKindSandboxWorker {
		var env sandboxListEnvelope
		if err := s.client.getJSON(ctx, "/sandboxes", &env); err != nil {
			out.Errors = append(out.Errors, deviceListError(DeviceSourceSandboxes, err))
		} else {
			for _, row := range env.items() {
				out.Devices = append(out.Devices, normalizeSandboxDevice(row))
			}
		}
	}

	if kind == "" || kind == DeviceKindComputer {
		var env ComputerListResponse
		if err := s.client.getJSON(ctx, "/computers", &env); err != nil {
			out.Errors = append(out.Errors, deviceListError(DeviceSourceComputers, err))
		} else {
			for _, row := range env.Data {
				out.Devices = append(out.Devices, normalizeComputerDevice(row))
			}
		}
	}

	return out, nil
}

type sandboxListEnvelope struct {
	Data      []sandboxDeviceRow `json:"data"`
	Sandboxes []sandboxDeviceRow `json:"sandboxes"`
}

func (e sandboxListEnvelope) items() []sandboxDeviceRow {
	if len(e.Data) > 0 {
		return e.Data
	}
	return e.Sandboxes
}

type sandboxDeviceRow struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	State              string `json:"state"`
	Status             string `json:"status"`
	Ready              *bool  `json:"ready"`
	Persistent         *bool  `json:"persistent"`
	AlwaysOn           *bool  `json:"always_on"`
	TemplateID         string `json:"template_id"`
	Template           string `json:"template"`
	PreviewURL         string `json:"preview_url"`
	TimeoutRemainingMS *int   `json:"timeout_remaining_ms"`
}

func normalizeSandboxDevice(row sandboxDeviceRow) DeviceRecord {
	state := row.State
	if state == "" {
		state = row.Status
	}
	template := row.TemplateID
	if template == "" {
		template = row.Template
	}
	return DeviceRecord{
		ID:                 row.ID,
		Kind:               DeviceKindSandboxWorker,
		Source:             DeviceSourceSandboxes,
		Name:               row.Name,
		State:              state,
		Ready:              row.Ready,
		Persistent:         row.Persistent,
		AlwaysOn:           row.AlwaysOn,
		Template:           template,
		PreviewURL:         row.PreviewURL,
		TimeoutRemainingMS: row.TimeoutRemainingMS,
	}
}

func normalizeComputerDevice(row ComputerData) DeviceRecord {
	return DeviceRecord{
		ID:       row.ID,
		Kind:     DeviceKindComputer,
		Source:   DeviceSourceComputers,
		Name:     row.Name,
		State:    string(row.Status),
		Template: row.TemplateType,
	}
}

func normalizeDeviceKind(kind DeviceKind) DeviceKind {
	switch kind {
	case "", "all":
		return ""
	case "sandbox":
		return DeviceKindSandboxWorker
	default:
		return kind
	}
}

func deviceListError(source DeviceSource, err error) DeviceListError {
	msg := err.Error()
	lower := strings.ToLower(msg)
	retryable := strings.Contains(lower, "502") ||
		strings.Contains(lower, "bad gateway") ||
		strings.Contains(lower, "econnreset") ||
		strings.Contains(lower, "fetch failed") ||
		strings.Contains(lower, "other side closed") ||
		strings.Contains(lower, "socket hang up")
	return DeviceListError{Source: source, Message: msg, Retryable: retryable}
}

var deviceCatalog = []DeviceCatalogEntry{
	{
		Kind:        DeviceKindSandboxWorker,
		Label:       "Sandbox Worker",
		Purpose:     "Isolated Linux workspace for agents to create files, run code, preview apps, snapshot, fork, and publish.",
		Lifecycle:   "Persistent by default; use stop/resume/snapshot/fork where the account backend supports saved state.",
		Persistence: "Use one-hour timeouts for interactive builds and checkpoint before long pauses.",
		PrimaryCommands: []string{
			"client.Sandboxes.Create(ctx, miosa.CreateSandboxInput{Template: \"nextjs\"})",
			"sandbox.Exec.Run(ctx, ...)",
		},
		UseWhen: []string{
			"Coding agents should build inside the remote filesystem.",
			"You need command execution, file writes, package installs, previews, or publish.",
		},
		AvoidWhen: []string{
			"The workflow requires full browser/desktop control.",
			"The app is ready for production; publish it to a deployment runtime.",
		},
	},
	{
		Kind:        DeviceKindComputer,
		Label:       "Computer",
		Purpose:     "Durable VM/desktop device for browser automation, CUA sessions, SSH, tunnels, and persistent agent control.",
		Lifecycle:   "Managed as a Computer with desktop/browser control surfaces.",
		Persistence: "Use checkpoints, volumes, tunnels, and agent sessions.",
		PrimaryCommands: []string{
			"client.Computers.Create(ctx, miosa.CreateComputerInput{Name: \"browser-agent\"})",
		},
		UseWhen: []string{
			"The agent needs Chromium or a full desktop.",
			"The workflow logs into dashboards, fills forms, clicks buttons, or captures screenshots.",
		},
		AvoidWhen: []string{
			"Simple code generation/build/test work fits a cheaper sandbox worker.",
			"You only need durable app hosting.",
		},
	},
	{
		Kind:        DeviceKindLocalDevice,
		Label:       "Local Device",
		Purpose:     "Developer-owned machine connected through CLI/MCP for local discovery.",
		Lifecycle:   "Not hosted by MIOSA; the user owns uptime and state.",
		Persistence: "State is local machine state. Do not assume cloud resume semantics.",
		PrimaryCommands: []string{
			"miosa mcp install",
			"miosa doctor --json",
		},
		UseWhen: []string{
			"The agent needs local repository discovery before cloud execution.",
			"The user intentionally wants local private tools.",
		},
		AvoidWhen: []string{
			"Customer code must stay isolated in MIOSA-hosted infrastructure.",
			"The workflow needs reproducible shared cloud state.",
		},
	},
	{
		Kind:        DeviceKindDockerDeployHost,
		Label:       "Docker Deploy Host",
		Purpose:     "Workspace appliance VM that runs Docker containers for durable apps published from sandboxes.",
		Lifecycle:   "Always-on deployment capacity; not an interactive coding workspace.",
		Persistence: "Versioned releases and routing are durable; edits happen in sandboxes before publish.",
		PrimaryCommands: []string{
			"miosa sandbox publish <id> --docker-deploy",
			"miosa deploy --docker-deploy",
		},
		UseWhen: []string{
			"You need many small apps, APIs, funnels, or client sites in one workspace appliance.",
			"You want stable public URLs backed by Docker containers.",
		},
		AvoidWhen: []string{
			"Interactive agent work is still happening.",
			"The app needs the standard MIOSA Deploy runtime.",
		},
	},
}
