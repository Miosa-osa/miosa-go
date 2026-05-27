package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Miosa-osa/miosa-go"
)

func workspaceMemberJSON(userID, role string) miosa.WorkspaceMember {
	return miosa.WorkspaceMember{
		UserID: userID,
		Email:  userID + "@example.com",
		Name:   "Test User",
		Role:   miosa.WorkspaceRole(role),
	}
}

func workspaceMemberRecordJSON(userID, workspaceID, role string) miosa.WorkspaceMemberRecord {
	return miosa.WorkspaceMemberRecord{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Role:        miosa.WorkspaceRole(role),
	}
}

func TestWorkspaceMembersList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws-abc/members", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []miosa.WorkspaceMember{
				workspaceMemberJSON("usr_alice", "owner"),
				workspaceMemberJSON("usr_bob", "member"),
			},
		})
	})
	client := newTestClient(t, mux)

	resp, err := client.WorkspaceMembers.List(context.Background(), "ws-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Errorf("want 2 members, got %d", len(resp.Data))
	}
	if resp.Data[0].UserID != "usr_alice" {
		t.Errorf("first member: want usr_alice, got %s", resp.Data[0].UserID)
	}
	if resp.Data[0].Role != "owner" {
		t.Errorf("first member role: want owner, got %s", resp.Data[0].Role)
	}
}

func TestWorkspaceMembersListEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws-empty/members", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}})
	})
	client := newTestClient(t, mux)

	resp, err := client.WorkspaceMembers.List(context.Background(), "ws-empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Errorf("want empty slice, got %d items", len(resp.Data))
	}
}

func TestWorkspaceMembersAdd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws-abc/members", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": workspaceMemberRecordJSON(body["user_id"], "ws-abc", body["role"]),
		})
	})
	client := newTestClient(t, mux)

	resp, err := client.WorkspaceMembers.Add(context.Background(), "ws-abc", miosa.AddWorkspaceMemberInput{
		UserID: "usr_bob",
		Role:   "member",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data.UserID != "usr_bob" {
		t.Errorf("UserID: want usr_bob, got %s", resp.Data.UserID)
	}
	if resp.Data.Role != "member" {
		t.Errorf("Role: want member, got %s", resp.Data.Role)
	}
}

func TestWorkspaceMembersUpdateRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws-abc/members/usr_bob", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		rec := workspaceMemberRecordJSON("usr_bob", "ws-abc", body["role"])
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": rec})
	})
	client := newTestClient(t, mux)

	resp, err := client.WorkspaceMembers.UpdateRole(context.Background(), "ws-abc", "usr_bob",
		miosa.UpdateWorkspaceMemberRoleInput{Role: "admin"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data.Role != "admin" {
		t.Errorf("Role: want admin, got %s", resp.Data.Role)
	}
}

func TestWorkspaceMembersRemove(t *testing.T) {
	removed := false
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws-abc/members/usr_bob", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		removed = true
		writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true})
	})
	client := newTestClient(t, mux)

	if _, err := client.WorkspaceMembers.Remove(context.Background(), "ws-abc", "usr_bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !removed {
		t.Error("DELETE was not called")
	}
}

func TestWorkspaceMembersRemoveLastOwnerConflict(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/ws-abc/members/usr_owner", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "LAST_OWNER",
				"message": "Cannot remove the last workspace owner",
			},
		})
	})
	client := newTestClient(t, mux)

	_, err := client.WorkspaceMembers.Remove(context.Background(), "ws-abc", "usr_owner")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
