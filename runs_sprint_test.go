package miosa_test

import (
	"context"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func TestRunsUsageGroupedByAgentAndDay(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"from": "2026-09-10T00:00:00Z", "to": "2026-10-10T00:00:00Z", "group_by": []string{"agent", "day"},
		"totals": map[string]interface{}{
			"runs": 5, "input_tokens": 500, "output_tokens": 60, "cache_read_tokens": 5000, "cache_write_tokens": 0,
			"total_tokens": 5560, "estimated_usd": 1.75, "priced_runs": 3, "subscription_runs": 1, "unpriced_runs": 1,
		},
		"groups": []map[string]interface{}{{
			"key":  map[string]interface{}{"agent_definition_id": "6f1c", "day": "2026-10-01"},
			"runs": 2, "input_tokens": 200, "output_tokens": 20, "cache_read_tokens": 2000, "cache_write_tokens": 0,
			"total_tokens": 2220, "estimated_usd": 1.25, "priced_runs": 2, "subscription_runs": 0, "unpriced_runs": 0,
		}, {
			"key":  map[string]interface{}{"agent_definition_id": nil, "day": "2026-10-02"},
			"runs": 1,
		}},
		"truncated": true,
	}})

	u, err := client.Runs.Usage(context.Background(), miosa.RunUsageOptions{
		GroupBy:  []miosa.RunUsageGroupBy{miosa.RunUsageByAgent, miosa.RunUsageByDay},
		From:     "2026-09-10",
		To:       "2026-10-10",
		Harness:  "claude-code",
		BilledTo: "own_key",
		Limit:    100,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/runs/usage")
	if got.RawQuery != "billed_to=own_key&from=2026-09-10&group_by=agent%2Cday&harness=claude-code&limit=100&to=2026-10-10" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if u.Totals.Runs != 5 || u.Totals.EstimatedUSD != 1.75 || u.Totals.UnpricedRuns != 1 || u.Totals.PricedRuns+u.Totals.SubscriptionRuns+u.Totals.UnpricedRuns != u.Totals.Runs {
		t.Errorf("totals = %+v", u.Totals)
	}
	if !u.More || len(u.Groups) != 2 || u.GroupBy[1] != "day" {
		t.Errorf("usage = %+v", u)
	}
	g0, g1 := u.Groups[0], u.Groups[1]
	if *g0.Key["agent_definition_id"] != "6f1c" || *g0.Key["day"] != "2026-10-01" || g0.TotalTokens != 2220 {
		t.Errorf("group 0 = %+v", g0)
	}
	if g1.Key["agent_definition_id"] != nil {
		t.Errorf("a run without an agent has a null key, got %v", g1.Key["agent_definition_id"])
	}
}

func TestRunsUsageTotalsOnlyAndErrors(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"totals": map[string]interface{}{"runs": 0}, "groups": []interface{}{}}})
	if _, err := client.Runs.Usage(context.Background(), miosa.RunUsageOptions{}); err != nil {
		t.Fatal(err)
	}
	if rec.Last(t).RawQuery != "" {
		t.Errorf("query = %q; the server default window applies", rec.Last(t).RawQuery)
	}

	client, _ = newFixedClient(t, 422, errorEnvelope("VALIDATION_FAILED", "bad group_by", map[string]interface{}{"details": map[string]interface{}{"group_by": "unknown"}}))
	if _, err := client.Runs.Usage(context.Background(), miosa.RunUsageOptions{GroupBy: []miosa.RunUsageGroupBy{"nope"}}); !miosa.IsCode(err, "VALIDATION_FAILED") {
		t.Errorf("error = %v", err)
	}
}

