package miosa_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Miosa-osa/miosa-go"
)

func TestCustomDomainsRegister(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_d/domains", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": miosa.CustomDomain{
				ID:                 "dom_001",
				ComputerID:         "cmp_d",
				FQDN:               "app.example.com",
				Status:             miosa.DomainPending,
				VerificationTarget: "cmp_d.sandbox.miosa.ai",
				Instructions:       "Add CNAME app.example.com → cmp_d.sandbox.miosa.ai",
			},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_d", mux)
	dom, err := computer.Domains.Register(context.Background(), "app.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dom.ID != "dom_001" {
		t.Errorf("ID: want dom_001, got %s", dom.ID)
	}
	if dom.Status != miosa.DomainPending {
		t.Errorf("Status: want pending, got %s", dom.Status)
	}
	if dom.FQDN != "app.example.com" {
		t.Errorf("FQDN: want app.example.com, got %s", dom.FQDN)
	}
	if dom.VerificationTarget == "" {
		t.Error("VerificationTarget should not be empty")
	}
}

func TestCustomDomainsList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_d/domains", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []miosa.CustomDomain{
				{ID: "dom_a", FQDN: "a.example.com", Status: miosa.DomainActive},
				{ID: "dom_b", FQDN: "b.example.com", Status: miosa.DomainPending},
			},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_d", mux)
	domains, err := computer.Domains.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(domains) != 2 {
		t.Errorf("want 2 domains, got %d", len(domains))
	}
	if domains[0].Status != miosa.DomainActive {
		t.Errorf("first domain status: want active, got %s", domains[0].Status)
	}
}

func TestCustomDomainsVerify(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_d/domains/dom_v/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.CustomDomain{
				ID:     "dom_v",
				Status: miosa.DomainVerified,
			},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_d", mux)
	dom, err := computer.Domains.Verify(context.Background(), "dom_v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dom.Status != miosa.DomainVerified {
		t.Errorf("Status: want verified, got %s", dom.Status)
	}
}

func TestCustomDomainsDelete(t *testing.T) {
	deleted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_d/domains/dom_del", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_d", mux)
	if err := computer.Domains.Delete(context.Background(), "dom_del"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("DELETE was not called")
	}
}
