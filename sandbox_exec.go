package miosa

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Exec limits the API enforces; checked before sending.
const (
	// MinExecTimeoutSeconds and MaxExecTimeoutSeconds bound the exec timeout.
	MinExecTimeoutSeconds = 1
	MaxExecTimeoutSeconds = 86400
	// MaxExecCwdBytes is the longest working directory the API accepts.
	MaxExecCwdBytes = 4096
)

// ExecSandboxInput describes a command run through POST /sandboxes/:id/exec
// or its streaming sibling.
type ExecSandboxInput struct {
	// Command is required.
	Command string `json:"command"`
	// TimeoutSeconds is 1 to 86400. Zero sends none and the server applies its
	// default (30 s on the fast lane, an hour otherwise).
	TimeoutSeconds int `json:"timeout,omitempty"`
	// Cwd is an absolute working directory that must exist in the guest.
	Cwd string `json:"cwd,omitempty"`
	// Env adds environment variables to the command.
	Env map[string]string `json:"env,omitempty"`
	// WaitForReady makes the call wait for a starting sandbox instead of
	// failing with SANDBOX_STARTING. WaitTimeoutMS bounds that wait (the API
	// default is 30 s, at most 120 s).
	WaitForReady  bool `json:"wait,omitempty"`
	WaitTimeoutMS int  `json:"wait_timeout_ms,omitempty"`
}

// Validate checks the input without sending anything.
func (in ExecSandboxInput) Validate() error {
	if strings.TrimSpace(in.Command) == "" {
		return errors.New("command is required")
	}
	if in.TimeoutSeconds != 0 && (in.TimeoutSeconds < MinExecTimeoutSeconds || in.TimeoutSeconds > MaxExecTimeoutSeconds) {
		return fmt.Errorf("timeout must be between %d and %d seconds", MinExecTimeoutSeconds, MaxExecTimeoutSeconds)
	}
	if in.Cwd != "" {
		if !strings.HasPrefix(in.Cwd, "/") || strings.ContainsRune(in.Cwd, 0) || len(in.Cwd) > MaxExecCwdBytes {
			return errors.New("cwd must be an absolute path without NUL bytes, at most 4096 bytes")
		}
	}
	return nil
}

// SandboxExecResult is the outcome of a blocking exec.
type SandboxExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	// TimedOut is true, with ExitCode -1, when the command hit its deadline.
	TimedOut   bool  `json:"timed_out,omitempty"`
	DurationMS int64 `json:"duration_ms,omitempty"`
}

// Exec runs a command in a running sandbox and waits for it. A sandbox that is
// still starting is refused with a retryable *ResourceStartingError unless
// WaitForReady is set. The scope is sandboxes:exec.
func (s *SandboxesService) Exec(ctx context.Context, id string, in ExecSandboxInput) (*SandboxExecResult, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	var out apiResponse[SandboxExecResult]
	if err := s.client.postJSON(ctx, "/sandboxes/"+url.PathEscape(id)+"/exec", in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// ExecStreamEventType names a frame of a streamed exec.
type ExecStreamEventType string

const (
	ExecEventStdout ExecStreamEventType = "stdout"
	ExecEventStderr ExecStreamEventType = "stderr"
	// ExecEventExit is the terminal frame, always last.
	ExecEventExit ExecStreamEventType = "exit"
)

// ExecStreamError is the failure folded into a terminal frame when the stream
// ended abnormally (a transport failure rather than the command exiting).
type ExecStreamError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	RequestID string `json:"request_id,omitempty"`
}

func (e *ExecStreamError) Error() string {
	return fmt.Sprintf("miosa: exec stream ended: %s (%s)", e.Message, e.Code)
}

// SandboxExecEvent is one frame of a streamed exec.
type SandboxExecEvent struct {
	Type ExecStreamEventType
	// Line is one line of output for stdout and stderr frames.
	Line string
	// ExitCode is set on the exit frame. -1 means the command timed out and -2
	// that the stream ended abnormally (see Error).
	ExitCode *int
	TimedOut bool
	// Error is set on an abnormal exit frame.
	Error *ExecStreamError
}

// ExecStream runs a command and streams its output as server-sent events. The
// channel yields stdout and stderr lines and ends with exactly one exit frame,
// then closes. Cancel ctx to stop reading.
func (s *SandboxesService) ExecStream(ctx context.Context, id string, in ExecSandboxInput) (<-chan SandboxExecEvent, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	resp, err := s.client.withoutTimeout().doWithHeaders(ctx, http.MethodPost, "/sandboxes/"+url.PathEscape(id)+"/exec/stream",
		jsonReader(in), map[string]string{"Accept": "text/event-stream"})
	if err != nil {
		return nil, err
	}
	ch := make(chan SandboxExecEvent, 32)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
		var name string
		var data []string
		flush := func() bool {
			defer func() { name, data = "", nil }()
			if len(data) == 0 {
				return false
			}
			ev, ok := decodeExecEvent(name, strings.Join(data, "\n"))
			if !ok {
				return false
			}
			select {
			case ch <- ev:
			case <-ctx.Done():
				return true
			}
			return ev.Type == ExecEventExit
		}
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == "":
				if flush() {
					return
				}
			case strings.HasPrefix(line, "event:"):
				name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		flush()
	}()
	return ch, nil
}

