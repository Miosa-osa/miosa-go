package miosa_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Miosa-osa/miosa-go"
)

func TestServicesCreate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_svc/services", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var input miosa.CreateServiceInput
		_ = json.NewDecoder(r.Body).Decode(&input)
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": miosa.Service{
				ID:         "svc_001",
				ComputerID: "cmp_svc",
				Name:       input.Name,
				Command:    input.Command,
				Status:     miosa.ServiceStopped,
			},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_svc", mux)
	svc, err := computer.Services.Create(context.Background(), miosa.CreateServiceInput{
		Name:    "web",
		Command: "python -m http.server 8080",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.ID != "svc_001" {
		t.Errorf("ID: want svc_001, got %s", svc.ID)
	}
	if svc.Name != "web" {
		t.Errorf("Name: want web, got %s", svc.Name)
	}
}

func TestServicesList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_svc/services", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []miosa.Service{
				{ID: "svc_a", Name: "api", Status: miosa.ServiceRunning},
				{ID: "svc_b", Name: "worker", Status: miosa.ServiceStopped},
			},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_svc", mux)
	svcs, err := computer.Services.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svcs) != 2 {
		t.Errorf("want 2 services, got %d", len(svcs))
	}
	if svcs[0].Status != miosa.ServiceRunning {
		t.Errorf("first status: want running, got %s", svcs[0].Status)
	}
}

func TestServicesGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_svc/services/svc_get", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.Service{ID: "svc_get", Name: "db", Status: miosa.ServiceRunning},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_svc", mux)
	svc, err := computer.Services.Get(context.Background(), "svc_get")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.ID != "svc_get" {
		t.Errorf("ID: want svc_get, got %s", svc.ID)
	}
}

func TestServicesStart(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_svc/services/svc_1/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.Service{ID: "svc_1", Status: miosa.ServiceRunning},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_svc", mux)
	svc, err := computer.Services.Start(context.Background(), "svc_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.Status != miosa.ServiceRunning {
		t.Errorf("Status: want running, got %s", svc.Status)
	}
}

func TestServicesStop(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_svc/services/svc_1/stop", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.Service{ID: "svc_1", Status: miosa.ServiceStopped},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_svc", mux)
	svc, err := computer.Services.Stop(context.Background(), "svc_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.Status != miosa.ServiceStopped {
		t.Errorf("Status: want stopped, got %s", svc.Status)
	}
}

func TestServicesRestart(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_svc/services/svc_1/restart", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.Service{ID: "svc_1", Status: miosa.ServiceRunning},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_svc", mux)
	svc, err := computer.Services.Restart(context.Background(), "svc_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.Status != miosa.ServiceRunning {
		t.Errorf("Status: want running after restart, got %s", svc.Status)
	}
}

func TestServicesDelete(t *testing.T) {
	deleted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_svc/services/svc_del", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_svc", mux)
	if err := computer.Services.Delete(context.Background(), "svc_del"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("DELETE was not called")
	}
}

func TestServicesLogsStream(t *testing.T) {
	logLines := []miosa.ServiceLogEvent{
		{Timestamp: "2026-01-01T00:00:01Z", Stream: "stdout", Message: "starting up"},
		{Timestamp: "2026-01-01T00:00:02Z", Stream: "stdout", Message: "listening on :8080"},
		{Timestamp: "2026-01-01T00:00:03Z", Stream: "stderr", Message: "connection error"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_svc/services/svc_log/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for _, line := range logLines {
			data, _ := json.Marshal(line)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_svc", mux)
	stream, err := computer.Services.Logs(context.Background(), "svc_log")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var received []miosa.ServiceLogEvent
	for ev := range stream.C {
		received = append(received, ev)
	}
	if len(received) != 3 {
		t.Errorf("want 3 log events, got %d", len(received))
	}
	if received[0].Message != "starting up" {
		t.Errorf("first message: want 'starting up', got %q", received[0].Message)
	}
	if received[2].Stream != "stderr" {
		t.Errorf("third stream: want stderr, got %s", received[2].Stream)
	}
}
