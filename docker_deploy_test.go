package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go"
)

func TestDockerDeployListHosts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/docker-deploy/hosts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("workspace_id") != "ws_123" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":               "ddh_123",
					"tenant_id":        "tenant_123",
					"workspace_id":     "ws_123",
					"status":           "active",
					"size":             "small",
					"region":           "iad",
					"appliance_status": "healthy",
				},
			},
		})
	})

	client := newTestClient(t, mux)
	hosts, err := client.DockerDeploy.ListHosts(context.Background(), miosa.ListDockerDeployHostsInput{
		WorkspaceID: "ws_123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 1 || hosts[0].ID != "ddh_123" {
		t.Fatalf("unexpected hosts: %#v", hosts)
	}
}

func TestDockerDeployEnsureHost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/docker-deploy/hosts/ensure", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body miosa.EnsureDockerDeployHostInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.WorkspaceID != "ws_123" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"host": map[string]interface{}{
				"id":               "ddh_123",
				"tenant_id":        "tenant_123",
				"workspace_id":     "ws_123",
				"status":           "bootstrapping",
				"size":             "small",
				"region":           "iad",
				"appliance_status": "starting",
			},
			"queued": true,
		})
	})

	client := newTestClient(t, mux)
	result, err := client.DockerDeploy.EnsureHost(context.Background(), miosa.EnsureDockerDeployHostInput{
		WorkspaceID: "ws_123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Queued || result.Host.ID != "ddh_123" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestDockerDeployTemplates(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/docker-deploy/templates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"templates": []map[string]interface{}{
				{"id": "nextjs", "name": "Next.js", "runtime": "node"},
			},
		})
	})
	mux.HandleFunc("/docker-deploy/templates/compose-full-stack", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"template": map[string]interface{}{
				"id":   "compose-full-stack",
				"name": "Compose Full Stack",
				"tags": []string{"node", "postgres"},
			},
		})
	})

	client := newTestClient(t, mux)
	templates, err := client.DockerDeploy.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 1 || templates[0].ID != "nextjs" {
		t.Fatalf("unexpected templates: %#v", templates)
	}
	template, err := client.DockerDeploy.GetTemplate(context.Background(), "compose-full-stack")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if template.ID != "compose-full-stack" {
		t.Fatalf("Template.ID: want compose-full-stack, got %s", template.ID)
	}
}
