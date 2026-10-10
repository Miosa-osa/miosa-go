package miosa_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func sandboxEnvelope(extra map[string]interface{}) map[string]interface{} {
	data := map[string]interface{}{"id": "sbx_1", "state": "provisioning", "ready": false}
	for k, v := range extra {
		data[k] = v
	}
	return map[string]interface{}{"data": data}
}

func TestSandboxCreateSendsEnvironmentSetupAndBillTo(t *testing.T) {
	client, rec := newFixedClient(t, 201, sandboxEnvelope(nil))

	_, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{
		Environment: "prod",
		NoEnv:       true,
		SetupFile:   "#!/bin/sh\nnpm ci\n",
		BillTo:      "acme",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/sandboxes")
	if got.RawQuery != "" {
		t.Errorf("query = %q; no wait was asked for", got.RawQuery)
	}
	body := got.Body(t)
	if body["environment"] != "prod" || body["no_env"] != true || body["bill_to"] != "acme" || body["setup_file"] != "#!/bin/sh\nnpm ci\n" {
		t.Errorf("body = %v", body)
	}
	if _, present := body["environment_id"]; present {
		t.Error("environment_id must be omitted when empty")
	}
}

func TestSandboxCreateWithEnvironmentIDOnly(t *testing.T) {
	client, rec := newFixedClient(t, 201, sandboxEnvelope(nil))
	if _, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{EnvironmentID: "env_1"}); err != nil {
		t.Fatal(err)
	}
	body := rec.Last(t).Body(t)
	if body["environment_id"] != "env_1" {
		t.Errorf("body = %v", body)
	}
	for _, k := range []string{"environment", "no_env", "setup_file", "bill_to"} {
		if _, present := body[k]; present {
			t.Errorf("%q must be omitted when unset", k)
		}
	}
}

func TestSandboxCreateRejectsBadSetupFileBeforeSending(t *testing.T) {
	client, rec := newFixedClient(t, 201, sandboxEnvelope(nil))

	_, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{SetupFile: strings.Repeat("x", miosa.MaxSetupFileBytes+1)})

	if err == nil {
		t.Fatal("expected an error")
	}
	if rec.Count() != 0 {
		t.Error("nothing should be sent")
	}
}

func TestSandboxCreateWaitSecondsAndHeaders(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(miosa.HeaderWaitOutcome, "timeout")
		w.Header().Set(miosa.HeaderBillTo, "org_1")
		w.Header().Set(miosa.HeaderBillToSource, "setting")
		writeJSON(w, 201, sandboxEnvelope(map[string]interface{}{"ready": false}))
	})

	res, err := client.Sandboxes.CreateDetailed(context.Background(), miosa.CreateSandboxInput{WaitSeconds: 45})
	if err != nil {
		t.Fatal(err)
	}
	if got := rec.Last(t); got.RawQuery != "wait=45" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if res.WaitOutcome != "timeout" || res.BilledOrganizationID != "org_1" || res.BillToSource != miosa.BillToSourceSetting {
		t.Errorf("result = %+v", res)
	}
	if res.Sandbox.ID != "sbx_1" || res.Sandbox.Ready {
		t.Errorf("a timed out wait still returns the created sandbox: %+v", res.Sandbox)
	}
}

func TestSandboxCreateWaitForReadyUsesBooleanForm(t *testing.T) {
	client, rec := newFixedClient(t, 201, sandboxEnvelope(map[string]interface{}{"ready": true}))

	sb, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{WaitForReady: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := rec.Last(t); got.RawQuery != "wait=true" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if !sb.Ready {
		t.Error("Ready = false")
	}
}

func TestSandboxCreateWaitSecondsRange(t *testing.T) {
	for _, secs := range []int{1, -3, 121} {
		client, rec := newFixedClient(t, 201, sandboxEnvelope(nil))
		if _, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{WaitSeconds: secs}); err == nil {
			t.Errorf("WaitSeconds=%d accepted; the API takes 2 to 120", secs)
		}
		if rec.Count() != 0 {
			t.Errorf("WaitSeconds=%d reached the server", secs)
		}
	}
}

