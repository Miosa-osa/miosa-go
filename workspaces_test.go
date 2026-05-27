package miosa_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/Miosa-osa/miosa-go"
)

func TestWorkspacesCreate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var input miosa.CreateWorkspaceInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, miosa.Workspace{
			ID:   "ws_001",
			Name: input.Name,
		})
	})
	client := newTestClient(t, mux)

	ws, err := client.Workspaces.Create(context.Background(), miosa.CreateWorkspaceInput{
		Name:        "my-workspace",
		Description: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.ID != "ws_001" {
		t.Errorf("ID: want ws_001, got %s", ws.ID)
	}
	if ws.Name != "my-workspace" {
		t.Errorf("Name: want my-workspace, got %s", ws.Name)
	}
}

func TestWorkspacesList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, miosa.WorkspaceListResponse{
			Data: []miosa.Workspace{
				{ID: "ws_1", Name: "alpha"},
				{ID: "ws_2", Name: "beta"},
			},
		})
	})
	client := newTestClient(t, mux)

	result, err := client.Workspaces.List(context.Background(), miosa.ListWorkspacesInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 2 {
		t.Errorf("want 2 workspaces, got %d", len(result.Data))
	}
}

func TestWorkspacesGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws_abc", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, miosa.Workspace{
			ID:   "ws_abc",
			Name: "target",
		})
	})
	client := newTestClient(t, mux)

	ws, err := client.Workspaces.Get(context.Background(), "ws_abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.ID != "ws_abc" {
		t.Errorf("ID: want ws_abc, got %s", ws.ID)
	}
}

func TestWorkspacesGetNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws_missing", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
	})
	client := newTestClient(t, mux)

	_, err := client.Workspaces.Get(context.Background(), "ws_missing")
	var nfe *miosa.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("want NotFoundError, got %T: %v", err, err)
	}
}

func TestWorkspacesUpdate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws_u", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var input miosa.UpdateWorkspaceInput
		_ = json.NewDecoder(r.Body).Decode(&input)
		writeJSON(w, http.StatusOK, miosa.Workspace{
			ID:   "ws_u",
			Name: input.Name,
		})
	})
	client := newTestClient(t, mux)

	ws, err := client.Workspaces.Update(context.Background(), "ws_u", miosa.UpdateWorkspaceInput{Name: "renamed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.Name != "renamed" {
		t.Errorf("Name: want renamed, got %s", ws.Name)
	}
}

func TestWorkspacesDelete(t *testing.T) {
	deleted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws_del", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	})
	client := newTestClient(t, mux)

	if err := client.Workspaces.Delete(context.Background(), "ws_del"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("DELETE was not called")
	}
}

func TestWorkspacesListComputers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws_x/computers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, miosa.ComputerListResponse{
			Data: []miosa.ComputerData{
				{ID: "cmp_1", Name: "one"},
				{ID: "cmp_2", Name: "two"},
			},
		})
	})
	client := newTestClient(t, mux)

	result, err := client.Workspaces.ListComputers(context.Background(), "ws_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 2 {
		t.Errorf("want 2 computers, got %d", len(result.Data))
	}
	if result.Data[0].ID != "cmp_1" {
		t.Errorf("first computer ID: want cmp_1, got %s", result.Data[0].ID)
	}
}
