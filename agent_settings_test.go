package miosa_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

var credJSON = map[string]interface{}{
	"id": "openrouter", "label": "OpenRouter API key", "vendor": "openrouter", "kind": "api_key",
	"connected": true, "status": "connected", "preview": "sk-or-...a1b2", "config": map[string]interface{}{},
	"source": "agent_credential", "scope": "workspace", "inherited": false, "usable_by": []string{"agents"}, "shared": true,
	"fields":    []map[string]interface{}{{"name": "api_key", "label": "API key", "input": "password", "secret": true, "required": true}},
	"signin":    nil,
	"harnesses": []string{"pi", "opencode"},
}

func TestAgentSettings(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"scope":                 map[string]interface{}{"level": "workspace", "workspace_id": "ws1"},
		"default_harness":       "claude-code",
		"default_harness_scope": "platform",
		"credentials":           []interface{}{credJSON},
		"harnesses": []map[string]interface{}{
			{
				"id": "claude-code", "label": "Claude Code", "provider": "claude", "mode": "single", "is_default": true,
				"status":      map[string]interface{}{"state": "ready", "text": nil, "enabled": 1, "total": 1, "hint": nil},
				"auth_method": "anthropic", "active_method": "anthropic", "active_method_source": "configured",
				"auth_methods":  []map[string]interface{}{{"id": "anthropic", "label": "Anthropic API key", "kind": "api_key", "connected": true, "active": true, "signin": false, "scope": "organization"}},
				"default_model": nil, "default_effort": nil, "inherited": []string{"auth_method"},
				"effective": map[string]interface{}{
					"model": "claude-sonnet-5-5", "model_name": "Claude Sonnet 5.5", "effort": "high", "effort_source": "model_default",
					"label":          "Sonnet 5.5 · High",
					"default_option": map[string]interface{}{"label": "Default (Sonnet 5.5)", "model": "claude-sonnet-5-5", "effort": "high"},
				},
			},
			{
				"id": "pi", "label": "pi", "provider": "pi", "mode": "multi", "is_default": false,
				"status":            map[string]interface{}{"state": "ready", "text": "2 of 8 enabled", "enabled": 2, "total": 8},
				"enabled_providers": nil,
				"providers":         []map[string]interface{}{{"id": "anthropic", "label": "Anthropic API key", "kind": "api_key", "connected": true, "enabled": true, "signin": false}},
				"enabled_count":     2, "provider_count": 8, "default_model": "claude-sonnet-5-5", "default_effort": "medium", "inherited": []string{},
			},
			{
				"id": "custom", "label": "Custom gateway", "provider": "custom", "mode": "gateway", "is_default": false, "prompt": false,
				"status": map[string]interface{}{"state": "not_applicable", "enabled": 0, "total": 0},
			},
		},
	}})

	s, err := client.Agents.Settings(context.Background(), "ws1")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/agents/settings")
	if got.RawQuery != "workspace_id=ws1" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if s.Scope.Level != "workspace" || s.DefaultHarness != "claude-code" || s.DefaultHarnessScope != "platform" {
		t.Errorf("settings = %+v", s)
	}
	cc, pi, custom := s.Harnesses[0], s.Harnesses[1], s.Harnesses[2]
	if !cc.IsDefault || cc.ActiveMethod != "anthropic" || cc.AuthMethods[0].Scope != "organization" || cc.Inherited[0] != "auth_method" {
		t.Errorf("claude-code = %+v", cc)
	}
	if cc.Effective == nil || cc.Effective.Label != "Sonnet 5.5 · High" || cc.Effective.DefaultOption.Model != "claude-sonnet-5-5" {
		t.Errorf("effective = %+v", cc.Effective)
	}
	if pi.Mode != "multi" || pi.EnabledProviders != nil || pi.EnabledCount != 2 || pi.Providers[0].ID != "anthropic" || pi.Status.Text != "2 of 8 enabled" {
		t.Errorf("pi = %+v", pi)
	}
	if custom.Prompt == nil || *custom.Prompt || custom.Status.State != "not_applicable" {
		t.Errorf("custom = %+v", custom)
	}
	if c := s.Credentials[0]; c.ID != "openrouter" || !c.Connected || c.Preview != "sk-or-...a1b2" || c.Scope != "workspace" || len(c.Fields) != 1 || !c.Fields[0].Secret {
		t.Errorf("credential = %+v", c)
	}
}

