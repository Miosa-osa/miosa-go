package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go"
)

func TestSandboxesDeployDefaultsToMiosaDeploy(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx_123/deploy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, ok := body["deployment_type"]; ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"deployment_product": "miosa_deploy"})
	})

	client := newTestClient(t, mux)
	result, err := client.Sandboxes.Deploy(context.Background(), "sbx_123", miosa.SandboxDeployInput{
		Name:       "normal-app",
		OutputPath: "/workspace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["deployment_product"] != "miosa_deploy" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSandboxesDeployDockerSetsDeploymentType(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx_123/deploy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body miosa.SandboxDeployInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.DeploymentType != "docker_deploy" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"deployment_product": "docker_deploy"})
	})

	client := newTestClient(t, mux)
	result, err := client.Sandboxes.DeployDocker(context.Background(), "sbx_123", miosa.SandboxDeployInput{
		Name:       "docker-app",
		OutputPath: "/workspace",
		Port:       8080,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["deployment_product"] != "docker_deploy" {
		t.Fatalf("unexpected result: %#v", result)
	}
}
