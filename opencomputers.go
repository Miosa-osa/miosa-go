package miosa

// ─── OpenComputers types ──────────────────────────────────────────────────────

// HostStatus is the lifecycle state of an OpenComputers host.
type HostStatus string

const (
	HostStatusPending HostStatus = "pending"
	HostStatusOnline  HostStatus = "online"
	HostStatusOffline HostStatus = "offline"
	HostStatusError   HostStatus = "error"
	HostStatusRevoked HostStatus = "revoked"
)

// HostData is the API representation of a registered host.
type HostData struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Region   *string           `json:"region"`
	Status   HostStatus        `json:"status"`
	TenantID string            `json:"tenant_id"`
	Labels   map[string]string `json:"labels"`
	// HostKey is only present on the create response — shown once.
	HostKey   *string `json:"host_key,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// HostListResponse wraps a paginated host list.
type HostListResponse struct {
	Data []HostData `json:"data"`
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
}

// CreateHostInput is the request body for POST /opencomputers/hosts.
type CreateHostInput struct {
	Name   string            `json:"name"`
	Region string            `json:"region,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// UpdateHostInput is the request body for PATCH /opencomputers/hosts/:id.
type UpdateHostInput struct {
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// HostEvent is an SSE event emitted from the host event stream.
type HostEvent struct {
	Type      string      `json:"type"`
	HostID    string      `json:"host_id"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

// ─── Job types ────────────────────────────────────────────────────────────────

// JobStatus is the lifecycle state of a remote exec job.
type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// JobData is the API representation of a remote exec job.
type JobData struct {
	ID          string    `json:"id"`
	HostID      string    `json:"host_id"`
	Status      JobStatus `json:"status"`
	Command     string    `json:"command"`
	Args        []string  `json:"args"`
	Env         []string  `json:"env"`
	Cwd         *string   `json:"cwd"`
	ExitCode    *int      `json:"exit_code"`
	Stdout      *string   `json:"stdout"`
	Stderr      *string   `json:"stderr"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
	CompletedAt *string   `json:"completed_at"`
}

// JobListResponse wraps a paginated job list.
type JobListResponse struct {
	Data []JobData `json:"data"`
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
}

// RunJobInput is the request body for POST /opencomputers/hosts/:id/exec.
type RunJobInput struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	Cwd     string   `json:"cwd,omitempty"`
	Timeout int      `json:"timeout,omitempty"`
}

