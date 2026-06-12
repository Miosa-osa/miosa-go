package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go"
)

func TestTokensCreateScoped(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tokens/scoped", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body miosa.CreateScopedTokenInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.UserID != "user_123" || body.WorkspaceID != "ws_123" || body.ExpiresInSeconds != 3600 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"token":      "mst_scoped_abc",
				"expires_at": "2026-06-12T12:00:00Z",
				"scopes":     []string{"sandboxes:read", "sandboxes:write"},
			},
		})
	})

	client := newTestClient(t, mux)
	result, err := client.Tokens.CreateScoped(context.Background(), miosa.CreateScopedTokenInput{
		UserID:           "user_123",
		WorkspaceID:      "ws_123",
		ExpiresInSeconds: 3600,
		Scopes:           []string{"sandboxes:read", "sandboxes:write"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Token != "mst_scoped_abc" {
		t.Fatalf("Token: want mst_scoped_abc, got %s", result.Token)
	}
	if len(result.Scopes) != 2 {
		t.Fatalf("Scopes: want 2, got %d", len(result.Scopes))
	}
}
