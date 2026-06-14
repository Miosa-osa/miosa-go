package miosa_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Miosa-osa/miosa-go"
)

func TestDevicesCatalog(t *testing.T) {
	client := miosa.NewClient("msk_u_test")

	catalog := client.Devices.Catalog()
	kinds := map[miosa.DeviceKind]bool{}
	for _, entry := range catalog {
		kinds[entry.Kind] = true
	}

	for _, kind := range []miosa.DeviceKind{
		miosa.DeviceKindSandboxWorker,
		miosa.DeviceKindComputer,
		miosa.DeviceKindLocalDevice,
		miosa.DeviceKindDockerDeployHost,
	} {
		if !kinds[kind] {
			t.Fatalf("catalog missing %s", kind)
		}
	}
}

func TestDevicesList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{
					"id":          "sbx_123",
					"name":        "builder",
					"state":       "running",
					"ready":       true,
					"persistent":  true,
					"preview_url": "https://3000-sbx_123.sandbox.miosa.app",
				},
			},
		})
	})
	mux.HandleFunc("/computers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, miosa.ComputerListResponse{
			Data: []miosa.ComputerData{
				{ID: "cmp_123", Name: "desktop", Status: miosa.StatusRunning},
			},
		})
	})
	client := newTestClient(t, mux)

	result, err := client.Devices.List(context.Background(), miosa.DeviceListInput{})
	if err != nil {
		t.Fatalf("Devices.List: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", result.Errors)
	}
	if got, want := len(result.Devices), 2; got != want {
		t.Fatalf("devices length: got %d want %d", got, want)
	}
	if result.Devices[0].Kind != miosa.DeviceKindSandboxWorker {
		t.Fatalf("first kind: got %s", result.Devices[0].Kind)
	}
	if result.Devices[1].Kind != miosa.DeviceKindComputer {
		t.Fatalf("second kind: got %s", result.Devices[1].Kind)
	}
}

func TestDevicesListPartialFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{{"id": "sbx_123", "state": "running"}},
		})
	})
	mux.HandleFunc("/computers", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	})
	client := newTestClient(t, mux)

	result, err := client.Devices.List(context.Background(), miosa.DeviceListInput{})
	if err != nil {
		t.Fatalf("Devices.List: %v", err)
	}
	if got, want := len(result.Devices), 1; got != want {
		t.Fatalf("devices length: got %d want %d", got, want)
	}
	if got, want := len(result.Errors), 1; got != want {
		t.Fatalf("errors length: got %d want %d", got, want)
	}
	if !result.Errors[0].Retryable {
		t.Fatalf("expected retryable partial error")
	}
}
