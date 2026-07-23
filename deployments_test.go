package miosa_test

import (
	"context"
	"net/http"
	"testing"
)

func TestDeploymentReleasePromoteGeneratesIdempotencyKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/deployments/dep_123/releases/rel_456/promote", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Idempotency-Key"); got != "promote:dep_123:rel_456" {
			t.Fatalf("Idempotency-Key: want promote:dep_123:rel_456, got %q", got)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "dep_123",
				"name": "Clinic Intake",
			},
		})
	})

	client := newTestClient(t, mux)
	if _, err := client.Deployments.Releases("dep_123").Promote(context.Background(), "rel_456", ""); err != nil {
		t.Fatalf("promote release: %v", err)
	}
}
