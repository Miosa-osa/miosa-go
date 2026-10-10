package miosa

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
	"unicode/utf8"
)

// Wire contract: miosa-compute docs/api/setup-and-scripts.md and
// docs/api/environments.md ("Machine JSON").

// Setup limits the API enforces; the SDK checks them before sending so a bad
// request fails without a round trip.
const (
	// MaxSetupFileBytes is the size limit of a setup file (64 KiB).
	MaxSetupFileBytes = 65536
	// MaxCommandBytes is the size limit of a command (64 KiB).
	MaxCommandBytes = 65536
	// MaxCommandStdinBytes is the size limit of a command's stdin (32 KiB).
	MaxCommandStdinBytes = 32768
	// MinCommandTimeoutSeconds and MaxCommandTimeoutSeconds bound
	// timeout_seconds on the commands endpoint.
	MinCommandTimeoutSeconds = 1
	MaxCommandTimeoutSeconds = 600
	// DefaultCommandTimeoutSeconds is what the API applies when none is sent.
	DefaultCommandTimeoutSeconds = 60
	// MaxAgentPromptBytes is the size limit of a queued agent prompt.
	MaxAgentPromptBytes = 65536
)

// SetupStatus is the state of machine setup (the setup file and the
// environment steps together).
type SetupStatus string

const (
	SetupPending SetupStatus = "pending"
	SetupRunning SetupStatus = "running"
	SetupDone    SetupStatus = "done"
	SetupFailed  SetupStatus = "failed"
)

// EnvironmentStatus is the state of environment delivery on a machine.
type EnvironmentStatus string

const (
	EnvironmentPending  EnvironmentStatus = "pending"
	EnvironmentApplying EnvironmentStatus = "applying"
	EnvironmentSecuring EnvironmentStatus = "securing"
	EnvironmentApplied  EnvironmentStatus = "applied"
	EnvironmentFailed   EnvironmentStatus = "failed"
)

// MachineEnvironment is the environment block carried by every sandbox and
// computer JSON. Every field is empty for a machine that predates environments.
type MachineEnvironment struct {
	Environment                 string            `json:"environment,omitempty"`
	EnvironmentID               string            `json:"environment_id,omitempty"`
	EnvironmentVersion          int               `json:"environment_version,omitempty"`
	EnvironmentLatestVersion    int               `json:"environment_latest_version,omitempty"`
	EnvironmentBaseID           string            `json:"environment_base_id,omitempty"`
	EnvironmentBaseVersion      int               `json:"environment_base_version,omitempty"`
	EnvironmentUpgradeAvailable bool              `json:"environment_upgrade_available,omitempty"`
	EnvironmentProtected        bool              `json:"environment_protected,omitempty"`
	EnvironmentStatus           EnvironmentStatus `json:"environment_status,omitempty"`
	EnvironmentError            string            `json:"environment_error,omitempty"`
}

// SetupRepoStatus is the setup status of one repository script.
type SetupRepoStatus struct {
	Repo       string      `json:"repo"`
	Path       string      `json:"path,omitempty"`
	Blocking   bool        `json:"blocking"`
	Status     SetupStatus `json:"status"`
	Error      string      `json:"error,omitempty"`
	StartedAt  string      `json:"started_at,omitempty"`
	FinishedAt string      `json:"finished_at,omitempty"`
}

// SetupStepStatus is the setup status of the environment-level script or of
// the CLI tool installs. Tools is set only for the latter.
type SetupStepStatus struct {
	Blocking   bool              `json:"blocking"`
	Status     SetupStatus       `json:"status"`
	Error      string            `json:"error,omitempty"`
	Tools      map[string]string `json:"tools,omitempty"`
	StartedAt  string            `json:"started_at,omitempty"`
	FinishedAt string            `json:"finished_at,omitempty"`
}

// MachineSetup is the setup block carried by every sandbox and computer JSON.
// SetupStatus is empty when the machine has no setup. The script itself is
// never returned.
type MachineSetup struct {
	SetupStatus          SetupStatus       `json:"setup_status,omitempty"`
	SetupError           string            `json:"setup_error,omitempty"`
	SetupFileStatus      SetupStatus       `json:"setup_file_status,omitempty"`
	SetupFileError       string            `json:"setup_file_error,omitempty"`
	SetupStartedAt       string            `json:"setup_started_at,omitempty"`
	SetupFinishedAt      string            `json:"setup_finished_at,omitempty"`
	SetupRepos           []SetupRepoStatus `json:"setup_repos,omitempty"`
	SetupEnvironmentStep *SetupStepStatus  `json:"setup_environment,omitempty"`
	SetupCLITools        *SetupStepStatus  `json:"setup_cli_tools,omitempty"`
}