func TestSandboxCreateWaitOutlastsShortClientTimeout(t *testing.T) {
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(120 * time.Millisecond)
		writeJSON(w, 201, sandboxEnvelope(map[string]interface{}{"ready": true}))
	}, miosa.WithTimeout(40*time.Millisecond))

	// Without the wait budget the 40ms client timeout would fire first.
	sb, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{WaitSeconds: 5})
	if err != nil {
		t.Fatalf("a waiting create must raise the HTTP timeout above the wait: %v", err)
	}
	if !sb.Ready {
		t.Error("not ready")
	}
}

func TestSandboxCreateSendsIdempotencyKey(t *testing.T) {
	client, rec := newFixedClient(t, 201, sandboxEnvelope(nil))
	if _, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{IdempotencyKey: "k-1", WaitSeconds: 10}); err != nil {
		t.Fatal(err)
	}
	if got := rec.Last(t).Header.Get("Idempotency-Key"); got != "k-1" {
		t.Errorf("Idempotency-Key = %q", got)
	}
}

func TestComputerCreateSendsEnvironmentAndSetup(t *testing.T) {
	client, rec := newFixedClient(t, 201, map[string]interface{}{"id": "cmp_1", "status": "creating"})

	_, err := client.Computers.Create(context.Background(), miosa.CreateComputerInput{
		Name: "dev", EnvironmentID: "env_1", SetupFile: "echo hi", BillTo: "acme", NoEnv: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	body := rec.Last(t).Body(t)
	if body["environment_id"] != "env_1" || body["setup_file"] != "echo hi" || body["bill_to"] != "acme" || body["no_env"] != true {
		t.Errorf("body = %v", body)
	}

	if _, err := client.Computers.Create(context.Background(), miosa.CreateComputerInput{Name: "x", SetupFile: "a\x00b"}); err == nil {
		t.Error("NUL in setup file accepted")
	}
}

func TestSandboxResumeWithEnvironment(t *testing.T) {
	client, rec := newFixedClient(t, 200, sandboxEnvelope(map[string]interface{}{"state": "running"}))

	_, err := client.Sandboxes.ResumeWith(context.Background(), "sbx_1", miosa.MachineEnvironmentOptions{
		Environment: "staging", Env: map[string]string{"A": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/sandboxes/sbx_1/resume")
	body := got.Body(t)
	if body["environment"] != "staging" || body["env"].(map[string]interface{})["A"] != "1" {
		t.Errorf("body = %v", body)
	}
}

func TestSandboxForkAndRestoreCarryEnvironment(t *testing.T) {
	client, rec := newFixedClient(t, 201, sandboxEnvelope(nil))

	_, err := client.Sandboxes.ForkSandbox(context.Background(), "sbx_1", miosa.PublicForkSandboxInput{
		TimeoutSec: 600, MachineEnvironmentOptions: miosa.MachineEnvironmentOptions{NoEnv: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/sandboxes/sbx_1/fork")
	if b := got.Body(t); b["no_env"] != true || b["timeout_sec"] != float64(600) {
		t.Errorf("fork body = %v", b)
	}

	if _, err := client.Sandboxes.RestoreSnapshotWith(context.Background(), "sbx_1", "snap_9", miosa.MachineEnvironmentOptions{EnvironmentID: "env_2"}); err != nil {
		t.Fatal(err)
	}
	got = rec.Last(t)
	assertReq(t, got, "POST", "/sandboxes/sbx_1/restore/snap_9")
	if got.Body(t)["environment_id"] != "env_2" {
		t.Errorf("restore body = %v", got.Body(t))
	}
}

func TestComputerStartWithEnvironment(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeJSON(w, 200, map[string]interface{}{"id": "cmp_1", "status": "stopped"})
			return
		}
		w.WriteHeader(200)
	})
	c, err := client.Computers.Get(context.Background(), "cmp_1")
	if err != nil {
		t.Fatal(err)
	}

	if err := c.StartWith(context.Background(), miosa.MachineEnvironmentOptions{Environment: "base"}); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/computers/cmp_1/start")
	if got.Body(t)["environment"] != "base" {
		t.Errorf("body = %v", got.Body(t))
	}
}

func TestSSHCertificate(t *testing.T) {
	client, rec := newFixedClient(t, 201, map[string]interface{}{"data": map[string]interface{}{
		"certificate": "ssh-ed25519-cert-v01@openssh.com AAAA", "principal": "sandbox-sbx_1", "user": "root",
		"key_id": "k1", "serial": 42, "valid_after": "2026-10-10T00:00:00Z", "valid_before": "2026-10-10T00:15:00Z",
		"ttl_seconds": 900, "ca_public_key": "ssh-ed25519 CA", "sandbox_id": "sbx_1", "transport": "websocket",
		"tunnel_url": "wss://api.miosa.ai/api/v1/sandboxes/sbx_1/ssh-tunnel",
	}})

	cert, err := client.Sandboxes.IssueSSHCertificate(context.Background(), "sbx_1", miosa.SSHCertificateInput{
		PublicKey: "ssh-ed25519 AAAA user@host", TTLSeconds: 900,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/sandboxes/sbx_1/ssh-certificates")
	if b := got.Body(t); b["public_key"] != "ssh-ed25519 AAAA user@host" || b["ttl_seconds"] != float64(900) {
		t.Errorf("body = %v", b)
	}
	if cert.Principal != "sandbox-sbx_1" || cert.User != "root" || cert.Serial != 42 || cert.TTLSeconds != 900 || cert.TunnelURL == "" || cert.CAPublicKey == "" {
		t.Errorf("cert = %+v", cert)
	}
}

func TestSSHCertificateInputValidationAndErrors(t *testing.T) {
	client, rec := newFixedClient(t, 409, errorEnvelope("SANDBOX_NOT_RUNNING", "sandbox must be running", nil))

	if _, err := client.Sandboxes.IssueSSHCertificate(context.Background(), "sbx_1", miosa.SSHCertificateInput{}); err == nil {
		t.Error("empty public key accepted")
	}
	if _, err := client.Sandboxes.IssueSSHCertificate(context.Background(), "sbx_1", miosa.SSHCertificateInput{PublicKey: "k", TTLSeconds: 3601}); err == nil {
		t.Error("ttl above an hour accepted")
	}
	if rec.Count() != 0 {
		t.Fatal("invalid input reached the server")
	}

	_, err := client.Sandboxes.IssueSSHCertificate(context.Background(), "sbx_1", miosa.SSHCertificateInput{PublicKey: "k"})
	if !miosa.IsCode(err, "SANDBOX_NOT_RUNNING") {
		t.Errorf("error = %v", err)
	}
	if b := rec.Last(t).Body(t); b["ttl_seconds"] != nil {
		t.Errorf("zero ttl must be omitted so the server default applies: %v", b)
	}
}

func TestWhoami(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{
		"user":         map[string]interface{}{"id": "u1", "email": "a@b.co", "name": "Ada"},
		"organization": map[string]interface{}{"id": "o1", "name": "Acme", "slug": "acme"},
		"workspace":    nil,
		"plan":         map[string]interface{}{"name": "pro"},
		"auth": map[string]interface{}{
			"method": "api_key", "scopes": []string{"sandboxes:read"}, "unrestricted": false,
			"key": map[string]interface{}{"id": "k1", "name": "ci", "prefix": "msk_u_ab", "type": "user", "expires_at": "2027-01-01T00:00:00Z"},
		},
	})

	who, err := client.Whoami(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/whoami")
	if who.User.Email != "a@b.co" || who.Organization.Slug != "acme" || who.Workspace != nil || who.Plan.Name != "pro" {
		t.Errorf("whoami = %+v", who)
	}
	if who.Auth.Method != "api_key" || who.Auth.Scopes[0] != "sandboxes:read" || who.Auth.Key == nil || who.Auth.Key.Prefix != "msk_u_ab" || who.Auth.Key.ExpiresAt == nil {
		t.Errorf("auth = %+v", who.Auth)
	}
}

func TestWhoamiSessionHasNoKey(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{
		"user": nil, "organization": nil, "workspace": nil, "plan": nil,
		"auth": map[string]interface{}{"method": "session", "scopes": []string{}, "unrestricted": true, "key": nil},
	})
	who, err := client.Whoami(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if who.User != nil || who.Auth.Key != nil || !who.Auth.Unrestricted {
		t.Errorf("whoami = %+v", who)
	}
}