func TestRunCreatePromptPathChatAndMachine(t *testing.T) {
	computerUse := true
	client, rec := newFixedClient(t, http.StatusAccepted, map[string]interface{}{"data": map[string]interface{}{
		"id": "run_1", "status": "queued", "target_kind": "sandbox", "target_id": "sbx_9", "runner": "claude-code",
		"chat_id": "chat-1", "harness_session_id": "sess-1", "continued_from_run_id": "run_0", "session_mode": "resumed", "overdrive": true,
		"source": "api",
		"machine": map[string]interface{}{
			"lease_id": "l1", "kind": "sandbox", "id": "sbx_9", "origin": "provisioned", "state": "provisioning", "reused": false,
			"reuse": "chat", "after_run": "destroy", "idle_timeout_seconds": 900,
		},
		"notify": map[string]interface{}{"email": true, "on": []string{"failed"}},
	}})

	run, err := client.Runs.Run(context.Background(), miosa.RunCreateInput{
		Instruction:     "fix the build",
		ReasoningEffort: "high",
		ChatID:          "chat-1",
		Target:          "new",
		WorkspaceID:     "ws1",
		EnvironmentID:   "env_1",
		Machine:         &miosa.RunMachineRequest{Target: "computer", Reuse: "chat", AfterRun: "pause", ComputerUse: &computerUse},
		Notify:          &miosa.RunNotify{Email: true, On: []string{"failed"}},
		Source:          "ci",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/runs")
	body := got.Body(t)
	for k, want := range map[string]interface{}{
		"instruction": "fix the build", "reasoning_effort": "high", "chat_id": "chat-1", "target": "new",
		"workspace_id": "ws1", "environment_id": "env_1", "source": "ci",
	} {
		if body[k] != want {
			t.Errorf("body[%s] = %v, want %v", k, body[k], want)
		}
	}
	m := body["machine"].(map[string]interface{})
	if m["target"] != "computer" || m["reuse"] != "chat" || m["after_run"] != "pause" || m["computer_use"] != true {
		t.Errorf("machine = %v", m)
	}
	if n := body["notify"].(map[string]interface{}); n["email"] != true {
		t.Errorf("notify = %v", n)
	}
	for _, k := range []string{"runner", "sandbox_id", "target_id", "no_env", "continue_from_run_id"} {
		if _, present := body[k]; present {
			t.Errorf("%q must be omitted when unset", k)
		}
	}

	if run.ChatID != "chat-1" || run.HarnessSessionID != "sess-1" || run.ContinuedFromRunID != "run_0" || run.SessionMode != "resumed" || run.Overdrive == nil || !*run.Overdrive {
		t.Errorf("session fields: %+v", run)
	}
	if run.Machine == nil || run.Machine.Origin != "provisioned" || run.Machine.State != "provisioning" || run.Machine.IdleTimeoutSeconds != 900 || run.Source != "api" {
		t.Errorf("machine: %+v", run.Machine)
	}
	if run.Notify == nil || !run.Notify.Email || run.Notify.On[0] != "failed" {
		t.Errorf("notify: %+v", run.Notify)
	}
}

func TestRunDecodesCostAndProviderCost(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"id": "run_1", "status": "succeeded",
		"cost": map[string]interface{}{"model_credits": nil, "machine_credits": 3, "total_credits": 3, "model_metered": false, "machine_estimated": true},
		"provider_cost": map[string]interface{}{
			"input_tokens": 18, "output_tokens": 176, "cache_read_tokens": 204268, "cache_write_tokens": 26660,
			"estimated_usd": 0.074645, "source": "harness", "model": "claude-haiku-4-5-20251001", "billed_to": "own_key",
		},
		"context": map[string]interface{}{"context_tokens": 1000, "context_window": 1000000},
	}})

	run, err := client.Runs.Get(context.Background(), "run_1")
	if err != nil {
		t.Fatal(err)
	}
	if run.Cost == nil || run.Cost.ModelCredits != nil || run.Cost.MachineCredits != 3 || run.Cost.ModelMetered || !run.Cost.MachineEstimated {
		t.Errorf("cost = %+v", run.Cost)
	}
	pc := run.ProviderCost
	if pc == nil || pc.CacheReadTokens != 204268 || *pc.EstimatedUSD != 0.074645 || pc.Source != "harness" || pc.BilledTo != "own_key" {
		t.Errorf("provider cost = %+v", pc)
	}
	if run.Context["context_tokens"] != float64(1000) {
		t.Errorf("context = %v", run.Context)
	}
}

func TestRunSubscriptionProviderCostHasNullUSD(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"id": "run_1", "provider_cost": map[string]interface{}{"input_tokens": 5, "estimated_usd": nil, "source": "catalog", "billed_to": "subscription"},
	}})
	run, err := client.Runs.Get(context.Background(), "run_1")
	if err != nil {
		t.Fatal(err)
	}
	if run.ProviderCost == nil || run.ProviderCost.EstimatedUSD != nil || run.ProviderCost.BilledTo != "subscription" {
		t.Errorf("provider cost = %+v", run.ProviderCost)
	}
}

func TestRunsListNewFilters(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": []interface{}{}})

	_, err := client.Runs.List(context.Background(), miosa.RunListInput{
		ChatID: "chat-1", AgentDefinitionID: "a1", Harness: "codex", Source: "api", Limit: 200, WorkspaceID: "ws1",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/runs")
	if got.RawQuery != "agent_definition_id=a1&chat_id=chat-1&harness=codex&limit=200&source=api&workspace_id=ws1" {
		t.Errorf("query = %q", got.RawQuery)
	}
}

func TestRunSessionNotResumable(t *testing.T) {
	client, _ := newFixedClient(t, 409, errorEnvelope("RUN_SESSION_NOT_RESUMABLE", "not resumable", map[string]interface{}{
		"details": map[string]interface{}{"reason": "still_running"},
	}))
	_, err := client.Runs.Run(context.Background(), miosa.RunCreateInput{Instruction: "x", ChatID: "c", ContinueFromRunID: "r"})
	m := miosa.AsMiosaError(err)
	if m == nil || m.Code != "RUN_SESSION_NOT_RESUMABLE" || m.Details.(map[string]interface{})["reason"] != "still_running" {
		t.Fatalf("error = %#v", err)
	}
}

func TestRunOwnCredentialsRequired(t *testing.T) {
	client, _ := newFixedClient(t, 422, errorEnvelope("OWN_CREDENTIALS_REQUIRED", "connect your own key", nil))
	_, err := client.Runs.Run(context.Background(), miosa.RunCreateInput{Instruction: "hello"})
	if !miosa.IsCode(err, miosa.CodeOwnCredentialsRequired) {
		t.Fatalf("error = %v", err)
	}
}
