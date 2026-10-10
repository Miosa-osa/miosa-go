package miosa

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RunsService dispatches and inspects runs against MIOSA sandboxes and computers.
type RunsService struct {
	client *Client
}

type RunTargetKind string

const (
	RunTargetSandbox  RunTargetKind = "sandbox"
	RunTargetComputer RunTargetKind = "computer"
)

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

type Run struct {
	ID                  string                 `json:"id"`
	RunGroupID          string                 `json:"run_group_id,omitempty"`
	ParentRunID         string                 `json:"parent_run_id,omitempty"`
	OrchestrationRole   string                 `json:"orchestration_role,omitempty"`
	ExternalWorkspaceID string                 `json:"external_workspace_id,omitempty"`
	ExternalUserID      string                 `json:"external_user_id,omitempty"`
	ExternalProjectID   string                 `json:"external_project_id,omitempty"`
	TargetKind          RunTargetKind          `json:"target_kind"`
	TargetID            string                 `json:"target_id"`
	Runner              string                 `json:"runner"`
	Provider            string                 `json:"provider,omitempty"`
	Instruction         string                 `json:"instruction"`
	Status              RunStatus              `json:"status"`
	Result              map[string]interface{} `json:"result,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	StartedAt           string                 `json:"started_at,omitempty"`
	FinishedAt          string                 `json:"finished_at,omitempty"`
	CreatedAt           string                 `json:"created_at,omitempty"`
	UpdatedAt           string                 `json:"updated_at,omitempty"`

	// Chat and harness session (docs/api/harness-sessions.md).
	ChatID             string `json:"chat_id,omitempty"`
	HarnessSessionID   string `json:"harness_session_id,omitempty"`
	ContinuedFromRunID string `json:"continued_from_run_id,omitempty"`
	// Overdrive is true when the harness ran with its approval bypass, false
	// when it did not, nil for runs that are not harness runs.
	Overdrive *bool `json:"overdrive,omitempty"`
	// SessionMode is "new", "resumed" or "reseeded"; "" for non-harness runs.
	SessionMode string `json:"session_mode,omitempty"`
	// Context is the harness context meter once the run finished.
	Context map[string]interface{} `json:"context,omitempty"`

	// Agent binding and origin.
	AgentDefinitionID string `json:"agent_definition_id,omitempty"`
	AgentVersionID    string `json:"agent_version_id,omitempty"`
	// Source is console, api, workflow, trigger, test or a client value.
	Source string `json:"source,omitempty"`
	// Machine is the platform-provisioned machine lease, nil for a run on a
	// caller-named target.
	Machine *RunMachine `json:"machine,omitempty"`
	// Cost is the MIOSA credits the run used; nil until the run finishes.
	Cost   *RunCost   `json:"cost,omitempty"`
	Notify *RunNotify `json:"notify,omitempty"`
	// ProviderCost is the user's own provider spend, set when a Claude Code,
	// Codex or OSA run finishes. Nil while the run is queued or running.
	ProviderCost *ProviderCost `json:"provider_cost,omitempty"`
}

// RunMachine is the machine lease of a run that provisions its own machine.
type RunMachine struct {
	LeaseID string `json:"lease_id,omitempty"`
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	// Origin is "provisioned" or "bound".
	Origin string `json:"origin"`
	// State is provisioning, ready or released.
	State              string `json:"state"`
	Reused             bool   `json:"reused"`
	Reuse              string `json:"reuse,omitempty"`
	AfterRun           string `json:"after_run,omitempty"`
	IdleTimeoutSeconds int    `json:"idle_timeout_seconds,omitempty"`
	ReleaseAction      string `json:"release_action,omitempty"`
	ReleasedAt         string `json:"released_at,omitempty"`
}

// RunCost is the MIOSA credits (cents) a run used.
type RunCost struct {
	ModelCredits     *int64 `json:"model_credits"`
	MachineCredits   int64  `json:"machine_credits"`
	TotalCredits     int64  `json:"total_credits"`
	ModelMetered     bool   `json:"model_metered"`
	MachineEstimated bool   `json:"machine_estimated"`
	ComputedAt       string `json:"computed_at,omitempty"`
}

// RunNotify says who is told when a run finishes.
type RunNotify struct {
	Email     bool     `json:"email"`
	WebhookID string   `json:"webhook_id,omitempty"`
	On        []string `json:"on,omitempty"`
}

// ProviderCost is a run's spend on the user's own model provider key or
// subscription (docs/api/agent-usage-and-cost.md).
type ProviderCost struct {
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`
	// EstimatedUSD is nil for a subscription login and for a model the catalog
	// has no price for.
	EstimatedUSD *float64 `json:"estimated_usd"`
	// Source is "harness" (the harness reported the cost) or "catalog".
	Source string `json:"source"`
	Model  string `json:"model"`
	// BilledTo is "subscription" or "own_key".
	BilledTo string `json:"billed_to"`
}