func TestAgentSetDefaultHarness(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"default_harness": "pi", "default_harness_scope": "workspace"}})

	s, err := client.Agents.SetDefaultHarness(context.Background(), "pi", "ws1")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PATCH", "/agents/settings")
	if b := got.Body(t); b["default_harness"] != "pi" || b["workspace_id"] != "ws1" {
		t.Errorf("body = %v", b)
	}
	if s.DefaultHarness != "pi" {
		t.Errorf("settings = %+v", s)
	}
	if _, err := client.Agents.SetDefaultHarness(context.Background(), "", ""); err == nil {
		t.Error("empty harness accepted")
	}

	client, _ = newFixedClient(t, 404, errorEnvelope("UNKNOWN_HARNESS", "no such harness", nil))
	if _, err := client.Agents.SetDefaultHarness(context.Background(), "custom", ""); !miosa.IsCode(err, "UNKNOWN_HARNESS") {
		t.Errorf("error = %v", err)
	}
}

func TestAgentUpdateHarnessBody(t *testing.T) {
	method, model, effort := "bedrock", "claude-opus-5", "xhigh"
	cases := []struct {
		name string
		in   miosa.UpdateAgentHarnessInput
		want string
	}{
		{"single method", miosa.UpdateAgentHarnessInput{AuthMethod: &method}, `{"auth_method":"bedrock"}`},
		{"enabled providers", miosa.UpdateAgentHarnessInput{EnabledProviders: []string{"anthropic", "openai"}}, `{"enabled_providers":["anthropic","openai"]}`},
		{"every connected provider", miosa.UpdateAgentHarnessInput{EnableAllProviders: true}, `{"enabled_providers":null}`},
		{"model and effort", miosa.UpdateAgentHarnessInput{DefaultModel: &model, DefaultEffort: &effort, WorkspaceID: "ws1"},
			`{"default_effort":"xhigh","default_model":"claude-opus-5","workspace_id":"ws1"}`},
		{"clear model", miosa.UpdateAgentHarnessInput{ClearDefaultModel: true, ClearDefaultEffort: true},
			`{"default_effort":null,"default_model":null}`},
		{"nothing", miosa.UpdateAgentHarnessInput{}, `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != tc.want {
				t.Errorf("got  %s\nwant %s", raw, tc.want)
			}
		})
	}
}

func TestAgentUpdateHarness(t *testing.T) {
	model := "gpt-6.1-sol"
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"id": "codex", "label": "Codex", "mode": "single", "default_model": "gpt-6.1-sol",
	}, "default_harness": "claude-code"})

	h, err := client.Agents.UpdateHarness(context.Background(), "codex", miosa.UpdateAgentHarnessInput{DefaultModel: &model})
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "PATCH", "/agents/settings/harnesses/codex")
	if h.ID != "codex" || h.DefaultModel != "gpt-6.1-sol" {
		t.Errorf("harness = %+v", h)
	}

	client, _ = newFixedClient(t, 422, errorEnvelope("VALIDATION_FAILED", "invalid", map[string]interface{}{"details": map[string]interface{}{"default_effort": "not one the model takes"}}))
	_, err = client.Agents.UpdateHarness(context.Background(), "codex", miosa.UpdateAgentHarnessInput{DefaultModel: &model})
	var v *miosa.ValidationError
	if !errors.As(err, &v) || v.Details.(map[string]interface{})["default_effort"] == nil {
		t.Errorf("error = %#v", err)
	}
}

func TestAgentModels(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"harness": "claude-code", "auth_method": "bedrock", "usable_credentials": []string{"bedrock"},
		"default_model":  "us.anthropic.claude-sonnet-4-6",
		"default_option": map[string]interface{}{"label": "Default (Sonnet 4.6 (Bedrock))", "model": "us.anthropic.claude-sonnet-4-6", "effort": "high"},
		"efforts":        []string{"max", "xhigh", "high"},
		"models": []map[string]interface{}{{
			"id": "claude-sonnet-5-5", "name": "Claude Sonnet 5.5", "vendor": "anthropic", "variant": nil,
			"served_by": []string{"anthropic"}, "efforts": []string{"high", "low"}, "default_effort": "high", "default": false,
			"available": false, "unavailable_reason": "Amazon Bedrock is the active credential", "via": nil,
			"context_window": 1000000, "capabilities": map[string]bool{"vision": true, "tools": true},
			"tier": "balanced", "price_per_mtok": map[string]interface{}{"input": 2.0, "output": 10.0, "cache_read": 0.1, "cache_write": nil},
		}},
	}})

	m, err := client.Agents.Models(context.Background(), "claude-code", "ws1")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/agents/models")
	if got.RawQuery != "harness=claude-code&workspace_id=ws1" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if m.AuthMethod != "bedrock" || m.UsableCredentials[0] != "bedrock" || m.DefaultOption.Model != "us.anthropic.claude-sonnet-4-6" {
		t.Errorf("models = %+v", m)
	}
	mod := m.Models[0]
	if mod.Available || mod.UnavailableReason == "" || *mod.ContextWindow != 1000000 || !mod.Capabilities["vision"] {
		t.Errorf("model = %+v", mod)
	}
	if mod.PricePerMTok.Input != 2.0 || *mod.PricePerMTok.CacheRead != 0.1 || mod.PricePerMTok.CacheWrite != nil {
		t.Errorf("price = %+v", mod.PricePerMTok)
	}
	if _, err := client.Agents.Models(context.Background(), "", ""); err == nil {
		t.Error("empty harness accepted")
	}
}

func TestAgentCredentials(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": []interface{}{credJSON}})
	creds, err := client.Agents.Credentials(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/agents/credentials")
	if got.RawQuery != "" || creds[0].Harnesses[1] != "opencode" {
		t.Errorf("query=%q creds=%+v", got.RawQuery, creds)
	}
}

func TestAgentPutCredentialFlattensFields(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": credJSON})

	_, err := client.Agents.PutCredential(context.Background(), "bedrock", miosa.PutAgentCredentialInput{
		Fields:      map[string]string{"region": "us-east-1", "api_key": "abc"},
		WorkspaceID: "ws1",
		UsableBy:    []string{"agents"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PUT", "/agents/credentials/bedrock")
	b := got.Body(t)
	if b["region"] != "us-east-1" || b["api_key"] != "abc" || b["workspace_id"] != "ws1" {
		t.Errorf("body = %v", b)
	}
	if ub := b["usable_by"].([]interface{}); len(ub) != 1 || ub[0] != "agents" {
		t.Errorf("usable_by = %v", ub)
	}
	if _, err := client.Agents.PutCredential(context.Background(), "", miosa.PutAgentCredentialInput{}); err == nil {
		t.Error("empty credential accepted")
	}
}

func TestAgentPutCredentialOmitsUsableByWhenUnset(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": credJSON})
	if _, err := client.Agents.PutCredential(context.Background(), "openai", miosa.PutAgentCredentialInput{Fields: map[string]string{"api_key": "k"}}); err != nil {
		t.Fatal(err)
	}
	if _, present := rec.Last(t).Body(t)["usable_by"]; present {
		t.Error("usable_by must be omitted so the server default applies")
	}
}

func TestAgentPutCredentialUseSigninRefusal(t *testing.T) {
	client, _ := newFixedClient(t, 422, errorEnvelope("USE_SIGNIN", "sign in instead", nil))
	_, err := client.Agents.PutCredential(context.Background(), "claude_subscription", miosa.PutAgentCredentialInput{})
	if !miosa.IsCode(err, "USE_SIGNIN") {
		t.Errorf("error = %v", err)
	}
}

func TestAgentDeleteCredential(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "openrouter", "connected": false}})
	c, err := client.Agents.DeleteCredential(context.Background(), "openrouter", "ws1")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "DELETE", "/agents/credentials/openrouter")
	if got.RawQuery != "workspace_id=ws1" || c.Connected {
		t.Errorf("query=%q cred=%+v", got.RawQuery, c)
	}
}

func TestAgentHarnessesCatalog(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{
		"data":                 []map[string]interface{}{{"value": "claude-code", "runtime": map[string]interface{}{"mcp_servers": true}}},
		"runtime_capabilities": map[string]interface{}{"version": 1},
	})
	cat, err := client.Agents.Harnesses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/agents/harnesses")
	if cat.Data[0]["value"] != "claude-code" || cat.RuntimeCapabilities["version"] != float64(1) {
		t.Errorf("catalog = %+v", cat)
	}
}

func TestChatContext(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"chat_id": "chat-1", "harness": "claude-code", "harness_session_id": "sess", "runs": 4,
		"latest_run": map[string]interface{}{"id": "run_4", "status": "succeeded", "created_at": "2026-10-10T00:00:00Z"},
		"machine":    map[string]interface{}{"kind": "sandbox", "id": "sbx_1"},
		"context": map[string]interface{}{
			"used_tokens": 120000, "window_tokens": 1000000, "remaining_tokens": 880000, "used_ratio": 0.12,
			"compactions": 1, "compacted": true, "window_mode": "1m", "measured_by_run_id": "run_4",
		},
		"compactions":        []map[string]interface{}{{"run_id": "run_2", "before_tokens": 900000, "after_tokens": 100000}},
		"compactable":        true,
		"compactable_reason": nil,
	}})

	c, err := client.Agents.ChatContext(context.Background(), "chat-1")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/agents/chats/chat-1/context")
	if c.Runs != 4 || c.Harness != "claude-code" || c.LatestRun.Status != "succeeded" || c.Machine.ID != "sbx_1" || !c.Compactable {
		t.Errorf("chat = %+v", c)
	}
	if c.Context == nil || *c.Context.UsedTokens != 120000 || *c.Context.UsedRatio != 0.12 || c.Context.WindowMode != "1m" || !c.Context.Compacted {
		t.Errorf("context = %+v", c.Context)
	}
	if len(c.Compactions) != 1 || *c.Compactions[0].BeforeTokens != 900000 || c.Compactions[0].RunID != "run_2" {
		t.Errorf("compactions = %+v", c.Compactions)
	}
}

func TestChatContextUnmeasuredAndNotFound(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"chat_id": "c", "context": nil, "compactions": []interface{}{}, "compactable": false, "compactable_reason": "harness_unsupported",
	}})
	c, err := client.Agents.ChatContext(context.Background(), "c")
	if err != nil || c.Context != nil || c.CompactableReason != "harness_unsupported" {
		t.Fatalf("chat=%+v err=%v", c, err)
	}

	client, _ = newFixedClient(t, 404, errorEnvelope("CHAT_NOT_FOUND", "no chat with that id", nil))
	if _, err := client.Agents.ChatContext(context.Background(), "nope"); !miosa.IsCode(err, "CHAT_NOT_FOUND") {
		t.Errorf("error = %v", err)
	}
}

func TestCompactChat(t *testing.T) {
	client, rec := newFixedClient(t, http.StatusAccepted, map[string]interface{}{
		"data":        map[string]interface{}{"id": "run_5", "status": "queued", "chat_id": "chat-1"},
		"chat_id":     "chat-1",
		"instruction": "/compact keep the schema",
	})

	res, err := client.Agents.CompactChat(context.Background(), "chat-1", "keep the schema")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/agents/chats/chat-1/compact")
	if got.Body(t)["focus"] != "keep the schema" {
		t.Errorf("body = %v", got.Body(t))
	}
	if res.Run.ID != "run_5" || res.ChatID != "chat-1" || res.Instruction != "/compact keep the schema" {
		t.Errorf("result = %+v", res)
	}

	if _, err := client.Agents.CompactChat(context.Background(), "chat-1", ""); err != nil {
		t.Fatal(err)
	}
	if _, present := rec.Last(t).Body(t)["focus"]; present {
		t.Error("focus must be omitted when empty")
	}
}

func TestCompactChatRefusals(t *testing.T) {
	client, _ := newFixedClient(t, 422, errorEnvelope("COMPACTION_UNSUPPORTED", "cannot compact codex chats", nil))
	if _, err := client.Agents.CompactChat(context.Background(), "c", ""); !miosa.IsCode(err, "COMPACTION_UNSUPPORTED") {
		t.Errorf("error = %v", err)
	}
	client, _ = newFixedClient(t, 409, errorEnvelope("CHAT_BUSY", "a run is in progress", nil))
	if _, err := client.Agents.CompactChat(context.Background(), "c", ""); !miosa.IsCode(err, "CHAT_BUSY") {
		t.Errorf("error = %v", err)
	}
}

// ─── Sign-ins and accounts ───────────────────────────────────────────────────

func TestSigninFlows(t *testing.T) {
	cases := []struct {
		provider miosa.SigninProvider
		mode     string
		userCode string
	}{
		{miosa.SigninClaudeCode, "paste_code", ""},
		{miosa.SigninCodex, "device_code", "ABCD-1234"},
		{miosa.SigninKimiCode, "device_code", "KIMI-0001"},
		{miosa.SigninMistralVibe, "device_code", ""},
	}
	for _, tc := range cases {
		t.Run(string(tc.provider), func(t *testing.T) {
			client, rec := newFixedClient(t, 201, map[string]interface{}{"data": map[string]interface{}{
				"id": "s1", "provider": string(tc.provider), "status": "awaiting_code", "mode": tc.mode,
				"verification_url": "https://auth.example/device", "user_code": tc.userCode, "expires_at": "2026-10-10T00:15:00Z",
			}})

			s, err := client.AgentAccounts.StartSignin(context.Background(), miosa.StartSigninInput{Provider: tc.provider, WorkspaceID: "ws1"})
			if err != nil {
				t.Fatal(err)
			}
			got := rec.Last(t)
			assertReq(t, got, "POST", "/agent-signins")
			if b := got.Body(t); b["provider"] != string(tc.provider) || b["workspace_id"] != "ws1" {
				t.Errorf("body = %v", b)
			}
			if s.Mode != tc.mode || s.UserCode != tc.userCode || s.VerificationURL == "" || s.Status != "awaiting_code" {
				t.Errorf("session = %+v", s)
			}
		})
	}
}

func TestSigninPollSubmitCancel(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "s1", "status": "succeeded"}})

	s, err := client.AgentAccounts.GetSignin(context.Background(), "s1")
	if err != nil || s.Status != "succeeded" {
		t.Fatalf("%+v %v", s, err)
	}
	assertReq(t, rec.Last(t), "GET", "/agent-signins/s1")

	if _, err := client.AgentAccounts.SubmitSigninCode(context.Background(), "s1", "code#state"); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/agent-signins/s1/code")
	if got.Body(t)["code"] != "code#state" {
		t.Errorf("body = %v", got.Body(t))
	}
	if _, err := client.AgentAccounts.SubmitSigninCode(context.Background(), "s1", ""); err == nil {
		t.Error("empty code accepted")
	}

	if _, err := client.AgentAccounts.CancelSignin(context.Background(), "s1"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "DELETE", "/agent-signins/s1")
}

func TestSigninStartValidation(t *testing.T) {
	client, rec := newFixedClient(t, 201, nil)
	if _, err := client.AgentAccounts.StartSignin(context.Background(), miosa.StartSigninInput{}); err == nil {
		t.Fatal("empty provider accepted")
	}
	if rec.Count() != 0 {
		t.Error("reached the server")
	}
	client, _ = newFixedClient(t, 422, errorEnvelope("UNKNOWN_PROVIDER", "bad", nil))
	if _, err := client.AgentAccounts.StartSignin(context.Background(), miosa.StartSigninInput{Provider: "nope"}); !miosa.IsCode(err, "UNKNOWN_PROVIDER") {
		t.Errorf("error = %v", err)
	}
}

func TestAgentAccountsListCreateDelete(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeJSON(w, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "a1", "provider": "claude_code"}}, "total": 1})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": "a1"}})
	})

	accts, err := client.AgentAccounts.List(context.Background(), miosa.ListAccountsOptions{WorkspaceID: "ws1", Provider: "codex"})
	if err != nil || len(accts) != 1 || accts[0]["id"] != "a1" {
		t.Fatalf("%v %v", accts, err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/agent-accounts")
	if got.RawQuery != "provider=codex&workspace_id=ws1" {
		t.Errorf("query = %q", got.RawQuery)
	}

	if _, err := client.AgentAccounts.CreateAPIKey(context.Background(), "codex", "sk-1", "ws1", "ci"); err != nil {
		t.Fatal(err)
	}
	got = rec.Last(t)
	assertReq(t, got, "POST", "/agent-accounts/api-keys")
	if b := got.Body(t); b["provider"] != "codex" || b["api_key"] != "sk-1" || b["label"] != "ci" || b["workspace_id"] != "ws1" {
		t.Errorf("body = %v", b)
	}

	if _, err := client.AgentAccounts.Delete(context.Background(), "a1"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "DELETE", "/agent-accounts/a1")
}

// ─── Connections ─────────────────────────────────────────────────────────────

func TestConnectionsList(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id": "openrouter", "ref": "agent_credential:openrouter", "type": "agent_credential", "family": "models", "kind": "api_key",
				"provider": "openrouter", "label": "OpenRouter API key", "account": map[string]interface{}{"id": nil, "display_name": nil},
				"scope": map[string]interface{}{"level": "organization", "workspace_id": nil, "project_id": nil}, "inherited": true,
				"scopes_granted": []string{}, "status": "connected", "preview": "sk-or-...a1b2", "usable_by": []string{"agents", "gateway"},
				"shared": true, "used_by": []string{"pi", "opencode"}, "used_by_count": 2, "owner": nil,
			},
			{
				"id": "m1", "ref": "mcp_server:m1", "type": "mcp_server", "family": "tools", "kind": "http", "provider": "linear",
				"status": "reachable", "usable_by": []string{"agents", "workflows"}, "used_by": map[string]interface{}{"agent_versions": 3}, "used_by_count": 3,
			},
			{
				"id": "gh1", "ref": "integration:gh1", "type": "integration", "family": "apps", "kind": "oauth", "provider": "github",
				"status": "connected", "shared": false, "used_by": nil, "used_by_count": 4, "owner": map[string]interface{}{"user_id": "u1", "mine": true},
			},
		},
		"total": 3, "counts": map[string]int{"models": 1, "apps": 1, "tools": 1},
	})

	list, err := client.Connections.List(context.Background(), miosa.ListConnectionsOptions{WorkspaceID: "ws1", Family: "models", UsableBy: "gateway"})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/connections")
	if got.RawQuery != "family=models&usable_by=gateway&workspace_id=ws1" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if list.Total != 3 || list.Counts["tools"] != 1 || len(list.Data) != 3 {
		t.Errorf("list = %+v", list)
	}
	models, tools, apps := list.Data[0], list.Data[1], list.Data[2]
	if models.Ref != "agent_credential:openrouter" || !models.Inherited || models.Scope.Level != "organization" || models.Preview == "" || models.UsedByCount != 2 {
		t.Errorf("models conn = %+v", models)
	}
	var harnesses []string
	if err := json.Unmarshal(models.UsedBy, &harnesses); err != nil || harnesses[0] != "pi" {
		t.Errorf("used_by = %s (%v)", models.UsedBy, err)
	}
	var agentVersions map[string]int
	if err := json.Unmarshal(tools.UsedBy, &agentVersions); err != nil || agentVersions["agent_versions"] != 3 {
		t.Errorf("tool used_by = %s (%v)", tools.UsedBy, err)
	}
	if apps.Owner == nil || !apps.Owner.Mine || apps.Shared || apps.Family != "apps" {
		t.Errorf("app conn = %+v", apps)
	}
}

func TestConnectionsShareApp(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "gh1", "shared": true, "family": "apps"}})

	c, err := client.Connections.ShareApp(context.Background(), "gh1", true)
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PATCH", "/connections/apps/gh1")
	if got.Body(t)["shared"] != true || !c.Shared {
		t.Errorf("body=%v conn=%+v", got.Body(t), c)
	}

	client, _ = newFixedClient(t, 404, errorEnvelope("NOT_FOUND", "not found", nil))
	_, err = client.Connections.ShareApp(context.Background(), "someone-elses", true)
	var nf *miosa.NotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("error = %#v", err)
	}
}
