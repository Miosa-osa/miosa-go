package miosa_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Miosa-osa/miosa-go"
)

func TestSnapshotsCreate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_s/snapshots", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var input miosa.CreateSnapshotInput
		_ = json.NewDecoder(r.Body).Decode(&input)
		comment := input.Comment
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": miosa.Snapshot{
				ID:         "snap_001",
				ComputerID: "cmp_s",
				Status:     miosa.SnapshotCreating,
				Comment:    comment,
			},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_s", mux)
	snap, err := computer.Snapshots.Create(context.Background(), miosa.CreateSnapshotInput{Comment: "before upgrade"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.ID != "snap_001" {
		t.Errorf("ID: want snap_001, got %s", snap.ID)
	}
	if snap.Status != miosa.SnapshotCreating {
		t.Errorf("Status: want creating, got %s", snap.Status)
	}
}

func TestSnapshotsList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_s/snapshots", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []miosa.Snapshot{
				{ID: "snap_a", ComputerID: "cmp_s", Status: miosa.SnapshotReady},
				{ID: "snap_b", ComputerID: "cmp_s", Status: miosa.SnapshotFailed},
			},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_s", mux)
	snaps, err := computer.Snapshots.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snaps) != 2 {
		t.Errorf("want 2 snapshots, got %d", len(snaps))
	}
}

func TestSnapshotsGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_s/snapshots/snap_xyz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.Snapshot{
				ID:         "snap_xyz",
				ComputerID: "cmp_s",
				Status:     miosa.SnapshotReady,
			},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_s", mux)
	snap, err := computer.Snapshots.Get(context.Background(), "snap_xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.ID != "snap_xyz" {
		t.Errorf("ID: want snap_xyz, got %s", snap.ID)
	}
}

func TestSnapshotsDelete(t *testing.T) {
	deleted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_s/snapshots/snap_del", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		deleted = true
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": miosa.Snapshot{ID: "snap_del", Status: miosa.SnapshotDeleted},
		})
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_s", mux)
	snap, err := computer.Snapshots.Delete(context.Background(), "snap_del")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("DELETE was not called")
	}
	if snap.Status != miosa.SnapshotDeleted {
		t.Errorf("Status: want deleted, got %s", snap.Status)
	}
}

func TestRestoreComputer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/snapshots/snap_r/restore", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": miosa.ComputerData{
				ID:     "cmp_restored",
				Status: miosa.StatusProvisioning,
			},
		})
	})
	client := newTestClient(t, mux)

	computer, err := client.RestoreComputer(context.Background(), "snap_r")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if computer.ID != "cmp_restored" {
		t.Errorf("ID: want cmp_restored, got %s", computer.ID)
	}
	if computer.Status != miosa.StatusProvisioning {
		t.Errorf("Status: want provisioning, got %s", computer.Status)
	}
}

func TestSnapshotsSSEStream(t *testing.T) {
	events := []miosa.SnapshotProgressEvent{
		{Type: "snapshot_progress", SnapshotID: "snap_sse", Status: "creating", Progress: intPtr(20)},
		{Type: "snapshot_progress", SnapshotID: "snap_sse", Status: "uploading", Progress: intPtr(60)},
		{Type: "snapshot_progress", SnapshotID: "snap_sse", Status: "ready", Progress: intPtr(100)},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_s/snapshots/snap_sse/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for _, ev := range events {
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	})
	client := newTestClient(t, mux)

	computer, _ := makeComputer(client, "cmp_s", mux)
	stream, err := computer.Snapshots.Events(context.Background(), "snap_sse", "ticket123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var received []miosa.SnapshotProgressEvent
	for ev := range stream.C {
		received = append(received, ev)
	}
	if len(received) != 3 {
		t.Errorf("want 3 events, got %d", len(received))
	}
	if received[2].Status != "ready" {
		t.Errorf("last status: want ready, got %s", received[2].Status)
	}
}

// makeComputer registers a GET /computers/{id} handler and returns the Computer.
func makeComputer(client *miosa.Client, id string, mux *http.ServeMux) (*miosa.Computer, error) {
	mux.HandleFunc("/computers/"+id, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, miosa.ComputerData{ID: id, Status: miosa.StatusRunning})
	})
	return client.Computers.Get(context.Background(), id)
}

func intPtr(i int) *int { return &i }
