package miosa

import (
	"context"
	"fmt"
	"net/url"
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
