package miosa

// ─── Computers ───────────────────────────────────────────────────────────────

// ComputerSize is the hardware tier for a computer.
type ComputerSize string

const (
	SizeSmall  ComputerSize = "small"
	SizeMedium ComputerSize = "medium"
	SizeLarge  ComputerSize = "large"
)

// ComputerStatus is the lifecycle state of a computer.
type ComputerStatus string

const (
	StatusCreating  ComputerStatus = "creating"
	StatusStarting  ComputerStatus = "starting"
	StatusRunning   ComputerStatus = "running"
	StatusStopping  ComputerStatus = "stopping"
	StatusStopped   ComputerStatus = "stopped"
	StatusError     ComputerStatus = "error"
	StatusDestroyed ComputerStatus = "destroyed"
)

// ComputerData is the API representation of a computer resource.
type ComputerData struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Slug         string             `json:"slug"`
	Status       ComputerStatus     `json:"status"`
	Visibility   ComputerVisibility `json:"visibility"`
	TemplateType string             `json:"template_type"`
	Size         ComputerSize       `json:"size"`
	TenantID     string             `json:"tenant_id"`
	WorkspaceID  string             `json:"workspace_id"`
	IPAddress    string             `json:"ip_address"`
	Metadata     map[string]string  `json:"metadata"`
	CreatedAt    string             `json:"created_at"`
	UpdatedAt    string             `json:"updated_at"`
}

