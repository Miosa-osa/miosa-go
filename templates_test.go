package miosa_test

import (
	"context"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func TestTemplatesCatalogListAndReadiness(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/templates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		writeJSON(w, 200, map[string]interface{}{
			"templates": []map[string]interface{}{
				{
					"id":           "miosa-sandbox",
					"product":      "sandbox",
					"default_size": "small",
					"sdk_name":     "client.sandboxes",
					"cli_name":     "miosa sandbox",
					"sizes": []map[string]interface{}{
						{
							"size":          "small",
							"state":         "fast_ready",
							"ready_nodes":   10,
							"checked_nodes": 10,
						},
					},
				},
				{
					"id":           "miosa-desktop",
					"product":      "computer",
					"default_size": "small",
					"sdk_name":     "client.computers",
					"cli_name":     "miosa computers",
					"sizes": []map[string]interface{}{
						{
							"size":          "small",
							"state":         "fast_ready",
							"ready_nodes":   6,
							"checked_nodes": 10,
						},
					},
				},
			},
			"readiness_states": []string{"fast_ready", "cold_boot_only", "missing"},
		})
	})

	c := newTestClient(t, mux)
	ctx := context.Background()

	all, err := c.Templates.List(ctx, miosa.ListProductTemplatesInput{})
	if err != nil {
		t.Fatalf("Templates.List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(all))
	}

	computers, err := c.Templates.List(ctx, miosa.ListProductTemplatesInput{Product: "computer"})
	if err != nil {
		t.Fatalf("Templates.List filtered: %v", err)
	}
	if len(computers) != 1 || computers[0].ID != "miosa-desktop" {
		t.Fatalf("expected miosa-desktop, got %#v", computers)
	}

	readiness, err := c.Templates.Readiness(ctx, "miosa-sandbox")
	if err != nil {
		t.Fatalf("Templates.Readiness: %v", err)
	}
	if len(readiness) != 1 || readiness[0].State != "fast_ready" {
		t.Fatalf("unexpected readiness %#v", readiness)
	}
}
