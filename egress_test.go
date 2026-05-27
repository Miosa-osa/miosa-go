package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Miosa-osa/miosa-go"
)

// ─── EgressSecrets ───────────────────────────────────────────────────────────

func TestEgressSecretsSet(t *testing.T) {
	var got miosa.SecretSetInput
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/secrets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.EgressSecretData{
				ID:          "sec_1",
				Name:        got.Name,
				Type:        got.Type,
				Scope:       got.Scope,
				MaskedValue: "sk-***",
			},
		})
	})
	c := newTestClient(t, mux)

	secret, err := c.Secrets.Set(context.Background(), miosa.SecretSetInput{
		Name:  "OPENAI_API_KEY",
		Value: "sk-abc",
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if secret.ID != "sec_1" {
		t.Errorf("ID: want sec_1, got %s", secret.ID)
	}
	if got.Name != "OPENAI_API_KEY" || got.Value != "sk-abc" {
		t.Errorf("body name=%q value=%q", got.Name, got.Value)
	}
	if got.Type != "api_key" {
		t.Errorf("default type: want api_key, got %q", got.Type)
	}
	if got.Scope != "user" {
		t.Errorf("default scope: want user, got %q", got.Scope)
	}
}

func TestEgressSecretsList(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/secrets", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"secrets": []miosa.EgressSecretData{
				{ID: "sec_1", Name: "A"},
				{ID: "sec_2", Name: "B"},
			},
		})
	})
	c := newTestClient(t, mux)

	out, err := c.Secrets.List(context.Background(), miosa.SecretListInput{ResourceType: "sandbox"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 2 {
		t.Errorf("len: want 2, got %d", len(out))
	}
	if !strings.Contains(gotQuery, "resource_type=sandbox") {
		t.Errorf("query: want resource_type=sandbox in %q", gotQuery)
	}
}

func TestEgressSecretsRotate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/secrets/sec_42", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["value"] != "new-secret" {
			t.Errorf("rotate body value: %v", body["value"])
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.EgressSecretData{ID: "sec_42", MaskedValue: "new-***"},
		})
	})
	c := newTestClient(t, mux)

	out, err := c.Secrets.Rotate(context.Background(), "sec_42", miosa.SecretRotateInput{NewValue: "new-secret"})
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if out.ID != "sec_42" {
		t.Errorf("ID: want sec_42, got %s", out.ID)
	}
}

func TestEgressSecretsConnect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/oauth/start", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["provider"] != "github" {
			t.Errorf("provider: %v", body["provider"])
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"authorize_url": "https://github.com/oauth/authorize?state=abc",
			"state":         "abc",
		})
	})
	statusCalls := 0
	mux.HandleFunc("/egress/oauth/status", func(w http.ResponseWriter, r *http.Request) {
		statusCalls++
		if statusCalls < 2 {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status": "pending",
				"state":  r.URL.Query().Get("state"),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":    "completed",
			"state":     r.URL.Query().Get("state"),
			"secret_id": "sec_oauth_1",
		})
	})
	c := newTestClient(t, mux)

	flow, err := c.Secrets.Connect(context.Background(), miosa.OauthConnectInput{
		Provider: "github",
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if flow.AuthorizeURL == "" {
		t.Error("AuthorizeURL empty")
	}
	if flow.State != "abc" {
		t.Errorf("State: %s", flow.State)
	}

	result, err := flow.WaitForCompletion(context.Background(), miosa.WaitForCompletionOptions{
		PollInterval: 5 * time.Millisecond,
		Timeout:      2 * time.Second,
	})
	if err != nil {
		t.Fatalf("WaitForCompletion: %v", err)
	}
	if result.SecretID != "sec_oauth_1" {
		t.Errorf("SecretID: %s", result.SecretID)
	}
}

func TestEgressSecretsBindings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/bindings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var b map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&b)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": miosa.EgressBindingData{
					ID:           "bind_1",
					SecretID:     b["secret_id"].(string),
					ResourceID:   b["resource_id"].(string),
					ResourceType: b["resource_type"].(string),
					ExposeAsEnv:  b["expose_as_env"].(string),
				},
			})
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"bindings": []miosa.EgressBindingData{{ID: "bind_1", ResourceID: "cmp_1"}},
			})
		}
	})
	c := newTestClient(t, mux)

	b, err := c.Secrets.CreateBinding(context.Background(), miosa.BindingCreateInput{
		SecretID:     "sec_1",
		ResourceID:   "cmp_1",
		ResourceType: "computer",
		ExposeAsEnv:  "OPENAI_API_KEY",
	})
	if err != nil {
		t.Fatalf("CreateBinding: %v", err)
	}
	if b.ID != "bind_1" {
		t.Errorf("ID: %s", b.ID)
	}

	list, err := c.Secrets.ListBindings(context.Background(), miosa.BindingListInput{ResourceID: "cmp_1"})
	if err != nil {
		t.Fatalf("ListBindings: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len: %d", len(list))
	}
}

// ─── EgressNetwork ───────────────────────────────────────────────────────────

