package miosa_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

// Every SDK method the sprint added must surface an API failure instead of
// swallowing it. The response and request tables of the conformance test list
// the calls, so this needs no fixture files.
func TestSprintMethodsReturnAPIFailures(t *testing.T) {
	newFailing := func(t *testing.T) *miosa.Client {
		client, _ := newFixedClient(t, http.StatusInternalServerError, errorEnvelope("BOOM", "boom", nil))
		return client
	}
	for _, tc := range responseCases {
		tc := tc
		t.Run("response/"+tc.file, func(t *testing.T) {
			if tc.drift != "" {
				t.Skip(tc.drift)
			}
			if _, err := tc.call(newFailing(t)); err == nil {
				t.Error("a 500 was swallowed")
			}
		})
	}
	for _, tc := range requestCases {
		tc := tc
		t.Run("request/"+tc.file, func(t *testing.T) {
			if err := tc.send(newFailing(t)); err == nil {
				t.Error("a 500 was swallowed")
			}
		})
	}
}

func TestComputerHandleAgentSteering(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			writeJSON(w, 200, map[string]interface{}{"id": "cmp_1", "status": "running"})
		case strings.HasSuffix(r.URL.Path, "/agent/stop"):
			writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": "run_1", "status": "canceled"}})
		default:
			writeJSON(w, 202, map[string]interface{}{"data": map[string]interface{}{"queued": true, "runner": "codex"}})
		}
	})
	c, err := client.Computers.Get(context.Background(), "cmp_1")
	if err != nil {
		t.Fatal(err)
	}

	q, err := c.QueueAgentPrompt(context.Background(), "go on")
	if err != nil || !q.Queued || q.Runner != "codex" {
		t.Fatalf("%+v %v", q, err)
	}
	assertReq(t, rec.Last(t), "POST", "/computers/cmp_1/agent/prompts")

	run, err := c.StopAgent(context.Background())
	if err != nil || run.Status != miosa.RunStatusCanceled {
		t.Fatalf("%+v %v", run, err)
	}
	assertReq(t, rec.Last(t), "POST", "/computers/cmp_1/agent/stop")
}

func TestEnvironmentsServiceScopeAccessors(t *testing.T) {
	client := miosa.NewClient("k")
	if client.Environments.WorkspaceID() != "" {
		t.Error("the root service is organization-scoped")
	}
	scoped := client.Environments.ForWorkspace("ws_1")
	if scoped.WorkspaceID() != "ws_1" || client.Environments.WorkspaceID() != "" {
		t.Error("ForWorkspace must return a scoped copy and leave the original alone")
	}
	if client.Environments.ForWorkspace("").WorkspaceID() != "" {
		t.Error("an empty id is the organization level")
	}
}

func TestEnvironmentsSetDefaultAliasesMakeDefault(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"environment": envJSON})
	if _, err := client.Environments.SetDefault(context.Background(), "env_1"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "POST", "/environments/env_1/default")
}

func TestEnvironmentsDefaultWithoutOneIsAnError(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{"environments": []interface{}{}, "default_environment_id": ""})
	if _, err := client.Environments.Default(context.Background()); err == nil {
		t.Fatal("expected an error when the API returns no default")
	}
}

func TestErrorStringsAreUseful(t *testing.T) {
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-ID", "req_9")
		writeJSON(w, 409, errorEnvelope("SANDBOX_STARTING", "still starting", nil))
	})
	err := getSandbox(client)
	if got := err.Error(); !strings.Contains(got, "still starting") || !strings.Contains(got, "req_9") {
		t.Errorf("Error() = %q", got)
	}
	if got := (&miosa.SnapshotWarmingError{RetryAfter: 2 * time.Second}).Error(); !strings.Contains(got, "2s") {
		t.Errorf("Error() = %q", got)
	}
}

func TestRetryAfterHTTPDateHeader(t *testing.T) {
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", time.Now().Add(90*time.Second).UTC().Format(http.TimeFormat))
		writeJSON(w, 503, errorEnvelope("UNAVAILABLE", "later", nil))
	})
	d, ok := miosa.RetryAfter(getSandbox(client))
	if !ok || d < 80*time.Second || d > 91*time.Second {
		t.Errorf("RetryAfter = %v, %v", d, ok)
	}
}

func TestRetryAfterIgnoresGarbageAndPastDates(t *testing.T) {
	for _, v := range []string{"soon", "-5", "0", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)} {
		v := v
		client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", v)
			writeJSON(w, 503, errorEnvelope("UNAVAILABLE", "later", nil))
		})
		if d, ok := miosa.RetryAfter(getSandbox(client)); ok {
			t.Errorf("Retry-After %q gave a hint of %v", v, d)
		}
	}
}

func TestPlainTextAndEmptyErrorBodies(t *testing.T) {
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream down"))
	})
	m := miosa.AsMiosaError(getSandbox(client))
	if m == nil || m.Message != "upstream down" || m.Code != "" || m.Details != nil || m.Retryable {
		t.Errorf("plain text error = %+v", m)
	}

	client, _ = newFixedClient(t, http.StatusBadGateway, nil)
	m = miosa.AsMiosaError(getSandbox(client))
	if m == nil || m.Message != "request failed with status 502" {
		t.Errorf("empty body error = %+v", m)
	}
}

func TestAsMiosaErrorOnForeignErrors(t *testing.T) {
	if miosa.AsMiosaError(nil) != nil || miosa.AsMiosaError(context.Canceled) != nil {
		t.Error("non-API errors have no MiosaError")
	}
	if miosa.ErrorCode(context.Canceled) != "" || miosa.IsCode(context.Canceled, "X") || miosa.IsCode(nil, "") {
		t.Error("non-API errors have no code")
	}
	if _, ok := miosa.RetryAfter(nil); ok {
		t.Error("nil has no retry hint")
	}
	if miosa.IsStarting(nil) {
		t.Error("nil is not starting")
	}
}

func TestInstallSSHKeyPlainReturnsNothing(t *testing.T) {
	client, rec := newFixedClient(t, http.StatusNoContent, nil)

	cert, err := client.Sandboxes.InstallSSHKey(context.Background(), "sbx_1", miosa.InstallSSHKeyInput{PublicKey: "ssh-ed25519 AAAA user@host"})

	if err != nil || cert != nil {
		t.Fatalf("cert=%v err=%v", cert, err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/sandboxes/sbx_1/ssh-keys")
	if _, present := got.Body(t)["certificate"]; present {
		t.Error("certificate must be omitted for the plain form")
	}
	if _, err := client.Sandboxes.InstallSSHKey(context.Background(), "sbx_1", miosa.InstallSSHKeyInput{}); err == nil {
		t.Error("empty key accepted")
	}
}

func TestSSHInfoNotRunning(t *testing.T) {
	client, _ := newFixedClient(t, 409, errorEnvelope("SANDBOX_NOT_RUNNING", "must be running", nil))
	if _, err := client.Sandboxes.SSHInfo(context.Background(), "sbx_1"); !miosa.IsCode(err, "SANDBOX_NOT_RUNNING") {
		t.Errorf("error = %v", err)
	}
}

func TestDeploymentInventoryUsesTheNewPath(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "d1", "name": "production"}}})
	envs, err := client.Environments.ForWorkspace("ws_1").DeploymentInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/deployment-environments")
	if got.RawQuery != "workspace_id=ws_1" || envs[0]["name"] != "production" {
		t.Errorf("query=%q envs=%v", got.RawQuery, envs)
	}
}