// CreateComputerInput is the request body for POST /computers.
type CreateComputerInput struct {
	Name         string            `json:"name"`
	TemplateType string            `json:"template_type,omitempty"`
	Size         ComputerSize      `json:"size,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	// White-label attribution. Phase 2A — stored as text on the row.
	ExternalWorkspaceID string `json:"external_workspace_id,omitempty"`
	ExternalUserID      string `json:"external_user_id,omitempty"`
	ExternalProjectID   string `json:"external_project_id,omitempty"`
}

// ListComputersInput are optional query parameters for GET /computers.
type ListComputersInput struct {
	Page    int            `json:"-"`
	PerPage int            `json:"-"`
	Status  ComputerStatus `json:"-"`
}

// ComputerListResponse wraps a paginated list of computers.
type ComputerListResponse struct {
	Data []ComputerData `json:"data"`
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
}

// ─── Desktop ─────────────────────────────────────────────────────────────────

// MouseButton identifies which mouse button to use.
type MouseButton string

const (
	ButtonLeft   MouseButton = "left"
	ButtonRight  MouseButton = "right"
	ButtonMiddle MouseButton = "middle"
)

// ScrollDirection is the scroll axis and direction.
type ScrollDirection string

const (
	ScrollUp    ScrollDirection = "up"
	ScrollDown  ScrollDirection = "down"
	ScrollLeft  ScrollDirection = "left"
	ScrollRight ScrollDirection = "right"
)

// ClickInput is the request body for POST /desktop/click.
type ClickInput struct {
	X      int         `json:"x"`
	Y      int         `json:"y"`
	Button MouseButton `json:"button,omitempty"`
}

// DoubleClickInput is the request body for POST /desktop/double-click.
type DoubleClickInput struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// TypeInput is the request body for POST /desktop/type.
type TypeInput struct {
	Text  string `json:"text"`
	Delay int    `json:"delay,omitempty"` // milliseconds between keystrokes
}

// KeyInput is the request body for POST /desktop/key.
type KeyInput struct {
	Key string `json:"key"`
}

// ScrollInput is the request body for POST /desktop/scroll.
type ScrollInput struct {
	X         *int            `json:"x,omitempty"`
	Y         *int            `json:"y,omitempty"`
	Direction ScrollDirection `json:"direction"`
	Clicks    int             `json:"clicks,omitempty"`
}

// DragInput is the request body for POST /desktop/drag.
type DragInput struct {
	FromX int `json:"from_x"`
	FromY int `json:"from_y"`
	ToX   int `json:"to_x"`
	ToY   int `json:"to_y"`
}

// WaitInput is the request body for POST /desktop/wait.
type WaitInput struct {
	Seconds float64 `json:"seconds"`
}

// WindowFocusInput is the request body for POST /desktop/window/focus.
type WindowFocusInput struct {
	WindowID string `json:"window_id"`
}

// LaunchInput is the request body for POST /desktop/launch.
type LaunchInput struct {
	AppName string `json:"app_name"`
}

// DesktopActionResult is the common response for mutating desktop actions.
type DesktopActionResult struct {
	Success bool `json:"success"`
}

// WindowInfo describes an open window on the desktop.
type WindowInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	IsFocused bool   `json:"is_focused"`
}

// CursorInfo reports the current cursor position.
type CursorInfo struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ─── Exec ────────────────────────────────────────────────────────────────────

// ExecInput is the request body for POST /computers/{id}/exec.
type ExecInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // seconds
}

// ExecPythonInput is the request body for POST /computers/{id}/exec/python.
type ExecPythonInput struct {
	Code    string `json:"code"`
	Timeout int    `json:"timeout,omitempty"` // seconds
}

// ExecResult is the response for exec endpoints.
type ExecResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Success  bool   `json:"success"`
}

// ─── Files ───────────────────────────────────────────────────────────────────

// FileEntry describes a single file or directory on the computer.
type FileEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	IsDir      bool   `json:"is_dir"`
	ModifiedAt string `json:"modified_at"`
}

// FileListResult is the response for GET /computers/{id}/files.
type FileListResult struct {
	Entries []FileEntry `json:"entries"`
	Path    string      `json:"path"`
}

// FileExportResult is the response for POST /computers/{id}/files/export.
type FileExportResult struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

// ─── Agent / CUA ─────────────────────────────────────────────────────────────

// AgentSessionStatus is the lifecycle state of an agent session.
type AgentSessionStatus string

const (
	AgentPending   AgentSessionStatus = "pending"
	AgentRunning   AgentSessionStatus = "running"
	AgentCompleted AgentSessionStatus = "completed"
	AgentFailed    AgentSessionStatus = "failed"
	AgentCancelled AgentSessionStatus = "cancelled"
)

// AgentEventType identifies the kind of SSE event emitted by an agent session.
type AgentEventType string

const (
	EventSessionStarted   AgentEventType = "session_started"
	EventTurnStarted      AgentEventType = "turn_started"
	EventThinking         AgentEventType = "thinking"
	EventToolCall         AgentEventType = "tool_call"
	EventToolResult       AgentEventType = "tool_result"
	EventStreamingToken   AgentEventType = "streaming_token"
	EventAgentResponse    AgentEventType = "agent_response"
	EventTurnCompleted    AgentEventType = "turn_completed"
	EventSessionCompleted AgentEventType = "session_completed"
	EventSessionFailed    AgentEventType = "session_failed"
	EventDone             AgentEventType = "done"
	EventError            AgentEventType = "error"
)

// RunAgentInput is the request body for POST /computers/{id}/cua/sessions.
type RunAgentInput struct {
	Goal     string `json:"goal"`
	ModelID  string `json:"model_id,omitempty"`
	MaxTurns int    `json:"max_turns,omitempty"`
}

// AgentSessionData is the API representation of an agent session.
type AgentSessionData struct {
	ID          string             `json:"id"`
	ComputerID  string             `json:"computer_id"`
	Goal        string             `json:"goal"`
	ModelID     string             `json:"model_id"`
	Status      AgentSessionStatus `json:"status"`
	MaxTurns    int                `json:"max_turns"`
	TurnsUsed   int                `json:"turns_used"`
	CreatedAt   string             `json:"created_at"`
	UpdatedAt   string             `json:"updated_at"`
	CompletedAt string             `json:"completed_at"`
	Error       string             `json:"error"`
}

// AgentSessionListResponse wraps a list of agent sessions.
type AgentSessionListResponse struct {
	Data []AgentSessionData `json:"data"`
}

// AgentEvent is a single SSE event from an agent session stream.
type AgentEvent struct {
	Type      AgentEventType `json:"type"`
	SessionID string         `json:"session_id"`
	Data      interface{}    `json:"data"`
	Timestamp string         `json:"timestamp"`
}

// ─── Credits ─────────────────────────────────────────────────────────────────

// CreditBalance is the current balance for the authenticated tenant.
type CreditBalance struct {
	Balance   int    `json:"balance"`
	ExpiresAt string `json:"expires_at"`
}

// CreditTransaction is a single credit ledger entry.
type CreditTransaction struct {
	ID          string `json:"id"`
	Amount      int    `json:"amount"`
	Type        string `json:"type"` // "credit" | "debit"
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// CreditTransactionListResponse wraps a paginated list of transactions.
type CreditTransactionListResponse struct {
	Data []CreditTransaction `json:"data"`
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
}

// CreditUsage summarises credit consumption for a billing period.
type CreditUsage struct {
	PeriodStart    string `json:"period_start"`
	PeriodEnd      string `json:"period_end"`
	ComputeCredits int    `json:"compute_credits"`
	AICredits      int    `json:"ai_credits"`
	TotalCredits   int    `json:"total_credits"`
}

// ─── Workspaces ───────────────────────────────────────────────────────────────

// Workspace groups related computers under a shared namespace.
type Workspace struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	TenantID    string            `json:"tenant_id"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// CreateWorkspaceInput is the request body for POST /workspaces.
type CreateWorkspaceInput struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// UpdateWorkspaceInput is the request body for PATCH /workspaces/{id}.
type UpdateWorkspaceInput struct {
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ListWorkspacesInput holds optional pagination parameters.
type ListWorkspacesInput struct {
	Page    int
	PerPage int
}

// WorkspaceListResponse wraps a paginated list of workspaces.
type WorkspaceListResponse struct {
	Data []Workspace `json:"data"`
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
}

// ─── Workspace Members ────────────────────────────────────────────────────────

// WorkspaceRole is the role a user holds within a workspace.
type WorkspaceRole string

const (
	WorkspaceRoleOwner  WorkspaceRole = "owner"
	WorkspaceRoleAdmin  WorkspaceRole = "admin"
	WorkspaceRoleMember WorkspaceRole = "member"
	WorkspaceRoleViewer WorkspaceRole = "viewer"
)

// WorkspaceMember is a member row with denormalised user fields for display.
type WorkspaceMember struct {
	UserID    string        `json:"user_id"`
	Email     string        `json:"email"`
	Name      string        `json:"name"`
	AvatarURL string        `json:"avatar_url"`
	Role      WorkspaceRole `json:"role"`
	JoinedAt  string        `json:"joined_at"`
	AddedBy   string        `json:"added_by"`
}

// WorkspaceMemberRecord is the raw workspace_members row returned after
// add/update operations.
type WorkspaceMemberRecord struct {
	UserID      string        `json:"user_id"`
	WorkspaceID string        `json:"workspace_id"`
	Role        WorkspaceRole `json:"role"`
	JoinedAt    string        `json:"joined_at"`
	AddedBy     string        `json:"added_by"`
}

// AddWorkspaceMemberInput is the request body for POST /workspaces/:id/members.
type AddWorkspaceMemberInput struct {
	UserID string        `json:"user_id"`
	Role   WorkspaceRole `json:"role,omitempty"`
}

// UpdateWorkspaceMemberRoleInput is the request body for PATCH /workspaces/:id/members/:user_id.
type UpdateWorkspaceMemberRoleInput struct {
	Role WorkspaceRole `json:"role"`
}

// WorkspaceMemberListResponse wraps the list response for workspace members.
type WorkspaceMemberListResponse struct {
	Data []WorkspaceMember `json:"data"`
}

// WorkspaceMemberRecordResponse wraps the single-record response for add/update.
type WorkspaceMemberRecordResponse struct {
	Data WorkspaceMemberRecord `json:"data"`
}

// WorkspaceMemberDeleteResponse is the response from removing a workspace member.
type WorkspaceMemberDeleteResponse struct {
	Deleted bool `json:"deleted"`
}

// ─── Workspace Invites ────────────────────────────────────────────────────────

// WorkspaceInvite is a pending workspace invite row.
type WorkspaceInvite struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	TenantID    string `json:"tenant_id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	InvitedBy   string `json:"invited_by"`
	ExpiresAt   string `json:"expires_at"`
	AcceptedAt  string `json:"accepted_at"`
	InsertedAt  string `json:"inserted_at"`
}

// WorkspaceInvitePreview is the public invite preview (no auth required).
type WorkspaceInvitePreview struct {
	WorkspaceName string `json:"workspace_name"`
	TenantName    string `json:"tenant_name"`
	Role          string `json:"role"`
	Email         string `json:"email"`
	ExpiresAt     string `json:"expires_at"`
	Expired       bool   `json:"expired"`
	Revoked       bool   `json:"revoked"`
	Accepted      bool   `json:"accepted"`
}

// CreateWorkspaceInviteInput is the request body for POST /workspaces/:id/invites.
type CreateWorkspaceInviteInput struct {
	Email string `json:"email"`
	Role  string `json:"role,omitempty"`
}

// CreateWorkspaceInviteResponse is the response from creating a workspace invite.
// Type is "invited" when an invite row was created, or "added" when the user was
// added directly because they were already a tenant member.
type CreateWorkspaceInviteResponse struct {
	Data interface{} `json:"data"`
	Type string      `json:"type"`
}

// WorkspaceInviteListResponse wraps the list response for workspace invites.
type WorkspaceInviteListResponse struct {
	Data  []WorkspaceInvite `json:"data"`
	Total int               `json:"total"`
}

// WorkspaceInviteRevokeResponse is the response from revoking a workspace invite.
type WorkspaceInviteRevokeResponse struct {
	InviteID string `json:"invite_id"`
	Revoked  bool   `json:"revoked"`
}

// WorkspaceInvitePreviewResponse wraps the data envelope for GET /workspace-invites/:token.
type WorkspaceInvitePreviewResponse struct {
	Data WorkspaceInvitePreview `json:"data"`
}

// AcceptWorkspaceInviteResponse is the response from accepting a workspace invite.
type AcceptWorkspaceInviteResponse struct {
	Accepted    bool   `json:"accepted"`
	WorkspaceID string `json:"workspace_id"`
	TenantID    string `json:"tenant_id"`
	Role        string `json:"role"`
}

// ─── Org Invites ──────────────────────────────────────────────────────────────

// OrgInvite is a pending org (tenant) invite row.
type OrgInvite struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	InvitedBy  string `json:"invited_by"`
	ExpiresAt  string `json:"expires_at"`
	AcceptedAt string `json:"accepted_at"`
	CreatedAt  string `json:"created_at"`
}

// OrgInviteCreated is the response data from creating an org invite.
type OrgInviteCreated struct {
	InviteID  string `json:"invite_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expires_at"`
	InviteURL string `json:"invite_url"`
}

// OrgInvitePreview is the public preview of an org invite (no auth required).
type OrgInvitePreview struct {
	Email      string `json:"email"`
	TenantName string `json:"tenant_name"`
	Role       string `json:"role"`
	ExpiresAt  string `json:"expires_at"`
	Expired    bool   `json:"expired"`
	Accepted   bool   `json:"accepted"`
}

// CreateOrgInviteInput is the request body for POST /tenants/:id/invites.
type CreateOrgInviteInput struct {
	Email string `json:"email"`
	Role  string `json:"role,omitempty"`
}

// OrgInviteCreatedResponse wraps the data envelope from creating an org invite.
type OrgInviteCreatedResponse struct {
	Data OrgInviteCreated `json:"data"`
}

// OrgInviteListResponse wraps the list response for org invites.
type OrgInviteListResponse struct {
	Data  []OrgInvite `json:"data"`
	Total int         `json:"total"`
}

// OrgInviteRevokeResponse is the response from revoking an org invite.
type OrgInviteRevokeResponse struct {
	InviteID string `json:"invite_id"`
	Revoked  bool   `json:"revoked"`
}

// OrgInvitePreviewResponse wraps the data envelope for GET /invites/:token.
type OrgInvitePreviewResponse struct {
	Data OrgInvitePreview `json:"data"`
}

// AcceptOrgInviteResponse is the response from accepting an org invite.
type AcceptOrgInviteResponse struct {
	Accepted bool        `json:"accepted"`
	TenantID string      `json:"tenant_id"`
	Tenant   interface{} `json:"tenant"`
}

// AdminUserWorkspaceRow is one entry in the admin user-workspaces response.
type AdminUserWorkspaceRow struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	WorkspaceSlug string `json:"workspace_slug"`
	TenantID      string `json:"tenant_id"`
	TenantName    string `json:"tenant_name"`
	Role          string `json:"role"`
	JoinedAt      string `json:"joined_at"`
}

// ─── Snapshots ────────────────────────────────────────────────────────────────

// SnapshotStatus is the lifecycle state of a Firecracker snapshot.
type SnapshotStatus string

const (
	SnapshotCreating  SnapshotStatus = "creating"
	SnapshotUploading SnapshotStatus = "uploading"
	SnapshotReady     SnapshotStatus = "ready"
	SnapshotRestoring SnapshotStatus = "restoring"
	SnapshotFailed    SnapshotStatus = "failed"
	SnapshotDeleted   SnapshotStatus = "deleted"
)

// Snapshot is the API representation of a Firecracker checkpoint.
type Snapshot struct {
	ID                  string         `json:"id"`
	ComputerID          string         `json:"computer_id"`
	TenantID            string         `json:"tenant_id"`
	Comment             string         `json:"comment"`
	Status              SnapshotStatus `json:"status"`
	StateSizeBytes      *int64         `json:"state_size_bytes"`
	MemorySizeBytes     *int64         `json:"memory_size_bytes"`
	RootfsSizeBytes     *int64         `json:"rootfs_size_bytes"`
	CompressedSizeBytes *int64         `json:"compressed_size_bytes"`
	S3Bucket            string         `json:"s3_bucket"`
	S3Prefix            string         `json:"s3_prefix"`
	ParentSnapshotID    string         `json:"parent_snapshot_id"`
	Error               string         `json:"error"`
	CreatedAt           string         `json:"created_at"`
	UpdatedAt           string         `json:"updated_at"`
}

// CreateSnapshotInput is the request body for POST /computers/{id}/snapshots.
type CreateSnapshotInput struct {
	Comment string `json:"comment,omitempty"`
}

// SnapshotProgressEvent is an SSE frame emitted during snapshot operations.
type SnapshotProgressEvent struct {
	Type       string `json:"type"`
	SnapshotID string `json:"snapshot_id"`
	Status     string `json:"status"`
	Step       string `json:"step,omitempty"`
	Progress   *int   `json:"progress,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SnapshotStream is a live SSE channel for snapshot progress events.
// The underlying channel is closed when the stream ends.
// Drain C or let the context cancel to avoid goroutine leaks.
type SnapshotStream struct {
	ch <-chan SnapshotProgressEvent
	// C is the read-only channel exposed to callers.
	C <-chan SnapshotProgressEvent
}

// ─── Services ─────────────────────────────────────────────────────────────────

// ServiceStatus is the runtime state of a managed service.
type ServiceStatus string

const (
	ServiceRunning  ServiceStatus = "running"
	ServiceStopped  ServiceStatus = "stopped"
	ServiceStarting ServiceStatus = "starting"
	ServiceFailed   ServiceStatus = "failed"
)

// Service is the API representation of a long-running process managed on a computer.
type Service struct {
	ID         string            `json:"id"`
	ComputerID string            `json:"computer_id"`
	Name       string            `json:"name"`
	Command    string            `json:"command"`
	Status     ServiceStatus     `json:"status"`
	Port       *int              `json:"port"`
	Env        map[string]string `json:"env"`
	CreatedAt  string            `json:"created_at"`
	UpdatedAt  string            `json:"updated_at"`
}

// CreateServiceInput is the request body for POST /computers/{id}/services.
type CreateServiceInput struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Port    *int              `json:"port,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// ServiceLogEvent is a single log line from a running service.
type ServiceLogEvent struct {
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"` // "stdout" | "stderr"
	Message   string `json:"message"`
}

// ServiceLogStream is a live channel of log events from a service.
// Drain C or let the context cancel to avoid goroutine leaks.
type ServiceLogStream struct {
	ch <-chan ServiceLogEvent
	// C is the read-only channel exposed to callers.
	C <-chan ServiceLogEvent
}

// ─── Custom Domains ───────────────────────────────────────────────────────────

// CustomDomainStatus is the lifecycle state of a custom domain mapping.
type CustomDomainStatus string

const (
	DomainPending  CustomDomainStatus = "pending"
	DomainVerified CustomDomainStatus = "verified"
	DomainActive   CustomDomainStatus = "active"
	DomainFailed   CustomDomainStatus = "failed"
	DomainRemoved  CustomDomainStatus = "removed"
)

// CustomDomain is the API representation of a custom domain mapping.
type CustomDomain struct {
	ID                 string             `json:"id"`
	ComputerID         string             `json:"computer_id"`
	TenantID           string             `json:"tenant_id"`
	FQDN               string             `json:"fqdn"`
	Status             CustomDomainStatus `json:"status"`
	VerificationTarget string             `json:"verification_target"`
	Instructions       string             `json:"instructions"`
	VerifiedAt         string             `json:"verified_at"`
	TLSIssuedAt        string             `json:"tls_issued_at"`
	CreatedAt          string             `json:"created_at"`
	UpdatedAt          string             `json:"updated_at"`
}

// ─── Network Policy ───────────────────────────────────────────────────────────

// NetworkEffect is the action applied to matching traffic.
type NetworkEffect string

const (
	NetworkEffectAllow NetworkEffect = "allow"
	NetworkEffectDeny  NetworkEffect = "deny"
)

// NetworkProtocol is the transport protocol for a policy rule.
type NetworkProtocol string

const (
	NetworkProtocolTCP NetworkProtocol = "tcp"
	NetworkProtocolUDP NetworkProtocol = "udp"
	NetworkProtocolAny NetworkProtocol = "any"
)

// NetworkPolicyRule is a single egress rule evaluated by the host firewall.
type NetworkPolicyRule struct {
	Effect      NetworkEffect   `json:"effect"`
	Destination string          `json:"destination"`
	Ports       string          `json:"ports,omitempty"`
	Protocol    NetworkProtocol `json:"protocol,omitempty"`
}

// NetworkPolicy is the full egress policy for a computer.
type NetworkPolicy struct {
	ComputerID    string              `json:"computer_id"`
	Rules         []NetworkPolicyRule `json:"rules"`
	DefaultEffect NetworkEffect       `json:"default_effect"`
}

// SetNetworkPolicyInput is the request body for PUT /computers/{id}/network-policy.
type SetNetworkPolicyInput struct {
	Rules         []NetworkPolicyRule `json:"rules"`
	DefaultEffect NetworkEffect       `json:"default_effect"`
}

// ─── Files (extended) ─────────────────────────────────────────────────────────

// FileStat contains metadata for a path inside the computer.
type FileStat struct {
	Path          string `json:"path"`
	Size          int64  `json:"size"`
	Mode          string `json:"mode"`
	IsDir         bool   `json:"is_dir"`
	IsSymlink     bool   `json:"is_symlink"`
	SymlinkTarget string `json:"symlink_target,omitempty"`
	ModifiedAt    string `json:"modified_at"`
}

// DirEntry is a single entry returned by Readdir.
type DirEntry struct {
	Name       string `json:"name"`
	IsDir      bool   `json:"is_dir"`
	IsSymlink  bool   `json:"is_symlink"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

// MkdirOptions configures the Mkdir operation.
type MkdirOptions struct {
	// Recursive creates parent directories as needed (default true).
	Recursive bool
	// Mode is the octal permission string (default "0755").
	Mode string
}

// CopyOptions configures the Copy operation.
type CopyOptions struct {
	// Recursive is required when copying a directory tree.
	Recursive bool
}

// ─── Events ───────────────────────────────────────────────────────────────────

// EventProducer names the category of in-VM events to subscribe to.
type EventProducer string

const (
	ProducerWindow    EventProducer = "window"
	ProducerClipboard EventProducer = "clipboard"
	ProducerFile      EventProducer = "file"
	ProducerProcess   EventProducer = "process"
	ProducerIdle      EventProducer = "idle"
)

// EventSubscribeOptions configures an EventsService.Subscribe call.
type EventSubscribeOptions struct {
	// Subscribe lists the producers to enable. At least one is required.
	Subscribe []EventProducer
	// Paths are the filesystem paths to watch (file producer only).
	// Defaults to ["/home/user"] when empty.
	Paths []string
	// IdleThresholdSec is the inactivity threshold for the idle producer (default 30).
	IdleThresholdSec int
}

// EventType is the dot-separated event type string (e.g. "window.focus_changed").
type EventType string

// Event is a single typed event received from the EventStream.
type Event struct {
	Type      EventType `json:"type"`
	Timestamp string    `json:"timestamp"`
	// Payload is a typed struct (e.g. WindowFocusChangedPayload) for known types,
	// or json.RawMessage for unknown types.
	Payload interface{} `json:"payload"`
}

// ── Typed payload structs ─────────────────────────────────────────────────────

// WindowFocusChangedPayload is the payload for "window.focus_changed".
type WindowFocusChangedPayload struct {
	WindowID string `json:"window_id"`
	PID      string `json:"pid"`
	Title    string `json:"title"`
}

// WindowOpenedPayload is the payload for "window.opened".
type WindowOpenedPayload struct {
	WindowID string `json:"window_id"`
	PID      string `json:"pid"`
	Title    string `json:"title"`
}

// WindowClosedPayload is the payload for "window.closed".
type WindowClosedPayload struct {
	WindowID string `json:"window_id"`
	PID      string `json:"pid"`
	Title    string `json:"title"`
}

// ClipboardChangedPayload is the payload for "clipboard.changed".
type ClipboardChangedPayload struct {
	SizeBytes int `json:"size_bytes"`
}

// FileCreatedPayload is the payload for "file.created".
type FileCreatedPayload struct {
	Path string `json:"path"`
}

// FileModifiedPayload is the payload for "file.modified".
type FileModifiedPayload struct {
	Path string `json:"path"`
}

// FileDeletedPayload is the payload for "file.deleted".
type FileDeletedPayload struct {
	Path string `json:"path"`
}

// ProcessStartedPayload is the payload for "process.started".
type ProcessStartedPayload struct {
	PID  int    `json:"pid"`
	Cmd  string `json:"cmd"`
	PPID string `json:"ppid"`
}

// ProcessStoppedPayload is the payload for "process.stopped".
type ProcessStoppedPayload struct {
	PID int    `json:"pid"`
	Cmd string `json:"cmd"`
}

// IdleInactivePayload is the payload for "idle.inactive".
type IdleInactivePayload struct {
	IdleMs int `json:"idle_ms"`
}

// IdleActivePayload is the payload for "idle.active".
type IdleActivePayload struct {
	IdleMs int `json:"idle_ms"`
}

// ProducerUnavailablePayload is the payload for "producer.unavailable".
type ProducerUnavailablePayload struct {
	Producer EventProducer `json:"producer"`
	Reason   string        `json:"reason"`
}

// ─── ComputerStatus extras ────────────────────────────────────────────────────

const (
	// StatusProvisioning is the initial provisioning state.
	StatusProvisioning ComputerStatus = "provisioning"
	// StatusActive is an alias for StatusRunning for SDK parity.
	StatusActive ComputerStatus = "active"
	// StatusPaused is the paused/hibernated state.
	StatusPaused ComputerStatus = "paused"
)

// ─── Deployments (Phase 2A attribution + Phase 2B/3 publish surface) ─────────

// DeploymentState is the lifecycle state of a Deployment.
type DeploymentState string

const (
	DeploymentStatePending  DeploymentState = "pending"
	DeploymentStateBuilding DeploymentState = "building"
	DeploymentStateRunning  DeploymentState = "running"
	DeploymentStateStopped  DeploymentState = "stopped"
	DeploymentStateFailed   DeploymentState = "failed"
)

// DeploymentVersionKind is "static" | "dynamic" | "sandbox_backed".
type DeploymentVersionKind string

const (
	DeploymentVersionStatic        DeploymentVersionKind = "static"
	DeploymentVersionDynamic       DeploymentVersionKind = "dynamic"
	DeploymentVersionSandboxBacked DeploymentVersionKind = "sandbox_backed"
)

// DeploymentVersionState — lifecycle of a single publish.
type DeploymentVersionState string

const (
	DeploymentVersionStateCreated  DeploymentVersionState = "created"
	DeploymentVersionStateBuilding DeploymentVersionState = "building"
	DeploymentVersionStateReady    DeploymentVersionState = "ready"
	DeploymentVersionStateFailed   DeploymentVersionState = "failed"
	DeploymentVersionStateArchived DeploymentVersionState = "archived"
)

// DeploymentSourceType — where a deployment's code comes from.
type DeploymentSourceType string

const (
	DeploymentSourceRepo    DeploymentSourceType = "repo"
	DeploymentSourceSandbox DeploymentSourceType = "sandbox"
	DeploymentSourceUpload  DeploymentSourceType = "upload"
)

// DeploymentServiceType — one row per web/api/worker/cron/etc.
type DeploymentServiceType string

const (
	DeploymentServiceStaticWeb DeploymentServiceType = "static_web"
	DeploymentServiceWeb       DeploymentServiceType = "web"
	DeploymentServiceAPI       DeploymentServiceType = "api"
	DeploymentServiceFunction  DeploymentServiceType = "function"
	DeploymentServiceWorker    DeploymentServiceType = "worker"
	DeploymentServiceCron      DeploymentServiceType = "cron"
	DeploymentServicePostgres  DeploymentServiceType = "postgres"
	DeploymentServiceRedis     DeploymentServiceType = "redis"
	DeploymentServiceBucket    DeploymentServiceType = "bucket"
	DeploymentServiceVolume    DeploymentServiceType = "volume"
)

// RuntimeInstanceState — running production VM state.
type RuntimeInstanceState string

const (
	RuntimeInstancePending   RuntimeInstanceState = "pending"
	RuntimeInstanceStarting  RuntimeInstanceState = "starting"
	RuntimeInstanceHealthy   RuntimeInstanceState = "healthy"
	RuntimeInstanceUnhealthy RuntimeInstanceState = "unhealthy"
	RuntimeInstanceStopping  RuntimeInstanceState = "stopping"
	RuntimeInstanceStoppedRI RuntimeInstanceState = "stopped"
	RuntimeInstanceFailed    RuntimeInstanceState = "failed"
)

// DeploymentData — Stable production object for a published app/site/API.
type DeploymentData struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	OwnerID  string `json:"owner_id,omitempty"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	// Deprecated: repo-based model. New deployments use SourceType="sandbox".
	RepoURL             string                 `json:"repo_url,omitempty"`
	RepoProvider        string                 `json:"repo_provider,omitempty"`
	Branch              string                 `json:"branch,omitempty"`
	BuildCommand        string                 `json:"build_command,omitempty"`
	RunCommand          string                 `json:"run_command,omitempty"`
	RuntimeImage        string                 `json:"runtime_image,omitempty"`
	CurrentBuildID      string                 `json:"current_build_id,omitempty"`
	ActiveVersionID     string                 `json:"active_version_id,omitempty"`
	SourceType          DeploymentSourceType   `json:"source_type,omitempty"`
	State               DeploymentState        `json:"state"`
	AutoDeploy          bool                   `json:"auto_deploy,omitempty"`
	CustomDomainID      string                 `json:"custom_domain_id,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	ExternalWorkspaceID string                 `json:"external_workspace_id,omitempty"`
	ExternalUserID      string                 `json:"external_user_id,omitempty"`
	ExternalProjectID   string                 `json:"external_project_id,omitempty"`
	PublicURL           string                 `json:"public_url,omitempty"`
	CreatedAt           string                 `json:"created_at,omitempty"`
	UpdatedAt           string                 `json:"updated_at,omitempty"`
}

// DeploymentVersionData — Immutable history record of one publish.
type DeploymentVersionData struct {
	ID                  string                 `json:"id"`
	DeploymentID        string                 `json:"deployment_id"`
	TenantID            string                 `json:"tenant_id"`
	CreatedBy           string                 `json:"created_by,omitempty"`
	SourceSandboxID     string                 `json:"source_sandbox_id,omitempty"`
	BuildID             string                 `json:"build_id,omitempty"`
	VersionNumber       int                    `json:"version_number"`
	Kind                DeploymentVersionKind  `json:"kind"`
	State               DeploymentVersionState `json:"state"`
	ArtifactURI         string                 `json:"artifact_uri,omitempty"`
	ArtifactManifest    map[string]interface{} `json:"artifact_manifest,omitempty"`
	ArtifactSHA256      string                 `json:"artifact_sha256,omitempty"`
	RuntimeImage        string                 `json:"runtime_image,omitempty"`
	RuntimeCommand      string                 `json:"runtime_command,omitempty"`
	RuntimePort         int                    `json:"runtime_port,omitempty"`
	HealthCheckPath     string                 `json:"health_check_path,omitempty"`
	BuildLogURI         string                 `json:"build_log_uri,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	PromotedAt          string                 `json:"promoted_at,omitempty"`
	ArchivedAt          string                 `json:"archived_at,omitempty"`
	ExternalWorkspaceID string                 `json:"external_workspace_id,omitempty"`
	ExternalUserID      string                 `json:"external_user_id,omitempty"`
	ExternalProjectID   string                 `json:"external_project_id,omitempty"`
	CreatedAt           string                 `json:"created_at,omitempty"`
	UpdatedAt           string                 `json:"updated_at,omitempty"`
}

// DeploymentReleaseData — Immutable build artifact (static tarball, OCI, rootfs).
type DeploymentReleaseData struct {
	ID                  string                 `json:"id"`
	DeploymentVersionID string                 `json:"deployment_version_id"`
	ServiceID           string                 `json:"service_id,omitempty"`
	TenantID            string                 `json:"tenant_id"`
	Kind                string                 `json:"kind"`
	StorageURI          string                 `json:"storage_uri"`
	SHA256              string                 `json:"sha256"`
	SizeBytes           int64                  `json:"size_bytes"`
	StartCommand        string                 `json:"start_command,omitempty"`
	Port                int                    `json:"port,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt           string                 `json:"created_at,omitempty"`
}

// DeploymentServiceData — one row per web/api/worker within a deployment.
type DeploymentServiceData struct {
	ID              string                 `json:"id"`
	DeploymentID    string                 `json:"deployment_id"`
	EnvironmentID   string                 `json:"environment_id,omitempty"`
	TenantID        string                 `json:"tenant_id"`
	Type            DeploymentServiceType  `json:"type"`
	Name            string                 `json:"name,omitempty"`
	DesiredReplicas int                    `json:"desired_replicas,omitempty"`
	State           string                 `json:"state"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// RuntimeInstanceData — running production VM serving a dynamic release.
type RuntimeInstanceData struct {
	ID              string                 `json:"id"`
	ServiceID       string                 `json:"service_id"`
	ReleaseID       string                 `json:"release_id"`
	TenantID        string                 `json:"tenant_id"`
	HostID          string                 `json:"host_id,omitempty"`
	State           RuntimeInstanceState   `json:"state"`
	IPAddress       string                 `json:"ip_address,omitempty"`
	Port            int                    `json:"port,omitempty"`
	HealthCheckPath string                 `json:"health_check_path,omitempty"`
	StartedAt       string                 `json:"started_at,omitempty"`
	StoppedAt       string                 `json:"stopped_at,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// DeploymentBuildData — legacy GitHub-repo build attempt.
type DeploymentBuildData struct {
	ID                  string `json:"id"`
	DeploymentID        string `json:"deployment_id"`
	CommitSHA           string `json:"commit_sha,omitempty"`
	CommitMessage       string `json:"commit_message,omitempty"`
	TriggeredBy         string `json:"triggered_by,omitempty"`
	State               string `json:"state"`
	StartedAt           string `json:"started_at,omitempty"`
	FinishedAt          string `json:"finished_at,omitempty"`
	DurationMS          int64  `json:"duration_ms,omitempty"`
	LogURL              string `json:"log_url,omitempty"`
	ImageDigest         string `json:"image_digest,omitempty"`
	ErrorMessage        string `json:"error_message,omitempty"`
	ExternalWorkspaceID string `json:"external_workspace_id,omitempty"`
	ExternalUserID      string `json:"external_user_id,omitempty"`
	ExternalProjectID   string `json:"external_project_id,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
}

// PublishResult — response from POST /deployments/:id/publish.
type PublishResult struct {
	Deployment DeploymentData          `json:"deployment"`
	Version    DeploymentVersionData   `json:"version"`
	Services   []DeploymentServiceData `json:"services,omitempty"`
	Promoted   bool                    `json:"promoted"`
}

// ─── Internal helpers ────────────────────────────────────────────────────────

// apiResponse is the generic envelope returned by non-data list endpoints.
type apiResponse[T any] struct {
	Data T `json:"data"`
}

// actionResponse is the envelope for simple action endpoints.
type actionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