// SetupFinished reports whether setup reached a final state (done or failed).
// A machine with no setup counts as finished.
func (m MachineSetup) SetupFinished() bool {
	return m.SetupStatus == "" || m.SetupStatus == SetupDone || m.SetupStatus == SetupFailed
}

// MachineEnvironmentOptions picks the environment of a machine operation
// (resume, fork, start, clone, restore, deploy of a named snapshot) and adds
// per-machine variables. All fields are optional.
type MachineEnvironmentOptions struct {
	// Environment names an environment; EnvironmentID picks one by id.
	// Neither means the operation's default (see environments.md).
	Environment   string `json:"environment,omitempty"`
	EnvironmentID string `json:"environment_id,omitempty"`
	// NoEnv gives the machine nothing of the owner's, permanently.
	NoEnv bool `json:"no_env,omitempty"`
	// Env sets per-machine variables on top of the environment's.
	Env map[string]string `json:"env,omitempty"`
}

// ValidateSetupFile checks a setup file against the API limits: UTF-8, at most
// MaxSetupFileBytes, no NUL byte.
func ValidateSetupFile(script string) error {
	if len(script) > MaxSetupFileBytes {
		return fmt.Errorf("setup_file is %d bytes; the limit is %d", len(script), MaxSetupFileBytes)
	}
	if !utf8.ValidString(script) {
		return errors.New("setup_file must be UTF-8 text")
	}
	for i := 0; i < len(script); i++ {
		if script[i] == 0 {
			return errors.New("setup_file must not contain a NUL byte")
		}
	}
	return nil
}

// ─── Commands ─────────────────────────────────────────────────────────────────

// CommandInput describes one synchronous command for the commands endpoint.
type CommandInput struct {
	// Command is required, at most MaxCommandBytes.
	Command string `json:"command"`
	// Cwd is absolute, or relative to the work directory (/workspace for a
	// sandbox, the command user's home for a computer).
	Cwd string `json:"cwd,omitempty"`
	// TimeoutSeconds is 1 to 600; 0 uses the API default (60). A larger value
	// is refused by the API (422 TIMEOUT_TOO_LONG), not clamped: run longer
	// work detached.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
	// Stdin is text for the command's standard input, at most 32 KiB.
	Stdin string `json:"stdin,omitempty"`

	// WaitUntilReady, when positive, makes the call retry the retryable
	// "still starting" refusal (SANDBOX_STARTING, COMPUTER_STARTING) for up to
	// this long, honoring the server's Retry-After hint. Zero returns the
	// refusal once the client's own retries (WithMaxRetries) are spent.
	WaitUntilReady time.Duration `json:"-"`
}

// Validate checks the input without sending anything.
func (in CommandInput) Validate() error {
	if in.Command == "" {
		return errors.New("command is required")
	}
	if len(in.Command) > MaxCommandBytes {
		return fmt.Errorf("command is %d bytes; the limit is %d", len(in.Command), MaxCommandBytes)
	}
	if in.TimeoutSeconds < 0 || in.TimeoutSeconds > MaxCommandTimeoutSeconds {
		return fmt.Errorf("timeout_seconds must be between %d and %d", MinCommandTimeoutSeconds, MaxCommandTimeoutSeconds)
	}
	if len(in.Stdin) > MaxCommandStdinBytes {
		return fmt.Errorf("stdin is %d bytes; the limit is %d", len(in.Stdin), MaxCommandStdinBytes)
	}
	return nil
}

// CommandResult is the outcome of a synchronous command.
type CommandResult struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
	// TimedOut is true, with ExitCode -1, when the command was killed at its deadline.
	TimedOut bool `json:"timed_out"`
	// StdoutCut and StderrCut are set when the guest cut the output short.
	StdoutCut bool `json:"stdout_truncated,omitempty"`
	StderrCut bool `json:"stderr_truncated,omitempty"`
}

// AgentPromptQueued is the acknowledgement of a queued agent prompt.
type AgentPromptQueued struct {
	Queued     bool   `json:"queued"`
	AfterRunID string `json:"after_run_id,omitempty"`
	Runner     string `json:"runner,omitempty"`
}

// defaultStartingWait is the wait between retries of a "starting" refusal when
// the server gave no hint.
const defaultStartingWait = 2 * time.Second

