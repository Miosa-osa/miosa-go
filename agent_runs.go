package miosa

import (
	"context"
	"fmt"
)

// AgentRunsService dispatches prompts to MIOSA runtime targets and returns
// durable agent-run records.
type AgentRunsService struct {
	client *Client
}

// AgentRunStatus is the lifecycle state of a durable agent run.
type AgentRunStatus string

const (
	AgentRunRunning   AgentRunStatus = "running"
	AgentRunSucceeded AgentRunStatus = "succeeded"
	AgentRunFailed    AgentRunStatus = "failed"
	AgentRunCanceled  AgentRunStatus = "canceled"
)

// AgentRunTargetKind identifies the runtime target family for an agent run.
type AgentRunTargetKind string

const (
	AgentRunTargetSandbox  AgentRunTargetKind = "sandbox"
	AgentRunTargetComputer AgentRunTargetKind = "computer"
)

// AgentRunData is the API representation of a persisted prompt dispatch.
type AgentRunData struct {
	ID           string                 `json:"id"`
	TenantID     string                 `json:"tenant_id,omitempty"`
	UserID       string                 `json:"user_id,omitempty"`
	TargetKind   AgentRunTargetKind     `json:"target_kind"`
	TargetID     string                 `json:"target_id"`
	TargetName   string                 `json:"target_name,omitempty"`
	Provider     string                 `json:"provider"`
	Model        string                 `json:"model,omitempty"`
	Prompt       string                 `json:"prompt"`
	Status       AgentRunStatus         `json:"status"`
	Output       string                 `json:"output,omitempty"`
	Stderr       string                 `json:"stderr,omitempty"`
	ExitCode     *int                   `json:"exit_code,omitempty"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	StartedAt    string                 `json:"started_at,omitempty"`
	FinishedAt   string                 `json:"finished_at,omitempty"`
	CreatedAt    string                 `json:"created_at,omitempty"`
	UpdatedAt    string                 `json:"updated_at,omitempty"`
}

// CreateAgentRunInput is the request body for POST /agent-runs.
type CreateAgentRunInput struct {
	TargetID       string                 `json:"target_id,omitempty"`
	SandboxID      string                 `json:"sandbox_id,omitempty"`
	ComputerID     string                 `json:"computer_id,omitempty"`
	TargetKind     AgentRunTargetKind     `json:"target_kind,omitempty"`
	Provider       string                 `json:"provider,omitempty"`
	Prompt         string                 `json:"prompt"`
	Instruction    string                 `json:"instruction,omitempty"`
	Command        string                 `json:"command,omitempty"`
	Model          string                 `json:"model,omitempty"`
	Cwd            string                 `json:"cwd,omitempty"`
	Timeout        int                    `json:"timeout,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	IdempotencyKey string                 `json:"-"`
}

// ListAgentRunsInput filters GET /agent-runs.
type ListAgentRunsInput struct {
	TargetID   string
	SandboxID  string
	ComputerID string
	Limit      int
}

// AgentRunListResponse wraps recent agent runs.
type AgentRunListResponse struct {
	Data []AgentRunData `json:"data"`
	Runs []AgentRunData `json:"runs,omitempty"`
}

type agentRunResponse struct {
	Data *AgentRunData `json:"data"`
	Run  *AgentRunData `json:"run"`
}

// Create dispatches a prompt to a sandbox/computer and returns its run record.
func (s *AgentRunsService) Create(ctx context.Context, input CreateAgentRunInput) (*AgentRunData, error) {
	if input.TargetID == "" {
		if input.SandboxID != "" {
			input.TargetID = input.SandboxID
		} else if input.ComputerID != "" {
			input.TargetID = input.ComputerID
		}
	}

	var out agentRunResponse
	if err := s.client.postJSONIdempotent(ctx, "/agent-runs", input, &out, input.IdempotencyKey); err != nil {
		return nil, fmt.Errorf("AgentRunsService.Create: %w", err)
	}
	run := unwrapAgentRun(out)
	if run == nil {
		return nil, fmt.Errorf("AgentRunsService.Create: empty agent run response")
	}
	return run, nil
}

// Run is an alias for Create, matching the product language.
func (s *AgentRunsService) Run(ctx context.Context, input CreateAgentRunInput) (*AgentRunData, error) {
	return s.Create(ctx, input)
}

// List returns recent agent runs for the authenticated tenant.
func (s *AgentRunsService) List(ctx context.Context, input ListAgentRunsInput) (*AgentRunListResponse, error) {
	targetID := input.TargetID
	if targetID == "" {
		if input.SandboxID != "" {
			targetID = input.SandboxID
		} else if input.ComputerID != "" {
			targetID = input.ComputerID
		}
	}

	params := map[string]string{"target_id": targetID}
	if input.Limit > 0 {
		params["limit"] = fmt.Sprintf("%d", input.Limit)
	}

	var out AgentRunListResponse
	if err := s.client.getJSON(ctx, "/agent-runs"+buildQuery(params), &out); err != nil {
		return nil, fmt.Errorf("AgentRunsService.List: %w", err)
	}
	if out.Data == nil && out.Runs != nil {
		out.Data = out.Runs
	}
	return &out, nil
}

// Get fetches one persisted agent run by ID.
func (s *AgentRunsService) Get(ctx context.Context, runID string) (*AgentRunData, error) {
	var out agentRunResponse
	if err := s.client.getJSON(ctx, "/agent-runs/"+runID, &out); err != nil {
		return nil, fmt.Errorf("AgentRunsService.Get: %w", err)
	}
	run := unwrapAgentRun(out)
	if run == nil {
		return nil, fmt.Errorf("AgentRunsService.Get: empty agent run response")
	}
	return run, nil
}

func unwrapAgentRun(out agentRunResponse) *AgentRunData {
	if out.Data != nil {
		return out.Data
	}
	return out.Run
}
