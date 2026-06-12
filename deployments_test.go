package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go"
)

func TestDeploymentsCreateDockerDeploy(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/deployments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body miosa.CreateDeploymentInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Metadata["deployment_product"] != "docker_deploy" || body.Metadata["client"] != "cliniciq" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"id":                    "dep_123",
				"tenant_id":             "tenant_123",
				"name":                  body.Name,
				"slug":                  "clinic-app",
				"state":                 "running",
				"deployment_product":    "docker_deploy",
				"docker_deploy_host_id": "ddh_123",
				"metadata":              body.Metadata,
			},
		})
	})

	client := newTestClient(t, mux)
	deployment, err := client.Deployments.CreateDockerDeploy(context.Background(), miosa.CreateDeploymentInput{
		Name:     "clinic-app",
		Metadata: map[string]interface{}{"client": "cliniciq"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deployment.DeploymentProduct != miosa.DeploymentProductDockerDeploy {
		t.Fatalf("DeploymentProduct: want docker_deploy, got %s", deployment.DeploymentProduct)
	}
	if deployment.DockerDeployHostID != "ddh_123" {
		t.Fatalf("DockerDeployHostID: want ddh_123, got %s", deployment.DockerDeployHostID)
	}
}

func TestDeploymentsPublishFromSandboxDocker(t *testing.T) {
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
		if body["deployment_type"] != "docker_deploy" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"deployment_product": "docker_deploy",
			"data": map[string]interface{}{
				"deployment": map[string]interface{}{"id": "dep_123"},
			},
		})
	})

	client := newTestClient(t, mux)
	result, err := client.Deployments.PublishFromSandboxDocker(context.Background(), "sbx_123", miosa.PublishInput{
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