func TestEgressNetworkAllowDeny(t *testing.T) {
	var lastBody map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/allowlist", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&lastBody)
			effect, _ := lastBody["effect"].(string)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": miosa.EgressAllowlistRule{
					ID:     "rule_1",
					Host:   lastBody["host"].(string),
					Effect: effect,
				},
			})
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"rules": []miosa.EgressAllowlistRule{
					{ID: "rule_1", Host: "api.example.com", Effect: "allow"},
				},
			})
		}
	})
	c := newTestClient(t, mux)

	allow, err := c.Network.Allow(context.Background(), "api.example.com", miosa.AllowInput{})
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if allow.Effect != "allow" {
		t.Errorf("effect: %s", allow.Effect)
	}
	if lastBody["effect"] != "allow" {
		t.Errorf("body effect: %v", lastBody["effect"])
	}

	deny, err := c.Network.Deny(context.Background(), "169.254.169.254", miosa.AllowInput{})
	if err != nil {
		t.Fatalf("Deny: %v", err)
	}
	if deny.Effect != "deny" {
		t.Errorf("effect: %s", deny.Effect)
	}

	rules, err := c.Network.Rules(context.Background(), miosa.RulesListInput{})
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("len: %d", len(rules))
	}
}

func TestEgressNetworkLockdownDefault(t *testing.T) {
	var gotBody map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/policies", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.EgressPolicy{ID: "pol_default", Mode: gotBody["mode"].(string)},
		})
	})
	c := newTestClient(t, mux)

	p, err := c.Network.Lockdown(context.Background(), miosa.LockdownInput{})
	if err != nil {
		t.Fatalf("Lockdown: %v", err)
	}
	if p.Mode != "enforce" {
		t.Errorf("Mode: %s", p.Mode)
	}
	if gotBody["mode"] != "enforce" {
		t.Errorf("body mode: %v", gotBody["mode"])
	}
}

func TestEgressNetworkSuggestions(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/audit/suggestions", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"suggestions": []miosa.EgressSuggestion{
				{Host: "api.openai.com", DeniedCount: 12},
			},
		})
	})
	c := newTestClient(t, mux)

	out, err := c.Network.Suggestions(context.Background(), "cmp_1", "24h")
	if err != nil {
		t.Fatalf("Suggestions: %v", err)
	}
	if len(out) != 1 || out[0].Host != "api.openai.com" {
		t.Errorf("out: %+v", out)
	}
	if !strings.Contains(gotQuery, "resource_id=cmp_1") || !strings.Contains(gotQuery, "since=24h") {
		t.Errorf("query: %s", gotQuery)
	}
}

// ─── EgressAudit ─────────────────────────────────────────────────────────────

func TestEgressAuditList(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/audit", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"events": []miosa.EgressAuditEvent{
				{ID: "evt_1", Host: "api.example.com", Effect: "allow"},
			},
		})
	})
	c := newTestClient(t, mux)

	events, err := c.Audit.List(context.Background(), miosa.EgressAuditListInput{ResourceID: "cmp_1", Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("len: %d", len(events))
	}
	if !strings.Contains(gotQuery, "resource_id=cmp_1") || !strings.Contains(gotQuery, "limit=50") {
		t.Errorf("query: %s", gotQuery)
	}
}

func TestEgressAuditTailRestFallback(t *testing.T) {
	// Tenant-wide Tail uses REST polling. Simulate a server that returns
	// one event on the first poll and nothing thereafter.
	call := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/audit", func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"events": []miosa.EgressAuditEvent{
					{ID: "evt_1", Host: "api.example.com", InsertedAt: "2026-01-01T00:00:01Z"},
				},
			})
		} else {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"events": []miosa.EgressAuditEvent{},
			})
		}
	})
	c := newTestClient(t, mux)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := c.Audit.Tail(ctx, miosa.EgressAuditListInput{}, miosa.TailOptions{
		PollInterval: 5 * time.Millisecond,
		BufferSize:   2,
	})
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}

	select {
	case ev := <-ch:
		if ev.ID != "evt_1" {
			t.Errorf("ID: %s", ev.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
	cancel()
	// Drain — channel should close.
	deadline := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				break loop
			}
		case <-deadline:
			t.Fatal("channel not closed after cancel")
		}
	}
}

// ─── Resource-scoped bindings ────────────────────────────────────────────────

func TestSandboxSecretsBindingScopesResource(t *testing.T) {
	var got miosa.SecretSetInput
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/secrets", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.EgressSecretData{ID: "sec_1", ResourceID: got.ResourceID, ResourceType: got.ResourceType},
		})
	})
	c := newTestClient(t, mux)

	handle := c.Sandboxes.GetHandle("sb_42")
	_, err := handle.Secrets.Set(context.Background(), miosa.SecretSetInput{
		Name:  "API_KEY",
		Value: "v",
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got.ResourceID != "sb_42" || got.ResourceType != "sandbox" {
		t.Errorf("scoping: id=%q type=%q", got.ResourceID, got.ResourceType)
	}
}

func TestComputerNetworkBindingAllow(t *testing.T) {
	var got map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/allowlist", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.EgressAllowlistRule{ID: "rule_1"},
		})
	})
	c := newTestClient(t, mux)

	computer, _ := makeComputer(c, "cmp_42", mux)
	_, err := computer.Network.Allow(context.Background(), "api.example.com", miosa.AllowInput{})
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if got["resource_id"] != "cmp_42" || got["resource_type"] != "computer" {
		t.Errorf("scoping: id=%v type=%v", got["resource_id"], got["resource_type"])
	}
}

func TestComputerAuditBindingList(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/egress/audit", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"events": []miosa.EgressAuditEvent{{ID: "evt_1"}},
		})
	})
	c := newTestClient(t, mux)

	computer, _ := makeComputer(c, "cmp_99", mux)
	events, err := computer.Audit.List(context.Background(), miosa.EgressAuditListInput{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("len: %d", len(events))
	}
	if !strings.Contains(gotQuery, "resource_id=cmp_99") || !strings.Contains(gotQuery, "resource_type=computer") {
		t.Errorf("query missing resource scope: %s", gotQuery)
	}
}