type RunMessage struct {
	ID        string                 `json:"id,omitempty"`
	RunID     string                 `json:"run_id,omitempty"`
	Role      string                 `json:"role,omitempty"`
	Text      string                 `json:"text,omitempty"`
	Content   string                 `json:"content,omitempty"`
	Format    string                 `json:"format,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
}

type RunCommandOutput struct {
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
}

type RunFile struct {
	ID             string                 `json:"id"`
	RunID          string                 `json:"run_id,omitempty"`
	TargetKind     RunTargetKind          `json:"target_kind,omitempty"`
	TargetID       string                 `json:"target_id,omitempty"`
	Path           string                 `json:"path"`
	Name           string                 `json:"name,omitempty"`
	Kind           string                 `json:"kind,omitempty"`
	MimeType       string                 `json:"mime_type,omitempty"`
	SizeBytes      int64                  `json:"size_bytes,omitempty"`
	Sha256         string                 `json:"sha256,omitempty"`
	Status         string                 `json:"status,omitempty"`
	Persisted      bool                   `json:"persisted,omitempty"`
	StorageBackend string                 `json:"storage_backend,omitempty"`
	PersistedAt    string                 `json:"persisted_at,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      string                 `json:"created_at,omitempty"`
	UpdatedAt      string                 `json:"updated_at,omitempty"`
}

type RunActivity struct {
	ID        string                 `json:"id"`
	RunID     string                 `json:"run_id,omitempty"`
	Sequence  int64                  `json:"sequence,omitempty"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
}

type RunPreview struct {
	ID        string                 `json:"id,omitempty"`
	RunID     string                 `json:"run_id,omitempty"`
	Type      string                 `json:"type,omitempty"`
	URL       string                 `json:"url,omitempty"`
	Label     string                 `json:"label,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
}

type RunDiagnostic struct {
	ID        string                 `json:"id,omitempty"`
	RunID     string                 `json:"run_id,omitempty"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
}

type RunExpectedFile struct {
	Path     string `json:"path"`
	Name     string `json:"name,omitempty"`
	Kind     string `json:"kind,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Preview  bool   `json:"preview,omitempty"`
}

