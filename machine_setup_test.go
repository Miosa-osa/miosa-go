package miosa_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func commandResult(stdout string, exit int) map[string]interface{} {
	return map[string]interface{}{"data": map[string]interface{}{
		"stdout": stdout, "stderr": "", "exit_code": exit, "duration_ms": 412, "timed_out": false,
	}}
}

func TestSandboxRunCommandSendsContractBody(t *testing.T) {
	client, rec := newFixedClient(t, 200, commandResult("ok\n", 0))

	res, err := client.Sandboxes.RunCommand(context.Background(), "sbx_1", miosa.CommandInput{
		Command: "bash ./setup.sh", Cwd: "my-repo", TimeoutSeconds: 120, Stdin: "input",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/sandboxes/sbx_1/commands")
	body := got.Body(t)
	if body["command"] != "bash ./setup.sh" || body["cwd"] != "my-repo" || body["timeout_seconds"] != float64(120) || body["stdin"] != "input" {
		t.Errorf("body = %v", body)
	}
	for _, k := range []string{"detach", "name", "tty", "sudo", "user"} {
		if _, present := body[k]; present {
			t.Errorf("process-only option %q would turn this into a detached process", k)
		}
	}
	if res.Stdout != "ok\n" || res.ExitCode != 0 || res.DurationMS != 412 || res.TimedOut {
		t.Errorf("result = %+v", res)
	}
}

func TestComputerRunCommandAndHandle(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeJSON(w, 200, map[string]interface{}{"id": "cmp_2", "status": "running"})
			return
		}
		writeJSON(w, 200, commandResult("hi", 0))
	})

	if _, err := client.Computers.RunCommand(context.Background(), "cmp_1", miosa.CommandInput{Command: "echo hi"}); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "POST", "/computers/cmp_1/commands")

	comp, err := client.Computers.Get(context.Background(), "cmp_2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := comp.RunCommand(context.Background(), miosa.CommandInput{Command: "echo hi"}); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "POST", "/computers/cmp_2/commands")
}

func TestCommandInputValidation(t *testing.T) {
	cases := []struct {
		name string
		in   miosa.CommandInput
		want string
	}{
		{"empty command", miosa.CommandInput{}, "command is required"},
		{"command too large", miosa.CommandInput{Command: strings.Repeat("x", miosa.MaxCommandBytes+1)}, "limit is 65536"},
		{"timeout too long is refused, not clamped", miosa.CommandInput{Command: "x", TimeoutSeconds: 601}, "between 1 and 600"},
		{"negative timeout", miosa.CommandInput{Command: "x", TimeoutSeconds: -1}, "between 1 and 600"},
		{"stdin too large", miosa.CommandInput{Command: "x", Stdin: strings.Repeat("y", miosa.MaxCommandStdinBytes+1)}, "stdin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, rec := newFixedClient(t, 200, commandResult("", 0))
			_, err := client.Sandboxes.RunCommand(context.Background(), "sbx_1", tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
			if rec.Count() != 0 {
				t.Error("an invalid command must not reach the server")
			}
		})
	}
}

func TestRunCommandReturnsStartingRefusalWithHint(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		writeJSON(w, http.StatusConflict, errorEnvelope("SANDBOX_STARTING", "the sandbox is still starting; retry shortly",
			map[string]interface{}{"retryable": true, "retry_after_ms": 2000}))
	})

	_, err := client.Sandboxes.RunCommand(context.Background(), "sbx_1", miosa.CommandInput{Command: "ls"})

	if !miosa.IsStarting(err) {
		t.Fatalf("error = %v", err)
	}
	if rec.Count() != 1 {
		t.Errorf("requests = %d; without WaitUntilReady the refusal is returned at once", rec.Count())
	}
}

func TestRunCommandWaitsOutStarting(t *testing.T) {
	var calls int32
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			writeJSON(w, http.StatusConflict, errorEnvelope("SANDBOX_STARTING", "starting",
				map[string]interface{}{"retryable": true, "retry_after_ms": 10}))
			return
		}
		writeJSON(w, 200, commandResult("ready", 0))
	})

	res, err := client.Sandboxes.RunCommand(context.Background(), "sbx_1", miosa.CommandInput{
		Command: "ls", WaitUntilReady: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 3 || res.Stdout != "ready" {
		t.Errorf("calls = %d, stdout = %q", calls, res.Stdout)
	}
}

func TestRunCommandGivesUpWhenWaitBudgetIsShort(t *testing.T) {
	client, rec := newFixedClient(t, http.StatusConflict, errorEnvelope("COMPUTER_STARTING", "starting",
		map[string]interface{}{"retryable": true, "retry_after_ms": 60000}))

	_, err := client.Computers.RunCommand(context.Background(), "cmp_1", miosa.CommandInput{Command: "ls", WaitUntilReady: 50 * time.Millisecond})

	if !miosa.IsStarting(err) {
		t.Fatalf("error = %v", err)
	}
	if rec.Count() != 1 {
		t.Errorf("requests = %d; a 60s hint does not fit a 50ms budget", rec.Count())
	}
}

func TestRunCommandStopsWhenContextCancelled(t *testing.T) {
	client, _ := newFixedClient(t, http.StatusConflict, errorEnvelope("SANDBOX_STARTING", "starting",
		map[string]interface{}{"retry_after_ms": 200}))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := client.Sandboxes.RunCommand(ctx, "sbx_1", miosa.CommandInput{Command: "ls", WaitUntilReady: time.Minute})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline", err)
	}
}

