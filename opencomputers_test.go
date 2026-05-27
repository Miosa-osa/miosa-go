package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go"
)

// newTestClientOC builds a test server from mux and returns a client pointed at it.
// (newTestClient is already defined in client_test.go; this avoids redeclaration.)
func newTestClientOC(t *testing.T, mux *http.ServeMux) *miosa.Client {
	t.Helper()
	// Reuse the existing helper from client_test.go — same package, visible here.
	return newTestClient(t, mux)
}

var sampleHost = map[string]interface{}{
	"id":         "host_abc",
	"name":       "my-mac",
	"region":     nil,
	"status":     "online",
	"tenant_id":  "t_1",
	"labels":     map[string]interface{}{},
	"created_at": "2026-01-01T00:00:00Z",
	"updated_at": "2026-01-01T00:00:00Z",
}

var sampleJob = map[string]interface{}{
	"id":           "job_1",
	"host_id":      "host_abc",
	"status":       "completed",
	"command":      "npm test",
	"args":         []interface{}{},
	"env":          []interface{}{},
	"cwd":          nil,
	"exit_code":    0,
	"stdout":       "ok",
	"stderr":       "",
	"created_at":   "2026-01-01T00:00:00Z",
	"updated_at":   "2026-01-01T00:00:00Z",
	"completed_at": nil,
}

var sampleTunnel = map[string]interface{}{
	"id":          "tun_1",
	"host_id":     "host_abc",
	"slug":        "abc123",
	"target_port": 3000,
	"auth_mode":   "public",
	"public_url":  "https://api.miosa.ai/t/abc123",
	"enabled":     true,
	"created_at":  "2026-01-01T00:00:00Z",
	"updated_at":  "2026-01-01T00:00:00Z",
}

var sampleSession = map[string]interface{}{
	"id":           "sess_1",
	"host_id":      "host_abc",
	"task":         "run tests",
	"model_id":     nil,
	"status":       "pending",
	"max_turns":    20,
	"turns_used":   0,
	"created_at":   "2026-01-01T00:00:00Z",
	"updated_at":   "2026-01-01T00:00:00Z",
	"completed_at": nil,
	"error":        nil,
}

// ─── Hosts ────────────────────────────────────────────────────────────────────

func TestOcHosts_List(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []interface{}{sampleHost},
			"meta": map[string]interface{}{"total": 1, "page": 1, "per_page": 20},
		})
	})

	c := newTestClientOC(t, mux)
	result, err := c.OpenComputers.Hosts.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("want 1 host, got %d", len(result.Data))
	}
	if result.Data[0].ID != "host_abc" {
		t.Errorf("want host_abc, got %s", result.Data[0].ID)
	}
}

func TestOcHosts_Create_WithHostKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		resp := make(map[string]interface{})
		for k, v := range sampleHost {
			resp[k] = v
		}
		resp["host_key"] = "hk_secret"
		writeJSON(w, http.StatusOK, resp)
	})

	c := newTestClientOC(t, mux)
	host, err := c.OpenComputers.Hosts.Create(context.Background(), miosa.CreateHostInput{Name: "my-mac"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if host.HostKey == nil || *host.HostKey != "hk_secret" {
		t.Errorf("want host_key=hk_secret, got %v", host.HostKey)
	}
}

func TestOcHosts_Get(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts/host_abc", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, sampleHost)
	})

	c := newTestClientOC(t, mux)
	host, err := c.OpenComputers.Hosts.Get(context.Background(), "host_abc")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if host.Status != miosa.HostStatusOnline {
		t.Errorf("want online, got %s", host.Status)
	}
}

func TestOcHosts_Revoke(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts/host_abc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := newTestClientOC(t, mux)
	if err := c.OpenComputers.Hosts.Revoke(context.Background(), "host_abc"); err != nil {
		t.Fatalf("Revoke() error: %v", err)
	}
}

// ─── Jobs ─────────────────────────────────────────────────────────────────────

func TestOcJobs_Run(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts/host_abc/exec", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, sampleJob)
	})

	c := newTestClientOC(t, mux)
	job, err := c.OpenComputers.Jobs.Run(context.Background(), "host_abc", miosa.RunJobInput{Command: "npm test"})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if job.ID != "job_1" {
		t.Errorf("want job_1, got %s", job.ID)
	}
}

func TestOcJobs_Cancel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts/host_abc/exec/job_1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := newTestClientOC(t, mux)
	if err := c.OpenComputers.Jobs.Cancel(context.Background(), "host_abc", "job_1"); err != nil {
		t.Fatalf("Cancel() error: %v", err)
	}
}

// ─── Tunnels ──────────────────────────────────────────────────────────────────

func TestOcTunnels_Create(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts/host_abc/tunnels", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, sampleTunnel)
	})

	c := newTestClientOC(t, mux)
	tunnel, err := c.OpenComputers.Tunnels.Create(context.Background(), "host_abc", miosa.CreateTunnelInput{TargetPort: 3000})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if tunnel.PublicURL != "https://api.miosa.ai/t/abc123" {
		t.Errorf("unexpected public_url: %s", tunnel.PublicURL)
	}
}

func TestOcTunnels_Delete(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts/host_abc/tunnels/tun_1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := newTestClientOC(t, mux)
	if err := c.OpenComputers.Tunnels.Delete(context.Background(), "host_abc", "tun_1"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
}

// ─── Agents ───────────────────────────────────────────────────────────────────

func TestOcAgents_Dispatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts/host_abc/agent/dispatch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, sampleSession)
	})

	c := newTestClientOC(t, mux)
	sess, err := c.OpenComputers.Agents.Dispatch(context.Background(), "host_abc", miosa.DispatchAgentInput{Task: "run tests"})
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}
	if sess.ID != "sess_1" {
		t.Errorf("want sess_1, got %s", sess.ID)
	}
}

func TestOcAgents_Cancel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts/host_abc/agent/sessions/sess_1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := newTestClientOC(t, mux)
	if err := c.OpenComputers.Agents.Cancel(context.Background(), "host_abc", "sess_1"); err != nil {
		t.Fatalf("Cancel() error: %v", err)
	}
}

// ─── Error paths ──────────────────────────────────────────────────────────────

func TestOcHosts_List_401(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "UNAUTHORIZED",
				"message": "Unauthorized",
			},
		})
	})

	c := newTestClientOC(t, mux)
	_, err := c.OpenComputers.Hosts.List(context.Background())
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestOcHosts_Get_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/hosts/bad", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "NOT_FOUND",
				"message": "Host not found",
			},
		})
	})

	c := newTestClientOC(t, mux)
	_, err := c.OpenComputers.Hosts.Get(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

// ─── Clusters ─────────────────────────────────────────────────────────────────

func TestOcClusters_List(t *testing.T) {
	cluster := map[string]interface{}{
		"id":            "clus_1",
		"name":          "llama-cluster",
		"model":         "llama3:70b",
		"slug":          "llama",
		"status":        "active",
		"host_ids":      []interface{}{"host_abc"},
		"inference_url": "https://api.miosa.ai/inference/llama/v1",
		"created_at":    "2026-01-01T00:00:00Z",
		"updated_at":    "2026-01-01T00:00:00Z",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/opencomputers/clusters", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{cluster}})
	})

	c := newTestClientOC(t, mux)
	result, err := c.OpenComputers.Clusters.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("want 1 cluster, got %d", len(result.Data))
	}
	if result.Data[0].InferenceURL == "" {
		t.Error("inference_url should not be empty")
	}
}