type RunExpectedOutputs struct {
	Messages bool                   `json:"messages,omitempty"`
	Files    []RunExpectedFile      `json:"files,omitempty"`
	Previews interface{}            `json:"previews,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type RunCreateInput struct {
	Instruction             string                 `json:"instruction"`
	TargetKind              RunTargetKind          `json:"target_kind,omitempty"`
	TargetID                string                 `json:"target_id,omitempty"`
	SandboxID               string                 `json:"sandbox_id,omitempty"`
	ComputerID              string                 `json:"computer_id,omitempty"`
	Runner                  string                 `json:"runner,omitempty"`
	Provider                string                 `json:"provider,omitempty"`
	Model                   string                 `json:"model,omitempty"`
	Command                 string                 `json:"command,omitempty"`
	RuntimeCommand          string                 `json:"runtime_command,omitempty"`
	Cwd                     string                 `json:"cwd,omitempty"`
	Timeout                 int                    `json:"timeout,omitempty"`
	Wait                    bool                   `json:"wait,omitempty"`
	Env                     map[string]string      `json:"env,omitempty"`
	AgentRuntimeProfileID   string                 `json:"agent_runtime_profile_id,omitempty"`
	AgentProfileID          string                 `json:"agent_profile_id,omitempty"`
	RunGroupID              string                 `json:"run_group_id,omitempty"`
	ParentRunID             string                 `json:"parent_run_id,omitempty"`
	OrchestrationRole       string                 `json:"orchestration_role,omitempty"`
	ExternalWorkspaceID     string                 `json:"external_workspace_id,omitempty"`
	ExternalUserID          string                 `json:"external_user_id,omitempty"`
	ExternalProjectID       string                 `json:"external_project_id,omitempty"`
	SkipAgentRuntimeProfile bool                   `json:"skip_agent_runtime_profile,omitempty"`
	ExecutionPacket         map[string]interface{} `json:"execution_packet,omitempty"`
	ExpectedOutputs         *RunExpectedOutputs    `json:"expected_outputs,omitempty"`
	ApprovalPolicy          map[string]interface{} `json:"approval_policy,omitempty"`
	CapabilityRequirements  []string               `json:"capability_requirements,omitempty"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`

	// Prompt path (docs/api/agents.md): an Instruction with no Runner runs on
	// the scope's default harness and its default model.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	// AgentDefinitionID and AgentVersionID bind the run to an agent.
	AgentDefinitionID string `json:"agent_definition_id,omitempty"`
	AgentVersionID    string `json:"agent_version_id,omitempty"`

	// Chat sessions (docs/api/harness-sessions.md). ChatID alone continues the
	// chat: one chat is one harness session. ContinueFromRunID names a run
	// explicitly; 409 RUN_SESSION_NOT_RESUMABLE when it cannot be resumed.
	ChatID            string `json:"chat_id,omitempty"`
	ContinueFromRunID string `json:"continue_from_run_id,omitempty"`

	// Run on a platform machine (docs/api/agents-run-anywhere.md). Target "new"
	// is mutually exclusive with SandboxID, ComputerID and TargetID.
	Target      string             `json:"target,omitempty"`
	WorkspaceID string             `json:"workspace_id,omitempty"`
	ProjectID   string             `json:"project_id,omitempty"`
	Machine     *RunMachineRequest `json:"machine,omitempty"`
	Notify      *RunNotify         `json:"notify,omitempty"`
	// Source is a client label (up to 40 characters); workflow and trigger
	// are reserved for the platform.
	Source string `json:"source,omitempty"`

	// Environment names the environment a machine started for this run gets
	// (Target "new"); EnvironmentID picks one by id. NoEnv gives it nothing of
	// the owner's.
	Environment   string `json:"environment,omitempty"`
	EnvironmentID string `json:"environment_id,omitempty"`
	NoEnv         bool   `json:"no_env,omitempty"`
}

// RunMachineRequest overrides an agent's machine policy for one run, or is the
// whole policy for a run with no agent.
type RunMachineRequest struct {
	// Target is "sandbox" (default) or "computer".
	Target     string `json:"target,omitempty"`
	ComputerID string `json:"computer_id,omitempty"`
	Template   string `json:"template,omitempty"`
	Size       string `json:"size,omitempty"`
	// Reuse is "run" (default) or "chat".
	Reuse string `json:"reuse,omitempty"`
	// AfterRun is "destroy" (default), "pause" or "keep".
	AfterRun           string `json:"after_run,omitempty"`
	IdleTimeoutSeconds int    `json:"idle_timeout_seconds,omitempty"`
	ComputerUse        *bool  `json:"computer_use,omitempty"`
	// EnvironmentID names the environment the machine starts from.
	EnvironmentID string `json:"environment_id,omitempty"`
}

type RunListInput struct {
	TargetKind          RunTargetKind
	TargetID            string
	SandboxID           string
	ComputerID          string
	RunGroupID          string
	ExternalWorkspaceID string
	ExternalUserID      string
	ExternalProjectID   string
	Status              RunStatus

	// Filters added with chats and agents.
	WorkspaceID       string
	ProjectID         string
	Runner            string
	AgentDefinitionID string
	AgentVersionID    string
	// Harness is claude-code, codex, osa or custom.
	Harness          string
	HarnessSessionID string
	// ChatID loads a chat's history: newest first, pair with Limit 200.
	ChatID string
	Source string
	Limit  int
}

type RunWaitOptions struct {
	Timeout          time.Duration
	PollInterval     time.Duration
	TerminalStatuses []RunStatus
}

func (s *RunsService) List(ctx context.Context, input RunListInput) ([]Run, error) {
	params := map[string]string{
		"target_kind":           string(input.TargetKind),
		"target_id":             input.TargetID,
		"sandbox_id":            input.SandboxID,
		"computer_id":           input.ComputerID,
		"run_group_id":          input.RunGroupID,
		"external_workspace_id": input.ExternalWorkspaceID,
		"external_user_id":      input.ExternalUserID,
		"external_project_id":   input.ExternalProjectID,
		"status":                string(input.Status),
		"workspace_id":          input.WorkspaceID,
		"project_id":            input.ProjectID,
		"runner":                input.Runner,
		"agent_definition_id":   input.AgentDefinitionID,
		"agent_version_id":      input.AgentVersionID,
		"harness":               input.Harness,
		"harness_session_id":    input.HarnessSessionID,
		"chat_id":               input.ChatID,
		"source":                input.Source,
	}
	if input.Limit > 0 {
		params["limit"] = strconv.Itoa(input.Limit)
	}
	var envelope apiResponse[[]Run]
	if err := s.client.getJSON(ctx, "/runs"+buildQuery(params), &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *RunsService) Get(ctx context.Context, id string) (*Run, error) {
	var envelope apiResponse[Run]
	if err := s.client.getJSON(ctx, "/runs/"+url.PathEscape(id), &envelope); err != nil {
		return nil, fmt.Errorf("RunsService.Get: %w", err)
	}
	return &envelope.Data, nil
}

func (s *RunsService) Run(ctx context.Context, input RunCreateInput) (*Run, error) {
	var envelope apiResponse[Run]
	if err := s.client.postJSON(ctx, "/runs", input, &envelope); err != nil {
		return nil, fmt.Errorf("RunsService.Run: %w", err)
	}
	return &envelope.Data, nil
}

func (s *RunsService) Cancel(ctx context.Context, id string) (*Run, error) {
	var envelope apiResponse[Run]
	if err := s.client.postJSON(ctx, "/runs/"+url.PathEscape(id)+"/cancel", map[string]interface{}{}, &envelope); err != nil {
		return nil, fmt.Errorf("RunsService.Cancel: %w", err)
	}
	return &envelope.Data, nil
}

func (s *RunsService) Messages(ctx context.Context, id string) ([]RunMessage, error) {
	var envelope apiResponse[[]RunMessage]
	if err := s.client.getJSON(ctx, "/runs/"+url.PathEscape(id)+"/messages", &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *RunsService) CommandOutput(ctx context.Context, id string) (*RunCommandOutput, error) {
	var envelope apiResponse[RunCommandOutput]
	if err := s.client.getJSON(ctx, "/runs/"+url.PathEscape(id)+"/command-output", &envelope); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *RunsService) Files(ctx context.Context, id string) ([]RunFile, error) {
	var envelope apiResponse[[]RunFile]
	if err := s.client.getJSON(ctx, "/runs/"+url.PathEscape(id)+"/files", &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *RunsService) Previews(ctx context.Context, id string) ([]RunPreview, error) {
	var envelope apiResponse[[]RunPreview]
	if err := s.client.getJSON(ctx, "/runs/"+url.PathEscape(id)+"/previews", &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *RunsService) Activity(ctx context.Context, id string) ([]RunActivity, error) {
	var envelope apiResponse[[]RunActivity]
	if err := s.client.getJSON(ctx, "/runs/"+url.PathEscape(id)+"/activity", &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *RunsService) Diagnostics(ctx context.Context, id string) ([]RunDiagnostic, error) {
	var envelope apiResponse[[]RunDiagnostic]
	if err := s.client.getJSON(ctx, "/runs/"+url.PathEscape(id)+"/diagnostics", &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *RunsService) DownloadFile(ctx context.Context, id, fileID string, inline bool) ([]byte, string, error) {
	path := "/runs/" + url.PathEscape(id) + "/files/" + url.PathEscape(fileID) + "/download"
	if inline {
		path += "?disposition=inline"
	}
	return s.client.getRaw(ctx, path)
}

func (s *RunsService) WaitForCompletion(ctx context.Context, id string, opts RunWaitOptions) (*Run, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 15 * time.Minute
	}
	poll := opts.PollInterval
	if poll == 0 {
		poll = 2 * time.Second
	}
	terminal := opts.TerminalStatuses
	if len(terminal) == 0 {
		terminal = []RunStatus{
			RunStatusSucceeded,
			RunStatusFailed,
			RunStatusCanceled,
		}
	}
	deadline := time.Now().Add(timeout)
	for {
		run, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if isRunTerminal(run.Status, terminal) {
			return run, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for run %s", id)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(minDuration(poll, time.Until(deadline))):
		}
	}
}

func isRunTerminal(status RunStatus, terminal []RunStatus) bool {
	got := strings.ToLower(string(status))
	for _, item := range terminal {
		if got == strings.ToLower(string(item)) {
			return true
		}
	}
	return false
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// RunUsageGroupBy is a dimension of RunUsageOptions.GroupBy.
type RunUsageGroupBy string

const (
	RunUsageByAgent     RunUsageGroupBy = "agent"
	RunUsageByChat      RunUsageGroupBy = "chat"
	RunUsageByWorkspace RunUsageGroupBy = "workspace"
	RunUsageByDay       RunUsageGroupBy = "day"
	RunUsageByModel     RunUsageGroupBy = "model"
	RunUsageByHarness   RunUsageGroupBy = "harness"
)

// RunUsageOptions selects the window, filters and grouping of Usage.
type RunUsageOptions struct {
	// GroupBy groups by every combination of the listed dimensions; empty
	// returns totals only.
	GroupBy []RunUsageGroupBy
	// From and To are ISO 8601 datetimes or dates (a date is UTC midnight). The
	// window is [From, To) on the run's finished_at; the default is the last
	// 30 days and at most 366 days are allowed.
	From string
	To   string

	AgentDefinitionID string
	WorkspaceID       string
	ChatID            string
	// Harness is claude-code, codex or osa.
	Harness string
	Model   string
	// BilledTo is "own_key" or "subscription".
	BilledTo string
	// Limit caps the groups returned (default 500, at most 2000).
	Limit int
}

// RunUsageCounters are the counters every total and group carries.
// PricedRuns + SubscriptionRuns + UnpricedRuns = Runs.
type RunUsageCounters struct {
	Runs             int     `json:"runs"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	EstimatedUSD     float64 `json:"estimated_usd"`
	PricedRuns       int     `json:"priced_runs"`
	SubscriptionRuns int     `json:"subscription_runs"`
	// UnpricedRuns are own-key runs on a model the catalog does not price:
	// their tokens are real but their cost is unknown.
	UnpricedRuns int `json:"unpriced_runs"`
}

