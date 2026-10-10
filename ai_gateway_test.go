package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

var policyJSON = map[string]interface{}{
	"id": "p1", "name": "prod routing", "selector": map[string]interface{}{"environment": "production"},
	"primary_model": "claude-sonnet-5-5", "fallback_models": []string{"gpt-6.1-sol"}, "alias_name": "smart",
	"strategy": "priority", "weights": nil, "priority": 10, "enabled": true,
	"created_at": "2026-10-09T00:00:00Z", "updated_at": "2026-10-09T00:00:00Z",
}

func TestAIGatewayPolicies(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ai-gateway/policies":
			writeJSON(w, 200, map[string]interface{}{"data": []interface{}{policyJSON}})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost:
			writeJSON(w, 201, map[string]interface{}{"data": policyJSON})
		default:
			writeJSON(w, 200, map[string]interface{}{"data": policyJSON})
		}
	})
	ctx := context.Background()

	list, err := client.AIGateway.ListPolicies(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	if p := list[0]; p.PrimaryModel != "claude-sonnet-5-5" || p.FallbackModels[0] != "gpt-6.1-sol" || p.Selector["environment"] != "production" || !p.Enabled || p.Priority != 10 {
		t.Errorf("policy = %+v", p)
	}

	if _, err := client.AIGateway.GetPolicy(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/ai-gateway/policies/p1")

	prio, off := 5, false
	if _, err := client.AIGateway.CreatePolicy(ctx, miosa.GatewayPolicyInput{Name: "n", PrimaryModel: "m", Priority: &prio, Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/ai-gateway/policies")
	if b := got.Body(t); b["name"] != "n" || b["primary_model"] != "m" || b["priority"] != float64(5) || b["enabled"] != false {
		t.Errorf("body = %v", b)
	}

	newName := "renamed"
	if _, err := client.AIGateway.UpdatePolicy(ctx, "p1", miosa.GatewayPolicyUpdate{Name: &newName}); err != nil {
		t.Fatal(err)
	}
	got = rec.Last(t)
	assertReq(t, got, "PATCH", "/ai-gateway/policies/p1")
	if len(got.Body(t)) != 1 {
		t.Errorf("a patch carries only what changed: %v", got.Body(t))
	}

	if err := client.AIGateway.DeletePolicy(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "DELETE", "/ai-gateway/policies/p1")
}

func TestAIGatewayBudget(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"id": "b1", "monthly_credit_limit": 50000, "per_request_credit_limit": nil, "requests_per_minute": 600,
		"tokens_per_day": nil, "consumed_credits": 1200, "hard_limit": true, "period_start": "2026-10-01T00:00:00Z",
	}})

	b, err := client.AIGateway.Budget(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/ai-gateway/budget")
	if *b.MonthlyCreditLimit != 50000 || b.PerRequestCreditLimit != nil || *b.RequestsPerMinute != 600 || b.ConsumedCredits != 1200 || !b.HardLimit {
		t.Errorf("budget = %+v", b)
	}

	limit, hard := int64(100000), false
	if _, err := client.AIGateway.PutBudget(context.Background(), miosa.GatewayBudgetInput{MonthlyCreditLimit: &limit, HardLimit: &hard}); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PUT", "/ai-gateway/budget")
	if body := got.Body(t); body["monthly_credit_limit"] != float64(100000) || body["hard_limit"] != false || len(body) != 2 {
		t.Errorf("body = %v", body)
	}
}

func TestAIGatewayBudgetNotConfigured(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{"data": nil})
	b, err := client.AIGateway.Budget(context.Background())
	if err != nil || b != nil {
		t.Fatalf("budget = %+v, err = %v; want nil, nil", b, err)
	}
}

func TestAIGatewayHealthAndProviderHealth(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"status": "ok"}})
	h, err := client.AIGateway.Health(context.Background())
	if err != nil || h["status"] != "ok" {
		t.Fatalf("%v %v", h, err)
	}
	assertReq(t, rec.Last(t), "GET", "/ai-gateway/health")

	if _, err := client.AIGateway.ProviderHealth(context.Background(), "7d"); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/ai-gateway/provider-health")
	if got.RawQuery != "window=7d" {
		t.Errorf("query = %q", got.RawQuery)
	}
}

func TestAIGatewayLimits(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut:
			writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{
				"scope_type": "environment", "scope_value": "production", "requests_per_minute": 120,
				"tokens_per_day": nil, "monthly_credit_limit": nil, "hard_limit": true,
			}})
		default:
			writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{
				"organization": map[string]interface{}{"limits": map[string]interface{}{"configured": true}},
				"keys":         []map[string]interface{}{{"id": "k1", "name": "ci", "environment": "staging"}},
				"environments": []map[string]interface{}{{"environment": "production"}},
				"resets":       map[string]interface{}{"minute_seconds": 12},
			}})
		}
	})
	ctx := context.Background()

	o, err := client.AIGateway.Limits(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/ai-gateway/limits")
	if len(o.Keys) != 1 || o.Keys[0]["environment"] != "staging" || o.Resets["minute_seconds"] != float64(12) {
		t.Errorf("overview = %+v", o)
	}

	rpm := int64(120)
	l, err := client.AIGateway.PutLimit(ctx, miosa.GatewayScopeEnvironment, "production", miosa.GatewayLimitInput{RequestsPerMinute: &rpm})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PUT", "/ai-gateway/limits/environment/production")
	if got.Body(t)["requests_per_minute"] != float64(120) || *l.RequestsPerMinute != 120 || l.TokensPerDay != nil || !l.HardLimit {
		t.Errorf("body=%v limit=%+v", got.Body(t), l)
	}

	if err := client.AIGateway.DeleteLimit(ctx, miosa.GatewayScopeKey, "k1"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "DELETE", "/ai-gateway/limits/key/k1")
}

