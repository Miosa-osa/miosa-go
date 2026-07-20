package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func TestSandboxesCreateLetsServerApplyDefaultSmall(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["template_id"] != miosa.SandboxTemplate {
			t.Fatalf("template_id = %q, want %q", body["template_id"], miosa.SandboxTemplate)
		}
		if _, exists := body["size"]; exists {
			t.Fatal("default create must omit size so the server applies the canonical default")
		}
		if _, exists := body["template_type"]; exists {
			t.Fatal("native sandbox request must not contain template_type")
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": map[string]interface{}{
				"id": "sbx_123", "template_id": miosa.SandboxTemplate, "size": miosa.SizeSmall,
				"state": "provisioning", "cpu_count": 2, "memory_mb": 4096, "disk_size_mb": 10240,
				"resource_contract": map[string]interface{}{
					"id": "sandbox/small@v1", "version": "v1", "product": "sandbox",
					"size": "small", "vcpus": 2, "memory_mb": 4096, "disk_size_mb": 10240,
				},
			},
		})
	})

	client := newTestClient(t, mux)
	sandbox, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sandbox.Size != miosa.SizeSmall {
		t.Fatalf("response size = %q, want %q", sandbox.Size, miosa.SizeSmall)
	}
	if sandbox.ID != "sbx_123" || sandbox.TemplateID != miosa.SandboxTemplate {
		t.Fatalf("unexpected native sandbox: %#v", sandbox)
	}
	if sandbox.ResourceContract == nil {
		t.Fatal("resource contract is nil")
	}
	contract := sandbox.ResourceContract
	if contract.ID != "sandbox/small@v1" || contract.Version != "v1" {
		t.Fatalf("contract identity = %q/%q", contract.ID, contract.Version)
	}
	if contract.Size != miosa.SizeSmall || contract.VCPUs != 2 || contract.MemoryMB != 4096 || contract.DiskSizeMB != 10240 {
		t.Fatalf("unexpected exact resource contract: %#v", contract)
	}
}

func TestSandboxesCreateUsesCanonicalFieldsAndIdempotency(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Idempotency-Key"); got != "idem-123" {
			t.Fatalf("Idempotency-Key = %q, want idem-123", got)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["template_id"] != "node-22" || body["size"] != "medium" || body["name"] != "builder" {
			t.Fatalf("unexpected request body: %#v", body)
		}
		if body["external_workspace_id"] != "workspace-1" || body["external_user_id"] != "user-1" || body["external_project_id"] != "project-1" {
			t.Fatalf("missing attribution: %#v", body)
		}
		if body["timeout_sec"] != float64(3600) || body["idle_timeout_sec"] != float64(900) || body["disk_size_mb"] != float64(20480) {
			t.Fatalf("missing lifecycle or disk parameters: %#v", body)
		}
		if body["persistent"] != false || body["region"] != "us-east-1" {
			t.Fatalf("missing persistence or region parameters: %#v", body)
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": map[string]interface{}{
				"id": "sbx_medium", "name": "builder", "template_id": "node-22", "size": "medium",
				"state": "provisioning", "cpu_count": 4, "memory_mb": 8192, "disk_size_mb": 20480,
				"resource_contract": map[string]interface{}{
					"id": "sandbox/medium@v1", "version": "v1", "product": "sandbox",
					"size": "medium", "vcpus": 4, "memory_mb": 8192, "disk_size_mb": 20480,
				},
			},
		})
	})

	client := newTestClient(t, mux)
	persistent := false
	sandbox, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{
		Name:                "builder",
		Size:                miosa.SizeMedium,
		TemplateID:          "node-22",
		ExternalWorkspaceID: "workspace-1",
		ExternalUserID:      "user-1",
		ExternalProjectID:   "project-1",
		IdempotencyKey:      "idem-123",
		TimeoutSec:          3600,
		IdleTimeoutSec:      900,
		CPUCount:            4,
		MemoryMB:            8192,
		DiskSizeMB:          20480,
		Persistent:          &persistent,
		Region:              "us-east-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sandbox.ID != "sbx_medium" || sandbox.ResourceContract == nil || sandbox.ResourceContract.Size != miosa.SizeMedium {
		t.Fatalf("unexpected sandbox: %#v", sandbox)
	}
}

func TestSandboxesLifecycleUsesNativeEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	for _, action := range []string{"extend", "stop", "pause", "resume"} {
		action := action
		mux.HandleFunc("/sandboxes/sbx_123/"+action, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("%s method = %s, want POST", action, r.Method)
			}
			if action == "extend" {
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode extend request: %v", err)
				}
				if body["timeout_sec"] != float64(7200) {
					t.Fatalf("timeout_sec = %#v, want 7200", body["timeout_sec"])
				}
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{"id": "sbx_123", "state": "running", "timeout_sec": 7200},
			})
		})
	}

	client := newTestClient(t, mux)
	if sandbox, err := client.Sandboxes.Extend(context.Background(), "sbx_123", 7200); err != nil || sandbox.TimeoutSec != 7200 {
		t.Fatalf("extend = %#v, %v", sandbox, err)
	}
	if sandbox, err := client.Sandboxes.Stop(context.Background(), "sbx_123"); err != nil || sandbox.ID != "sbx_123" {
		t.Fatalf("stop = %#v, %v", sandbox, err)
	}
	if sandbox, err := client.Sandboxes.Pause(context.Background(), "sbx_123"); err != nil || sandbox.ID != "sbx_123" {
		t.Fatalf("pause = %#v, %v", sandbox, err)
	}
	if sandbox, err := client.Sandboxes.Resume(context.Background(), "sbx_123"); err != nil || sandbox.ID != "sbx_123" {
		t.Fatalf("resume = %#v, %v", sandbox, err)
	}
}

func TestSandboxesExtendZeroOmitsTimeoutAndNegativeIsRejected(t *testing.T) {
	requests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx_123/extend", func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 0 {
			t.Fatalf("zero extend body = %#v, want empty object", body)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"id": "sbx_123"}})
	})
	client := newTestClient(t, mux)
	if _, err := client.Sandboxes.Extend(context.Background(), "sbx_123", 0); err != nil {
		t.Fatalf("zero timeout compatibility request failed: %v", err)
	}
	if _, err := client.Sandboxes.Extend(context.Background(), "sbx_123", -1); err == nil {
		t.Fatal("expected negative timeout to fail before sending a request")
	}
	if requests != 1 {
		t.Fatalf("server received %d requests, want only zero extend", requests)
	}
}

func TestSandboxesCreatePreservesTemplateAliasAndNormalizesXLarge(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["template_id"] != "legacy-template" || body["size"] != "xl" {
			t.Fatalf("unexpected compatibility request: %#v", body)
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": map[string]interface{}{"id": "sbx_xl", "template_id": "legacy-template", "size": "xl"},
		})
	})

	client := newTestClient(t, mux)
	sandbox, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{
		Template: "legacy-template",
		Size:     miosa.SizeXLarge,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sandbox.Size != miosa.SizeXL {
		t.Fatalf("response size = %q, want xl", sandbox.Size)
	}
}

func TestSandboxesGetUsesNativeEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx_123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{"id": "sbx_123", "state": "running", "size": "small"},
		})
	})

	client := newTestClient(t, mux)
	sandbox, err := client.Sandboxes.Get(context.Background(), "sbx_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sandbox.ID != "sbx_123" || sandbox.State != "running" || sandbox.Size != miosa.SizeSmall {
		t.Fatalf("unexpected sandbox: %#v", sandbox)
	}
}

func TestSandboxesDeleteUsesNativeEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx_123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	client := newTestClient(t, mux)
	if err := client.Sandboxes.Delete(context.Background(), "sbx_123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

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
