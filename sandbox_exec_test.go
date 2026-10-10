package miosa_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func TestSandboxExec(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"stdout": "hi\n", "stderr": "warn\n", "exit_code": 3, "duration_ms": 12,
	}})

	res, err := client.Sandboxes.Exec(context.Background(), "sbx_1", miosa.ExecSandboxInput{
		Command: "echo hi", TimeoutSeconds: 30, Cwd: "/workspace", Env: map[string]string{"A": "1"}, WaitForReady: true, WaitTimeoutMS: 5000,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/sandboxes/sbx_1/exec")
	body := got.Body(t)
	if body["command"] != "echo hi" || body["timeout"] != float64(30) || body["cwd"] != "/workspace" || body["wait"] != true || body["wait_timeout_ms"] != float64(5000) {
		t.Errorf("body = %v", body)
	}
	if body["env"].(map[string]interface{})["A"] != "1" {
		t.Errorf("env = %v", body["env"])
	}
	if res.Stdout != "hi\n" || res.Stderr != "warn\n" || res.ExitCode != 3 || res.DurationMS != 12 {
		t.Errorf("result = %+v", res)
	}
}

func TestSandboxExecInputValidation(t *testing.T) {
	cases := []struct {
		name string
		in   miosa.ExecSandboxInput
	}{
		{"empty", miosa.ExecSandboxInput{}},
		{"blank", miosa.ExecSandboxInput{Command: "   "}},
		{"timeout over a day", miosa.ExecSandboxInput{Command: "x", TimeoutSeconds: 86401}},
		{"negative timeout", miosa.ExecSandboxInput{Command: "x", TimeoutSeconds: -1}},
		{"relative cwd", miosa.ExecSandboxInput{Command: "x", Cwd: "repo"}},
		{"NUL in cwd", miosa.ExecSandboxInput{Command: "x", Cwd: "/a\x00b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, rec := newFixedClient(t, 200, nil)
			if _, err := client.Sandboxes.Exec(context.Background(), "sbx_1", tc.in); err == nil {
				t.Fatal("accepted")
			}
			if _, err := client.Sandboxes.ExecStream(context.Background(), "sbx_1", tc.in); err == nil {
				t.Fatal("stream accepted")
			}
			if rec.Count() != 0 {
				t.Error("reached the server")
			}
		})
	}
}

func TestSandboxExecStartingIsTyped(t *testing.T) {
	client, _ := newFixedClient(t, 409, errorEnvelope("SANDBOX_STARTING", "starting", map[string]interface{}{"retryable": true, "retry_after_ms": 1500}))
	_, err := client.Sandboxes.Exec(context.Background(), "sbx_1", miosa.ExecSandboxInput{Command: "ls"})
	if !miosa.IsStarting(err) {
		t.Fatalf("error = %v", err)
	}
	if d, _ := miosa.RetryAfter(err); d != 1500*time.Millisecond {
		t.Errorf("RetryAfter = %v", d)
	}
}

const execSSE = "event: stdout\ndata: {\"line\":\"one\"}\n\n" +
	"event: stderr\ndata: {\"line\":\"oops\"}\n\n" +
	"event: stdout\ndata: {\"line\":\"two\"}\n\n" +
	"event: exit\ndata: {\"exit_code\":0}\n\n"

func serveSSE(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(body))
	}
}

func TestSandboxExecStreamYieldsFramesThenExit(t *testing.T) {
	client, rec := newHandlerClient(t, serveSSE(execSSE))

	events, err := client.Sandboxes.ExecStream(context.Background(), "sbx_1", miosa.ExecSandboxInput{Command: "make"})
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	var last miosa.SandboxExecEvent
	for ev := range events {
		seen = append(seen, string(ev.Type)+":"+ev.Line)
		last = ev
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/sandboxes/sbx_1/exec/stream")
	if got.Header.Get("Accept") != "text/event-stream" {
		t.Errorf("Accept = %q", got.Header.Get("Accept"))
	}
	if strings.Join(seen, "|") != "stdout:one|stderr:oops|stdout:two|exit:" {
		t.Errorf("frames = %v", seen)
	}
	if last.Type != miosa.ExecEventExit || last.ExitCode == nil || *last.ExitCode != 0 {
		t.Errorf("last = %+v", last)
	}
}

func TestSandboxExecStreamTimedOutAndAbnormalExit(t *testing.T) {
	client, _ := newHandlerClient(t, serveSSE("event: exit\ndata: {\"exit_code\":-1,\"timed_out\":true}\n\n"))
	events, _ := client.Sandboxes.ExecStream(context.Background(), "sbx_1", miosa.ExecSandboxInput{Command: "sleep 99"})
	res, err := miosa.CollectExec(events)
	if err != nil || !res.TimedOut || res.ExitCode != -1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}

	client, _ = newHandlerClient(t, serveSSE("event: stdout\ndata: {\"line\":\"partial\"}\n\n"+
		"event: exit\ndata: {\"exit_code\":-2,\"error\":{\"code\":\"AGENT_UNAVAILABLE\",\"message\":\"guest went away\",\"retryable\":true,\"request_id\":\"r1\"}}\n\n"))
	events, _ = client.Sandboxes.ExecStream(context.Background(), "sbx_1", miosa.ExecSandboxInput{Command: "x"})
	_, err = miosa.CollectExec(events)
	streamErr, ok := err.(*miosa.ExecStreamError)
	if !ok || streamErr.Code != "AGENT_UNAVAILABLE" || !streamErr.Retryable || streamErr.RequestID != "r1" {
		t.Fatalf("err = %#v", err)
	}
	if !strings.Contains(streamErr.Error(), "guest went away") {
		t.Errorf("Error() = %q", streamErr.Error())
	}
}

