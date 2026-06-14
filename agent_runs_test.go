package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go"
)

func TestAgentRunsCreate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/agent-runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("Idempotency-Key"); got != "run-key-1" {
			t.Fatalf("Idempotency-Key: want run-key-1, got %q", got)
		}
		var body miosa.CreateAgentRunInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.TargetID != "sbx_123" || body.Provider != "custom" || body.Command != "echo" {
			t.Fatalf("unexpected body: %#v", body)
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": map[string]interface{}{
				"id":          "run_123",
				"target_kind": "sandbox",
				"target_id":   "sbx_123",
				"provider":    "custom",
				"prompt":      "inspect files",
				"status":      "succeeded",
				"output":      "ok\n",
			},
		})
	})

	client := newTestClient(t, mux)
	run, err := client.AgentRuns.Create(context.Background(), miosa.CreateAgentRunInput{
		SandboxID:      "sbx_123",
		Provider:       "custom",
		Command:        "echo",
		Prompt:         "inspect files",
		Cwd:            "/workspace",
		Timeout:        900,
		IdempotencyKey: "run-key-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ID != "run_123" || run.Output != "ok\n" {
		t.Fatalf("unexpected run: %#v", run)
	}
}

func TestAgentRunsList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/agent-runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("target_id") != "cmp_123" || r.URL.Query().Get("limit") != "5" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":          "run_123",
					"target_kind": "computer",
					"target_id":   "cmp_123",
					"provider":    "codex",
					"prompt":      "fix tests",
					"status":      "failed",
				},
			},
		})
	})

	client := newTestClient(t, mux)
	runs, err := client.AgentRuns.List(context.Background(), miosa.ListAgentRunsInput{
		ComputerID: "cmp_123",
		Limit:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs.Data) != 1 || runs.Data[0].Provider != "codex" {
		t.Fatalf("unexpected runs: %#v", runs)
	}
}

func TestAgentRunsGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/agent-runs/run_123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"run": map[string]interface{}{
				"id":          "run_123",
				"target_kind": "sandbox",
				"target_id":   "sbx_123",
				"provider":    "osa",
				"prompt":      "ship it",
				"status":      "running",
			},
		})
	})

	client := newTestClient(t, mux)
	run, err := client.AgentRuns.Get(context.Background(), "run_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != miosa.AgentRunRunning {
		t.Fatalf("Status: want running, got %s", run.Status)
	}
}