func (c *Client) runMachineCommand(ctx context.Context, kind, id string, in CommandInput) (*CommandResult, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	path := "/" + kind + "/" + url.PathEscape(id) + "/commands"
	deadline := time.Now().Add(in.WaitUntilReady)
	for {
		var out apiResponse[CommandResult]
		err := c.postJSON(ctx, path, in, &out)
		if err == nil {
			return &out.Data, nil
		}
		if in.WaitUntilReady <= 0 || !IsStarting(err) {
			return nil, err
		}
		wait, ok := RetryAfter(err)
		if !ok {
			wait = defaultStartingWait
		}
		if time.Now().Add(wait).After(deadline) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

func (c *Client) queueMachineAgentPrompt(ctx context.Context, kind, id, prompt string) (*AgentPromptQueued, error) {
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if len(prompt) > MaxAgentPromptBytes {
		return nil, fmt.Errorf("prompt is %d bytes; the limit is %d", len(prompt), MaxAgentPromptBytes)
	}
	var out apiResponse[AgentPromptQueued]
	body := map[string]string{"prompt": prompt}
	if err := c.postJSON(ctx, "/"+kind+"/"+url.PathEscape(id)+"/agent/prompts", body, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

func (c *Client) stopMachineAgent(ctx context.Context, kind, id string) (*Run, error) {
	var out apiResponse[Run]
	if err := c.postJSON(ctx, "/"+kind+"/"+url.PathEscape(id)+"/agent/stop", map[string]interface{}{}, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// RunCommand runs a command synchronously in a sandbox through
// POST /sandboxes/:id/commands. A sandbox that is still starting is refused
// with a retryable *ResourceStartingError; set CommandInput.WaitUntilReady to
// have the SDK wait it out.
func (s *SandboxesService) RunCommand(ctx context.Context, id string, in CommandInput) (*CommandResult, error) {
	return s.client.runMachineCommand(ctx, "sandboxes", id, in)
}

// QueueAgentPrompt sends a message to the agent run attached to the sandbox, as
// the next turn of its chat. It fails with 409 NO_AGENT_RUN when none is attached.
func (s *SandboxesService) QueueAgentPrompt(ctx context.Context, id, prompt string) (*AgentPromptQueued, error) {
	return s.client.queueMachineAgentPrompt(ctx, "sandboxes", id, prompt)
}

// StopAgent halts the agent run attached to the sandbox and returns the canceled run.
func (s *SandboxesService) StopAgent(ctx context.Context, id string) (*Run, error) {
	return s.client.stopMachineAgent(ctx, "sandboxes", id)
}

// RunCommand runs a command synchronously in a computer through
// POST /computers/:id/commands.
func (s *ComputersService) RunCommand(ctx context.Context, id string, in CommandInput) (*CommandResult, error) {
	return s.client.runMachineCommand(ctx, "computers", id, in)
}

// QueueAgentPrompt sends a message to the agent run attached to the computer.
func (s *ComputersService) QueueAgentPrompt(ctx context.Context, id, prompt string) (*AgentPromptQueued, error) {
	return s.client.queueMachineAgentPrompt(ctx, "computers", id, prompt)
}

// StopAgent halts the agent run attached to the computer.
func (s *ComputersService) StopAgent(ctx context.Context, id string) (*Run, error) {
	return s.client.stopMachineAgent(ctx, "computers", id)
}

// RunCommand runs a command synchronously in this computer.
func (c *Computer) RunCommand(ctx context.Context, in CommandInput) (*CommandResult, error) {
	return c.client.runMachineCommand(ctx, "computers", c.ID, in)
}

// QueueAgentPrompt sends a message to the agent run attached to this computer.
func (c *Computer) QueueAgentPrompt(ctx context.Context, prompt string) (*AgentPromptQueued, error) {
	return c.client.queueMachineAgentPrompt(ctx, "computers", c.ID, prompt)
}

// StopAgent halts the agent run attached to this computer.
func (c *Computer) StopAgent(ctx context.Context) (*Run, error) {
	return c.client.stopMachineAgent(ctx, "computers", c.ID)
}

// ─── Waiting for setup ────────────────────────────────────────────────────────

// WaitForSetupOptions tunes WaitForSetup.
type WaitForSetupOptions struct {
	// Timeout bounds the wait. Defaults to 10 minutes.
	Timeout time.Duration
	// PollInterval defaults to 2 seconds.
	PollInterval time.Duration
}

// WaitForSetup polls a sandbox until its setup (setup file and environment
// steps) reaches done or failed, and returns the sandbox. A sandbox with no
// setup returns at once. A failed setup is returned without an error: read
// SetupStatus and SetupError.
func (s *SandboxesService) WaitForSetup(ctx context.Context, id string, opts WaitForSetupOptions) (*Sandbox, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}
	deadline := time.Now().Add(opts.Timeout)
	for {
		sb, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if sb.SetupFinished() {
			return sb, nil
		}
		if time.Now().Add(opts.PollInterval).After(deadline) {
			return sb, fmt.Errorf("sandbox %s setup still %s after %s", id, sb.SetupStatus, opts.Timeout)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(opts.PollInterval):
		}
	}
}