// JobEvent is a streamed event from a running job.
type JobEvent struct {
	Type      string      `json:"type"`
	JobID     string      `json:"job_id"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

// ─── File system types ────────────────────────────────────────────────────────

// FsEntry is a single directory entry.
type FsEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	IsDir      bool   `json:"is_dir"`
	ModifiedAt string `json:"modified_at"`
}

// FsStat holds metadata about a path.
type FsStat struct {
	Path          string  `json:"path"`
	Size          int64   `json:"size"`
	Mode          int     `json:"mode"`
	IsDir         bool    `json:"is_dir"`
	IsSymlink     bool    `json:"is_symlink"`
	SymlinkTarget *string `json:"symlink_target,omitempty"`
	ModifiedAt    string  `json:"modified_at"`
}

// FsListResponse wraps a directory listing.
type FsListResponse struct {
	Entries []FsEntry `json:"entries"`
	Path    string    `json:"path"`
}

// ─── Terminal / Desktop types ─────────────────────────────────────────────────

// WsTicket is a short-lived WebSocket authentication ticket.
type WsTicket struct {
	Ticket    string `json:"ticket"`
	WsURL     string `json:"ws_url"`
	ExpiresAt string `json:"expires_at"`
}

// ─── Tunnel types ─────────────────────────────────────────────────────────────

// TunnelAuthMode controls public accessibility of a tunnel.
type TunnelAuthMode string

const (
	TunnelAuthPublic     TunnelAuthMode = "public"
	TunnelAuthTenantOnly TunnelAuthMode = "tenant_only"
	TunnelAuthPassword   TunnelAuthMode = "password"
)

// TunnelData is the API representation of an HTTP tunnel.
type TunnelData struct {
	ID         string         `json:"id"`
	HostID     string         `json:"host_id"`
	Slug       string         `json:"slug"`
	TargetPort int            `json:"target_port"`
	AuthMode   TunnelAuthMode `json:"auth_mode"`
	PublicURL  string         `json:"public_url"`
	Enabled    bool           `json:"enabled"`
	CreatedAt  string         `json:"created_at"`
	UpdatedAt  string         `json:"updated_at"`
}

// TunnelListResponse wraps a list of tunnels.
type TunnelListResponse struct {
	Data []TunnelData `json:"data"`
}

// CreateTunnelInput is the request body for POST /opencomputers/hosts/:id/tunnels.
type CreateTunnelInput struct {
	TargetPort int            `json:"target_port"`
	AuthMode   TunnelAuthMode `json:"auth_mode,omitempty"`
	Slug       string         `json:"slug,omitempty"`
}

// UpdateTunnelInput is the request body for PATCH /opencomputers/hosts/:id/tunnels/:id.
type UpdateTunnelInput struct {
	TargetPort int            `json:"target_port,omitempty"`
	AuthMode   TunnelAuthMode `json:"auth_mode,omitempty"`
	Enabled    *bool          `json:"enabled,omitempty"`
}

// ─── Agent types ──────────────────────────────────────────────────────────────

// OcAgentSessionStatus is the lifecycle state of an agent session.
type OcAgentSessionStatus string

const (
	OcAgentPending   OcAgentSessionStatus = "pending"
	OcAgentRunning   OcAgentSessionStatus = "running"
	OcAgentCompleted OcAgentSessionStatus = "completed"
	OcAgentFailed    OcAgentSessionStatus = "failed"
	OcAgentCancelled OcAgentSessionStatus = "cancelled"
)

// OcAgentSession is the API representation of an agent dispatch session.
type OcAgentSession struct {
	ID          string               `json:"id"`
	HostID      string               `json:"host_id"`
	Task        string               `json:"task"`
	ModelID     *string              `json:"model_id"`
	Status      OcAgentSessionStatus `json:"status"`
	MaxTurns    int                  `json:"max_turns"`
	TurnsUsed   int                  `json:"turns_used"`
	CreatedAt   string               `json:"created_at"`
	UpdatedAt   string               `json:"updated_at"`
	CompletedAt *string              `json:"completed_at"`
	Error       *string              `json:"error"`
}

// OcAgentSessionListResponse wraps a list of agent sessions.
type OcAgentSessionListResponse struct {
	Data []OcAgentSession `json:"data"`
}

// DispatchAgentInput is the request body for POST /opencomputers/hosts/:id/agent/dispatch.
type DispatchAgentInput struct {
	Task     string                 `json:"task"`
	ModelID  string                 `json:"model_id,omitempty"`
	MaxTurns int                    `json:"max_turns,omitempty"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// OcAgentEvent is a streamed event from an agent session.
type OcAgentEvent struct {
	Type      string      `json:"type"`
	SessionID string      `json:"session_id"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

// ─── Cluster types ────────────────────────────────────────────────────────────

// ClusterStatus is the lifecycle state of an inference cluster.
type ClusterStatus string

const (
	ClusterProvisioning ClusterStatus = "provisioning"
	ClusterActive       ClusterStatus = "active"
	ClusterStopped      ClusterStatus = "stopped"
	ClusterError        ClusterStatus = "error"
	ClusterDestroyed    ClusterStatus = "destroyed"
)

// ClusterData is the API representation of an inference cluster.
type ClusterData struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Model        string        `json:"model"`
	Slug         string        `json:"slug"`
	Status       ClusterStatus `json:"status"`
	HostIDs      []string      `json:"host_ids"`
	InferenceURL string        `json:"inference_url"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
}

// ClusterListResponse wraps a list of clusters.
type ClusterListResponse struct {
	Data []ClusterData `json:"data"`
}

// CreateClusterInput is the request body for POST /opencomputers/clusters.
type CreateClusterInput struct {
	Name    string   `json:"name"`
	Model   string   `json:"model"`
	HostIDs []string `json:"host_ids"`
}

// ─── Secret types ─────────────────────────────────────────────────────────────

// SecretData is the API representation of an encrypted secret.
type SecretData struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	HostID      *string `json:"host_id"`
	TenantID    string  `json:"tenant_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// CreateSecretInput is the request body for creating a secret.
type CreateSecretInput struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// UpdateSecretInput is the request body for updating a secret.
type UpdateSecretInput struct {
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
}