func decodeExecEvent(name, data string) (SandboxExecEvent, bool) {
	switch ExecStreamEventType(name) {
	case ExecEventStdout, ExecEventStderr:
		var p struct {
			Line string `json:"line"`
		}
		if json.Unmarshal([]byte(data), &p) != nil {
			return SandboxExecEvent{}, false
		}
		return SandboxExecEvent{Type: ExecStreamEventType(name), Line: p.Line}, true
	case ExecEventExit:
		var p struct {
			ExitCode *int             `json:"exit_code"`
			TimedOut bool             `json:"timed_out"`
			Error    *ExecStreamError `json:"error"`
		}
		if json.Unmarshal([]byte(data), &p) != nil {
			return SandboxExecEvent{}, false
		}
		return SandboxExecEvent{Type: ExecEventExit, ExitCode: p.ExitCode, TimedOut: p.TimedOut, Error: p.Error}, true
	}
	return SandboxExecEvent{}, false
}

// CollectExec drains an ExecStream channel into a result. It returns an
// *ExecStreamError when the stream ended abnormally, and an error when the
// channel closed without an exit frame.
func CollectExec(events <-chan SandboxExecEvent) (*SandboxExecResult, error) {
	var out, errOut strings.Builder
	for ev := range events {
		switch ev.Type {
		case ExecEventStdout:
			out.WriteString(ev.Line)
			out.WriteByte('\n')
		case ExecEventStderr:
			errOut.WriteString(ev.Line)
			errOut.WriteByte('\n')
		case ExecEventExit:
			if ev.Error != nil {
				return nil, ev.Error
			}
			res := &SandboxExecResult{Stdout: out.String(), Stderr: errOut.String(), TimedOut: ev.TimedOut}
			if ev.ExitCode != nil {
				res.ExitCode = *ev.ExitCode
			}
			return res, nil
		}
	}
	return nil, errors.New("miosa: exec stream closed without an exit frame")
}

// GetByName fetches a sandbox by its unique name (GET /sandboxes/by-name/:name).
// A create that fails with a name conflict points here to reconcile.
func (s *SandboxesService) GetByName(ctx context.Context, name string) (*Sandbox, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	var response sandboxResponse
	if err := s.client.getJSON(ctx, "/sandboxes/by-name/"+url.PathEscape(name), &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// UpdateSandboxInput changes a sandbox's mutable identity, metadata and
// timeout policy. Nil and empty members are left alone. The external_*
// attribution fields are immutable.
type UpdateSandboxInput struct {
	Name           *string                `json:"name,omitempty"`
	Slug           *string                `json:"slug,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	AgentSessionID *string                `json:"agent_session_id,omitempty"`
	// AlwaysOn keeps the sandbox from idle auto-destroy; it also clears the idle timeout.
	AlwaysOn       *bool `json:"always_on,omitempty"`
	TimeoutSec     *int  `json:"timeout_sec,omitempty"`
	IdleTimeoutSec *int  `json:"idle_timeout_sec,omitempty"`
}

// Update changes a sandbox (PATCH /sandboxes/:id, scope sandboxes:write).
func (s *SandboxesService) Update(ctx context.Context, id string, in UpdateSandboxInput) (*Sandbox, error) {
	var response sandboxResponse
	if err := s.client.patchJSON(ctx, "/sandboxes/"+url.PathEscape(id), in, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}
