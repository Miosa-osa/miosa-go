package miosa_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Miosa-osa/miosa-go/v2"
)

func TestForgeRepositoryContentReads(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/forge/repositories/repo-1/refs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{
			"default_branch": "main", "head_oid": "abc", "branches": []interface{}{map[string]interface{}{"name": "main", "oid": "abc", "is_default": true}}, "tags": []interface{}{},
		}})
	})
	mux.HandleFunc("/forge/repositories/repo-1/tree", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ref") != "main" || r.URL.Query().Get("path") != "src" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{
			"ref": "main", "commit_oid": "abc", "path": "src", "entries": []interface{}{map[string]interface{}{"name": "main.go", "path": "src/main.go", "type": "blob", "oid": "def", "size": 10}}, "truncated": false,
		}})
	})
	mux.HandleFunc("/forge/repositories/repo-1/blob", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{
			"ref": "main", "commit_oid": "abc", "path": "README.md", "oid": "def", "size": 5, "encoding": "utf-8", "content": "hello",
		}})
	})
	mux.HandleFunc("/forge/repositories/repo-1/commits", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "25" || r.URL.Query().Get("cursor") != "cursor-1" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{
			"ref": "main", "path": "", "commits": []interface{}{map[string]interface{}{"oid": "abc", "short_oid": "abc", "subject": "initial", "author_name": "Ada", "author_email": "ada@example.test", "authored_at": "now", "committer_name": "Ada", "committed_at": "now", "parents": []interface{}{}}}, "page": map[string]interface{}{"has_more": false, "next_cursor": nil},
		}})
	})

	client := newTestClient(t, mux)
	refs, err := client.Forge.Refs(context.Background(), "repo-1")
	if err != nil || refs.DefaultBranch != "main" {
		t.Fatalf("refs: %#v, %v", refs, err)
	}
	tree, err := client.Forge.Tree(context.Background(), "repo-1", miosa.ForgeContentOptions{Ref: "main", Path: "src"})
	if err != nil || len(tree.Entries) != 1 {
		t.Fatalf("tree: %#v, %v", tree, err)
	}
	blob, err := client.Forge.Blob(context.Background(), "repo-1", miosa.ForgeContentOptions{Ref: "main", Path: "README.md"})
	if err != nil || blob.Content != "hello" {
		t.Fatalf("blob: %#v, %v", blob, err)
	}
	history, err := client.Forge.Commits(context.Background(), "repo-1", miosa.ForgeCommitOptions{Limit: 25, Cursor: "cursor-1"})
	if err != nil || len(history.Commits) != 1 {
		t.Fatalf("commits: %#v, %v", history, err)
	}
}

func TestForgeRepositoryListAcceptsPublicVisibility(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/forge/repositories", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{map[string]interface{}{
			"id": "repo-1", "name": "Platform", "slug": "platform", "default_branch": "main",
			"visibility": "public", "state": "active", "clone_ready": true,
			"clone_url": "https://forge.miosa.ai/acme/platform.git", "project_ids": []interface{}{},
			"created_at": "2026-08-14T12:00:00Z", "updated_at": "2026-08-14T12:00:00Z",
		}}})
	})

	repositories, err := newTestClient(t, mux).Forge.List(context.Background())
	if err != nil || len(repositories) != 1 || repositories[0].Visibility != "public" {
		t.Fatalf("repositories: %#v, %v", repositories, err)
	}
}