func TestAIGatewayEnvironments(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": "k1", "name": "ci", "environment": "staging"}})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"data": []map[string]interface{}{{
			"environment": "production",
			"keys":        []map[string]interface{}{{"id": "k1", "name": "ci", "key_prefix": "msk_i_ab"}},
			"key_count":   1, "policy_count": 2, "limits": map[string]interface{}{}, "usage": map[string]interface{}{},
		}}})
	})

	envs, err := client.AIGateway.Environments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/ai-gateway/environments")
	if envs[0].Environment != "production" || envs[0].KeyCount != 1 || envs[0].PolicyCount != 2 || envs[0].Keys[0].KeyPrefix != "msk_i_ab" {
		t.Errorf("envs = %+v", envs)
	}

	k, err := client.AIGateway.SetKeyEnvironment(context.Background(), "k1", "staging")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PUT", "/ai-gateway/keys/k1/environment")
	if got.Body(t)["environment"] != "staging" || k.Environment != "staging" || k.Name != "ci" {
		t.Errorf("body=%v key=%+v", got.Body(t), k)
	}
	if _, err := client.AIGateway.SetKeyEnvironment(context.Background(), "k1", ""); err == nil {
		t.Error("empty environment accepted")
	}
}

func TestAIGatewaySettings(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"default_model": "claude-sonnet-5-5", "logging_mode": "metadata", "retention_days": 7, "max_retention_days": 30,
		"default_classification": "internal",
		"transcription":          map[string]interface{}{"enabled": false, "explicit": nil, "default_enabled": false, "healthcare_tenant": true},
	}})

	s, err := client.AIGateway.Settings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/ai-gateway/settings")
	if s.LoggingMode != "metadata" || s.RetentionDays != 7 || s.MaxRetention != 30 || !s.Transcription.HealthcareTenant || s.Transcription.Explicit != nil {
		t.Errorf("settings = %+v", s)
	}

	yes := true
	mode := "off"
	if _, err := client.AIGateway.UpdateSettings(context.Background(), miosa.GatewaySettingsInput{LoggingMode: &mode, TranscriptionEnabled: &yes, AcknowledgeNoBAA: true}); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PUT", "/ai-gateway/settings")
	b := got.Body(t)
	if b["logging_mode"] != "off" || b["transcription_enabled"] != true || b["acknowledge_no_baa"] != true {
		t.Errorf("body = %v", b)
	}
}

func TestAIGatewaySettingsResetTranscriptionSendsNull(t *testing.T) {
	raw, err := json.Marshal(miosa.GatewaySettingsInput{ResetTranscription: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"transcription_enabled":null}` {
		t.Errorf("got %s", raw)
	}
	raw, _ = json.Marshal(miosa.GatewaySettingsInput{})
	if string(raw) != `{}` {
		t.Errorf("an empty update must send nothing, got %s", raw)
	}
}

func TestAIGatewaySettingsAcknowledgementRequired(t *testing.T) {
	client, _ := newFixedClient(t, 422, errorEnvelope("ACKNOWLEDGEMENT_REQUIRED", "send acknowledge_no_baa", nil))
	yes := true
	_, err := client.AIGateway.UpdateSettings(context.Background(), miosa.GatewaySettingsInput{TranscriptionEnabled: &yes})
	if !miosa.IsCode(err, "ACKNOWLEDGEMENT_REQUIRED") {
		t.Fatalf("error = %v", err)
	}
}

func TestAIGatewayRequests(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{
		"data": []map[string]interface{}{{
			"id": "t1", "request_id": "r1", "attempt": 1, "operation": "chat", "requested_model": "smart", "model": "claude-sonnet-5-5",
			"provider": "anthropic", "api_key_id": "k1", "status": "success", "http_status": 200, "error_code": nil, "stream": true,
			"prompt_tokens": 10, "completion_tokens": 20, "cost_credits": 0.5, "latency_ms": 840, "created_at": "2026-10-10T00:00:00Z",
		}},
		"summary": map[string]interface{}{"requests": 1}, "next_cursor": "c2", "retention_days": 14,
	})

	list, err := client.AIGateway.Requests(context.Background(), miosa.GatewayRequestsOptions{Status: "error", Provider: "anthropic", Limit: 50, Cursor: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/ai-gateway/requests")
	if got.RawQuery != "cursor=c1&limit=50&provider=anthropic&status=error" {
		t.Errorf("query = %q", got.RawQuery)
	}
	tr := list.Data[0]
	if tr.Provider != "anthropic" || !tr.Stream || tr.LatencyMS != 840 || tr.CostCredits != 0.5 || tr.RequestedModel != "smart" || list.NextCursor != "c2" || list.RetentionDays != 14 {
		t.Errorf("list = %+v", list)
	}
}

func TestAIGatewayRequestAndNotFound(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "t1", "status": "error", "error_code": "RATE_LIMITED"}})
	tr, err := client.AIGateway.Request(context.Background(), "t1")
	if err != nil || tr.ErrorCode != "RATE_LIMITED" {
		t.Fatalf("%+v %v", tr, err)
	}
	assertReq(t, rec.Last(t), "GET", "/ai-gateway/requests/t1")

	client, _ = newFixedClient(t, 404, errorEnvelope("NOT_FOUND", "request trace not found", nil))
	if _, err := client.AIGateway.Request(context.Background(), "nope"); err == nil {
		t.Error("expected not found")
	}
}