// RunUsageGroup is one row of a grouped usage report. Key holds the grouping
// dimensions (agent_definition_id, chat_id, workspace_id, day, model,
// harness); a value is nil for runs without it.
type RunUsageGroup struct {
	Key map[string]*string `json:"key"`
	RunUsageCounters
}

// RunUsage sums provider cost over the tenant's finished runs.
type RunUsage struct {
	From    string           `json:"from"`
	To      string           `json:"to"`
	GroupBy []string         `json:"group_by"`
	Totals  RunUsageCounters `json:"totals"`
	Groups  []RunUsageGroup  `json:"groups"`
	// More is true when Limit groups came back and more may exist (the API
	// field is named truncated).
	More bool `json:"truncated"`
}

// Usage returns the user's own model spend over finished runs, overall and
// grouped (GET /runs/usage). An agent key sees only its own agents' runs.
func (s *RunsService) Usage(ctx context.Context, opts RunUsageOptions) (*RunUsage, error) {
	groups := make([]string, 0, len(opts.GroupBy))
	for _, g := range opts.GroupBy {
		groups = append(groups, string(g))
	}
	params := map[string]string{
		"group_by":            strings.Join(groups, ","),
		"from":                opts.From,
		"to":                  opts.To,
		"agent_definition_id": opts.AgentDefinitionID,
		"workspace_id":        opts.WorkspaceID,
		"chat_id":             opts.ChatID,
		"harness":             opts.Harness,
		"model":               opts.Model,
		"billed_to":           opts.BilledTo,
	}
	if opts.Limit > 0 {
		params["limit"] = strconv.Itoa(opts.Limit)
	}
	var envelope apiResponse[RunUsage]
	if err := s.client.getJSON(ctx, "/runs/usage"+buildQuery(params), &envelope); err != nil {
		return nil, fmt.Errorf("RunsService.Usage: %w", err)
	}
	return &envelope.Data, nil
}
