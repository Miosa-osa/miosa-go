package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Miosa-osa/miosa-go"
)

func TestNetworkPolicyGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_np/network-policy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.NetworkPolicy{
				ComputerID:    "cmp_np",
				DefaultEffect: miosa.NetworkEffectAllow,
				Rules:         []miosa.NetworkPolicyRule{},
			},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_np", mux)
	policy, err := computer.NetworkPolicy.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.DefaultEffect != miosa.NetworkEffectAllow {
		t.Errorf("DefaultEffect: want allow, got %s", policy.DefaultEffect)
	}
	if len(policy.Rules) != 0 {
		t.Errorf("Rules: want 0, got %d", len(policy.Rules))
	}
}

func TestNetworkPolicySet(t *testing.T) {
	var gotBody miosa.SetNetworkPolicyInput
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_np/network-policy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.NetworkPolicy{
				ComputerID:    "cmp_np",
				DefaultEffect: gotBody.DefaultEffect,
				Rules:         gotBody.Rules,
			},
		})
	})
	client := newTestClient(t, mux)

	input := miosa.SetNetworkPolicyInput{
		DefaultEffect: miosa.NetworkEffectAllow,
		Rules: []miosa.NetworkPolicyRule{
			{Effect: miosa.NetworkEffectDeny, Destination: "169.254.169.254/32"},
		},
	}

	computer, _ := makeComputer(client, "cmp_np", mux)
	policy, err := computer.NetworkPolicy.Set(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.DefaultEffect != miosa.NetworkEffectAllow {
		t.Errorf("DefaultEffect: want allow, got %s", policy.DefaultEffect)
	}
	if len(policy.Rules) != 1 {
		t.Errorf("Rules: want 1, got %d", len(policy.Rules))
	}
	if policy.Rules[0].Destination != "169.254.169.254/32" {
		t.Errorf("Rule destination: want 169.254.169.254/32, got %s", policy.Rules[0].Destination)
	}
	if gotBody.DefaultEffect != miosa.NetworkEffectAllow {
		t.Errorf("request body DefaultEffect: want allow, got %s", gotBody.DefaultEffect)
	}
}

func TestNetworkPolicySetDenyAll(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_np/network-policy", func(w http.ResponseWriter, r *http.Request) {
		var body miosa.SetNetworkPolicyInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.NetworkPolicy{
				ComputerID:    "cmp_np",
				DefaultEffect: body.DefaultEffect,
				Rules:         body.Rules,
			},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_np", mux)
	policy, err := computer.NetworkPolicy.Set(context.Background(), miosa.SetNetworkPolicyInput{
		DefaultEffect: miosa.NetworkEffectDeny,
		Rules: []miosa.NetworkPolicyRule{
			{
				Effect:      miosa.NetworkEffectAllow,
				Destination: "example.com",
				Ports:       "443",
				Protocol:    miosa.NetworkProtocolTCP,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.DefaultEffect != miosa.NetworkEffectDeny {
		t.Errorf("DefaultEffect: want deny, got %s", policy.DefaultEffect)
	}
	if policy.Rules[0].Effect != miosa.NetworkEffectAllow {
		t.Errorf("rule effect: want allow, got %s", policy.Rules[0].Effect)
	}
}

func TestNetworkPolicyReset(t *testing.T) {
	reset := false
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_np/network-policy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		reset = true
		w.WriteHeader(http.StatusNoContent)
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_np", mux)
	if err := computer.NetworkPolicy.Reset(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reset {
		t.Error("DELETE was not called for Reset")
	}
}