func TestCommandTimeoutTooLongFromServerIsValidationError(t *testing.T) {
	client, _ := newFixedClient(t, 422, errorEnvelope("TIMEOUT_TOO_LONG", "max 600", nil))
	// Bypass client-side validation by using a value the SDK allows.
	_, err := client.Sandboxes.RunCommand(context.Background(), "sbx_1", miosa.CommandInput{Command: "x", TimeoutSeconds: 600})
	var v *miosa.ValidationError
	if !errors.As(err, &v) || v.Code != miosa.CodeTimeoutTooLong {
		t.Fatalf("error = %#v", err)
	}
}

func TestAgentPromptAndStop(t *testing.T) {
	t.Run("sandbox prompt", func(t *testing.T) {
		client, rec := newFixedClient(t, 202, map[string]interface{}{"data": map[string]interface{}{"queued": true, "after_run_id": "run_1", "runner": "claude-code"}})
		q, err := client.Sandboxes.QueueAgentPrompt(context.Background(), "sbx_1", "fix the install")
		if err != nil {
			t.Fatal(err)
		}
		got := rec.Last(t)
		assertReq(t, got, "POST", "/sandboxes/sbx_1/agent/prompts")
		if got.Body(t)["prompt"] != "fix the install" {
			t.Errorf("body = %v", got.Body(t))
		}
		if !q.Queued || q.AfterRunID != "run_1" || q.Runner != "claude-code" {
			t.Errorf("ack = %+v", q)
		}
	})
	t.Run("computer prompt", func(t *testing.T) {
		client, rec := newFixedClient(t, 202, map[string]interface{}{"data": map[string]interface{}{"queued": true}})
		if _, err := client.Computers.QueueAgentPrompt(context.Background(), "cmp_1", "go"); err != nil {
			t.Fatal(err)
		}
		assertReq(t, rec.Last(t), "POST", "/computers/cmp_1/agent/prompts")
	})
	t.Run("stop", func(t *testing.T) {
		client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "run_1", "status": "canceled"}})
		run, err := client.Sandboxes.StopAgent(context.Background(), "sbx_1")
		if err != nil {
			t.Fatal(err)
		}
		assertReq(t, rec.Last(t), "POST", "/sandboxes/sbx_1/agent/stop")
		if run.ID != "run_1" || run.Status != miosa.RunStatusCanceled {
			t.Errorf("run = %+v", run)
		}
	})
	t.Run("stop computer", func(t *testing.T) {
		client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "run_2"}})
		if _, err := client.Computers.StopAgent(context.Background(), "cmp_1"); err != nil {
			t.Fatal(err)
		}
		assertReq(t, rec.Last(t), "POST", "/computers/cmp_1/agent/stop")
	})
	t.Run("no agent attached", func(t *testing.T) {
		client, _ := newFixedClient(t, 409, errorEnvelope("NO_AGENT_RUN", "no agent run is attached", map[string]interface{}{"retryable": false}))
		_, err := client.Sandboxes.QueueAgentPrompt(context.Background(), "sbx_1", "x")
		if !miosa.IsCode(err, miosa.CodeNoAgentRun) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("rejects empty and oversize prompts", func(t *testing.T) {
		client, rec := newFixedClient(t, 202, nil)
		if _, err := client.Sandboxes.QueueAgentPrompt(context.Background(), "sbx_1", ""); err == nil {
			t.Error("empty prompt accepted")
		}
		if _, err := client.Sandboxes.QueueAgentPrompt(context.Background(), "sbx_1", strings.Repeat("a", miosa.MaxAgentPromptBytes+1)); err == nil {
			t.Error("oversize prompt accepted")
		}
		if rec.Count() != 0 {
			t.Error("invalid prompts must not reach the server")
		}
	})
}

func TestSetupFileValidation(t *testing.T) {
	cases := []struct {
		name   string
		script string
		ok     bool
	}{
		{"plain", "#!/bin/sh\necho hi\n", true},
		{"max size", strings.Repeat("a", miosa.MaxSetupFileBytes), true},
		{"over size counts bytes", strings.Repeat("é", miosa.MaxSetupFileBytes/2+1), false},
		{"NUL byte", "echo\x00hi", false},
		{"invalid utf8", "echo \xff\xfe", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := miosa.ValidateSetupFile(tc.script)
			if (err == nil) != tc.ok {
				t.Fatalf("ValidateSetupFile = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestMachineJSONCarriesEnvironmentAndSetupBlocks(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"id": "sbx_1", "state": "running", "ready": true,
		"environment": "prod", "environment_id": "env_1", "environment_version": 2, "environment_latest_version": 3,
		"environment_base_id": "base_1", "environment_base_version": 5, "environment_upgrade_available": true,
		"environment_protected": false, "environment_status": "applied", "environment_error": nil,
		"setup_status": "failed", "setup_error": "setup.sh: exit code 1\nboom",
		"setup_file_status": "failed", "setup_file_error": "exit code 1",
		"setup_started_at": "2026-10-09T18:01:10.512Z", "setup_finished_at": "2026-10-09T18:01:42.087Z",
		"setup_repos":       []map[string]interface{}{{"repo": "o/r", "path": "r", "blocking": true, "status": "done", "error": nil}},
		"setup_environment": map[string]interface{}{"blocking": true, "status": "failed", "error": "exit code 1"},
		"setup_cli_tools":   map[string]interface{}{"blocking": true, "status": "done", "tools": map[string]string{"gh": "latest"}},
	}})

	sb, err := client.Sandboxes.Get(context.Background(), "sbx_1")
	if err != nil {
		t.Fatal(err)
	}
	if sb.Environment != "prod" || sb.EnvironmentVersion != 2 || sb.EnvironmentLatestVersion != 3 || !sb.EnvironmentUpgradeAvailable {
		t.Errorf("environment block: %+v", sb.MachineEnvironment)
	}
	if sb.EnvironmentStatus != miosa.EnvironmentApplied || sb.EnvironmentBaseID != "base_1" {
		t.Errorf("environment status: %+v", sb.MachineEnvironment)
	}
	if sb.SetupStatus != miosa.SetupFailed || !strings.Contains(sb.SetupError, "boom") || sb.SetupFileStatus != miosa.SetupFailed {
		t.Errorf("setup block: %+v", sb.MachineSetup)
	}
	if len(sb.SetupRepos) != 1 || !sb.SetupRepos[0].Blocking || sb.SetupEnvironmentStep == nil || sb.SetupEnvironmentStep.Status != miosa.SetupFailed {
		t.Errorf("steps: %+v", sb.MachineSetup)
	}
	if sb.SetupCLITools == nil || sb.SetupCLITools.Tools["gh"] != "latest" {
		t.Errorf("cli tools step: %+v", sb.SetupCLITools)
	}
	if !sb.SetupFinished() {
		t.Error("failed counts as finished")
	}
}

func TestComputerJSONCarriesSetupBlock(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{"id": "cmp_1", "status": "running", "setup_status": "running", "environment": "base"})
	c, err := client.Computers.Get(context.Background(), "cmp_1")
	if err != nil {
		t.Fatal(err)
	}
	if c.SetupStatus != miosa.SetupRunning || c.Environment != "base" || c.SetupFinished() {
		t.Errorf("computer: %+v %+v", c.MachineSetup, c.MachineEnvironment)
	}
}

func TestWaitForSetupPollsUntilFinal(t *testing.T) {
	var calls int32
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		status := "running"
		if atomic.AddInt32(&calls, 1) >= 3 {
			status = "done"
		}
		writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": "sbx_1", "setup_status": status}})
	})

	sb, err := client.Sandboxes.WaitForSetup(context.Background(), "sbx_1", miosa.WaitForSetupOptions{PollInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if sb.SetupStatus != miosa.SetupDone || atomic.LoadInt32(&calls) != 3 {
		t.Errorf("status = %q after %d calls", sb.SetupStatus, calls)
	}
}

func TestWaitForSetupNoSetupReturnsImmediately(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "sbx_1", "setup_status": nil}})
	if _, err := client.Sandboxes.WaitForSetup(context.Background(), "sbx_1", miosa.WaitForSetupOptions{}); err != nil {
		t.Fatal(err)
	}
	if rec.Count() != 1 {
		t.Errorf("requests = %d", rec.Count())
	}
}

func TestWaitForSetupTimesOut(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "sbx_1", "setup_status": "pending"}})
	sb, err := client.Sandboxes.WaitForSetup(context.Background(), "sbx_1", miosa.WaitForSetupOptions{Timeout: 20 * time.Millisecond, PollInterval: 15 * time.Millisecond})
	if err == nil || sb == nil || sb.SetupStatus != miosa.SetupPending {
		t.Fatalf("sb=%+v err=%v", sb, err)
	}
}