func TestCollectExecAccumulatesOutput(t *testing.T) {
	client, _ := newHandlerClient(t, serveSSE(execSSE))
	events, _ := client.Sandboxes.ExecStream(context.Background(), "sbx_1", miosa.ExecSandboxInput{Command: "make"})

	res, err := miosa.CollectExec(events)

	if err != nil {
		t.Fatal(err)
	}
	if res.Stdout != "one\ntwo\n" || res.Stderr != "oops\n" || res.ExitCode != 0 {
		t.Errorf("result = %+v", res)
	}
}

func TestCollectExecWithoutExitFrame(t *testing.T) {
	client, _ := newHandlerClient(t, serveSSE("event: stdout\ndata: {\"line\":\"cut\"}\n\n"))
	events, _ := client.Sandboxes.ExecStream(context.Background(), "sbx_1", miosa.ExecSandboxInput{Command: "x"})
	if _, err := miosa.CollectExec(events); err == nil {
		t.Fatal("a stream that ends without an exit frame must be an error")
	}
}

func TestSandboxExecStreamPreflightErrorsAreTyped(t *testing.T) {
	client, _ := newFixedClient(t, 409, errorEnvelope("SANDBOX_NOT_RUNNING", "must be running", nil))
	_, err := client.Sandboxes.ExecStream(context.Background(), "sbx_1", miosa.ExecSandboxInput{Command: "x"})
	if !miosa.IsCode(err, "SANDBOX_NOT_RUNNING") {
		t.Fatalf("error = %v", err)
	}
}

func TestSandboxExecStreamIsNotCutByTheClientTimeout(t *testing.T) {
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte("event: exit\ndata: {\"exit_code\":0}\n\n"))
	}, miosa.WithTimeout(50*time.Millisecond))

	events, err := client.Sandboxes.ExecStream(context.Background(), "sbx_1", miosa.ExecSandboxInput{Command: "slow"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := miosa.CollectExec(events); err != nil {
		t.Fatalf("a long stream must outlive the request timeout: %v", err)
	}
}

func TestSandboxExecStreamStopsOnCancel(t *testing.T) {
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: stdout\ndata: {\"line\":\"tick\"}\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	events, err := client.Sandboxes.ExecStream(ctx, "sbx_1", miosa.ExecSandboxInput{Command: "tail -f"})
	if err != nil {
		t.Fatal(err)
	}
	if ev := <-events; ev.Line != "tick" {
		t.Fatalf("first frame = %+v", ev)
	}
	cancel()
	select {
	case _, open := <-events:
		for open {
			_, open = <-events
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the channel did not close after cancel")
	}
}

func TestSandboxGetByName(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "sbx_1", "name": "my build"}})

	sb, err := client.Sandboxes.GetByName(context.Background(), "my build")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/sandboxes/by-name/my build")
	if sb.ID != "sbx_1" || sb.Name != "my build" {
		t.Errorf("sandbox = %+v", sb)
	}
	if _, err := client.Sandboxes.GetByName(context.Background(), ""); err == nil {
		t.Error("empty name accepted")
	}
}

func TestSandboxUpdate(t *testing.T) {
	name, on := "renamed", true
	idle := 0
	client, rec := newFixedClient(t, 200, map[string]interface{}{"id": "sbx_1", "name": "renamed", "always_on": true})

	sb, err := client.Sandboxes.Update(context.Background(), "sbx_1", miosa.UpdateSandboxInput{
		Name: &name, Tags: []string{"a", "b"}, Metadata: map[string]interface{}{"team": "x"}, AlwaysOn: &on, IdleTimeoutSec: &idle,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PATCH", "/sandboxes/sbx_1")
	body := got.Body(t)
	if body["name"] != "renamed" || body["always_on"] != true || body["idle_timeout_sec"] != float64(0) {
		t.Errorf("body = %v", body)
	}
	if _, present := body["slug"]; present {
		t.Error("unset members must be omitted")
	}
	if !sb.AlwaysOn || sb.Name != "renamed" {
		t.Errorf("sandbox = %+v", sb)
	}
}

// Regression: the JSON body reader used for streaming POSTs returned a
// non-io.EOF error at the end of the body, so every streaming POST failed with
// a connection error before reaching the server.
func TestStreamingPostBodyIsSentWhole(t *testing.T) {
	client, rec := newHandlerClient(t, serveSSE("data: {\"id\":\"1\"}\n\ndata: [DONE]\n\n"))

	_, ch, err := client.Completions.Chat(context.Background(), miosa.ChatCompletionRequest{
		Model: "m", Messages: []map[string]interface{}{{"role": "user", "content": "hi"}}, Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for range ch {
		n++
	}
	body := rec.Last(t).Body(t)
	if body["model"] != "m" || body["stream"] != true || n != 1 {
		t.Errorf("body=%v events=%d", body, n)
	}
}
