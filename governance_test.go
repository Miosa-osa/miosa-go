package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go"
)

// ── TenantPolicy ──────────────────────────────────────────────────────────────

func TestTenantPolicyGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tenant/policy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"quotas": map[string]interface{}{"max_sandboxes": 10},
			},
		})
	})
	client := newTestClient(t, mux)
	policy, err := client.GovernanceTenant.Policy.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy == nil {
		t.Fatal("expected non-nil policy")
	}
}

func TestTenantPolicySet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tenant/policy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body miosa.PolicyDoc
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": body})
	})
	client := newTestClient(t, mux)
	input := miosa.PolicyDoc{"quotas": map[string]interface{}{"max_sandboxes": 5.0}}
	result, err := client.GovernanceTenant.Policy.Set(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ── TenantMembers ─────────────────────────────────────────────────────────────

func TestTenantMembersList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tenant/members", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{"id": "m1", "email": "a@b.com", "role": "admin"},
			},
		})
	})
	client := newTestClient(t, mux)
	members, err := client.GovernanceTenant.Members.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("want 1 member, got %d", len(members))
	}
	if members[0].Role != "admin" {
		t.Errorf("Role: want admin, got %s", members[0].Role)
	}
}

func TestTenantMembersInvite(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tenant/members", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, miosa.MemberRecord{ID: "m2", Email: "x@y.com", Role: "developer"})
	})
	client := newTestClient(t, mux)
	m, err := client.GovernanceTenant.Members.Invite(context.Background(), "x@y.com", "developer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Role != "developer" {
		t.Errorf("Role: want developer, got %s", m.Role)
	}
}

// ── WorkspaceGovernance ───────────────────────────────────────────────────────

func TestWorkspacePolicyGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws_1/policy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{"quotas": map[string]interface{}{"max_sandboxes": 3}},
		})
	})
	client := newTestClient(t, mux)
	policy, err := client.GovernanceWorkspaces.Workspace("ws_1").Policy.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy == nil {
		t.Fatal("expected non-nil policy")
	}
}

func TestGovernanceWorkspaceMembersList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws_1/members", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{"id": "m1", "role": "developer"},
			},
		})
	})
	client := newTestClient(t, mux)
	members, err := client.GovernanceWorkspaces.Workspace("ws_1").Members.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("want 1 member, got %d", len(members))
	}
	if members[0].Role != "developer" {
		t.Errorf("Role: want developer, got %s", members[0].Role)
	}
}

// ── ExternalUserPolicy ────────────────────────────────────────────────────────

func TestExternalUserEffectivePolicy(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/external-users/alice/effective-policy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"lifecycle": map[string]interface{}{
				"default_idle_timeout_sec": map[string]interface{}{"value": 600, "source": "user"},
				"default_timeout_sec":      map[string]interface{}{"value": 86400, "source": "tenant"},
			},
			"quotas": map[string]interface{}{
				"max_sandboxes": map[string]interface{}{"value": 5, "source": "workspace"},
			},
		})
	})
	client := newTestClient(t, mux)
	eff, err := client.ExternalUsers.User("alice").Effective(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eff == nil {
		t.Fatal("expected non-nil effective policy")
	}
	if f, ok := eff.Lifecycle["default_idle_timeout_sec"]; !ok {
		t.Error("expected lifecycle.default_idle_timeout_sec field")
	} else {
		if f.Source != "user" {
			t.Errorf("source: want user, got %s", f.Source)
		}
	}
}

// ── BulkOps ───────────────────────────────────────────────────────────────────

func TestBulkSandboxesPause(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/bulk/sandboxes/pause", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, miosa.BulkJobResponse{Queued: 3, JobID: "job_1"})
	})
	client := newTestClient(t, mux)
	result, err := client.Bulk.Sandboxes.Pause(context.Background(), miosa.BulkSandboxInput{
		IDs: []string{"sbx_1", "sbx_2", "sbx_3"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.JobID != "job_1" {
		t.Errorf("JobID: want job_1, got %s", result.JobID)
	}
	if result.Queued != 3 {
		t.Errorf("Queued: want 3, got %d", result.Queued)
	}
}

func TestBulkJobsGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/bulk/jobs/job_1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"id": "job_1", "status": "completed", "processed": 3,
			},
		})
	})
	client := newTestClient(t, mux)
	job, err := client.Bulk.Jobs.Get(context.Background(), "job_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Status != "completed" {
		t.Errorf("Status: want completed, got %s", job.Status)
	}
	if job.Processed != 3 {
		t.Errorf("Processed: want 3, got %d", job.Processed)
	}
}

// ── ApiKeys.CreateScoped ──────────────────────────────────────────────────────

func TestApiKeysCreateScoped(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api-keys/scoped", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body miosa.CreateScopedApiKeyInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, miosa.ScopedApiKeyResult{ID: "key_1", Token: "msk_l2_abc"})
	})
	client := newTestClient(t, mux)
	result, err := client.ApiKeys.CreateScoped(context.Background(), miosa.CreateScopedApiKeyInput{
		ExternalUserID: "alice-42",
		Scopes:         []string{"sandboxes:read"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Token != "msk_l2_abc" {
		t.Errorf("Token: want msk_l2_abc, got %s", result.Token)
	}
}

// ── Admin.Impersonate ─────────────────────────────────────────────────────────

func TestAdminImpersonate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/impersonate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body miosa.ImpersonateInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.TTLSec != 1800 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, miosa.ImpersonateResult{
			Token:     "msi_abc",
			ExpiresAt: "2026-06-01T00:00:00Z",
		})
	})
	client := newTestClient(t, mux)
	result, err := client.Admin.Impersonate(context.Background(), miosa.ImpersonateInput{
		ExternalUserID: "alice-42",
		TTLSec:         1800,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Token != "msi_abc" {
		t.Errorf("Token: want msi_abc, got %s", result.Token)
	}
}

// ── Billing ───────────────────────────────────────────────────────────────────

func TestBillingInvoicesList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/billing/invoices", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{"id": "inv_1", "amount": 5000},
			},
		})
	})
	client := newTestClient(t, mux)
	invoices, err := client.Billing.Invoices.List(context.Background(), 20, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(invoices) != 1 {
		t.Fatalf("want 1 invoice, got %d", len(invoices))
	}
	if invoices[0].ID != "inv_1" {
		t.Errorf("ID: want inv_1, got %s", invoices[0].ID)
	}
}

func TestBillingUpcoming(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/billing/upcoming", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{"amount": 1200},
		})
	})
	client := newTestClient(t, mux)
	upcoming, err := client.Billing.Upcoming(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upcoming["amount"] == nil {
		t.Error("expected amount in upcoming invoice")
	}
}
