package miosa_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Miosa-osa/miosa-go/v2"
)

func TestRegionsCatalog(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/compute/catalog", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, miosa.ComputeCatalogResponse{
			Data: miosa.ComputeCatalog{
				Products: []miosa.ComputeCatalogProduct{
					{
						ID:              "sandbox",
						DefaultTemplate: "miosa-sandbox-prod-1",
						DefaultSize:     "medium",
						Templates: []miosa.ComputeCatalogTemplate{
							{
								ID:      "nextjs",
								SizeIDs: []string{"small", "medium"},
								ArtifactReadiness: []miosa.ComputeArtifactReadiness{
									{
										Size:         "medium",
										State:        miosa.ArtifactReadinessFastReady,
										CheckedNodes: 10,
										ReadyNodes:   10,
									},
								},
							},
						},
					},
				},
			},
		})
	})
	client := newTestClient(t, mux)

	catalog, err := client.Regions.Catalog(context.Background())
	if err != nil {
		t.Fatalf("Regions.Catalog: %v", err)
	}
	if got := catalog.Data.Products[0].ID; got != "sandbox" {
		t.Fatalf("id: got %q", got)
	}
	if got := catalog.Data.Products[0].Templates[0].ID; got != "nextjs" {
		t.Fatalf("template id: got %q", got)
	}
	readiness := catalog.Data.Products[0].Templates[0].ArtifactReadiness[0]
	if readiness.State != miosa.ArtifactReadinessFastReady {
		t.Fatalf("readiness state: got %q", readiness.State)
	}
	if readiness.ReadyNodes != 10 {
		t.Fatalf("ready nodes: got %d", readiness.ReadyNodes)
	}
}
